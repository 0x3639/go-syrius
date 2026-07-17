package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	definition "github.com/zenon-network/go-zenon/vm/embedded/definition"
)

// walletConnectBridgeTemplate validates the dapp's immutable intent and
// reconstructs a clean block. In particular, no dapp-supplied frontier, PoW,
// hash, public key, or signature field can enter the signing path.
func walletConnectBridgeTemplate(req WalletConnectSendRequest, active types.Address) (*nom.AccountBlock, callExpect, *TransactionEffect, error) {
	from, err := types.ParseAddress(req.FromAddress)
	if err != nil {
		return nil, callExpect{}, nil, fmt.Errorf("invalid WalletConnect sender: %w", err)
	}
	if from != active {
		return nil, callExpect{}, nil, errors.New("WalletConnect sender is not the active wallet account")
	}
	b := req.AccountBlock
	if b.Version != 1 {
		return nil, callExpect{}, nil, fmt.Errorf("unsupported account-block version %d", b.Version)
	}
	if b.ChainIdentifier != mainnetChainID {
		return nil, callExpect{}, nil, fmt.Errorf("WalletConnect bridge requests must use zenon:%d", mainnetChainID)
	}
	if b.BlockType != uint64(nom.BlockTypeUserSend) {
		return nil, callExpect{}, nil, errors.New("WalletConnect bridge request must be a user-send block")
	}
	// SDK contract templates normally carry ZeroAddress until the wallet fills
	// the sender. If a dapp does populate it, it must match the active account.
	if b.Address != "" {
		blockFrom, parseErr := types.ParseAddress(b.Address)
		if parseErr != nil {
			return nil, callExpect{}, nil, fmt.Errorf("invalid account-block sender: %w", parseErr)
		}
		if blockFrom != types.ZeroAddress && blockFrom != active {
			return nil, callExpect{}, nil, errors.New("account-block sender is not the active wallet account")
		}
	}
	to, err := types.ParseAddress(b.ToAddress)
	if err != nil {
		return nil, callExpect{}, nil, fmt.Errorf("invalid WalletConnect destination: %w", err)
	}
	if to != types.BridgeContract {
		return nil, callExpect{}, nil, errors.New("WalletConnect currently permits only the Zenon Bridge contract")
	}
	zts, err := types.ParseZTS(b.TokenStandard)
	if err != nil {
		return nil, callExpect{}, nil, fmt.Errorf("invalid WalletConnect token standard: %w", err)
	}
	amount, ok := new(big.Int).SetString(b.Amount, 10)
	if !ok || amount.Sign() < 0 {
		return nil, callExpect{}, nil, errors.New("invalid WalletConnect amount")
	}
	data, err := base64.StdEncoding.DecodeString(b.Data)
	if err != nil || base64.StdEncoding.EncodeToString(data) != b.Data {
		return nil, callExpect{}, nil, errors.New("WalletConnect call data must be canonical base64")
	}
	effect, err := decodeContractCall(to, data)
	if err != nil {
		return nil, callExpect{}, nil, fmt.Errorf("invalid WalletConnect Bridge call: %w", err)
	}
	if effect.Method != definition.WrapTokenMethodName && effect.Method != definition.RedeemUnwrapMethodName {
		return nil, callExpect{}, nil, fmt.Errorf("WalletConnect Bridge.%s is not an approved user bridge operation", effect.Method)
	}
	switch effect.Method {
	case definition.WrapTokenMethodName:
		if amount.Sign() <= 0 {
			return nil, callExpect{}, nil, errors.New("WalletConnect Bridge.WrapToken requires a positive contract-call amount")
		}
	case definition.RedeemUnwrapMethodName:
		// The canonical SDK Redeem template is a zero-value ZNN contract call.
		// Attaching funds (or another token standard) is never part of redeem
		// intent and could strand value in the embedded contract.
		if amount.Sign() != 0 {
			return nil, callExpect{}, nil, errors.New("WalletConnect Bridge.Redeem must not attach funds")
		}
		if zts != types.ZnnTokenStandard {
			return nil, callExpect{}, nil, errors.New("WalletConnect Bridge.Redeem must use the ZNN token standard")
		}
	}
	template := &nom.AccountBlock{
		Version:         1,
		ChainIdentifier: mainnetChainID,
		BlockType:       nom.BlockTypeUserSend,
		Address:         active,
		ToAddress:       to,
		Amount:          new(big.Int).Set(amount),
		TokenStandard:   zts,
		Data:            append([]byte(nil), data...),
	}
	expect := callExpect{
		from:   active,
		to:     to,
		zts:    zts,
		amount: new(big.Int).Set(amount),
		data:   append([]byte(nil), data...),
	}
	return template, expect, effect, nil
}

// wcOutcomeUnknownMarker prefixes every error whose publication outcome is
// UNRESOLVED (the block may be on chain). The frontend routes such errors into
// the reconcile flow instead of the retryable-error flow.
const wcOutcomeUnknownMarker = "walletconnect publication outcome unknown"

// PrepareWalletConnectSend validates and holds a Bridge request for the same
// confirm-what-you-sign flow used by first-party wallet actions. A request
// whose identity is already journaled replays the stored outcome instead of
// ever building a fresh block (WC-01).
func (t *TxService) PrepareWalletConnectSend(req WalletConnectSendRequest) (WalletConnectPrepareResult, error) {
	none := WalletConnectPrepareResult{}
	active, ok := t.wallet.activeAddress()
	if !ok {
		return none, errLocked
	}
	if t.configuredChainID() != mainnetChainID {
		return none, errors.New("set Chain ID 1 in Settings before using WalletConnect")
	}
	if t.node.currentChainID() != mainnetChainID {
		return none, errors.New("connect to a Zenon mainnet node before using WalletConnect")
	}
	if req.Topic == "" || req.RequestID == 0 {
		return none, errors.New("missing WalletConnect request identity (topic and request id)")
	}
	template, expect, effect, err := walletConnectBridgeTemplate(req, active)
	if err != nil {
		return none, err
	}
	intentHash := walletConnectIntentHash(template)
	key := wcJournalKey(req.Topic, req.RequestID)
	if rec, found, jerr := t.wcJournal.get(key); jerr != nil {
		return none, fmt.Errorf("cannot read the publication journal: %v", jerr)
	} else if found {
		if rec.IntentHash != intentHash {
			return none, errors.New("this WalletConnect request id was already used for a different transaction; refusing")
		}
		return t.walletConnectReplayResult(rec)
	}
	// WC-03: the confirmation renders a human amount from token decimals. ZNN
	// and QSR are protocol-fixed; a custom token whose decimals cannot be
	// resolved must fail preparation instead of being formatted with a guess.
	// The lookup is lazy so native tokens never require the client here and the
	// downstream connectivity/mainnet gates keep their error precedence.
	lookup := t.decimalsLookup
	if lookup == nil {
		lookup = func(zts types.ZenonTokenStandard) (int, error) {
			client := t.node.currentClient()
			if client == nil {
				return 0, errors.New("not connected")
			}
			return clientTokenDecimals(client)(zts)
		}
	}
	if _, err := resolveDecimalsChecked(template.TokenStandard.String(), lookup); err != nil {
		return none, err
	}
	preview, err := t.prepareCallWithEffect(template, expect, "Bridge."+effect.Method, effect)
	if err != nil {
		return none, err
	}
	if !t.attachWalletConnectIdentity(preview.HoldID, req.Topic, req.RequestID, intentHash) {
		// The hold vanished between holdPending and here (a racing clear).
		// Never allow an unjournaled WalletConnect publication.
		t.clearPendingIf(preview.HoldID)
		return none, errors.New("wallet state changed during prepare")
	}
	return WalletConnectPrepareResult{Outcome: "prepare", Preview: &preview}, nil
}

// walletConnectReplayResult renders a journaled record as a prepare outcome:
// published records replay the stored result; signed records surface the
// reconcile flow with a preview rebuilt from the journaled block.
func (t *TxService) walletConnectReplayResult(rec wcPublicationRecord) (WalletConnectPrepareResult, error) {
	if rec.State == wcStatePublished {
		result, err := wcRecordResult(rec)
		if err != nil {
			return WalletConnectPrepareResult{}, err
		}
		return WalletConnectPrepareResult{Outcome: "published", Published: result, PublishedHash: rec.Hash}, nil
	}
	preview, err := t.wcPreviewFromRecord(rec)
	if err != nil {
		return WalletConnectPrepareResult{}, err
	}
	return WalletConnectPrepareResult{Outcome: "unknown", Preview: preview, PublishedHash: rec.Hash}, nil
}

// wcRecordResult decodes the journaled block JSON into the WalletConnect
// result map both bridges expect.
func wcRecordResult(rec wcPublicationRecord) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(rec.BlockJSON, &result); err != nil {
		return nil, fmt.Errorf("corrupt journaled block for %s: %v", rec.Hash, err)
	}
	return result, nil
}

// wcPreviewFromRecord rebuilds a display preview from the journaled signed
// block (no hold — HoldID stays 0; nothing here can be confirmed or signed).
func (t *TxService) wcPreviewFromRecord(rec wcPublicationRecord) (*CallPreview, error) {
	var block nom.AccountBlock
	if err := json.Unmarshal(rec.BlockJSON, &block); err != nil {
		return nil, fmt.Errorf("corrupt journaled block for %s: %v", rec.Hash, err)
	}
	summary := "Bridge call"
	effect, err := decodeContractCall(block.ToAddress, block.Data)
	if err == nil {
		summary = effect.Contract + "." + effect.Method
	} else {
		effect = nil
	}
	zts := block.TokenStandard.String()
	amount := "0"
	if block.Amount != nil {
		amount = block.Amount.String()
	}
	return &CallPreview{
		FromAddress: block.Address.String(),
		ToAddress:   block.ToAddress.String(),
		Zts:         zts,
		Symbol:      t.symbolFor(zts),
		Amount:      amount,
		Decimals:    resolveDecimals(zts, nil), // display only; base units are shown alongside
		Hash:        rec.Hash,
		Summary:     summary,
		Effect:      effect,
	}, nil
}

// ConfirmWalletConnectPublish finalizes and publishes the exact held request,
// then returns the SDK-compatible account-block JSON expected by both bridges.
func (t *TxService) ConfirmWalletConnectPublish(holdID uint64) (map[string]interface{}, error) {
	built, err := t.confirmPublishBlock(holdID)
	if err != nil {
		return nil, err
	}
	return walletConnectBlockJSON(built)
}

// ReconcileWalletConnectPublication resolves an unknown broadcast outcome for
// a journaled request: query the node by block hash; if absent, rebroadcast
// the EXACT journaled signed block — never a rebuilt one. Retryable until the
// outcome is known.
func (t *TxService) ReconcileWalletConnectPublication(topic string, requestID uint64) (map[string]interface{}, error) {
	key := wcJournalKey(topic, requestID)
	rec, found, err := t.wcJournal.get(key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("no journaled WalletConnect publication for this request")
	}
	if rec.State == wcStatePublished {
		return wcRecordResult(rec)
	}
	client := t.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	hash, err := types.HexToHash(rec.Hash)
	if err != nil {
		return nil, fmt.Errorf("corrupt journal record hash %q: %v", rec.Hash, err)
	}
	query := t.blockByHashFn
	if query == nil {
		query = func(c *rpc_client.RpcClient, h types.Hash) (bool, error) {
			block, err := c.LedgerApi.GetAccountBlockByHash(h)
			if err != nil {
				return false, err
			}
			return block != nil && block.Hash == h, nil
		}
	}
	found, queryErr := query(client, hash)
	if found {
		_ = t.wcJournal.markPublished(key)
		return wcRecordResult(rec)
	}
	if queryErr != nil {
		return nil, fmt.Errorf("%s: cannot verify the outcome: %v; check again when the node is reachable", wcOutcomeUnknownMarker, queryErr)
	}
	var block nom.AccountBlock
	if err := json.Unmarshal(rec.BlockJSON, &block); err != nil {
		return nil, fmt.Errorf("corrupt journaled block for %s: %v", rec.Hash, err)
	}
	if block.Hash != hash {
		return nil, errors.New("journal record integrity check failed; refusing to rebroadcast")
	}
	// The user approved this exact block, but respect a since-revoked mainnet
	// opt-in for the (re)broadcast itself; querying stays available regardless.
	if err := t.guardChain(block.ChainIdentifier); err != nil {
		return nil, err
	}
	publish := t.publishFn
	if publish == nil {
		publish = func(c *rpc_client.RpcClient, b *nom.AccountBlock) error {
			return c.LedgerApi.PublishRawTransaction(b)
		}
	}
	if err := publish(client, &block); err != nil {
		return nil, fmt.Errorf("%s: rebroadcast failed: %v; check again later", wcOutcomeUnknownMarker, err)
	}
	_ = t.wcJournal.markPublished(key)
	return wcRecordResult(rec)
}

// AckWalletConnectResult removes a journaled record once its result reached
// the dapp (or the user explicitly closed the delivery-failure state).
func (t *TxService) AckWalletConnectResult(topic string, requestID uint64) error {
	return t.wcJournal.delete(wcJournalKey(topic, requestID))
}

func walletConnectBlockJSON(built *nom.AccountBlock) (map[string]interface{}, error) {
	raw, err := json.Marshal(built)
	if err != nil {
		return nil, fmt.Errorf("encode published account block: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("encode published account block: %w", err)
	}
	return result, nil
}
