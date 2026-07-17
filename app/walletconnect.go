package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

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

// PrepareWalletConnectSend validates and holds a Bridge request for the same
// confirm-what-you-sign flow used by first-party wallet actions.
func (t *TxService) PrepareWalletConnectSend(req WalletConnectSendRequest) (CallPreview, error) {
	active, ok := t.wallet.activeAddress()
	if !ok {
		return CallPreview{}, errLocked
	}
	if t.configuredChainID() != mainnetChainID {
		return CallPreview{}, errors.New("set Chain ID 1 in Settings before using WalletConnect")
	}
	if t.node.currentChainID() != mainnetChainID {
		return CallPreview{}, errors.New("connect to a Zenon mainnet node before using WalletConnect")
	}
	template, expect, effect, err := walletConnectBridgeTemplate(req, active)
	if err != nil {
		return CallPreview{}, err
	}
	return t.prepareCallWithEffect(template, expect, "Bridge."+effect.Method, effect)
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
