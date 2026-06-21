package app

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/0x3639/znn-sdk-go/pow"
	"github.com/0x3639/znn-sdk-go/zenon"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

// TxService builds, confirms, and publishes transactions via the SDK zenon
// facade using prepare-then-publish: PrepareSend autofills+PoW+signs and holds
// the block; ConfirmPublish broadcasts only after re-asserting it matches.
type TxService struct {
	ctx    context.Context
	config *ConfigService
	wallet *WalletService
	node   *NodeService

	mu         sync.Mutex
	pending    *nom.AccountBlock
	pendingReq SendRequest
	pendingGen uint64 // wallet session generation captured at PrepareSend
}

func newTxService(c *ConfigService, w *WalletService, n *NodeService) *TxService {
	return &TxService{config: c, wallet: w, node: n}
}

func (t *TxService) symbolFor(zts string) string {
	switch zts {
	case types.ZnnTokenStandard.String():
		return "ZNN"
	case types.QsrTokenStandard.String():
		return "QSR"
	default:
		return ""
	}
}

// parseRequest validates a SendRequest into typed values.
func (t *TxService) parseRequest(req SendRequest) (types.Address, types.ZenonTokenStandard, *big.Int, error) {
	to, err := types.ParseAddress(req.ToAddress)
	if err != nil {
		return types.Address{}, types.ZenonTokenStandard{}, nil, fmt.Errorf("invalid recipient address: %w", err)
	}
	zts, err := types.ParseZTS(req.Zts)
	if err != nil {
		return types.Address{}, types.ZenonTokenStandard{}, nil, fmt.Errorf("invalid token: %w", err)
	}
	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		return types.Address{}, types.ZenonTokenStandard{}, nil, errors.New("invalid amount")
	}
	return to, zts, amount, nil
}

// guard rejects mainnet sends unless explicitly enabled.
func (t *TxService) guard() error {
	if t.node.currentChainID() == mainnetChainID {
		s, err := t.config.GetSettings()
		if err != nil {
			return err
		}
		if !s.AllowMainnetSend {
			return errors.New("mainnet sending is disabled")
		}
	}
	return nil
}

// RequiresPoW reports whether a send would need PoW (false ⇒ covered by plasma).
func (t *TxService) RequiresPoW(req SendRequest) (bool, error) {
	client := t.node.currentClient()
	if client == nil {
		return false, errors.New("not connected")
	}
	to, zts, amount, err := t.parseRequest(req)
	if err != nil {
		return false, err
	}
	kp, err := t.wallet.signingKeyPair()
	if err != nil {
		return false, err
	}
	template := client.LedgerApi.SendTemplate(to, zts, amount, nil)
	return zenon.NewZenon(client).RequiresPoW(template, kp)
}

// PrepareSend builds, PoWs, and signs the block, holds it, and returns a
// preview rendered from the built block. Nothing is broadcast.
func (t *TxService) PrepareSend(req SendRequest) (SendPreview, error) {
	if err := t.guard(); err != nil {
		return SendPreview{}, err
	}
	// Snapshot the wallet session at the START. If a Lock/Unlock happens while we
	// build (PoW can take seconds), we must NOT store the resulting pending block.
	gen := t.wallet.sessionGen()
	client := t.node.currentClient()
	if client == nil {
		return SendPreview{}, errors.New("not connected")
	}
	to, zts, amount, err := t.parseRequest(req)
	if err != nil {
		return SendPreview{}, err
	}
	kp, err := t.wallet.signingKeyPair()
	if err != nil {
		return SendPreview{}, err
	}

	template := client.LedgerApi.SendTemplate(to, zts, amount, nil)
	z := zenon.NewZenon(client)
	if t.ctx != nil {
		z.PowCallback = func(s pow.PowStatus) {
			runtime.EventsEmit(t.ctx, EventTxPowProgress, map[string]string{"state": s.String()})
		}
	}
	built, err := z.PrepareBlock(template, kp)
	if err != nil {
		return SendPreview{}, err
	}

	// If the wallet session changed mid-prepare (lock or unlock), the block was
	// built against a now-stale wallet state; refuse to hold it.
	if t.wallet.sessionGen() != gen {
		return SendPreview{}, errors.New("wallet state changed during prepare")
	}

	t.mu.Lock()
	t.pending = built
	t.pendingReq = req
	t.pendingGen = gen
	t.mu.Unlock()

	return SendPreview{
		ToAddress:  built.ToAddress.String(),
		Symbol:     t.symbolFor(built.TokenStandard.String()),
		Zts:        built.TokenStandard.String(),
		Amount:     built.Amount.String(),
		UsedPlasma: built.FusedPlasma,
		Difficulty: built.Difficulty,
		Hash:       built.Hash.String(),
		NeedsPoW:   built.Difficulty > 0,
	}, nil
}

// ConfirmPublish broadcasts the held block after re-asserting it matches the
// originating request, then clears it.
func (t *TxService) ConfirmPublish() (string, error) {
	// Re-assert the mainnet guard before publishing. If it fails (e.g. the block
	// was prepared on testnet but we are now connected to mainnet), refuse to
	// publish WITHOUT clearing pending so the user can reconnect and retry.
	if err := t.guard(); err != nil {
		return "", err
	}
	t.mu.Lock()
	b, req, pendingGen := t.pending, t.pendingReq, t.pendingGen
	t.mu.Unlock()
	if b == nil {
		return "", errors.New("no pending transaction")
	}

	// Refuse to publish if the wallet was locked or its session changed since the
	// block was prepared (lock/unlock TOCTOU). Clear pending: the held block is
	// no longer trustworthy and the user must re-prepare.
	if _, ok := t.wallet.activeAddress(); !ok || t.wallet.sessionGen() != pendingGen {
		t.clearPending()
		return "", errors.New("wallet locked or changed; not publishing")
	}

	to, zts, amount, err := t.parseRequest(req)
	if err != nil {
		t.clearPending()
		return "", err
	}
	if b.ToAddress != to || b.TokenStandard != zts || b.Amount == nil || b.Amount.Cmp(amount) != 0 {
		t.clearPending()
		return "", errors.New("prepared block does not match the request; not publishing")
	}

	// Snapshot the client and verify the prepared block's chain matches the
	// currently connected node. A node change between prepare and confirm could
	// otherwise broadcast a cross-chain block. Keep pending on mismatch so the
	// user can reconnect to the original network and retry.
	client := t.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	if b.ChainIdentifier != t.node.currentChainID() {
		return "", errors.New("connected node chain differs from the prepared transaction; reconnect to the original network and retry")
	}
	if err := client.LedgerApi.PublishRawTransaction(b); err != nil {
		return "", err
	}
	hash := b.Hash.String()
	t.clearPending()
	if t.ctx != nil {
		runtime.EventsEmit(t.ctx, EventTxPublished, map[string]string{"hash": hash})
	}
	return hash, nil
}

// Receive receives a single inbound block by its send-block hash.
func (t *TxService) Receive(fromHash string) (string, error) {
	hash, err := types.HexToHash(fromHash)
	if err != nil {
		return "", fmt.Errorf("invalid block hash: %w", err)
	}
	client := t.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	kp, err := t.wallet.signingKeyPair()
	if err != nil {
		return "", err
	}
	template := client.LedgerApi.ReceiveTemplate(hash)
	published, err := zenon.NewZenon(client).Send(template, kp)
	if err != nil {
		return "", err
	}
	h := published.Hash.String()
	if t.ctx != nil {
		runtime.EventsEmit(t.ctx, EventTxReceived, map[string]string{"hash": h})
	}
	return h, nil
}

// CancelPending discards the held block.
func (t *TxService) CancelPending() error {
	t.clearPending()
	return nil
}

func (t *TxService) clearPending() {
	t.mu.Lock()
	t.pending = nil
	t.pendingReq = SendRequest{}
	t.pendingGen = 0
	t.mu.Unlock()
}
