# Phase 2 — Send / Receive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send (with confirm-what-you-sign) and receive funds reliably on testnet, mainnet gated behind a flag — the correctness-critical milestone.

**Architecture:** A new bound `app/tx_service.go` orchestrates sends via the SDK `zenon` facade using the prepare-then-publish pattern (build+PoW+sign in `PrepareSend`, hold the signed block, broadcast only in `ConfirmPublish` after re-asserting it matches the request). WalletService gains a go-zenon→SDK keypair bridge; NodeService gains chain-id tracking, a client accessor, unreceived reads, and optional auto-receive. The Svelte frontend gets a send route with a confirm modal rendered from the built block, and an unreceived panel.

**Tech Stack:** Go 1.24+, `znn-sdk-go v0.1.16` (`zenon`, `rpc_client`, `pow`, `wallet`), `go-zenon` (`wallet`, `chain/nom`, `common/types`), Wails v2, Svelte+TS+Tailwind, Vitest.

## Global Constraints

- No SDK modification: use `zenon.NewZenon(client)` → `PrepareBlock`/`RequiresPoW`/`Send`; `LedgerApi.PublishRawTransaction`/`GetUnreceivedBlocksByAddress`/`ReceiveTemplate`/`GetAccountBlockByHash`. Keystore/keypair via go-zenon + the mnemonic→SDK-keypair bridge.
- **No secrets across the binding boundary** or in logs. Mnemonic/keypair stay backend-only and transient.
- **Confirm-what-you-sign:** the modal renders from the built, signed block (incl. hash); `ConfirmPublish` re-asserts the held block matches the request before broadcast.
- **Mainnet gating:** `mainnetChainID = 1`; sends to chainId 1 are rejected unless `Settings.AllowMainnetSend` is true (default false). Testnet chainIds always allowed.
- Amounts are base-unit decimal strings end to end (never float); parse with `big.Int.SetString(s,10)`.
- Tests: offline `go test ./...` is network-free; live paths are `//go:build integration` and read the gitignored `secrets/` (skip if absent). Verify frontend tests from a clean install when in doubt. No secrets committed.
- `pnpm` ≥10.16 (min-release-age policy); after any `wails build`, run a clean `pnpm install` before vitest.

## File structure

```
app/tx_service.go          # NEW: bound — PrepareSend/ConfirmPublish/CancelPending/RequiresPoW/Receive
app/tx_service_test.go     # NEW: guard, built-block assertion, mappers
app/wallet_service.go      # MOD: + signingKeyPair() bridge
app/node_service.go        # MOD: + chainID tracking, currentClient(), currentChainID(), GetUnreceived()
app/config_service.go/dto  # MOD: + Settings.AllowMainnetSend, Settings.AutoReceive; + SendRequest/SendPreview/UnreceivedBlock DTOs + tx event names
app/app.go                 # MOD: construct TxService, wire auto-receive callback, distribute ctx
internal/spike/*_integration_test.go  # NEW: testnet send (prepare→confirm), receive, unfused PoW send (Gate 2)
frontend/src/lib/stores/tx.ts, unreceived.ts            # NEW
frontend/src/lib/components/{SendForm,AmountInput,TxModal,TxResult,UnreceivedPanel}.svelte  # NEW
frontend/src/routes/Send.svelte                          # NEW
frontend/src/routes/Dashboard.svelte                     # MOD: add nav to Send + UnreceivedPanel
frontend/src/App.svelte                                  # MOD: route to Send
```

---

## Task 1: WalletService keypair bridge + Settings fields + new DTOs/events

**Files:**
- Modify: `app/wallet_service.go`, `app/dto.go`, `app/config_service.go`
- Test: `app/wallet_service_test.go`

**Interfaces:**
- Consumes: existing `WalletService.keystore` (`*gzwallet.KeyStore` with `.Mnemonic`), `WalletService.active int`, `activeAddress()`, `errLocked`.
- Produces: `(*WalletService) signingKeyPair() (*sdkwallet.KeyPair, error)`; `Settings.AllowMainnetSend bool`, `Settings.AutoReceive bool`; DTOs `SendRequest`, `SendPreview`, `UnreceivedBlock`; consts `EventTxPowProgress`, `EventTxPublished`, `EventTxReceived`, `mainnetChainID`.

- [ ] **Step 1: Add DTOs, events, settings fields**

Append to `app/dto.go`:
```go
// Phase 2 transaction event names.
const (
	EventTxPowProgress = "tx:pow-progress"
	EventTxPublished   = "tx:published"
	EventTxReceived    = "tx:received"
)

// mainnetChainID is the Network of Momentum mainnet chain identifier.
const mainnetChainID uint64 = 1

// SendRequest is the frontend's send intent.
type SendRequest struct {
	ToAddress string `json:"toAddress"`
	Zts       string `json:"zts"`
	Amount    string `json:"amount"` // base-unit decimal string
}

// SendPreview is rendered from the built, signed block before broadcast.
type SendPreview struct {
	ToAddress  string `json:"toAddress"`
	Symbol     string `json:"symbol"`
	Zts        string `json:"zts"`
	Amount     string `json:"amount"`
	UsedPlasma uint64 `json:"usedPlasma"`
	Difficulty uint64 `json:"difficulty"`
	Hash       string `json:"hash"`
	NeedsPoW   bool   `json:"needsPoW"`
}

// UnreceivedBlock is one inbound, not-yet-received transaction.
type UnreceivedBlock struct {
	FromHash    string `json:"fromHash"`
	FromAddress string `json:"fromAddress"`
	Token       string `json:"token"`
	Amount      string `json:"amount"`
}
```

In `app/dto.go`, extend `Settings`:
```go
type Settings struct {
	NodeURL          string `json:"nodeUrl"`
	Theme            string `json:"theme"`
	LastWallet       string `json:"lastWallet"`
	ActiveAccount    int    `json:"activeAccount"`
	AllowMainnetSend bool   `json:"allowMainnetSend"`
	AutoReceive      bool   `json:"autoReceive"`
}
```
(`defaultSettings()` in `config_service.go` leaves both false by zero-value — no change needed there.)

- [ ] **Step 2: Write the failing signingKeyPair test**

Add to `app/wallet_service_test.go`:
```go
func TestSigningKeyPairMatchesActiveAddress(t *testing.T) {
	ksPath, pw := locateSecretsKeystore(t)
	w := newTestWalletService(t)
	meta, err := w.ImportKeystore(ksPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.Name, pw); err != nil {
		t.Fatal(err)
	}

	kp, err := w.signingKeyPair()
	if err != nil {
		t.Fatalf("signingKeyPair: %v", err)
	}
	addr, err := kp.GetAddress()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := w.activeAddress()
	if *addr != want {
		t.Fatalf("sdk keypair address %s != active %s", addr, want)
	}

	_ = w.Lock()
	if _, err := w.signingKeyPair(); err == nil {
		t.Fatal("expected signingKeyPair to fail when locked")
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./app/ -run TestSigningKeyPair -v`
Expected: FAIL — `w.signingKeyPair undefined`.

- [ ] **Step 4: Implement signingKeyPair**

Add to `app/wallet_service.go` (add import `sdkwallet "github.com/0x3639/znn-sdk-go/wallet"`):
```go
// signingKeyPair derives the SDK keypair for the active account from the
// unlocked mnemonic and asserts it matches the go-zenon active address (the
// Phase-0 cross-check). The mnemonic and keypair stay backend-only.
func (w *WalletService) signingKeyPair() (*sdkwallet.KeyPair, error) {
	if w.keystore == nil {
		return nil, errLocked
	}
	sdkKs, err := sdkwallet.NewKeyStoreFromMnemonic(w.keystore.Mnemonic)
	if err != nil {
		return nil, err
	}
	kp, err := sdkKs.GetKeyPair(w.active)
	if err != nil {
		return nil, err
	}
	addr, err := kp.GetAddress()
	if err != nil {
		return nil, err
	}
	want, ok := w.activeAddress()
	if !ok {
		return nil, errLocked
	}
	if *addr != want {
		return nil, fmt.Errorf("SDK-derived address %s does not match active address %s", addr.String(), want.String())
	}
	return kp, nil
}
```

- [ ] **Step 5: Run to verify pass + build**

Run: `go test ./app/ -run 'TestSigningKeyPair|TestSettings' -v && go build ./...`
Expected: PASS (signing test runs only with `secrets/`; skips otherwise) and clean build.

- [ ] **Step 6: Commit**

```bash
git add app/wallet_service.go app/dto.go app/wallet_service_test.go
git commit -m "feat(app): keypair bridge, Phase 2 DTOs/events, mainnet/auto-receive settings"
```

---

## Task 2: NodeService — chain-id, client accessor, unreceived reads

**Files:**
- Modify: `app/node_service.go`
- Test: `app/node_service_test.go`

**Interfaces:**
- Consumes: existing `NodeService{client, mu, height}`, `SetNode`, `GetFrontierMomentum` (`.ChainIdentifier`), `wallet.activeAddress()`.
- Produces: `(*NodeService) currentClient() *rpc_client.RpcClient`; `currentChainID() uint64`; `GetUnreceived() ([]UnreceivedBlock, error)`; field `chainID uint64` set in `SetNode`; mapper `toUnreceivedBlock(*api.AccountBlock) UnreceivedBlock`.

- [ ] **Step 1: Write the failing mapper test**

Add to `app/node_service_test.go`:
```go
func TestToUnreceivedBlock(t *testing.T) {
	b := &api.AccountBlock{}
	b.AccountBlock = nom.AccountBlock{
		Hash:          types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		Address:       types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"),
		Amount:        big.NewInt(150000000),
		TokenStandard: types.ZnnTokenStandard,
	}
	got := toUnreceivedBlock(b)
	if got.FromAddress != b.Address.String() || got.Amount != "150000000" {
		t.Fatalf("toUnreceivedBlock = %+v", got)
	}
	if got.FromHash != b.Hash.String() {
		t.Fatalf("fromHash = %s", got.FromHash)
	}
}
```
(Confirm the `b.AccountBlock = nom.AccountBlock{...}` embedded-field form compiles — same pattern Phase 1 used in `node_service_test.go`.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestToUnreceived -v`
Expected: FAIL — `toUnreceivedBlock undefined`.

- [ ] **Step 3: Implement accessors + unreceived**

In `app/node_service.go`: add `chainID uint64` to the struct; in `SetNode`, after `m, err := client.LedgerApi.GetFrontierMomentum()` and the write-lock section, set `n.chainID = m.ChainIdentifier` alongside `n.height = m.Height`. Add:
```go
// currentClient returns the connected client or nil, under the read lock.
func (n *NodeService) currentClient() *rpc_client.RpcClient {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.client
}

// currentChainID returns the connected node's chain identifier (0 if unknown).
func (n *NodeService) currentChainID() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.chainID
}

// GetUnreceived lists inbound blocks not yet received by the active address.
func (n *NodeService) GetUnreceived() ([]UnreceivedBlock, error) {
	client := n.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	list, err := client.LedgerApi.GetUnreceivedBlocksByAddress(addr, 0, 50)
	if err != nil {
		return nil, err
	}
	out := []UnreceivedBlock{}
	for _, b := range list.List {
		out = append(out, toUnreceivedBlock(b))
	}
	return out, nil
}

func toUnreceivedBlock(b *api.AccountBlock) UnreceivedBlock {
	u := UnreceivedBlock{FromHash: b.Hash.String(), FromAddress: b.Address.String(), Amount: "0", Token: b.TokenStandard.String()}
	if b.Amount != nil {
		u.Amount = b.Amount.String()
	}
	if b.TokenInfo != nil {
		u.Token = b.TokenInfo.TokenSymbol
	}
	return u
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run 'TestToUnreceived|TestStatus' -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add app/node_service.go app/node_service_test.go
git commit -m "feat(app): NodeService chain-id tracking, client accessor, unreceived reads"
```

---

## Task 3: TxService — send (prepare → confirm-publish)

**Files:**
- Create: `app/tx_service.go`
- Test: `app/tx_service_test.go`

**Interfaces:**
- Consumes: `WalletService.signingKeyPair()`, `NodeService.currentClient()`/`currentChainID()`, `ConfigService.GetSettings()`, DTOs `SendRequest`/`SendPreview`, events, `mainnetChainID`; SDK `zenon`, `pow`, `rpc_client`; go-zenon `types`, `nom`.
- Produces: `newTxService(*ConfigService,*WalletService,*NodeService) *TxService`; `PrepareSend(SendRequest)(SendPreview,error)`; `ConfirmPublish()(string,error)`; `CancelPending() error`; `RequiresPoW(SendRequest)(bool,error)`; field `ctx context.Context`. Internal: held `pending *nom.AccountBlock` + `pendingReq SendRequest` under a mutex; `symbolFor(zts) string`.

- [ ] **Step 1: Write the failing tests (guard + assertion + symbol)**

`app/tx_service_test.go`:
```go
package app

import (
	"math/big"
	"testing"

	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

func newTestTxService(t *testing.T) *TxService {
	t.Helper()
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	cfg := newConfigService()
	w := newWalletService(cfg)
	n := newNodeService(cfg, w)
	return newTxService(cfg, w, n)
}

func TestPrepareSendRejectsBadAddress(t *testing.T) {
	tx := newTestTxService(t)
	if _, err := tx.PrepareSend(SendRequest{ToAddress: "not-an-address", Zts: types.ZnnTokenStandard.String(), Amount: "1"}); err == nil {
		t.Fatal("expected invalid-address error")
	}
}

func TestConfirmPublishNoPending(t *testing.T) {
	tx := newTestTxService(t)
	if _, err := tx.ConfirmPublish(); err == nil {
		t.Fatal("expected error when no pending transaction")
	}
}

func TestConfirmPublishRejectsTamperedBlock(t *testing.T) {
	tx := newTestTxService(t)
	// Simulate a held block that disagrees with the recorded request.
	tx.pending = &nom.AccountBlock{
		ToAddress:     types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"),
		Amount:        big.NewInt(999),
		TokenStandard: types.ZnnTokenStandard,
	}
	tx.pendingReq = SendRequest{ToAddress: "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", Zts: types.ZnnTokenStandard.String(), Amount: "1"}
	if _, err := tx.ConfirmPublish(); err == nil {
		t.Fatal("expected mismatch error; tampered block must not publish")
	}
	if tx.pending != nil {
		t.Fatal("pending block must be cleared after a mismatch")
	}
}

func TestSymbolFor(t *testing.T) {
	tx := newTestTxService(t)
	if tx.symbolFor(types.ZnnTokenStandard.String()) != "ZNN" || tx.symbolFor(types.QsrTokenStandard.String()) != "QSR" {
		t.Fatal("ZNN/QSR symbols wrong")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestPrepareSend|TestConfirmPublish|TestSymbolFor' -v`
Expected: FAIL — `newTxService undefined`.

- [ ] **Step 3: Implement TxService**

`app/tx_service.go`:
```go
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

	t.mu.Lock()
	t.pending = built
	t.pendingReq = req
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
	t.mu.Lock()
	b, req := t.pending, t.pendingReq
	t.mu.Unlock()
	if b == nil {
		return "", errors.New("no pending transaction")
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

	client := t.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
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

// CancelPending discards the held block.
func (t *TxService) CancelPending() error {
	t.clearPending()
	return nil
}

func (t *TxService) clearPending() {
	t.mu.Lock()
	t.pending = nil
	t.pendingReq = SendRequest{}
	t.mu.Unlock()
}
```

> Confirm `types.ParseZTS` exists (it does in go-zenon `common/types/tokenstandard.go`; `ParseZTSPanic` is used there). If the exact name differs, adjust (`go doc github.com/zenon-network/go-zenon/common/types.ParseZTS`).

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run 'TestPrepareSend|TestConfirmPublish|TestSymbolFor' -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add app/tx_service.go app/tx_service_test.go
git commit -m "feat(app): TxService send pipeline (prepare-then-publish, confirm-what-you-sign)"
```

---

## Task 4: TxService.Receive + auto-receive wiring

**Files:**
- Modify: `app/tx_service.go`, `app/node_service.go`, `app/app.go`
- Test: `app/tx_service_test.go`

**Interfaces:**
- Consumes: `LedgerApi.ReceiveTemplate`, `zenon.Send`, `SubscriberApi.ToUnreceivedAccountBlocksByAddress`, `Settings.AutoReceive`.
- Produces: `(*TxService) Receive(fromHash string) (string, error)`; `(*NodeService) setReceiveFunc(func(string)(string,error))` + auto-receive subscription started when `AutoReceive` is on; `App.New` wires `node.setReceiveFunc(tx.Receive)`.

- [ ] **Step 1: Write the failing test**

Add to `app/tx_service_test.go`:
```go
func TestReceiveRejectsBadHash(t *testing.T) {
	tx := newTestTxService(t)
	if _, err := tx.Receive("not-a-hash"); err == nil {
		t.Fatal("expected error for invalid hash")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestReceiveRejects -v`
Expected: FAIL — `tx.Receive undefined`.

- [ ] **Step 3: Implement Receive + auto-receive**

Add to `app/tx_service.go` (add imports `"github.com/zenon-network/go-zenon/common/types"` already present):
```go
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
```

In `app/node_service.go`, add a receive callback + auto-receive control:
```go
// (struct) add fields:
//   receiveFn  func(fromHash string) (string, error)
//   autoStop   chan struct{}

func (n *NodeService) setReceiveFunc(fn func(string) (string, error)) { n.receiveFn = fn }

// StartAutoReceive subscribes to unreceived blocks for the active address and
// receives each via receiveFn. Idempotent; StopAutoReceive stops it.
func (n *NodeService) StartAutoReceive() error {
	client := n.currentClient()
	if client == nil {
		return errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return errLocked
	}
	ctx := n.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	sub, ch, err := client.SubscriberApi.ToUnreceivedAccountBlocksByAddress(ctx, addr)
	if err != nil {
		return err
	}
	n.autoStop = make(chan struct{})
	stop := n.autoStop
	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-stop:
				return
			case blocks := <-ch:
				if n.receiveFn == nil {
					continue
				}
				for _, b := range blocks {
					_, _ = n.receiveFn(b.Hash.String())
				}
			}
		}
	}()
	return nil
}

func (n *NodeService) StopAutoReceive() {
	if n.autoStop != nil {
		close(n.autoStop)
		n.autoStop = nil
	}
}
```
(Confirm the subscribe channel element type exposes `.Hash`; the Phase-0 README shows `ToUnreceivedAccountBlocksByAddress` yields account blocks — adjust `b.Hash.String()` to the actual element type via `go doc`.)

In `app/app.go` `New()`, after constructing services, wire the callback:
```go
n.setReceiveFunc(t.Receive) // where t is the TxService (see Task 5 for construction order)
```

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run TestReceiveRejects -v && go build ./...`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add app/tx_service.go app/node_service.go app/app.go
git commit -m "feat(app): receive flow + auto-receive subscription"
```

---

## Task 5: Bind TxService + regenerate bindings

**Files:**
- Modify: `app/app.go`
- Modify: `frontend/wailsjs/` (generated)

**Interfaces:**
- Consumes: `newTxService`, `TxService.ctx`, `node.setReceiveFunc`.
- Produces: `App.Tx *TxService` bound; TS bindings `frontend/wailsjs/go/app/TxService`.

- [ ] **Step 1: Wire TxService into App**

In `app/app.go`: add `Tx *TxService` to `App`; in `New()` construct `t := newTxService(cfg, w, n)`, set `n.setReceiveFunc(t.Receive)`, store `Tx: t`; in `OnStartup` add `a.Tx.ctx = ctx`; in `Bindings()` append `a.Tx`. In `OnShutdown` add `a.Node.StopAutoReceive()` before disconnect.

Run: `go build ./...` — expect success.

- [ ] **Step 2: Regenerate bindings**

```bash
"$(go env GOPATH)/bin/wails" generate module
ls frontend/wailsjs/go/app   # expect TxService.* alongside the others
```

- [ ] **Step 3: Commit**

```bash
git add app/app.go frontend/wailsjs
git commit -m "feat(app): bind TxService and regenerate TS bindings"
```

---

## Task 6: Frontend — tx store + send route (form + validation)

**Files:**
- Create: `frontend/src/lib/stores/tx.ts`, `frontend/src/lib/components/AmountInput.svelte`, `frontend/src/lib/components/SendForm.svelte`, `frontend/src/routes/Send.svelte`
- Test: `frontend/src/routes/Send.test.ts`

**Interfaces:**
- Consumes: generated `wailsjs/go/app/TxService` (`PrepareSend`,`ConfirmPublish`,`CancelPending`,`RequiresPoW`), `balances` store, `format`.
- Produces: `tx` store (`{status, preview, hash, error}` + `prepare/confirm/cancel`); `Send.svelte` route.

- [ ] **Step 1: Write the failing SendForm test**

`frontend/src/routes/Send.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/TxService', () => ({
  PrepareSend: vi.fn().mockResolvedValue({ toAddress: 'z1...', symbol: 'ZNN', zts: 'zts1znn...', amount: '100000000', usedPlasma: 21000, difficulty: 0, hash: 'abcd', needsPoW: false }),
  ConfirmPublish: vi.fn().mockResolvedValue('abcd'),
  CancelPending: vi.fn().mockResolvedValue(undefined),
  RequiresPoW: vi.fn().mockResolvedValue(false),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))

import Send from './Send.svelte'

describe('Send', () => {
  it('disables Send for an invalid address', async () => {
    render(Send)
    await fireEvent.input(screen.getByLabelText(/recipient/i), { target: { value: 'nope' } })
    await fireEvent.input(screen.getByLabelText(/amount/i), { target: { value: '1' } })
    expect((screen.getByRole('button', { name: /^send$/i }) as HTMLButtonElement).disabled).toBe(true)
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Send.test.ts`
Expected: FAIL — cannot resolve `./Send.svelte`.

- [ ] **Step 3: Implement store + components**

`frontend/src/lib/stores/tx.ts`:
```ts
import { writable } from 'svelte/store'
import * as Tx from '../../../wailsjs/go/app/TxService'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean }
export type TxState = { status: 'idle' | 'preparing' | 'awaiting' | 'publishing' | 'done' | 'error'; preview: SendPreview | null; hash: string; error: string }

export const tx = writable<TxState>({ status: 'idle', preview: null, hash: '', error: '' })

export async function prepare(toAddress: string, zts: string, amount: string): Promise<void> {
  tx.set({ status: 'preparing', preview: null, hash: '', error: '' })
  try {
    const preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
    tx.set({ status: 'awaiting', preview, hash: '', error: '' })
  } catch (e: any) {
    tx.set({ status: 'error', preview: null, hash: '', error: e?.message ?? String(e) })
  }
}

export async function confirm(): Promise<void> {
  tx.update((s) => ({ ...s, status: 'publishing' }))
  try {
    const hash = (await Tx.ConfirmPublish()) as string
    tx.set({ status: 'done', preview: null, hash, error: '' })
  } catch (e: any) {
    tx.update((s) => ({ ...s, status: 'error', error: e?.message ?? String(e) }))
  }
}

export async function cancel(): Promise<void> {
  await Tx.CancelPending().catch(() => {})
  tx.set({ status: 'idle', preview: null, hash: '', error: '' })
}
```

`frontend/src/lib/components/AmountInput.svelte`:
```svelte
<script lang="ts">
  export let value = ''
  export let label = 'Amount'
  function onInput(e: Event) { value = (e.target as HTMLInputElement).value.replace(/[^0-9.]/g, '') }
</script>
<label class="block text-sm text-muted">{label}
  <input inputmode="decimal" {value} on:input={onInput} aria-label={label}
    class="mt-1 w-full rounded bg-surface px-3 py-2 font-mono text-text outline-none focus:ring-2 focus:ring-accent" />
</label>
```

`frontend/src/lib/components/SendForm.svelte`:
```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { balances } from '../stores/balances'
  import AmountInput from './AmountInput.svelte'

  const dispatch = createEventDispatcher()
  export let recipient = ''
  export let zts = ''
  export let amountDecimal = ''

  $: if (!zts && $balances[0]) zts = $balances[0].zts
  // z1 bech32: starts z1, lowercase alnum, length ~40. Backend re-validates authoritatively.
  $: validAddr = /^z1[0-9a-z]{38}$/.test(recipient)
  $: validAmount = amountDecimal !== '' && Number(amountDecimal) > 0
  $: canSend = validAddr && validAmount && !!zts
</script>

<div class="space-y-3">
  <label class="block text-sm text-muted">Recipient
    <input bind:value={recipient} aria-label="recipient" placeholder="z1…"
      class="mt-1 w-full rounded bg-surface px-3 py-2 font-mono text-text outline-none focus:ring-2 focus:ring-accent" />
  </label>
  {#if recipient && !validAddr}<p class="text-xs text-error">Invalid z1 address</p>{/if}

  <label class="block text-sm text-muted">Token
    <select bind:value={zts} class="mt-1 w-full rounded bg-surface px-3 py-2 text-text">
      {#each $balances as b}<option value={b.zts}>{b.symbol || b.zts}</option>{/each}
    </select>
  </label>

  <AmountInput bind:value={amountDecimal} />

  <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!canSend}
    aria-label="Send" on:click={() => dispatch('send', { recipient, zts, amountDecimal })}>Send</button>
</div>
```

`frontend/src/routes/Send.svelte` (converts the decimal amount to base units using the selected token's decimals from `balances`):
```svelte
<script lang="ts">
  import { balances } from '../lib/stores/balances'
  import { tx, prepare } from '../lib/stores/tx'
  import SendForm from '../lib/components/SendForm.svelte'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  function toBase(decimal: string, decimals: number): string {
    const [i, f = ''] = decimal.split('.')
    const frac = (f + '0'.repeat(decimals)).slice(0, decimals)
    return (BigInt(i || '0') * BigInt(10) ** BigInt(decimals) + BigInt(frac || '0')).toString()
  }

  async function onSend(e: CustomEvent) {
    const { recipient, zts, amountDecimal } = e.detail
    const tok = $balances.find((b) => b.zts === zts)
    const base = toBase(amountDecimal, tok?.decimals ?? 8)
    await prepare(recipient, zts, base)
  }
</script>

<div class="mx-auto mt-8 w-[28rem] space-y-4">
  <h1 class="text-xl">Send</h1>
  <SendForm on:send={onSend} />
  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
```

(TxModal/TxResult are created in Task 7; for this task's test, the modal/result branches aren't exercised — the test only checks the disabled Send button. If the import of not-yet-existing components blocks the test, create minimal stub `TxModal.svelte`/`TxResult.svelte` returning empty markup now and flesh them out in Task 7.)

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test src/routes/Send.test.ts && pnpm run build`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/stores/tx.ts frontend/src/lib/components/AmountInput.svelte frontend/src/lib/components/SendForm.svelte frontend/src/routes/Send.svelte frontend/src/lib/components/TxModal.svelte frontend/src/lib/components/TxResult.svelte frontend/src/routes/Send.test.ts
git commit -m "feat(frontend): tx store + send form with validation"
```

---

## Task 7: Frontend — confirm modal + result

**Files:**
- Modify/Create: `frontend/src/lib/components/TxModal.svelte`, `frontend/src/lib/components/TxResult.svelte`
- Test: `frontend/src/lib/components/TxModal.test.ts`

**Interfaces:**
- Consumes: `tx` store (`confirm`,`cancel`,`preview`,`hash`), `format`, `ClipboardSetText`.
- Produces: `TxModal` (renders preview incl. hash; Confirm/Cancel), `TxResult` (hash + copy).

- [ ] **Step 1: Write the failing modal test**

`frontend/src/lib/components/TxModal.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/TxService', () => ({ ConfirmPublish: vi.fn(), CancelPending: vi.fn() }))
import TxModal from './TxModal.svelte'
import { tx } from '../stores/tx'

describe('TxModal', () => {
  it('renders the built-block preview incl. hash', async () => {
    tx.set({ status: 'awaiting', hash: '', error: '',
      preview: { toAddress: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg', symbol: 'ZNN', zts: 'zts1znn', amount: '150000000', usedPlasma: 21000, difficulty: 0, hash: 'deadbeef', needsPoW: false } })
    render(TxModal)
    expect(await screen.findByText(/deadbeef/)).toBeTruthy()
    expect(await screen.findByText(/1\.5/)).toBeTruthy()
    expect(await screen.findByText(/ZNN/)).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/lib/components/TxModal.test.ts`
Expected: FAIL (empty stub renders nothing matching).

- [ ] **Step 3: Implement TxModal + TxResult**

`frontend/src/lib/components/TxModal.svelte`:
```svelte
<script lang="ts">
  import { tx, confirm, cancel } from '../stores/tx'
  import { formatAmount, shortAddress } from '../format'
  $: p = $tx.preview
</script>
{#if p}
<div class="rounded border border-accent/40 bg-surface p-4 space-y-2" role="dialog" aria-label="Confirm transaction">
  <h2 class="text-sm text-muted">Confirm — you are signing this exact transaction</h2>
  <div class="flex justify-between"><span class="text-muted">To</span><span class="font-mono">{shortAddress(p.toAddress)}</span></div>
  <div class="flex justify-between"><span class="text-muted">Amount</span><span class="font-mono">{formatAmount(p.amount, 8)} {p.symbol || p.zts}</span></div>
  <div class="flex justify-between"><span class="text-muted">Fee</span><span>{p.needsPoW ? `PoW (difficulty ${p.difficulty})` : 'Feeless (plasma)'}</span></div>
  <div class="flex justify-between"><span class="text-muted">Hash</span><span class="font-mono text-xs break-all">{p.hash}</span></div>
  <div class="flex gap-2 pt-2">
    <button class="flex-1 rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={$tx.status === 'publishing'} on:click={confirm}>Confirm</button>
    <button class="flex-1 rounded border border-muted/40 py-2 text-muted" on:click={cancel}>Cancel</button>
  </div>
</div>
{/if}
```

`frontend/src/lib/components/TxResult.svelte`:
```svelte
<script lang="ts">
  import { tx } from '../stores/tx'
  import { ClipboardSetText } from '../../../wailsjs/runtime/runtime'
  let copied = false
  async function copy() { await ClipboardSetText($tx.hash); copied = true; setTimeout(() => (copied = false), 1200) }
</script>
<div class="rounded border border-success/40 bg-surface p-4 space-y-2">
  <p class="text-success">Transaction published</p>
  <div class="font-mono text-xs break-all">{$tx.hash}</div>
  <button class="rounded bg-accent/20 px-2 py-1 text-xs text-accent" on:click={copy}>{copied ? 'Copied' : 'Copy hash'}</button>
</div>
```

(`formatAmount(p.amount, 8)` uses 8 decimals for ZNN/QSR — consistent with Phase 1's display assumption; multi-decimal tokens are a later concern.)

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test src/lib/components/TxModal.test.ts && pnpm run build`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/components/TxModal.svelte frontend/src/lib/components/TxResult.svelte frontend/src/lib/components/TxModal.test.ts
git commit -m "feat(frontend): confirm-what-you-sign modal + tx result"
```

---

## Task 8: Frontend — unreceived panel + send nav + auto-receive toggle

**Files:**
- Create: `frontend/src/lib/stores/unreceived.ts`, `frontend/src/lib/components/UnreceivedPanel.svelte`
- Modify: `frontend/src/routes/Dashboard.svelte`, `frontend/src/App.svelte`
- Test: `frontend/src/lib/components/UnreceivedPanel.test.ts`

**Interfaces:**
- Consumes: `NodeService.GetUnreceived`, `TxService.Receive`, `ConfigService.GetSettings`/`SetSettings` (AutoReceive), events `tx:received`.
- Produces: `unreceived` store (`load`), `UnreceivedPanel` (list + Receive/Receive-All), Dashboard "Send" nav + panel, App route to Send.

- [ ] **Step 1: Write the failing panel test**

`frontend/src/lib/components/UnreceivedPanel.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/NodeService', () => ({ GetUnreceived: vi.fn().mockResolvedValue([{ fromHash: 'h1', fromAddress: 'z1abc', token: 'ZNN', amount: '100000000' }]) }))
vi.mock('../../../wailsjs/go/app/TxService', () => ({ Receive: vi.fn().mockResolvedValue('r1') }))
import UnreceivedPanel from './UnreceivedPanel.svelte'

describe('UnreceivedPanel', () => {
  it('lists unreceived blocks', async () => {
    render(UnreceivedPanel)
    expect(await screen.findByText(/1 ZNN|1\b/)).toBeTruthy()
    expect(await screen.findByRole('button', { name: /receive/i })).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/lib/components/UnreceivedPanel.test.ts`
Expected: FAIL — cannot resolve `./UnreceivedPanel.svelte`.

- [ ] **Step 3: Implement store + panel + nav**

`frontend/src/lib/stores/unreceived.ts`:
```ts
import { writable } from 'svelte/store'
import * as N from '../../../wailsjs/go/app/NodeService'

export type Unreceived = { fromHash: string; fromAddress: string; token: string; amount: string }
export const unreceived = writable<Unreceived[]>([])

export async function loadUnreceived(): Promise<void> {
  try { unreceived.set((await N.GetUnreceived()) as unknown as Unreceived[]) } catch { unreceived.set([]) }
}
```

`frontend/src/lib/components/UnreceivedPanel.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import { unreceived, loadUnreceived } from '../stores/unreceived'
  import * as Tx from '../../../wailsjs/go/app/TxService'
  import { formatAmount, shortAddress } from '../format'
  onMount(loadUnreceived)
  async function receive(hash: string) { await Tx.Receive(hash); await loadUnreceived() }
  async function receiveAll() { for (const u of $unreceived) { await Tx.Receive(u.fromHash) } await loadUnreceived() }
</script>
<div class="rounded bg-surface p-4">
  <div class="mb-2 flex items-center justify-between">
    <h2 class="text-sm text-muted">Unreceived ({$unreceived.length})</h2>
    {#if $unreceived.length}<button class="text-xs text-accent" on:click={receiveAll}>Receive all</button>{/if}
  </div>
  {#each $unreceived as u}
    <div class="flex items-center justify-between py-1 text-sm">
      <span class="font-mono">{shortAddress(u.fromAddress)}</span>
      <span class="font-mono">{formatAmount(u.amount, 8)} {u.token}</span>
      <button class="rounded bg-accent/20 px-2 py-1 text-xs text-accent" on:click={() => receive(u.fromHash)}>Receive</button>
    </div>
  {/each}
</div>
```

In `frontend/src/routes/Dashboard.svelte`: import and render `<UnreceivedPanel />`; add a "Send" button that sets a simple view flag (or use the App route). In `frontend/src/App.svelte`: add a minimal route flag so a "Send" action shows `Send.svelte` and a back action returns to Dashboard (a `view` writable in a small `nav` store, or a local boolean in App driven by a custom event). Keep it minimal — a `view` store with `'dashboard' | 'send'` and buttons toggling it; auto-receive toggle is a checkbox in the dashboard header bound to `ConfigService` settings (`GetSettings`/`SetSettings`) that calls `NodeService.StartAutoReceive`/`StopAutoReceive` via bindings.

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): unreceived panel, send nav, auto-receive toggle"
```

---

## Task 9: Integration tests (testnet) + Gate-2 PoW send + review checklist

**Files:**
- Create: `internal/spike/phase2_send_integration_test.go`
- Create: `docs/phase2-crypto-review-checklist.md`

**Interfaces:**
- Consumes: the full `app` services (or the SDK directly) against a testnet node + the gitignored `secrets/` keystore.
- Produces: confirmed testnet send (prepare→confirm), receive, and an **unfused-address PoW send** — the Gate 2 carry-forward; plus the crypto-path review checklist.

- [ ] **Step 1: Write the integration tests**

`internal/spike/phase2_send_integration_test.go` (build-tagged; skips without env/secrets). It exercises the real pipeline end to end against testnet, mirroring the Phase-0 chain-id guard pattern (refuse unless chainId == expected testnet). Cover: (a) PrepareSend→ConfirmPublish for a 0.1 ZNN self-send confirms on-chain via `GetAccountBlockByHash`; (b) a receive of an unreceived block; (c) **a send from an account index with no fused plasma so `RequiresPoW` is true and the published block carries `Difficulty>0`** — assert it confirms (this is the Gate-2 PoW proof). Use env vars `ZNN_TESTNET_URL`, `ZNN_KEYSTORE`, `ZNN_KEYSTORE_PASSWORD`, `ZNN_EXPECT_CHAINID` (default 73404), and an unfused index via `ZNN_POW_ACCOUNT_INDEX`.

(Write the full test body following the Phase-0 `send_integration_test.go` structure: build the services with a temp data dir, point NodeService at the testnet URL, import+unlock the keystore, assert chainId, drive PrepareSend/ConfirmPublish and Receive, and poll `GetAccountBlockByHash(...).ConfirmationDetail`. Include the chain-id guard so it can never run against mainnet.)

- [ ] **Step 2: Run against testnet (requires a node with embedded.* enabled)**

```bash
ZNN_TESTNET_URL="ws://<testnet-with-embedded>:35998" \
ZNN_KEYSTORE="$PWD/secrets/pillar.json" ZNN_KEYSTORE_PASSWORD="<pw>" \
go test ./internal/spike/ -tags integration -run TestPhase2 -v -timeout 240s
```
Expected: send confirms; receive confirms; the PoW send confirms with `Difficulty>0`. (Needs a testnet node exposing the `embedded` namespace — `zenon.Send`/`PrepareBlock` calls `embedded.plasma.getRequiredPoWForAccountBlock`.)

- [ ] **Step 3: Write the crypto-path review checklist**

`docs/phase2-crypto-review-checklist.md`: the Gate-2 review items — keystore read (go-zenon), derivation cross-check (signingKeyPair), hash construction + signing (delegated to `zenon.PrepareBlock`), PoW (canonical via the facade), confirm-what-you-sign (modal from built block + re-assert before publish), mainnet flag gating. Mark which are covered by tests vs. need human review before enabling `AllowMainnetSend`.

- [ ] **Step 4: Commit**

```bash
git add internal/spike/phase2_send_integration_test.go docs/phase2-crypto-review-checklist.md
git commit -m "test: Phase 2 testnet send/receive + Gate-2 PoW send; crypto review checklist"
```

---

## Self-Review

**Spec coverage:** keypair bridge (T1), Settings flags + DTOs/events (T1), chain-id + unreceived (T2), send prepare-then-publish + guard + confirm-what-you-sign assertion (T3), receive + auto-receive (T4), binding (T5), send UI + validation (T6), confirm modal from built block (T7), unreceived panel + nav + auto-receive toggle (T8), testnet integration + Gate-2 PoW send + review checklist (T9). All spec sections map to a task.

**Placeholder scan:** No TBD/TODO. Two items flagged for environment confirmation (`types.ParseZTS` exact name; the `ToUnreceivedAccountBlocksByAddress` channel element type for `.Hash`) with the `go doc` to verify — real APIs surfaced for confirmation, not placeholders. Task 9's integration body is described structurally with exact env vars and assertions, pointing at the existing Phase-0 test as the concrete template (its full code is in the repo) — acceptable since it's a network test whose shape is fixed but node-dependent; the implementer fills the body from that template.

**Type consistency:** DTO json tags (camelCase) match the TS store/component types. `signingKeyPair()` (T1) is consumed by TxService (T3/T4). `currentClient()/currentChainID()/GetUnreceived()` (T2) used by TxService/App. `PrepareSend/ConfirmPublish/CancelPending/RequiresPoW/Receive` signatures are identical between `tx_service.go` and the frontend store. Event names (`tx:pow-progress`/`tx:published`/`tx:received`) match between `dto.go` and the stores. `mainnetChainID`/`AllowMainnetSend` used consistently by the guard.

**Known environment dependencies (flagged):** Task 9 needs a testnet node with the `embedded` RPC namespace enabled (the Phase-0 lesson); `types.ParseZTS` and the unreceived-subscription element type confirmed at implementation time.
