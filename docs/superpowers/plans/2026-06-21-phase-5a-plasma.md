# Phase 5a — Plasma (Fuse / Cancel) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** View plasma + fused QSR + fusion entries, fuse QSR for a beneficiary, and cancel revocable fusions — via a new `NomService` plus a generic contract-call path in TxService that reuses confirm-what-you-sign/PoW/chain-guard.

**Architecture:** `NomService` (new, bound) builds embedded-contract templates and exposes reads; state-changing actions delegate to `TxService.prepareCall(template, expect, summary)` which reuses the Phase-2 guard→`zenon.PrepareBlock`→hold→`ConfirmPublish` pipeline. `ConfirmPublish`'s re-assertion is unified on a stored `callExpect` for both Send and contract calls.

**Tech Stack:** Go 1.24+, `znn-sdk-go` (`PlasmaApi`, `zenon` facade), `go-zenon/common/types`, Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- Templates via `client.PlasmaApi.Fuse(beneficiary, amount)` / `Cancel(id)`; publish via the `zenon` facade. No SDK/go-zenon forks.
- One audited prepare/confirm/publish path: `prepareCall` holds a single built block + `callExpect{to,zts,amount}`; `ConfirmPublish` re-asserts the built block's `ToAddress`/`TokenStandard`/`Amount` == `expect` before broadcast. Mainnet stays behind `AllowMainnetSend` (default false); chain-id guard prevents wrong-network broadcast.
- No key material in NomService; mnemonic/keypair stay in WalletService/TxService; nothing sensitive logged.
- Plasma is QSR-only: `types.QsrTokenStandard`; plasma contract `types.PlasmaContract`.
- `IsRevocable` is computed (`currentFrontierHeight >= ExpirationHeight`) — the SDK `FusionEntry` has no such field.
- Residual (flagged, not fixed here): contract `Data` (fuse beneficiary) shown in the summary but not ABI-decode-re-asserted at publish — Phase-5 hardening.
- `go test ./...` offline (testnet fuse/cancel is `//go:build integration`, opt-in). Frontend `pnpm test` + `pnpm run build` pass.
- ENV HAZARD (iCloud repo): `" 2"` collision copies break builds (`find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} +`); `node_modules` eviction (`rm -rf frontend/node_modules && pnpm install`); codesign xattrs (`xattr -cr build/bin`). Commits GPG-signed.

## File structure

```
app/tx_service.go        # MOD: callExpect, pendingExpect (replaces pendingReq), assertMatches, prepareCall; ConfirmPublish unified
app/tx_service_test.go    # MOD: convert pendingReq→pendingExpect in 5 tests; add assertMatches test
app/dto.go               # MOD: CallPreview, PlasmaInfo, FusionEntry DTOs
app/nom_service.go       # NEW: NomService (reads + PrepareFuse/PrepareCancelFuse) + pure mappers
app/nom_service_test.go   # NEW: mapper + IsRevocable + input-validation tests
app/nom_plasma_integration_test.go  # NEW: //go:build integration testnet fuse/cancel
app/app.go               # MOD: construct + bind NomService
frontend/wailsjs/...     # regenerated bindings
frontend/src/lib/stores/plasma.ts        # NEW: plasma store + actions
frontend/src/lib/stores/nav.ts            # MOD: add 'plasma' view
frontend/src/routes/Plasma.svelte         # NEW: fuse form + fusion list
frontend/src/routes/Plasma.test.ts        # NEW
frontend/src/routes/Dashboard.svelte      # MOD: link to Plasma
frontend/src/App.svelte                   # MOD: route 'plasma'
```

---

## Task 1: TxService generic contract-call path

**Files:** Modify `app/tx_service.go`, `app/tx_service_test.go`, `app/dto.go`.

**Interfaces:**
- Consumes: `nom.AccountBlock`, `types.{Address,ZenonTokenStandard}`, `zenon.NewZenon`, `pow.PowStatus`, existing `guard()`/`signingKeyPair()`/`symbolFor()`/`parseRequest()`/`node.currentClient()`.
- Produces: unexported `callExpect{to types.Address; zts types.ZenonTokenStandard; amount *big.Int}`; field `pendingExpect callExpect` (replaces `pendingReq`); `assertMatches(b *nom.AccountBlock, e callExpect) error`; `prepareCall(template *nom.AccountBlock, expect callExpect, summary string) (CallPreview, error)`; `CallPreview` DTO.

- [ ] **Step 1: Write the failing test**

In `app/dto.go` add the DTO (so the test compiles):
```go
// CallPreview is the confirm-what-you-sign preview for an embedded-contract call,
// rendered from the built, signed block plus a human action summary.
type CallPreview struct {
	ToAddress  string `json:"toAddress"`
	Zts        string `json:"zts"`
	Symbol     string `json:"symbol"`
	Amount     string `json:"amount"`
	Hash       string `json:"hash"`
	Summary    string `json:"summary"`
	UsedPlasma uint64 `json:"usedPlasma"`
	Difficulty uint64 `json:"difficulty"`
	NeedsPoW   bool   `json:"needsPoW"`
}
```
Add to `app/tx_service_test.go`:
```go
func TestAssertMatches(t *testing.T) {
	to, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	other, _ := types.ParseAddress("z1qr4pexnnfaexqqz8nscjjcsajy5hdqfkgadvwx")
	e := callExpect{to: to, zts: types.QsrTokenStandard, amount: big.NewInt(100)}

	ok := &nom.AccountBlock{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(100)}
	if err := assertMatches(ok, e); err != nil {
		t.Fatalf("matching block should pass: %v", err)
	}
	for _, bad := range []*nom.AccountBlock{
		{ToAddress: other, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(100)},
		{ToAddress: to, TokenStandard: types.ZnnTokenStandard, Amount: big.NewInt(100)},
		{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(99)},
		{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: nil},
	} {
		if err := assertMatches(bad, e); err == nil {
			t.Fatalf("divergent block must be rejected: %+v", bad)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestAssertMatches -v`
Expected: FAIL — `assertMatches`/`callExpect` undefined.

- [ ] **Step 3: Implement**

In `app/tx_service.go`:
1. Replace the struct field `pendingReq SendRequest` with `pendingExpect callExpect`, and add the type:
```go
// callExpect captures the funds-moving effect a prepared block must match before
// it may be published (confirm-what-you-sign).
type callExpect struct {
	to     types.Address
	zts    types.ZenonTokenStandard
	amount *big.Int
}
```
2. Add the pure matcher:
```go
// assertMatches verifies a built block moves exactly the expected funds.
func assertMatches(b *nom.AccountBlock, e callExpect) error {
	if b.ToAddress != e.to || b.TokenStandard != e.zts || b.Amount == nil || e.amount == nil || b.Amount.Cmp(e.amount) != 0 {
		return errors.New("prepared block does not match the expected effect; not publishing")
	}
	return nil
}
```
3. In `PrepareSend`, replace `t.pendingReq = req` with `t.pendingExpect = callExpect{to: to, zts: zts, amount: amount}` (the `to, zts, amount` already come from `parseRequest(req)` earlier in the function).
4. In `ConfirmPublish`, replace the `req`-based re-derivation with `expect`:
```go
b, expect, pendingGen := t.pending, t.pendingExpect, t.pendingGen
```
and replace the inline mismatch block with:
```go
if err := assertMatches(b, expect); err != nil {
	t.clearPending()
	return "", err
}
```
(Remove the now-unused `parseRequest(req)` call inside ConfirmPublish.)
5. In `clearPending`, replace `t.pendingReq = SendRequest{}` with `t.pendingExpect = callExpect{}`.
6. Add the generic call path:
```go
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
	t.pendingExpect = expect
	t.pendingGen = gen
	t.mu.Unlock()
	return CallPreview{
		ToAddress:  built.ToAddress.String(),
		Zts:        built.TokenStandard.String(),
		Symbol:     t.symbolFor(built.TokenStandard.String()),
		Amount:     built.Amount.String(),
		Hash:       built.Hash.String(),
		Summary:    summary,
		UsedPlasma: built.FusedPlasma,
		Difficulty: built.Difficulty,
		NeedsPoW:   built.Difficulty > 0,
	}, nil
}
```

- [ ] **Step 4: Convert the existing ConfirmPublish tests**

In `app/tx_service_test.go`, the 5 tests that set `tx.pendingReq = SendRequest{...}` must instead set `tx.pendingExpect`. For each, replace the `pendingReq` line with a parsed `callExpect`. Example (apply the same shape to all 5 — `TestConfirmPublishRejectsTamperedBlock`, `…BlockedOnMainnet`, `…RejectsChainMismatch`, `…RejectsWhenLocked`, and any other referencing `pendingReq`):
```go
// was: tx.pendingReq = SendRequest{ToAddress: addr, Zts: types.ZnnTokenStandard.String(), Amount: "1"}
exTo, _ := types.ParseAddress(addr) // use the literal address each test already uses
tx.pendingExpect = callExpect{to: exTo, zts: types.ZnnTokenStandard, amount: big.NewInt(1)}
```
For `TestConfirmPublishRejectsTamperedBlock` the pending block's `ToAddress` already differs from the expect address, so `assertMatches` rejects it (same intent as before). Keep each test's other assertions (pending cleared/retained) unchanged.

- [ ] **Step 5: Run to verify pass + build**

Run: `go test ./app/ -run 'TestAssertMatches|TestConfirmPublish|TestLockClearsPending|TestPrepareSend' -v && go build ./...`
Expected: PASS (matcher + all converted ConfirmPublish tests).

- [ ] **Step 6: Commit**

```bash
git add app/tx_service.go app/tx_service_test.go app/dto.go
git commit -m "feat(app): generic contract-call path in TxService (prepareCall + unified re-assert)"
```

---

## Task 2: NomService reads

**Files:** Create `app/nom_service.go`; Modify `app/dto.go`, `app/app.go`; Test `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.PlasmaApi.Get/GetEntriesByAddress/GetPlasmaByQsr`, `client.LedgerApi.GetFrontierMomentum`, `node.currentClient()`, `wallet.activeAddress()`.
- Produces: `NomService{}` + `newNomService(node *NodeService, wallet *WalletService, tx *TxService) *NomService`; `GetPlasmaInfo() (PlasmaInfo, error)`; `GetFusionEntries() ([]FusionEntry, error)`; `EstimatePlasma(qsr string) (uint64, error)`; pure `fusionEntryDTO(e *embedded.FusionEntry, currentHeight uint64) FusionEntry`; DTOs `PlasmaInfo`, `FusionEntry`.

- [ ] **Step 1: Write the failing test**

In `app/dto.go` add:
```go
// PlasmaInfo is the active address's plasma snapshot.
type PlasmaInfo struct {
	QsrFused      string `json:"qsrFused"`
	CurrentPlasma uint64 `json:"currentPlasma"`
	MaxPlasma     uint64 `json:"maxPlasma"`
}

// FusionEntry is one QSR fusion. IsRevocable is derived (frontier >= expiration).
type FusionEntry struct {
	Id               string `json:"id"`
	Beneficiary      string `json:"beneficiary"`
	QsrAmount        string `json:"qsrAmount"`
	ExpirationHeight uint64 `json:"expirationHeight"`
	IsRevocable      bool   `json:"isRevocable"`
}
```
`app/nom_service_test.go`:
```go
package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestFusionEntryDTORevocable(t *testing.T) {
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	e := &embedded.FusionEntry{QsrAmount: big.NewInt(10_000_000_000), Beneficiary: addr, ExpirationHeight: 100, Id: id}

	// frontier below expiration → not revocable
	d := fusionEntryDTO(e, 50)
	if d.IsRevocable {
		t.Fatal("should not be revocable below expiration")
	}
	if d.Beneficiary != addr.String() || d.ExpirationHeight != 100 {
		t.Fatalf("bad mapping: %+v", d)
	}
	// frontier at/above expiration → revocable
	if !fusionEntryDTO(e, 100).IsRevocable {
		t.Fatal("should be revocable at expiration")
	}
	if !fusionEntryDTO(e, 150).IsRevocable {
		t.Fatal("should be revocable above expiration")
	}
}
```
(Use the exact `types.HexToHashPanic` if available; otherwise `h, _ := types.HexToHash(...)`. Verify the helper name at implementation time.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestFusionEntryDTO -v`
Expected: FAIL — `fusionEntryDTO` undefined.

- [ ] **Step 3: Implement**

`app/nom_service.go`:
```go
package app

import (
	"errors"
	"math/big"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
)

// NomService exposes Network-of-Momentum embedded-contract reads and builds
// state-changing templates that it hands to TxService for confirm/publish.
// No key material passes through NomService.
type NomService struct {
	node   *NodeService
	wallet *WalletService
	tx     *TxService
}

func newNomService(node *NodeService, wallet *WalletService, tx *TxService) *NomService {
	return &NomService{node: node, wallet: wallet, tx: tx}
}

// GetPlasmaInfo returns the active address's plasma snapshot.
func (s *NomService) GetPlasmaInfo() (PlasmaInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return PlasmaInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return PlasmaInfo{}, errLocked
	}
	info, err := client.PlasmaApi.Get(addr)
	if err != nil {
		return PlasmaInfo{}, err
	}
	qsr := "0"
	if info.QsrAmount != nil {
		qsr = info.QsrAmount.String()
	}
	return PlasmaInfo{QsrFused: qsr, CurrentPlasma: info.CurrentPlasma, MaxPlasma: info.MaxPlasma}, nil
}

// GetFusionEntries returns the active address's fusion entries with derived revocability.
func (s *NomService) GetFusionEntries() ([]FusionEntry, error) {
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	list, err := client.PlasmaApi.GetEntriesByAddress(addr, 0, 50)
	if err != nil {
		return nil, err
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	out := []FusionEntry{}
	for _, e := range list.List {
		out = append(out, fusionEntryDTO(e, m.Height))
	}
	return out, nil
}

// EstimatePlasma returns the plasma a QSR amount would yield (pure SDK helper).
func (s *NomService) EstimatePlasma(qsr string) (uint64, error) {
	client := s.node.currentClient()
	if client == nil {
		return 0, errors.New("not connected")
	}
	amt, ok := new(big.Int).SetString(qsr, 10)
	if !ok || amt.Sign() < 0 {
		return 0, errors.New("invalid qsr amount")
	}
	return client.PlasmaApi.GetPlasmaByQsr(amt).Uint64(), nil
}

// fusionEntryDTO maps an SDK FusionEntry, deriving revocability from the frontier height.
func fusionEntryDTO(e *embedded.FusionEntry, currentHeight uint64) FusionEntry {
	qsr := "0"
	if e.QsrAmount != nil {
		qsr = e.QsrAmount.String()
	}
	return FusionEntry{
		Id:               e.Id.String(),
		Beneficiary:      e.Beneficiary.String(),
		QsrAmount:        qsr,
		ExpirationHeight: e.ExpirationHeight,
		IsRevocable:      currentHeight >= e.ExpirationHeight,
	}
}
```
In `app/app.go` `New()`: construct `nom := newNomService(n, w, t)`, store it on `App` (add field `Nom *NomService`), and add it to `Bindings()`. Set its ctx in `OnStartup` if it needs one (it does not emit events, so ctx is optional — skip).

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run TestFusionEntryDTO -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go app/dto.go app/app.go
git commit -m "feat(app): NomService plasma reads (info, fusion entries, estimate)"
```

---

## Task 3: NomService actions (Fuse / Cancel)

**Files:** Modify `app/nom_service.go`, `app/nom_service_test.go`; create `app/nom_plasma_integration_test.go`.

**Interfaces:**
- Consumes: `client.PlasmaApi.Fuse(addr, amt)`/`Cancel(id)`, `tx.prepareCall`, `types.{ParseAddress,HexToHash,PlasmaContract,QsrTokenStandard}`.
- Produces: `PrepareFuse(beneficiary, qsrAmount string) (CallPreview, error)`; `PrepareCancelFuse(id string) (CallPreview, error)`.

- [ ] **Step 1: Write the failing test**

Add to `app/nom_service_test.go`:
```go
func TestPrepareFuseValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// Bad beneficiary and bad amount are rejected BEFORE any node/client use.
	if _, err := s.PrepareFuse("not-an-address", "100"); err == nil {
		t.Fatal("expected invalid beneficiary to be rejected")
	}
	if _, err := s.PrepareFuse("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", "0"); err == nil {
		t.Fatal("expected zero amount to be rejected")
	}
	if _, err := s.PrepareFuse("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", "abc"); err == nil {
		t.Fatal("expected non-numeric amount to be rejected")
	}
}

func TestPrepareCancelFuseValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareCancelFuse("not-a-hash"); err == nil {
		t.Fatal("expected invalid id to be rejected")
	}
}
```
(`newTestNode`/`newTestWalletService` exist from earlier phases. Validation must occur before touching the client so these run offline with no node.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestPrepareFuse|TestPrepareCancelFuse' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go` (and import `"fmt"`, `"github.com/zenon-network/go-zenon/common/types"`):
```go
// PrepareFuse builds a Fuse template for the beneficiary and hands it to TxService
// for confirm-what-you-sign. Inputs are validated before any node use.
func (s *NomService) PrepareFuse(beneficiary, qsrAmount string) (CallPreview, error) {
	addr, err := types.ParseAddress(beneficiary)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid beneficiary: %w", err)
	}
	amt, ok := new(big.Int).SetString(qsrAmount, 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("invalid QSR amount")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PlasmaApi.Fuse(addr, amt)
	return s.tx.prepareCall(template, callExpect{to: types.PlasmaContract, zts: types.QsrTokenStandard, amount: amt},
		fmt.Sprintf("Fuse %s QSR for %s", qsrAmount, beneficiary))
}

// PrepareCancelFuse builds a Cancel template for a fusion id (no funds move; the
// fused QSR returns to the sender on confirmation).
func (s *NomService) PrepareCancelFuse(id string) (CallPreview, error) {
	hash, err := types.HexToHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid fusion id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PlasmaApi.Cancel(hash)
	return s.tx.prepareCall(template, callExpect{to: types.PlasmaContract, zts: types.QsrTokenStandard, amount: big.NewInt(0)},
		fmt.Sprintf("Cancel fusion %s", id))
}
```

- [ ] **Step 4: Add the integration test (opt-in)**

`app/nom_plasma_integration_test.go`:
```go
//go:build integration

package app

import (
	"testing"
)

// TestNomFuseCancelIntegration fuses a small amount of QSR on testnet and (if the
// resulting entry is immediately revocable) cancels it. Heavy + needs a funded
// testnet keystore in secrets/; opt-in.
func TestNomFuseCancelIntegration(t *testing.T) {
	t.Skip("manual: requires a funded testnet keystore and a configured node; wire via the spike harness when running Gate-5a")
}
```
(Keep it a documented skip placeholder; the real on-chain proof is the manual acceptance in Task 6. This keeps the integration build tag present without a brittle auto-funding flow.)

- [ ] **Step 5: Run to verify pass + build**

Run: `go test ./app/ -run 'TestPrepareFuse|TestPrepareCancelFuse' -v && go build ./... && go vet ./app/`
Expected: PASS; integration file compiles under `-tags integration`.

- [ ] **Step 6: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go app/nom_plasma_integration_test.go
git commit -m "feat(app): NomService Fuse/Cancel actions via prepareCall"
```

---

## Task 4: Bindings + plasma store + nav

**Files:** Modify `frontend/wailsjs/` (generated), `frontend/src/lib/stores/nav.ts`; Create `frontend/src/lib/stores/plasma.ts`.

**Interfaces:**
- Consumes: bound `NomService.GetPlasmaInfo`/`GetFusionEntries`/`EstimatePlasma`/`PrepareFuse`/`PrepareCancelFuse`; existing `tx` store + `TxModal`.
- Produces: `plasma` store (info + entries) + actions `refreshPlasma()`/`estimatePlasma(qsr)`; `nav` `'plasma'` view.

- [ ] **Step 1: Regenerate bindings**

```bash
"$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/ 2>/dev/null || true
ls frontend/wailsjs/go/app/NomService.d.ts   # the 5 methods present; PlasmaInfo/FusionEntry/CallPreview in models.ts
```
Revert any `frontend/wailsjs/runtime/*` churn.

- [ ] **Step 2: Add the plasma store + nav view**

`frontend/src/lib/stores/plasma.ts`:
```ts
import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'

export type PlasmaInfo = { qsrFused: string; currentPlasma: number; maxPlasma: number }
export type FusionEntry = { id: string; beneficiary: string; qsrAmount: string; expirationHeight: number; isRevocable: boolean }

export const plasmaInfo = writable<PlasmaInfo | null>(null)
export const fusionEntries = writable<FusionEntry[]>([])

export async function refreshPlasma(): Promise<void> {
  try {
    plasmaInfo.set((await Nom.GetPlasmaInfo()) as PlasmaInfo)
    fusionEntries.set((await Nom.GetFusionEntries()) as FusionEntry[])
  } catch { /* not connected / locked — leave as-is */ }
}
export async function estimatePlasma(qsr: string): Promise<number> {
  try { return (await Nom.EstimatePlasma(qsr)) as number } catch { return 0 }
}
```
Add `'plasma'` to the `View` union in `frontend/src/lib/stores/nav.ts`.

- [ ] **Step 3: Build to verify**

Run: `cd frontend && pnpm run build`
Expected: clean (clean `pnpm install` first if node_modules is stale).

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs frontend/src/lib/stores/plasma.ts frontend/src/lib/stores/nav.ts
git commit -m "feat(frontend): plasma bindings + store + nav view"
```

---

## Task 5: Plasma route UI

**Files:** Create `frontend/src/routes/Plasma.svelte`, `frontend/src/routes/Plasma.test.ts`; Modify `frontend/src/routes/Dashboard.svelte`, `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: `plasma` store, `NomService.PrepareFuse`/`PrepareCancelFuse`, the `tx` store + `TxModal`/`TxResult`, `nav`.
- Produces: Plasma route (fuse form + estimate + fusion list with Cancel) + dashboard link + App route.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/Plasma.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetPlasmaInfo: vi.fn().mockResolvedValue({ qsrFused: '0', currentPlasma: 0, maxPlasma: 0 }),
  GetFusionEntries: vi.fn().mockResolvedValue([
    { id: 'abc', beneficiary: 'z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz', qsrAmount: '10000000000', expirationHeight: 100, isRevocable: false },
  ]),
  EstimatePlasma: vi.fn().mockResolvedValue(21000),
  PrepareFuse: vi.fn().mockResolvedValue({ toAddress: 'z1qxemdedded', zts: 'zts1qsr', symbol: 'QSR', amount: '10000000000', hash: 'h', summary: 'Fuse', usedPlasma: 0, difficulty: 0, needsPoW: false }),
  PrepareCancelFuse: vi.fn().mockResolvedValue({ toAddress: 'z1qxemdedded', zts: 'zts1qsr', symbol: 'QSR', amount: '0', hash: 'h', summary: 'Cancel', usedPlasma: 0, difficulty: 0, needsPoW: false }),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Plasma from './Plasma.svelte'

describe('Plasma', () => {
  it('disables Cancel for a non-revocable entry', async () => {
    render(Plasma)
    const btn = await screen.findByRole('button', { name: /cancel fusion/i })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Plasma.test.ts`
Expected: FAIL — cannot resolve `./Plasma.svelte`.

- [ ] **Step 3: Implement**

`frontend/src/routes/Plasma.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { plasmaInfo, fusionEntries, refreshPlasma, estimatePlasma } from '../lib/stores/plasma'
  import { tx, confirmPublish } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let beneficiary = ''
  let amount = ''
  let estimate = 0
  let error = ''

  onMount(refreshPlasma)
  $: if (amount) estimatePlasma(toBase(amount)).then((p) => (estimate = p)); else estimate = 0

  // QSR has 8 decimals; convert a decimal string to base units (exact BigInt).
  function toBase(v: string): string {
    const [whole, frac = ''] = v.trim().split('.')
    const f = (frac + '00000000').slice(0, 8)
    try { return (BigInt(whole || '0') * 100000000n + BigInt(f || '0')).toString() } catch { return '0' }
  }

  async function fuse() {
    error = ''
    try { await Nom.PrepareFuse(beneficiary, toBase(amount)) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function cancel(id: string) {
    error = ''
    try { await Nom.PrepareCancelFuse(id) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function onConfirm() { await confirmPublish(); await refreshPlasma() }
</script>

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Plasma</h1>
    <button class="text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  {#if $plasmaInfo}
    <p class="text-sm text-muted">Current plasma {$plasmaInfo.currentPlasma} / {$plasmaInfo.maxPlasma} · QSR fused {$plasmaInfo.qsrFused}</p>
  {/if}

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Fuse QSR</h2>
    <input class="w-full rounded bg-bg px-3 py-2 font-mono text-sm" placeholder="beneficiary z1…" bind:value={beneficiary} aria-label="beneficiary" />
    <input class="w-full rounded bg-bg px-3 py-2" placeholder="QSR amount" bind:value={amount} aria-label="qsr amount" />
    {#if estimate > 0}<p class="text-xs text-muted">≈ {estimate} plasma</p>{/if}
    <button class="rounded bg-accent px-3 py-1 text-bg" on:click={fuse}>Fuse</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Fusion entries</h2>
    {#each $fusionEntries as e}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">{e.qsrAmount} QSR → {e.beneficiary.slice(0, 10)}…</span>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs disabled:opacity-40" disabled={!e.isRevocable} on:click={() => cancel(e.id)} aria-label="cancel fusion">Cancel</button>
      </div>
    {/each}
    {#if $fusionEntries.length === 0}<p class="text-xs text-muted">No fusion entries.</p>{/if}
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'awaiting-confirm' && $tx.preview}<TxModal preview={$tx.preview} on:confirm={onConfirm} />{/if}
  {#if $tx.status === 'done' || $tx.status === 'error'}<TxResult />{/if}
</div>
```
(Match the actual `tx` store shape/API from Phase 2 — the status values, `preview`, `confirmPublish`, and how `TxModal`/`TxResult` are driven. If the Phase-2 Send route wires these differently, mirror that exactly; the key behaviors are: PrepareFuse/PrepareCancelFuse populate the tx preview → TxModal → confirm → publish → refreshPlasma.)

In `Dashboard.svelte` add a "Plasma" button → `view.set('plasma')`. In `App.svelte` (unlocked branch) route `$view === 'plasma'` → `Plasma`.

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS; clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Plasma.svelte frontend/src/routes/Plasma.test.ts frontend/src/routes/Dashboard.svelte frontend/src/App.svelte
git commit -m "feat(frontend): Plasma route (fuse form + estimate + fusion list)"
```

---

## Task 6: Verification + acceptance

**Files:** Create `docs/phase5a-acceptance.md`.

- [ ] **Step 1: Full automated verification**

```bash
find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} + 2>/dev/null
go test ./...
cd frontend && pnpm test && pnpm run build && cd ..
xattr -cr build/bin 2>/dev/null; "$(go env GOPATH)/bin/wails" build
```
Expected: backend green, frontend green, app builds.

- [ ] **Step 2: Manual acceptance (Phase 5a gate)**

1. On testnet, open the Plasma route → see current plasma + QSR fused + fusion entries.
2. Fuse a small QSR amount for self → TxModal shows "Fuse … QSR for z1…" rendered from the built block → Confirm → published; after a momentum, plasma rises and a fusion entry appears.
3. Fuse for a different beneficiary → confirm it lands.
4. Once a fusion entry is revocable (its expiration height passed), Cancel it → QSR returns; entry disappears.
5. Confirm the mainnet guard: with `AllowMainnetSend` false on a mainnet node, PrepareFuse is blocked.

- [ ] **Step 3: Record the result**

`docs/phase5a-acceptance.md`: automated results + the manual checks (fuse self/other, plasma rise, cancel returns QSR, confirm-modal correctness, mainnet-gated), with the testnet tx hashes observed.

- [ ] **Step 4: Commit**

```bash
git add docs/phase5a-acceptance.md
git commit -m "docs: Phase 5a acceptance record"
```

---

## Self-Review

**Spec coverage:** generic `prepareCall` + unified `ConfirmPublish` re-assert + `CallPreview` (T1); NomService reads `GetPlasmaInfo`/`GetFusionEntries`/`EstimatePlasma` + `IsRevocable` derivation + DTOs + binding (T2); `PrepareFuse`/`PrepareCancelFuse` + validation + integration placeholder (T3); bindings + plasma store + nav (T4); Plasma route UI with fuse form/estimate/fusion list/Cancel-gating (T5); verification + manual acceptance (T6). All spec sections mapped.

**Placeholder scan:** No TBD/TODO in product code. The integration test is an explicit documented `t.Skip` (auto-funding a testnet fuse is brittle; manual acceptance is the real gate) — called out, not silent. Bindings regen is environment-run with the revert caution. T5 notes mirroring the exact Phase-2 `tx` store API rather than inventing one.

**Type consistency:** `callExpect{to,zts,amount}` + `assertMatches` + `prepareCall` + `CallPreview` consistent T1↔T3. `PlasmaInfo`/`FusionEntry` Go fields ↔ camelCase TS (`qsrFused/currentPlasma/maxPlasma`, `id/beneficiary/qsrAmount/expirationHeight/isRevocable`). `NomService` methods (`GetPlasmaInfo`/`GetFusionEntries`/`EstimatePlasma`/`PrepareFuse`/`PrepareCancelFuse`) match the TS store/bindings. `types.PlasmaContract`/`types.QsrTokenStandard` used in both the expect and (by the SDK) the built template, so `assertMatches` passes on a genuine Fuse block.

**Known follow-up (not 5a):** ABI-decode re-assertion of contract `Data`; staking/tokens/pillars/sentinels/accelerator reuse `prepareCall` in later sub-phases.
