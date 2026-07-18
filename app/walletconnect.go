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

// walletConnectBridgeTemplate validates the dapp's immutable intent against
// the ACTIVE wallet account and reconstructs a clean block for a fresh
// prepare. In particular, no dapp-supplied frontier, PoW, hash, public key, or
// signature field can enter the signing path.
func walletConnectBridgeTemplate(req WalletConnectSendRequest, active types.Address) (*nom.AccountBlock, callExpect, *TransactionEffect, error) {
	from, err := types.ParseAddress(req.FromAddress)
	if err != nil {
		return nil, callExpect{}, nil, fmt.Errorf("invalid WalletConnect sender: %w", err)
	}
	if from != active {
		return nil, callExpect{}, nil, errors.New("WalletConnect sender is not the active wallet account")
	}
	return walletConnectValidateIntent(req, from)
}

// walletConnectValidateIntent is the sender-independent validation core: it
// rebuilds the clean template with `from` as the sender WITHOUT comparing it
// to the active account. Journal replays use it so an already-journaled
// request can be matched against its intent hash even while the wallet is
// locked or on another account — nothing on this path can sign.
func walletConnectValidateIntent(req WalletConnectSendRequest, from types.Address) (*nom.AccountBlock, callExpect, *TransactionEffect, error) {
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
	// the sender. If a dapp does populate it, it must match the envelope sender.
	if b.Address != "" {
		blockFrom, parseErr := types.ParseAddress(b.Address)
		if parseErr != nil {
			return nil, callExpect{}, nil, fmt.Errorf("invalid account-block sender: %w", parseErr)
		}
		if blockFrom != types.ZeroAddress && blockFrom != from {
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
		Address:         from,
		ToAddress:       to,
		Amount:          new(big.Int).Set(amount),
		TokenStandard:   zts,
		Data:            append([]byte(nil), data...),
	}
	expect := callExpect{
		from:   from,
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
// LookupWalletConnectPublication resolves an already-journaled request WITHOUT
// any live wallet/node gate and without ever creating a hold. The frontend
// calls it before its scam/busy policy gates so a redelivered published or
// unknown outcome is replayed rather than turned into a rejection — while a
// fresh (unjournaled) request still gets Outcome "none" and remains subject to
// those gates and PrepareWalletConnectSend. A reused request id carrying a
// different intent fails closed.
func (t *TxService) LookupWalletConnectPublication(req WalletConnectSendRequest) (WalletConnectPrepareResult, error) {
	none := WalletConnectPrepareResult{Outcome: "none"}
	if req.Topic == "" || req.RequestID == 0 {
		return none, nil
	}
	key := wcJournalKey(req.Topic, req.RequestID)
	rec, found, err := t.wcJournal.get(key)
	if err != nil {
		return WalletConnectPrepareResult{}, fmt.Errorf("cannot read the publication journal: %v", err)
	}
	from, perr := types.ParseAddress(req.FromAddress)
	if perr != nil {
		// A malformed sender can never match a journaled (validated) record;
		// let the fresh-prepare path produce the precise validation error.
		return none, nil
	}
	replayTemplate, _, _, verr := walletConnectValidateIntent(req, from)
	if verr != nil {
		return none, nil
	}
	intentHash := walletConnectIntentHash(replayTemplate)
	if found {
		if rec.IntentHash != intentHash {
			// A reused request id carrying a different intent. This is a resolved
			// outcome, NOT a Go error: only a genuine journal READ failure returns
			// an error, so the frontend can reject a conflict (5000) while treating
			// an unknown-status read failure as retryable rather than a definite
			// rejection.
			return WalletConnectPrepareResult{Outcome: "conflict"}, nil
		}
		return t.walletConnectReplayResult(rec, req.Topic, req.RequestID)
	}
	// No record under this exact id. Before treating it as fresh, check whether
	// an identical intent is still retained under a DIFFERENT id in the same
	// session — a dapp reissuing a transfer under a new id (SignClient suppresses
	// same-id re-emission) must resolve to that record's outcome, never build a
	// second block for the same transfer.
	match, matched, err := t.wcJournal.findByIntent(req.Topic, intentHash, key)
	if err != nil {
		return WalletConnectPrepareResult{}, fmt.Errorf("cannot read the publication journal: %v", err)
	}
	if matched {
		return t.walletConnectReplayResult(match, match.Topic, match.RequestID)
	}
	// Finally, check ANY other session (topic). The original session may have
	// expired and the dapp re-paired under a new topic, so the identical intent
	// arrives under both a new id and a new topic. This is NOT auto-replayed —
	// it may be an unrelated dapp that happens to share the intent — but it must
	// still block a second publication. Return a "duplicate" outcome that the
	// frontend refuses (no disclosure to the new dapp) while surfacing the
	// retained record for the user to reconcile or clear.
	crossMatch, crossMatched, err := t.wcJournal.findByIntentAnyTopic(intentHash, key, req.Topic)
	if err != nil {
		return WalletConnectPrepareResult{}, fmt.Errorf("cannot read the publication journal: %v", err)
	}
	if crossMatched {
		preview, perr := t.wcPreviewFromRecord(crossMatch)
		if perr != nil {
			return WalletConnectPrepareResult{}, perr
		}
		return WalletConnectPrepareResult{
			Outcome:          "duplicate",
			Preview:          preview,
			PublishedHash:    crossMatch.Hash,
			JournalTopic:     crossMatch.Topic,
			JournalRequestID: crossMatch.RequestID,
		}, nil
	}
	return none, nil
}

func (t *TxService) PrepareWalletConnectSend(req WalletConnectSendRequest) (WalletConnectPrepareResult, error) {
	none := WalletConnectPrepareResult{}
	if req.Topic == "" || req.RequestID == 0 {
		return none, errors.New("missing WalletConnect request identity (topic and request id)")
	}
	// Resolve an already-journaled request BEFORE any live wallet or node gate:
	// funds may already have moved for it, and a locked wallet, another active
	// account, or a different chain must never turn a known outcome into an
	// ordinary rejection. (The frontend also calls Lookup directly ahead of its
	// own gates; this keeps a direct PrepareWalletConnectSend caller safe too.)
	if replay, err := t.LookupWalletConnectPublication(req); err != nil {
		return none, err
	} else if replay.Outcome == "conflict" {
		return none, errors.New("this WalletConnect request id was already used for a different transaction; refusing")
	} else if replay.Outcome != "none" {
		return replay, nil
	}
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
	template, expect, effect, err := walletConnectBridgeTemplate(req, active)
	if err != nil {
		return none, err
	}
	intentHash := walletConnectIntentHash(template)
	// WC-03: the confirmation renders a human amount from token decimals. ZNN
	// and QSR are protocol-fixed; a custom token whose decimals cannot be
	// resolved (including missing metadata) must fail preparation instead of
	// being formatted with a guess. The value resolved HERE is stamped into
	// the preview — no second, fail-open lookup may replace it. The lookup is
	// lazy so native tokens never require the client and the downstream
	// connectivity/mainnet gates keep their error precedence.
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
	decimals, err := resolveDecimalsChecked(template.TokenStandard.String(), lookup)
	if err != nil {
		return none, err
	}
	preview, err := t.prepareCallWithEffectDecimals(template, expect, "Bridge."+effect.Method, effect, &decimals)
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
func (t *TxService) walletConnectReplayResult(rec wcPublicationRecord, journalTopic string, journalRequestID uint64) (WalletConnectPrepareResult, error) {
	preview, err := t.wcPreviewFromRecord(rec)
	if err != nil {
		return WalletConnectPrepareResult{}, err
	}
	if rec.State == wcStatePublished {
		result, err := wcRecordResult(rec)
		if err != nil {
			return WalletConnectPrepareResult{}, err
		}
		// The preview accompanies the result so a failed delivery can keep a
		// renderable retry dialog on the frontend.
		return WalletConnectPrepareResult{Outcome: "published", Published: result, Preview: preview, PublishedHash: rec.Hash, JournalTopic: journalTopic, JournalRequestID: journalRequestID}, nil
	}
	return WalletConnectPrepareResult{Outcome: "unknown", Preview: preview, PublishedHash: rec.Hash, JournalTopic: journalTopic, JournalRequestID: journalRequestID}, nil
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
	// A rebroadcast lands on the same account frontier as any other
	// publication: serialize with ConfirmPublish and Receive, or it could race
	// them into sibling blocks. TryLock (not Lock) so a stuck publish surfaces
	// as an explicit retryable error.
	if !t.publishMu.TryLock() {
		return nil, errors.New("a transaction is already being published")
	}
	defer t.publishMu.Unlock()
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
	// Read the client and its chain identifier together: a node transition
	// between two separate accessor calls could pair an old client with the new
	// chain id, letting the query/rebroadcast hit a connection the chain check
	// never validated.
	client, connectedChain := t.node.connectionSnapshot()
	if client == nil {
		return nil, errors.New("not connected")
	}
	hash, err := types.HexToHash(rec.Hash)
	if err != nil {
		return nil, fmt.Errorf("corrupt journal record hash %q: %v", rec.Hash, err)
	}
	var block nom.AccountBlock
	if err := json.Unmarshal(rec.BlockJSON, &block); err != nil {
		return nil, fmt.Errorf("corrupt journaled block for %s: %v", rec.Hash, err)
	}
	if block.Hash != hash {
		return nil, errors.New("journal record integrity check failed; refusing to rebroadcast")
	}
	// The connected node must be on the block's chain BEFORE the query counts
	// as evidence: "not found" from a node on another network proves nothing,
	// and rebroadcasting there would submit to the wrong chain.
	if block.ChainIdentifier != connectedChain {
		return nil, fmt.Errorf("the connected node is on chain %d but the journaled block belongs to chain %d; reconnect before checking the outcome", connectedChain, block.ChainIdentifier)
	}
	query := t.blockByHashFn
	if query == nil {
		query = func(c *rpc_client.RpcClient, h types.Hash) (bool, error) {
			queried, err := c.LedgerApi.GetAccountBlockByHash(h)
			if err != nil {
				return false, err
			}
			return queried != nil && queried.Hash == h, nil
		}
	}
	onChain, queryErr := query(client, hash)
	if onChain {
		_ = t.wcJournal.markPublished(key)
		return wcRecordResult(rec)
	}
	if queryErr != nil {
		return nil, fmt.Errorf("%s: cannot verify the outcome: %v; check again when the node is reachable", wcOutcomeUnknownMarker, queryErr)
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
