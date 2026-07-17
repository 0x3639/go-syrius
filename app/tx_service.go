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
	pendingHoldID uint64 // identity of the current hold (stamped into previews)
	holdCounter   uint64 // monotonically increasing hold-id source

	// publishMu serializes ConfirmPublish for its whole duration. PoW+publish run
	// for seconds outside mu, so without this a second (untrusted) caller could
	// double-publish or race the held template.
	publishMu sync.Mutex
}

// callExpect captures the funds-moving effect a prepared block must match before
// it may be published (confirm-what-you-sign).
type callExpect struct {
	from   types.Address // the sender account the preview displayed
	to     types.Address
	zts    types.ZenonTokenStandard
	amount *big.Int
	data   []byte
	// policy, when non-nil, re-asserts a prepare-time gate at publish time (e.g.
	// the governance testnet-only rule). Without it the gate would be checked
	// only while preparing and could be raced by a chain change before confirm.
	policy func() error
}

// assertMatches verifies a built block is sent by the expected account and
// moves exactly the expected funds with the expected contract-call data (Fuse
// beneficiary / Cancel id). The sender check matters because the SDK stamps the
// block's Address from the CURRENT keypair: without it an account switch could
// publish a reviewed transaction from a different account.
func assertMatches(b *nom.AccountBlock, e callExpect) error {
	if b.Address != e.from || b.ToAddress != e.to || b.TokenStandard != e.zts || b.Amount == nil || e.amount == nil || b.Amount.Cmp(e.amount) != 0 || !bytes.Equal(b.Data, e.data) {
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

	// Capture the sender the preview will display. signingKeyPair has already
	// asserted the keypair matches this address.
	from, ok := t.wallet.activeAddress()
	if !ok {
		return SendPreview{}, errLocked
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

	holdID, err := t.holdPending(template, callExpect{from: from, to: to, zts: zts, amount: new(big.Int).Set(amount), data: append([]byte(nil), template.Data...)}, gen)
	if err != nil {
		return SendPreview{}, err
	}

	return SendPreview{
		FromAddress: from.String(),
		ToAddress:   to.String(),
		Symbol:      t.symbolFor(zts.String()),
		Zts:         zts.String(),
		Amount:      amount.String(),
		Decimals:    resolveDecimals(zts.String(), clientTokenDecimals(client)),
		NeedsPoW:    needsPoW,
		HoldID:      holdID,
		// UsedPlasma / Difficulty / Hash are filled by ConfirmPublish's PoW.
	}, nil
}

// holdPending stores the un-PoW'd template + the effect to re-assert at publish.
// It returns a fresh hold id that identifies THIS hold: previews carry it so a
// cancel can be identity-checked (see CancelPending) — a stale cancel racing a
// newer Prepare must never release the newer block.
func (t *TxService) holdPending(template *nom.AccountBlock, expect callExpect, gen uint64) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending != nil {
		return 0, errors.New("another transaction is already awaiting confirmation")
	}
	t.holdCounter++
	t.pendingHoldID = t.holdCounter
	t.pending = template
	t.pendingExpect = expect
	t.pendingGen = gen
	return t.pendingHoldID, nil
}

// ConfirmPublish broadcasts the held block after re-asserting it matches the
// originating request, then clears it. holdId is the identity of the preview
// the USER confirmed; every preview stamps a non-zero id, so the gate fails
// CLOSED — a zero or mismatched id is refused (never trust frontend
// validation). A stale or corrupted caller therefore cannot confirm a block
// other than the exact hold represented by its preview.
func (t *TxService) ConfirmPublish(holdId uint64) (string, error) {
	built, err := t.confirmPublishBlock(holdId)
	if err != nil {
		return "", err
	}
	return built.Hash.String(), nil
}

// confirmPublishBlock is the single signing/publication implementation. The
// normal wallet flow returns only its hash; WalletConnect returns the complete
// finalized block JSON expected by both bridge dapps.
func (t *TxService) confirmPublishBlock(holdId uint64) (*nom.AccountBlock, error) {
	// Only one confirm may be in flight: PoW+publish run for seconds, so a second
	// concurrent call must be rejected rather than double-publish/race the template.
	if !t.publishMu.TryLock() {
		return nil, errors.New("a transaction is already being published")
	}
	defer t.publishMu.Unlock()

	// Re-assert the mainnet guard before publishing. If it fails (e.g. the block
	// was prepared on testnet but we are now connected to mainnet), refuse to
	// publish WITHOUT clearing pending so the user can reconnect and retry.
	if err := t.guard(); err != nil {
		return nil, err
	}
	t.mu.Lock()
	template, expect, pendingGen, holdID := t.pending, t.pendingExpect, t.pendingGen, t.pendingHoldID
	t.mu.Unlock()
	if template == nil {
		return nil, errors.New("no pending transaction")
	}
	// Re-assert any prepare-time policy gate (e.g. governance testnet-only)
	// before publishing. Like the mainnet guard: refuse WITHOUT clearing, so the
	// user can reconnect to the right network and retry.
	if expect.policy != nil {
		if err := expect.policy(); err != nil {
			return nil, err
		}
	}
	if holdId == 0 || holdId != holdID {
		// Held block ≠ the one the confirm dialog displayed (or no identity was
		// supplied at all — fail closed). Refuse without clearing: only the owner
		// of the actual hold may cancel or confirm it.
		return nil, errors.New("the pending transaction changed since it was displayed; please review and confirm again")
	}

	// Refuse if the wallet was locked, its session changed, or the active
	// account is no longer the sender the user approved. The session generation
	// already covers account switches, but the address comparison keeps this
	// fail-closed even if the two ever diverge — the SDK stamps the block sender
	// from the CURRENT keypair at signing time.
	if addr, ok := t.wallet.activeAddress(); !ok || t.wallet.sessionGen() != pendingGen || addr != expect.from {
		t.clearPendingIf(holdID)
		return nil, errors.New("wallet locked or changed; not publishing")
	}
	// Re-assert the approved effect on the held template BEFORE the expensive PoW
	// (and again on the built block after). PrepareBlock never alters the funds-
	// moving fields, so a template match guarantees the built block matches.
	if err := assertMatches(template, expect); err != nil {
		t.clearPendingIf(holdID)
		return nil, err
	}
	kp, err := t.wallet.signingKeyPair()
	if err != nil {
		t.clearPendingIf(holdID)
		return nil, err
	}
	client := t.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	// Chain check BEFORE the expensive PoW. The template is still un-PoW'd, so a
	// mismatch keeps it for retry after the user reconnects to the right network.
	if template.ChainIdentifier != t.node.currentChainID() {
		return nil, fmt.Errorf("configured Chain ID (%d) does not match the connected node's chain (%d); set the correct Chain ID in Settings or connect to a matching node", template.ChainIdentifier, t.node.currentChainID())
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
		t.clearPendingIf(holdID)
		return nil, err
	}
	// Re-assert the session after PoW (it took seconds — a lock or account
	// switch could have raced).
	if addr, ok := t.wallet.activeAddress(); !ok || t.wallet.sessionGen() != pendingGen || addr != expect.from {
		t.clearPendingIf(holdID)
		return nil, errors.New("wallet locked or changed; not publishing")
	}
	// Confirm-what-you-sign: the built block must move exactly the approved effect.
	if err := assertMatches(built, expect); err != nil {
		t.clearPendingIf(holdID)
		return nil, err
	}
	if built.ChainIdentifier != t.node.currentChainID() {
		t.clearPendingIf(holdID)
		return nil, fmt.Errorf("configured Chain ID (%d) does not match the connected node's chain (%d)", built.ChainIdentifier, t.node.currentChainID())
	}
	if err := client.LedgerApi.PublishRawTransaction(built); err != nil {
		t.clearPendingIf(holdID)
		return nil, err
	}
	hash := built.Hash.String()
	t.clearPendingIf(holdID)
	if t.ctx != nil {
		runtime.EventsEmit(t.ctx, EventTxPublished, map[string]string{"hash": hash})
	}
	return built, nil
}

// prepareCall builds, PoWs, and signs an embedded-contract call template (without
// publishing), holding it for ConfirmPublish. Reuses the Send guard/PoW path.
func (t *TxService) prepareCall(template *nom.AccountBlock, expect callExpect, summary string) (CallPreview, error) {
	return t.prepareCallWithEffect(template, expect, summary, nil)
}

// prepareCallWithEffect is prepareCall plus a decoded TransactionEffect for the
// confirm dialog (used by flows whose material parameters live in ABI data).
func (t *TxService) prepareCallWithEffect(template *nom.AccountBlock, expect callExpect, summary string, effect *TransactionEffect) (CallPreview, error) {
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
	// Capture the sender the preview will display. signingKeyPair has already
	// asserted the keypair matches this address.
	from, ok := t.wallet.activeAddress()
	if !ok {
		return CallPreview{}, errLocked
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
	holdID, err := t.holdPending(template, callExpect{from: from, to: expect.to, zts: expect.zts, amount: new(big.Int).Set(expect.amount), data: append([]byte(nil), expect.data...), policy: expect.policy}, gen)
	if err != nil {
		return CallPreview{}, err
	}
	return CallPreview{
		FromAddress: from.String(),
		ToAddress:   template.ToAddress.String(),
		Zts:         template.TokenStandard.String(),
		Symbol:      t.symbolFor(template.TokenStandard.String()),
		Amount:      template.Amount.String(),
		Decimals:    resolveDecimals(template.TokenStandard.String(), clientTokenDecimals(client)),
		Summary:     summary,
		Effect:      effect,
		NeedsPoW:    needsPoW,
		HoldID:      holdID,
		// UsedPlasma / Difficulty / Hash are filled by ConfirmPublish's PoW.
	}, nil
}

// Receive receives a single inbound block by its send-block hash.
func (t *TxService) Receive(fromHash string) (string, error) {
	hash, err := types.HexToHash(fromHash)
	if err != nil {
		return "", fmt.Errorf("invalid block hash: %w", err)
	}
	// A receive is a full account-block publication (build → PoW → sign →
	// publish) for the SAME account frontier a send uses. Serialize it with
	// ConfirmPublish under publishMu, or an auto-receive racing a confirmed send
	// produces sibling blocks on one frontier — the loser is rejected after
	// potentially expensive PoW. Blocking (not TryLock) is intentional: receives
	// are queue-driven and can wait for an in-flight publish to finish.
	t.publishMu.Lock()
	defer t.publishMu.Unlock()
	client := t.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	// Receiving signs and publishes a block too, so apply the same guards as a
	// send: the mainnet opt-in, and a chain-ID match. Auto-receive drives this
	// path automatically, so an unguarded receive could publish onto the wrong
	// network without the user ever clicking Confirm.
	if err := t.guard(); err != nil {
		return "", err
	}
	// Chain check before building/signing, independent of any client call (like
	// the send path), so a network mismatch fails fast without publishing onto
	// the wrong chain.
	cid := t.configuredChainID()
	if cid != t.node.currentChainID() {
		return "", fmt.Errorf("configured Chain ID (%d) does not match the connected node's chain (%d); set the correct Chain ID in Settings or connect to a matching node", cid, t.node.currentChainID())
	}
	kp, err := t.wallet.signingKeyPair()
	if err != nil {
		return "", err
	}
	template := client.LedgerApi.ReceiveTemplate(hash)
	template.ChainIdentifier = cid
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

// CancelPending discards the held block. A non-zero holdId makes the cancel
// identity-aware: it only discards the hold it was issued for, so a stale
// cancel that loses a race against a newer Prepare cannot release the newer
// block. holdId 0 discards whatever is held (unconditional).
func (t *TxService) CancelPending(holdId uint64) error {
	if holdId != 0 {
		t.clearPendingIf(holdId)
	} else {
		t.clearPending()
	}
	return nil
}

// clearPendingLocked zeroes the hold. Callers must hold t.mu — this is the ONE
// place the slot's fields are cleared, so adding a field can't be half-done.
func (t *TxService) clearPendingLocked() {
	t.pending = nil
	t.pendingExpect = callExpect{}
	t.pendingGen = 0
	t.pendingHoldID = 0
}

func (t *TxService) clearPending() {
	t.mu.Lock()
	t.clearPendingLocked()
	t.mu.Unlock()
}

// clearPendingIf zeroes the hold only if it is still the one identified by id.
// Terminal confirms and asynchronous cancels use this identity check so stale
// frontend work can never clear a different reviewed block.
func (t *TxService) clearPendingIf(id uint64) {
	t.mu.Lock()
	if t.pendingHoldID == id {
		t.clearPendingLocked()
	}
	t.mu.Unlock()
}
