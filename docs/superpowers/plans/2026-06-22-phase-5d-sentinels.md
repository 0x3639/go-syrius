# Phase 5d — Sentinels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** View the active address's sentinel status + escrowed QSR + uncollected rewards, deposit the QSR collateral, register a sentinel, collect rewards, revoke, and withdraw escrowed-but-unused QSR — reusing the 5a/5b/5c shared contract-call path.

**Architecture:** `NomService` gains sentinel reads (`GetSentinel`/`GetDepositedQsr`/`GetSentinelReward`) + five action builders (`PrepareDepositQsr`/`PrepareRegisterSentinel`/`PrepareCollectSentinelReward`/`PrepareRevokeSentinel`/`PrepareWithdrawQsr`) that build `SentinelApi` templates and delegate to `TxService.prepareCall` (confirm-what-you-sign, mainnet-gated). A guided Sentinels route drives the shared `tx` flow. No new backend pipeline — 5a's `prepareCall`/`assertMatches` is reused unchanged.

**Tech Stack:** Go 1.24+, `znn-sdk-go` (`SentinelApi`), `go-zenon/common/types`, Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- Templates via `client.SentinelApi.{DepositQsr,Register,Revoke,WithdrawQsr,CollectReward}`; publish via the existing `prepareCall`. No SDK/go-zenon forks.
- **Token-standard split is correctness-critical (the 5a "Cancel=ZNN" lesson):** `DepositQsr` uses `types.QsrTokenStandard`; `Register`/`Revoke`/`WithdrawQsr`/`CollectReward` all use `types.ZnnTokenStandard`. All use `types.SentinelContract`. A regression test locks each against the real SDK templates.
- **Amounts:** `Register` amount is read from the real template (`SentinelZnnRegisterAmount` = 5,000 ZNN = base `500000000000`), never hardcoded by us. `Revoke`/`WithdrawQsr`/`CollectReward` are Amount 0. `DepositQsr` amount is a caller-supplied base-unit value, validated `> 0`.
- `prepareCall` binds to/zts/amount AND ABI `Data`; mainnet stays behind `AllowMainnetSend`; no key material in NomService; inputs validated in Go before any node use.
- `GOWORK=off go test ./...` offline (a parent `/Users/dfriestedt/Github/go.work` references a missing module; bare `go test` fails to load the workspace — always prefix Go/wails commands with `GOWORK=off`). Frontend `pnpm test` + `pnpm run check` + `pnpm run build` pass.
- **Node prerequisite for manual acceptance:** the connected node must expose the `embedded` RPC namespace. Automated tests are offline and unaffected.
- ENV HAZARD (iCloud repo): `" 2"` collision copies break builds (`find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} +`); stale `node_modules` (`rm -rf frontend/node_modules && pnpm install`); codesign xattrs (`xattr -cr build/bin`). Commits GPG-signed.

## File structure

```
app/dto.go                 # MOD: SentinelInfo DTO (reuse RewardInfo from 5b)
app/nom_service.go         # MOD: sentinel reads + actions + pure sentinelDTO mapper
app/nom_service_test.go    # MOD: mapper + validation + template-token-standard tests
internal/spike/readonly_integration_test.go  # MOD: TestReadOnlySentinels live read smoke
frontend/wailsjs/...       # regenerated bindings
frontend/src/lib/stores/sentinel.ts # NEW: sentinel store + refresh
frontend/src/lib/stores/nav.ts      # MOD: add 'sentinels' view
frontend/src/routes/Sentinels.svelte    # NEW: guided register + status/collect/revoke
frontend/src/routes/Sentinels.test.ts   # NEW
frontend/src/routes/Dashboard.svelte # MOD: link to Sentinels
frontend/src/App.svelte             # MOD: route 'sentinels'
```

---

## Task 1: NomService sentinel reads + DTO

**Files:** Modify `app/dto.go`, `app/nom_service.go`; Test `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.SentinelApi.GetByOwner/GetDepositedQsr/GetUncollectedReward`, `s.node.currentClient()`, `s.wallet.activeAddress()`, `errLocked`, existing `RewardInfo` (5b).
- Produces: DTO `SentinelInfo{Owner string; RegistrationTimestamp int64; IsRevocable bool; RevokeCooldown int64; Active bool}`; `GetSentinel() (SentinelInfo, error)`; `GetDepositedQsr() (string, error)`; `GetSentinelReward() (RewardInfo, error)`; pure `sentinelDTO(s *embedded.SentinelInfo) SentinelInfo`.

- [ ] **Step 1: Write the failing test**

In `app/dto.go` add (next to the other NoM DTOs):
```go
// SentinelInfo is the active address's sentinel. An empty Owner means the
// address has no sentinel.
type SentinelInfo struct {
	Owner                 string `json:"owner"`
	RegistrationTimestamp int64  `json:"registrationTimestamp"`
	IsRevocable           bool   `json:"isRevocable"`
	RevokeCooldown        int64  `json:"revokeCooldown"`
	Active                bool   `json:"active"`
}
```
Add to `app/nom_service_test.go`:
```go
func TestSentinelDTO(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	s := &embedded.SentinelInfo{
		Owner:                 owner,
		RegistrationTimestamp: 1718000000,
		IsRevocable:           true,
		RevokeCooldown:        0,
		Active:                true,
	}
	d := sentinelDTO(s)
	if d.Owner != owner.String() || d.RegistrationTimestamp != 1718000000 {
		t.Fatalf("bad mapping: %+v", d)
	}
	if !d.IsRevocable || !d.Active {
		t.Fatalf("bad flags: %+v", d)
	}
	// no sentinel: nil → empty Owner
	if sentinelDTO(nil).Owner != "" {
		t.Fatal("nil should map to empty Owner")
	}
	// no sentinel: zero RegistrationTimestamp → empty Owner (treated as none)
	if sentinelDTO(&embedded.SentinelInfo{Owner: owner}).Owner != "" {
		t.Fatal("zero RegistrationTimestamp should map to empty Owner")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `GOWORK=off go test ./app/ -run 'TestSentinelDTO' -v`
Expected: FAIL — `sentinelDTO` undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go` (the `embedded`, `errors`, `types` imports already exist from 5a–5c):
```go
// sentinelDTO maps an SDK SentinelInfo to the DTO. A nil result or a zero
// RegistrationTimestamp means the address has no sentinel (empty Owner).
func sentinelDTO(s *embedded.SentinelInfo) SentinelInfo {
	if s == nil || s.RegistrationTimestamp == 0 {
		return SentinelInfo{}
	}
	return SentinelInfo{
		Owner:                 s.Owner.String(),
		RegistrationTimestamp: s.RegistrationTimestamp,
		IsRevocable:           s.IsRevocable,
		RevokeCooldown:        s.RevokeCooldown,
		Active:                s.Active,
	}
}

// GetSentinel returns the active address's sentinel (empty Owner = none).
func (s *NomService) GetSentinel() (SentinelInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return SentinelInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return SentinelInfo{}, errLocked
	}
	info, err := client.SentinelApi.GetByOwner(addr)
	if err != nil {
		return SentinelInfo{}, err
	}
	return sentinelDTO(info), nil
}

// GetDepositedQsr returns the active address's QSR escrowed toward registration
// (base-unit decimal string; "0" if none).
func (s *NomService) GetDepositedQsr() (string, error) {
	client := s.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return "", errLocked
	}
	q, err := client.SentinelApi.GetDepositedQsr(addr)
	if err != nil {
		return "", err
	}
	if q == nil {
		return "0", nil
	}
	return q.String(), nil
}

// GetSentinelReward returns the active address's uncollected sentinel reward.
func (s *NomService) GetSentinelReward() (RewardInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return RewardInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return RewardInfo{}, errLocked
	}
	r, err := client.SentinelApi.GetUncollectedReward(addr)
	if err != nil {
		return RewardInfo{}, err
	}
	znn, qsr := "0", "0"
	if r.ZnnAmount != nil {
		znn = r.ZnnAmount.String()
	}
	if r.QsrAmount != nil {
		qsr = r.QsrAmount.String()
	}
	return RewardInfo{Znn: znn, Qsr: qsr}, nil
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `GOWORK=off go test ./app/ -run 'TestSentinelDTO' -v && GOWORK=off go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/dto.go app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService sentinel reads (status, deposited QSR, reward)"
```

---

## Task 2: NomService sentinel actions

**Files:** Modify `app/nom_service.go`, `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.SentinelApi.{DepositQsr,Register,Revoke,WithdrawQsr,CollectReward}`, `tx.prepareCall`, `callExpect`, `types.{SentinelContract, ZnnTokenStandard, QsrTokenStandard}`, `embedded.NewSentinelApi`, `nom.AccountBlock`, `formatBaseAmount`, `new(big.Int).SetString`.
- Produces: `PrepareDepositQsr(qsr string) (CallPreview, error)`; `PrepareRegisterSentinel() (CallPreview, error)`; `PrepareCollectSentinelReward() (CallPreview, error)`; `PrepareRevokeSentinel() (CallPreview, error)`; `PrepareWithdrawQsr() (CallPreview, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/nom_service_test.go`:
```go
func TestPrepareDepositQsrValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// zero / negative / unparseable rejected before any node use.
	for _, bad := range []string{"0", "-1", "", "abc"} {
		if _, err := s.PrepareDepositQsr(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestSentinelTemplateTokenStandards(t *testing.T) {
	api := embedded.NewSentinelApi(nil) // builders construct blocks from args/constants; no client deref
	znn := types.ZnnTokenStandard.String()
	qsr := types.QsrTokenStandard.String()
	cases := []struct {
		name     string
		b        *nom.AccountBlock
		wantZts  string
		wantZero bool // Amount must be exactly 0
	}{
		{"deposit", api.DepositQsr(big.NewInt(123)), qsr, false},
		{"register", api.Register(), znn, false},
		{"revoke", api.Revoke(), znn, true},
		{"withdraw", api.WithdrawQsr(), znn, true},
		{"collect", api.CollectReward(), znn, true},
	}
	for _, c := range cases {
		if c.b.ToAddress != types.SentinelContract {
			t.Fatalf("%s: ToAddress=%v want SentinelContract", c.name, c.b.ToAddress)
		}
		if c.b.TokenStandard.String() != c.wantZts {
			t.Fatalf("%s: TokenStandard=%v want %v", c.name, c.b.TokenStandard.String(), c.wantZts)
		}
		if c.wantZero && (c.b.Amount == nil || c.b.Amount.Sign() != 0) {
			t.Fatalf("%s: Amount=%v want 0", c.name, c.b.Amount)
		}
	}
	// Register must carry the 5,000 ZNN collateral (5000 * 1e8).
	if api.Register().Amount.String() != "500000000000" {
		t.Fatalf("register amount=%v want 500000000000", api.Register().Amount)
	}
}
```
(Confirm `embedded.NewSentinelApi(nil)`'s builders don't deref the client — mirrors 5a/5b/5c. They construct `*nom.AccountBlock` from args/constants only.)

- [ ] **Step 2: Run to verify failure**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareDepositQsr|TestSentinelTemplate' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go` (`"errors"`, `"fmt"`, `"math/big"`, `"strings"` already imported):
```go
// PrepareDepositQsr builds a DepositQsr template (escrows QSR toward sentinel
// registration). qsr is a base-unit decimal string, validated before any node use.
func (s *NomService) PrepareDepositQsr(qsr string) (CallPreview, error) {
	amt, ok := new(big.Int).SetString(strings.TrimSpace(qsr), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("deposit amount must be a positive QSR value")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.DepositQsr(amt)
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.QsrTokenStandard, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Deposit %s QSR for sentinel", formatBaseAmount(amt.String(), 8)))
}

// PrepareRegisterSentinel builds a Register template (sends the 5,000 ZNN
// collateral; requires 50,000 QSR already deposited). Amount is read from the
// SDK template, never hardcoded.
func (s *NomService) PrepareRegisterSentinel() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.Register()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		"Register sentinel (5,000 ZNN)")
}

// PrepareCollectSentinelReward builds a CollectReward template.
func (s *NomService) PrepareCollectSentinelReward() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.CollectReward()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Collect sentinel rewards")
}

// PrepareRevokeSentinel builds a Revoke template (returns the collateral after
// the cooldown).
func (s *NomService) PrepareRevokeSentinel() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.Revoke()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Revoke sentinel")
}

// PrepareWithdrawQsr builds a WithdrawQsr template (recovers escrowed QSR not
// consumed by registration).
func (s *NomService) PrepareWithdrawQsr() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.WithdrawQsr()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Withdraw deposited QSR")
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareDepositQsr|TestSentinelTemplate' -v && GOWORK=off go build ./... && GOWORK=off go vet ./app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService sentinel actions (deposit/register/collect/revoke/withdraw) via prepareCall"
```

---

## Task 3: Bindings + sentinel store + nav

**Files:** Modify `frontend/wailsjs/` (generated), `frontend/src/lib/stores/nav.ts`; Create `frontend/src/lib/stores/sentinel.ts`.

**Interfaces:**
- Consumes: bound `NomService.GetSentinel`/`GetDepositedQsr`/`GetSentinelReward`/`PrepareDepositQsr`/`PrepareRegisterSentinel`/`PrepareCollectSentinelReward`/`PrepareRevokeSentinel`/`PrepareWithdrawQsr`; generated `app.SentinelInfo`/`app.RewardInfo`.
- Produces: `sentinel` store (`sentinel` + `depositedQsr` + `sentinelReward` writables) + `refreshSentinel()`; `nav` `'sentinels'` view.

- [ ] **Step 1: Regenerate bindings**

```bash
GOWORK=off "$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/ 2>/dev/null || true
ls frontend/wailsjs/go/app/NomService.d.ts   # GetSentinel/GetDepositedQsr/GetSentinelReward/PrepareDepositQsr/PrepareRegisterSentinel/PrepareCollectSentinelReward/PrepareRevokeSentinel/PrepareWithdrawQsr present; SentinelInfo in models.ts
```
Revert any `frontend/wailsjs/runtime/*` churn.

- [ ] **Step 2: Add the sentinel store + nav view**

`frontend/src/lib/stores/sentinel.ts`:
```ts
import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const sentinel = writable<app.SentinelInfo | null>(null)
export const depositedQsr = writable<string>('0')
export const sentinelReward = writable<app.RewardInfo | null>(null)

export async function refreshSentinel(): Promise<void> {
  try {
    sentinel.set(await Nom.GetSentinel())
    depositedQsr.set(await Nom.GetDepositedQsr())
    sentinelReward.set(await Nom.GetSentinelReward())
  } catch { /* not connected / locked — leave as-is */ }
}
```
Add `'sentinels'` to the `View` union in `frontend/src/lib/stores/nav.ts` (currently `'dashboard' | 'send' | 'create' | 'import' | 'unlock' | 'settings' | 'plasma' | 'stake' | 'pillars'`).

- [ ] **Step 3: Build to verify**

Run: `cd frontend && pnpm run check && pnpm run build`
Expected: `svelte-check` 0 errors; clean build (run `rm -rf node_modules && pnpm install` first if node_modules is stale).

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs frontend/src/lib/stores/sentinel.ts frontend/src/lib/stores/nav.ts
git commit -m "feat(frontend): sentinel bindings + store + nav view"
```

---

## Task 4: Sentinels route UI

**Files:** Create `frontend/src/routes/Sentinels.svelte`, `frontend/src/routes/Sentinels.test.ts`; Modify `frontend/src/routes/Dashboard.svelte`, `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: `sentinel` store, `NomService.PrepareDepositQsr`/`PrepareRegisterSentinel`/`PrepareCollectSentinelReward`/`PrepareRevokeSentinel`/`PrepareWithdrawQsr`, the `tx` store + `awaitConfirm`/`TxModal`/`TxResult`, `formatAmount`, `nav`.
- Produces: Sentinels route (guided register card + active-sentinel status/collect/revoke) + dashboard link + App route.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/Sentinels.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

const mocks = {
  GetSentinel: vi.fn(),
  GetDepositedQsr: vi.fn(),
  GetSentinelReward: vi.fn(),
  PrepareDepositQsr: vi.fn(), PrepareRegisterSentinel: vi.fn(),
  PrepareCollectSentinelReward: vi.fn(), PrepareRevokeSentinel: vi.fn(), PrepareWithdrawQsr: vi.fn(),
}
vi.mock('../../wailsjs/go/app/NomService', () => mocks)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Sentinels from './Sentinels.svelte'

describe('Sentinels', () => {
  it('shows Deposit (not Register) when escrowed QSR is below 50,000', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: '', registrationTimestamp: 0, isRevocable: false, revokeCooldown: 0, active: false })
    mocks.GetDepositedQsr.mockResolvedValue('1000000000000') // 10,000 QSR < 50,000
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    expect(await screen.findByRole('button', { name: /deposit qsr/i })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /register sentinel/i })).toBeNull()
  })

  it('shows Register when escrowed QSR reaches 50,000', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: '', registrationTimestamp: 0, isRevocable: false, revokeCooldown: 0, active: false })
    mocks.GetDepositedQsr.mockResolvedValue('5000000000000') // exactly 50,000 QSR
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    expect(await screen.findByRole('button', { name: /register sentinel/i })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /deposit qsr/i })).toBeNull()
  })

  it('shows status + disabled Revoke (not yet revocable) for an active sentinel', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: 'z1qtest', registrationTimestamp: 1718000000, isRevocable: false, revokeCooldown: 100, active: true })
    mocks.GetDepositedQsr.mockResolvedValue('0')
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    const revoke = await screen.findByRole('button', { name: /revoke sentinel/i }) as HTMLButtonElement
    expect(revoke.disabled).toBe(true)
    expect(screen.queryByRole('button', { name: /register sentinel/i })).toBeNull()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Sentinels.test.ts`
Expected: FAIL — cannot resolve `./Sentinels.svelte`.

- [ ] **Step 3: Implement**

READ `frontend/src/routes/Pillars.svelte` first and mirror its exact tx-flow wiring (`awaitConfirm`, `$tx.status` blocks, `TxModal`/`TxResult`, refresh-on-done). `frontend/src/routes/Sentinels.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { sentinel, depositedQsr, sentinelReward, refreshSentinel } from '../lib/stores/sentinel'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  const QSR_REQUIRED = 5000000000000n // 50,000 QSR in base units (1e8)
  let error = ''

  onMount(refreshSentinel)
  $: active = !!$sentinel && $sentinel.owner !== ''
  $: deposited = BigInt($depositedQsr ?? '0')
  $: shortfall = QSR_REQUIRED > deposited ? QSR_REQUIRED - deposited : 0n
  $: rewardZero = !$sentinelReward || ($sentinelReward.znn === '0' && $sentinelReward.qsr === '0')
  $: if ($tx.status === 'done') refreshSentinel()

  async function depositQsr() {
    error = ''
    try { awaitConfirm((await Nom.PrepareDepositQsr(shortfall.toString())) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function register() {
    error = ''
    try { awaitConfirm((await Nom.PrepareRegisterSentinel()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function collect() {
    error = ''
    try { awaitConfirm((await Nom.PrepareCollectSentinelReward()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function revoke() {
    error = ''
    try { awaitConfirm((await Nom.PrepareRevokeSentinel()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function withdrawQsr() {
    error = ''
    try { awaitConfirm((await Nom.PrepareWithdrawQsr()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Sentinels</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  {#if active}
    <section class="rounded bg-surface p-4 space-y-2">
      <h2 class="text-sm text-muted">Your sentinel</h2>
      <p class="text-sm">Status: {$sentinel.active ? 'Active' : 'Inactive'}</p>
      {#if $sentinelReward}<p class="text-sm">Uncollected reward {formatAmount($sentinelReward.znn, 8)} ZNN · {formatAmount($sentinelReward.qsr, 8)} QSR</p>{/if}
      <div class="flex gap-2">
        <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-40" disabled={rewardZero} on:click={collect}>Collect</button>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs disabled:opacity-40" disabled={!$sentinel.isRevocable} on:click={revoke} aria-label="revoke sentinel">Revoke{#if !$sentinel.isRevocable} (cooldown {$sentinel.revokeCooldown}s){/if}</button>
      </div>
    </section>
  {:else}
    <section class="rounded bg-surface p-4 space-y-2">
      <h2 class="text-sm text-muted">Register a Sentinel</h2>
      <p class="text-xs text-muted">Requires 50,000 QSR + 5,000 ZNN collateral (returned on revocation).</p>
      {#if deposited < QSR_REQUIRED}
        <p class="text-sm">Deposited {formatAmount($depositedQsr, 8)} / 50,000 QSR</p>
        <button class="rounded bg-accent px-3 py-1 text-bg" on:click={depositQsr} aria-label="deposit qsr">Deposit {formatAmount(shortfall.toString(), 8)} QSR</button>
        {#if deposited > 0n}
          <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={withdrawQsr} aria-label="withdraw qsr">Withdraw deposited QSR</button>
        {/if}
      {:else}
        <p class="text-sm">50,000 QSR deposited. Ready to register.</p>
        <button class="rounded bg-accent px-3 py-1 text-bg" on:click={register} aria-label="register sentinel">Register Sentinel (5,000 ZNN)</button>
      {/if}
    </section>
  {/if}

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
```
(If Pillars.svelte wires `TxModal`/`TxResult` or the tx status values differently, match Pillars exactly — it is the working reference.)

In `Dashboard.svelte` add a "Sentinels" button → `view.set('sentinels')` (next to the existing "Pillars" button at line ~51). In `App.svelte` import `Sentinels` and add an `{:else if $view === 'sentinels'}<Sentinels />` branch (next to the `'pillars'` branch).

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run check && pnpm run build`
Expected: full suite PASS; `svelte-check` 0 errors; clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Sentinels.svelte frontend/src/routes/Sentinels.test.ts frontend/src/routes/Dashboard.svelte frontend/src/App.svelte
git commit -m "feat(frontend): Sentinels route (guided register + status/collect/revoke)"
```

---

## Task 5: Verification + live smoke + acceptance

**Files:** Modify `internal/spike/readonly_integration_test.go`; Create `docs/phase5d-acceptance.md`.

- [ ] **Step 1: Add the live read smoke**

Add to `internal/spike/readonly_integration_test.go` (same `//go:build integration` tag; `rpc_client`, `types`, `os`, `testing` already imported):
```go
// TestReadOnlySentinels exercises the Phase-5d sentinel read path against a live
// node (proves the embedded namespace + the exact SentinelApi calls NomService
// uses). Read-only: no PoW, no signing.
//
// Env:
//   ZNN_NODE_URL  — ws:// or wss:// node URL (required)
//   ZNN_TEST_ADDR — a z1… address (required)
func TestReadOnlySentinels(t *testing.T) {
	url := os.Getenv("ZNN_NODE_URL")
	addrStr := os.Getenv("ZNN_TEST_ADDR")
	if url == "" || addrStr == "" {
		t.Skip("set ZNN_NODE_URL and ZNN_TEST_ADDR to run")
	}
	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}
	defer client.Stop()
	addr := types.ParseAddressPanic(addrStr)

	info, err := client.SentinelApi.GetByOwner(addr)
	if err != nil {
		t.Fatalf("GetByOwner (embedded namespace enabled?): %v", err)
	}
	t.Logf("sentinel: registrationTimestamp=%d active=%v isRevocable=%v cooldown=%d", info.RegistrationTimestamp, info.Active, info.IsRevocable, info.RevokeCooldown)

	q, err := client.SentinelApi.GetDepositedQsr(addr)
	if err != nil {
		t.Fatalf("GetDepositedQsr: %v", err)
	}
	t.Logf("deposited QSR: %v", q)

	r, err := client.SentinelApi.GetUncollectedReward(addr)
	if err != nil {
		t.Fatalf("GetUncollectedReward: %v", err)
	}
	t.Logf("uncollected sentinel reward: znn=%v qsr=%v", r.ZnnAmount, r.QsrAmount)
}
```

- [ ] **Step 2: Full automated verification**

```bash
find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} + 2>/dev/null
GOWORK=off go test ./...
GOWORK=off go build -tags integration ./...   # opt-in integration test still compiles
cd frontend && pnpm test && pnpm run check && pnpm run build && cd ..
xattr -cr build/bin 2>/dev/null; GOWORK=off "$(go env GOPATH)/bin/wails" build
```
Expected: backend green, frontend green (`svelte-check` 0 errors), app builds.

- [ ] **Step 3: Live read smoke (if a testnet node is available)**

```bash
ZNN_NODE_URL=ws://172.245.236.40:35998 ZNN_TEST_ADDR=<z1…> \
  GOWORK=off go test -tags integration ./internal/spike -run TestReadOnlySentinels -v -count=1
```
Expected: PASS; logs sentinel status + deposited QSR + uncollected reward. Record the output.

- [ ] **Step 4: Manual acceptance (Phase 5d gate)**

On a testnet node **with the `embedded` namespace enabled** (Sentinels route):
1. Open the Sentinels route → see "Register a Sentinel" (or your active sentinel) + escrowed QSR + uncollected reward.
2. Deposit QSR → TxModal shows "Deposit … QSR for sentinel" (zts = QSR) → Confirm → after a momentum the deposited total advances; once ≥ 50,000 the Register button appears.
3. Register → TxModal shows "Register sentinel (5,000 ZNN)" (zts = ZNN, amount 5,000) → Confirm → the sentinel shows as active.
4. Collect rewards (when uncollected > 0) → reward arrives.
5. Revoke (after the 27-day cooldown, when IsRevocable) → collateral returns.
6. WithdrawQsr (escrowed-but-unregistered) → QSR returns.
7. Mainnet guard: with `AllowMainnetSend` false on a mainnet node, the actions are blocked.

- [ ] **Step 5: Record the result**

`docs/phase5d-acceptance.md`: automated results + live smoke output + the manual checks (deposit/register/collect/revoke/withdraw, guided-flow correctness, confirm-modal token-standard correctness — Deposit=QSR vs others=ZNN, mainnet-gated), with testnet tx hashes where captured. Note the `embedded`-namespace node prerequisite (as in 5b/5c). Mirror the structure of `docs/phase5c-acceptance.md`.

- [ ] **Step 6: Commit**

```bash
git add internal/spike/readonly_integration_test.go docs/phase5d-acceptance.md
git commit -m "docs: Phase 5d acceptance record (+ live sentinel read smoke)"
```

---

## Self-Review

**Spec coverage:** sentinel reads + DTO + empty-Owner mapping (T1); Deposit/Register/Collect/Revoke/Withdraw actions + deposit validation + per-call token-standard regression test incl. Register-amount lock (T2); bindings + store + nav (T3); guided register flow (deposit-vs-register threshold + withdraw escape hatch) + active-sentinel status/collect/revoke route + Dashboard/App wiring (T4); automated verification + live read smoke + manual acceptance (T5). All spec sections mapped.

**Placeholder scan:** No TBD/TODO in product code. All steps carry full code. Bindings regen is environment-run with the revert caution. The `<z1…>` in T5 step 3 is a runtime input (the user's test address), not a code placeholder.

**Type consistency:** Go `SentinelInfo` fields ↔ camelCase TS (`owner/registrationTimestamp/isRevocable/revokeCooldown/active`); reuses `RewardInfo` (`znn/qsr`). `PrepareDepositQsr(qsr)`/`PrepareRegisterSentinel()`/`PrepareCollectSentinelReward()`/`PrepareRevokeSentinel()`/`PrepareWithdrawQsr()` match the TS store/route calls. `callExpect{to: SentinelContract, zts: QsrTokenStandard|ZnnTokenStandard, amount, data}` matches the real SDK templates (T2 regression test locks token standards + Register amount). Threshold `QSR_REQUIRED = 5000000000000n` (50,000 × 1e8) consistent between route and tests. Reuses `prepareCall`/`assertMatches`/`awaitConfirm`/`formatAmount`/`formatBaseAmount` from 5a–5c unchanged.

**Known follow-up (not 5d):** reward-history list (`GetFrontierRewardByPage`); active-sentinel browser (`GetAllActive`); tokens/accelerator in later sub-phases; deeper ABI `Data` semantic decode remains the Phase-5/7 hardening note.
