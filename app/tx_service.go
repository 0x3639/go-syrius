package app

import (
	"bytes"
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

	mu            sync.Mutex
	pending       *nom.AccountBlock
	pendingExpect callExpect
	pendingGen    uint64 // wallet session generation captured at PrepareSend
}

// callExpect captures the funds-moving effect a prepared block must match before
// it may be published (confirm-what-you-sign).
type callExpect struct {
	to     types.Address
	zts    types.ZenonTokenStandard
	amount *big.Int
	data   []byte
}

// assertMatches verifies a built block moves exactly the expected funds and
// carries the expected contract-call data (Fuse beneficiary / Cancel id).
func assertMatches(b *nom.AccountBlock, e callExpect) error {
	if b.ToAddress != e.to || b.TokenStandard != e.zts || b.Amount == nil || e.amount == nil || b.Amount.Cmp(e.amount) != 0 || !bytes.Equal(b.Data, e.data) {
		return errors.New("prepared block does not match the expected effect; not publishing")
	}
	return nil
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

// configuredChainID returns the chain identifier the wallet builds transactions
// for, from settings; unset/0 normalizes to mainnet. The built block is still
// validated against the connected node's chain before publish.
func (t *TxService) configuredChainID() uint64 {
	s, err := t.config.GetSettings()
	if err != nil || s.ChainID == 0 {
		return mainnetChainID
	}
	return s.ChainID
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
	template.ChainIdentifier = t.configuredChainID()
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
	t.pendingExpect = callExpect{to: to, zts: zts, amount: new(big.Int).Set(amount), data: append([]byte(nil), built.Data...)}
	t.pendingGen = gen
	t.mu.Unlock()

	return SendPreview{
		ToAddress:  built.ToAddress.String(),
		Symbol:     t.symbolFor(built.TokenStandard.String()),
		Zts:        built.TokenStandard.String(),
		Amount:     built.Amount.String(),
		Decimals:   resolveDecimals(built.TokenStandard.String(), clientTokenDecimals(client)),
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
	b, expect, pendingGen := t.pending, t.pendingExpect, t.pendingGen
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

	if err := assertMatches(b, expect); err != nil {
		t.clearPending()
		return "", err
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
		return "", fmt.Errorf("configured Chain ID (%d) does not match the connected node's chain (%d); set the correct Chain ID in Settings or connect to a matching node", b.ChainIdentifier, t.node.currentChainID())
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

// prepareCall builds, PoWs, and signs an embedded-contract call template (without
// publishing), holding it for ConfirmPublish. Reuses the Send guard/PoW path.
func (t *TxService) prepareCall(template *nom.AccountBlock, expect callExpect, summary string) (CallPreview, error) {
	if err := t.guard(); err != nil {
		return CallPreview{}, err
	}
	gen := t.wallet.sessionGen()
	client := t.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	kp, err := t.wallet.signingKeyPair()
	if err != nil {
		return CallPreview{}, err
	}
	template.ChainIdentifier = t.configuredChainID()
	z := zenon.NewZenon(client)
	if t.ctx != nil {
		z.PowCallback = func(s pow.PowStatus) {
			runtime.EventsEmit(t.ctx, EventTxPowProgress, map[string]string{"state": s.String()})
		}
	}
	built, err := z.PrepareBlock(template, kp)
	if err != nil {
		return CallPreview{}, err
	}
	if t.wallet.sessionGen() != gen {
		return CallPreview{}, errors.New("wallet state changed during prepare")
	}
	t.mu.Lock()
	t.pending = built
	t.pendingExpect = callExpect{to: expect.to, zts: expect.zts, amount: new(big.Int).Set(expect.amount), data: append([]byte(nil), expect.data...)}
	t.pendingGen = gen
	t.mu.Unlock()
	return CallPreview{
		ToAddress:  built.ToAddress.String(),
		Zts:        built.TokenStandard.String(),
		Symbol:     t.symbolFor(built.TokenStandard.String()),
		Amount:     built.Amount.String(),
		Decimals:   resolveDecimals(built.TokenStandard.String(), clientTokenDecimals(client)),
		Hash:       built.Hash.String(),
		Summary:    summary,
		UsedPlasma: built.FusedPlasma,
		Difficulty: built.Difficulty,
		NeedsPoW:   built.Difficulty > 0,
	}, nil
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
	template.ChainIdentifier = t.configuredChainID()
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
	t.pendingExpect = callExpect{}
	t.pendingGen = 0
	t.mu.Unlock()
}
