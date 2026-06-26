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

	// publishMu serializes ConfirmPublish for its whole duration. PoW+publish run
	// for seconds outside mu, so without this a second (untrusted) caller could
	// double-publish or race the held template.
	publishMu sync.Mutex
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
	// Determine whether PoW is needed (a cheap node query) but DON'T do it yet.
	// PoW — and therefore the block hash — is deferred to ConfirmPublish so the
	// user approves the effect BEFORE the wallet spends seconds generating plasma.
	needsPoW, err := zenon.NewZenon(client).RequiresPoW(template, kp)
	if err != nil {
		return SendPreview{}, err
	}
	if t.wallet.sessionGen() != gen {
		return SendPreview{}, errors.New("wallet state changed during prepare")
	}

	t.holdPending(template, callExpect{to: to, zts: zts, amount: new(big.Int).Set(amount), data: append([]byte(nil), template.Data...)}, gen)

	return SendPreview{
		ToAddress: to.String(),
		Symbol:    t.symbolFor(zts.String()),
		Zts:       zts.String(),
		Amount:    amount.String(),
		Decimals:  resolveDecimals(zts.String(), clientTokenDecimals(client)),
		NeedsPoW:  needsPoW,
		// UsedPlasma / Difficulty / Hash are filled by ConfirmPublish's PoW.
	}, nil
}

// holdPending stores the un-PoW'd template + the effect to re-assert at publish.
func (t *TxService) holdPending(template *nom.AccountBlock, expect callExpect, gen uint64) {
	t.mu.Lock()
	t.pending = template
	t.pendingExpect = expect
	t.pendingGen = gen
	t.mu.Unlock()
}

// ConfirmPublish broadcasts the held block after re-asserting it matches the
// originating request, then clears it.
func (t *TxService) ConfirmPublish() (string, error) {
	// Only one confirm may be in flight: PoW+publish run for seconds, so a second
	// concurrent call must be rejected rather than double-publish/race the template.
	if !t.publishMu.TryLock() {
		return "", errors.New("a transaction is already being published")
	}
	defer t.publishMu.Unlock()

	// Re-assert the mainnet guard before publishing. If it fails (e.g. the block
	// was prepared on testnet but we are now connected to mainnet), refuse to
	// publish WITHOUT clearing pending so the user can reconnect and retry.
	if err := t.guard(); err != nil {
		return "", err
	}
	t.mu.Lock()
	template, expect, pendingGen := t.pending, t.pendingExpect, t.pendingGen
	t.mu.Unlock()
	if template == nil {
		return "", errors.New("no pending transaction")
	}

	// Refuse if the wallet was locked or its session changed since prepare.
	if _, ok := t.wallet.activeAddress(); !ok || t.wallet.sessionGen() != pendingGen {
		t.clearPending()
		return "", errors.New("wallet locked or changed; not publishing")
	}
	// Re-assert the approved effect on the held template BEFORE the expensive PoW
	// (and again on the built block after). PrepareBlock never alters the funds-
	// moving fields, so a template match guarantees the built block matches.
	if err := assertMatches(template, expect); err != nil {
		t.clearPending()
		return "", err
	}
	kp, err := t.wallet.signingKeyPair()
	if err != nil {
		t.clearPending()
		return "", err
	}
	client := t.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	// Chain check BEFORE the expensive PoW. The template is still un-PoW'd, so a
	// mismatch keeps it for retry after the user reconnects to the right network.
	if template.ChainIdentifier != t.node.currentChainID() {
		return "", fmt.Errorf("configured Chain ID (%d) does not match the connected node's chain (%d); set the correct Chain ID in Settings or connect to a matching node", template.ChainIdentifier, t.node.currentChainID())
	}

	// The slow part, now that the user has approved: autofill against the current
	// frontier, PoW (generate plasma), hash, and sign. The template is mutated
	// here, so any failure from this point clears pending (a retry re-prepares).
	z := zenon.NewZenon(client)
	if t.ctx != nil {
		z.PowCallback = func(s pow.PowStatus) {
			runtime.EventsEmit(t.ctx, EventTxPowProgress, map[string]string{"state": s.String()})
		}
	}
	built, err := z.PrepareBlock(template, kp)
	if err != nil {
		t.clearPending()
		return "", err
	}
	// Re-assert the session after PoW (it took seconds — a lock could have raced).
	if _, ok := t.wallet.activeAddress(); !ok || t.wallet.sessionGen() != pendingGen {
		t.clearPending()
		return "", errors.New("wallet locked or changed; not publishing")
	}
	// Confirm-what-you-sign: the built block must move exactly the approved effect.
	if err := assertMatches(built, expect); err != nil {
		t.clearPending()
		return "", err
	}
	if built.ChainIdentifier != t.node.currentChainID() {
		t.clearPending()
		return "", fmt.Errorf("configured Chain ID (%d) does not match the connected node's chain (%d)", built.ChainIdentifier, t.node.currentChainID())
	}
	if err := client.LedgerApi.PublishRawTransaction(built); err != nil {
		t.clearPending()
		return "", err
	}
	hash := built.Hash.String()
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
	// PoW is deferred to ConfirmPublish (see PrepareSend); here we only learn
	// whether it will be needed, then hold the un-PoW'd template.
	needsPoW, err := zenon.NewZenon(client).RequiresPoW(template, kp)
	if err != nil {
		return CallPreview{}, err
	}
	if t.wallet.sessionGen() != gen {
		return CallPreview{}, errors.New("wallet state changed during prepare")
	}
	t.holdPending(template, callExpect{to: expect.to, zts: expect.zts, amount: new(big.Int).Set(expect.amount), data: append([]byte(nil), expect.data...)}, gen)
	return CallPreview{
		ToAddress: template.ToAddress.String(),
		Zts:       template.TokenStandard.String(),
		Symbol:    t.symbolFor(template.TokenStandard.String()),
		Amount:    template.Amount.String(),
		Decimals:  resolveDecimals(template.TokenStandard.String(), clientTokenDecimals(client)),
		Summary:   summary,
		NeedsPoW:  needsPoW,
		// UsedPlasma / Difficulty / Hash are filled by ConfirmPublish's PoW.
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
