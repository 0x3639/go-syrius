# Phase 5b — Staking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** View stakes + uncollected rewards, stake ZNN for 1–12 months, cancel a matured stake, and collect QSR rewards — reusing the 5a shared contract-call path.

**Architecture:** `NomService` gains stake reads (`GetStakeList`/`GetUncollectedReward`) + three action builders (`PrepareStake`/`PrepareCancelStake`/`PrepareCollectReward`) that build `StakeApi` templates and delegate to `TxService.prepareCall` (confirm-what-you-sign, mainnet-gated). A Stake route drives the shared `tx` flow. No new backend pipeline — 5a's `prepareCall`/`assertMatches` is reused unchanged.

**Tech Stack:** Go 1.24+, `znn-sdk-go` (`StakeApi`), `go-zenon/common/types` + `vm/constants`, Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- Templates via `client.StakeApi.Stake(durationInSec, amount)` / `Cancel(id)` / `CollectReward()`; publish via the existing `prepareCall`. No SDK/go-zenon forks.
- All three stake calls use `types.ZnnTokenStandard` and `types.StakeContract` (Stake moves ZNN; Cancel/Collect move 0 ZNN) — verified; a regression test locks this against the real SDK templates.
- Duration: integer **1–12** months; 1 month = `StakeTimeUnitSec = 30*86400 = 2_592_000` seconds. Min stake `StakeMinAmount = 100_000_000` base units (1 ZNN, 8 decimals).
- Maturity derived from chain time: `IsMatured = frontierMomentumUnixSeconds >= ExpirationTimestamp` (timestamp, not height).
- `prepareCall` binds to/zts/amount AND ABI `Data`; mainnet stays behind `AllowMainnetSend`; no key material in NomService; inputs validated in Go before any node use.
- `go test ./...` offline (testnet stake/cancel/collect is an opt-in integration skip). Frontend `pnpm test` + `pnpm run build` pass.
- ENV HAZARD (iCloud repo): `" 2"` collision copies break builds (`find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} +`); `node_modules` eviction (`rm -rf frontend/node_modules && pnpm install`); codesign xattrs (`xattr -cr build/bin`). Commits GPG-signed.

## File structure

```
app/dto.go                 # MOD: StakeInfo, StakeEntry, RewardInfo DTOs
app/nom_service.go         # MOD: stake reads + actions + pure stakeEntryDTO mapper
app/nom_service_test.go     # MOD: mapper + validation + template-token-standard tests
app/app.go                 # (unchanged — NomService already bound)
frontend/wailsjs/...       # regenerated bindings
frontend/src/lib/stores/stake.ts      # NEW: stake store + refresh
frontend/src/lib/stores/nav.ts         # MOD: add 'stake' view
frontend/src/routes/Stake.svelte       # NEW: stake form + stakes list + reward/collect
frontend/src/routes/Stake.test.ts      # NEW
frontend/src/routes/Dashboard.svelte   # MOD: link to Stake
frontend/src/App.svelte                # MOD: route 'stake'
```

---

## Task 1: NomService stake reads + DTOs

**Files:** Modify `app/dto.go`, `app/nom_service.go`; Test `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.StakeApi.GetEntriesByAddress/GetUncollectedReward`, `client.LedgerApi.GetFrontierMomentum`, `node.currentClient()`, `wallet.activeAddress()`, `errLocked`, the existing `formatBaseAmount`.
- Produces: DTOs `StakeInfo{TotalAmount string; Entries []StakeEntry}`, `StakeEntry{Id, Amount string; StartTimestamp, ExpirationTimestamp int64; DurationMonths int; IsMatured bool}`, `RewardInfo{Znn, Qsr string}`; `GetStakeList() (StakeInfo, error)`; `GetUncollectedReward() (RewardInfo, error)`; pure `stakeEntryDTO(e *embedded.StakeEntry, nowUnix int64) StakeEntry`; `const stakeTimeUnitSec int64 = 2_592_000`.

- [ ] **Step 1: Write the failing test**

In `app/dto.go` add:
```go
// StakeInfo is the active address's stake snapshot.
type StakeInfo struct {
	TotalAmount string       `json:"totalAmount"`
	Entries     []StakeEntry `json:"entries"`
}

// StakeEntry is one ZNN stake; IsMatured is derived (frontier time >= expiration).
type StakeEntry struct {
	Id                  string `json:"id"`
	Amount              string `json:"amount"`
	StartTimestamp      int64  `json:"startTimestamp"`
	ExpirationTimestamp int64  `json:"expirationTimestamp"`
	DurationMonths      int    `json:"durationMonths"`
	IsMatured           bool   `json:"isMatured"`
}

// RewardInfo is uncollected reward (base-unit decimal strings).
type RewardInfo struct {
	Znn string `json:"znn"`
	Qsr string `json:"qsr"`
}
```
Add to `app/nom_service_test.go`:
```go
func TestStakeEntryDTO(t *testing.T) {
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	start := int64(1_700_000_000)
	const unit = int64(2_592_000)
	// 3-month stake
	e := &embedded.StakeEntry{
		Amount:              big.NewInt(500_000_000), // 5 ZNN
		StartTimestamp:      start,
		ExpirationTimestamp: start + 3*unit,
		Address:             addr,
		Id:                  id,
	}
	// before expiration → not matured
	d := stakeEntryDTO(e, start+unit)
	if d.IsMatured {
		t.Fatal("should not be matured before expiration")
	}
	if d.DurationMonths != 3 {
		t.Fatalf("DurationMonths = %d, want 3", d.DurationMonths)
	}
	if d.Amount != "500000000" || d.Id != id.String() {
		t.Fatalf("bad mapping: %+v", d)
	}
	// at/after expiration → matured
	if !stakeEntryDTO(e, start+3*unit).IsMatured {
		t.Fatal("should be matured at expiration")
	}
	if !stakeEntryDTO(e, start+10*unit).IsMatured {
		t.Fatal("should be matured after expiration")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestStakeEntryDTO -v`
Expected: FAIL — `stakeEntryDTO` undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go` (the `embedded` import + `formatBaseAmount` already exist from 5a):
```go
const stakeTimeUnitSec int64 = 2_592_000 // 30 days; go-zenon StakeTimeUnitSec

// stakeEntryDTO maps an SDK StakeEntry, deriving duration (months) and maturity
// from chain time (nowUnix = frontier momentum timestamp).
func stakeEntryDTO(e *embedded.StakeEntry, nowUnix int64) StakeEntry {
	amt := "0"
	if e.Amount != nil {
		amt = e.Amount.String()
	}
	months := 0
	if e.ExpirationTimestamp > e.StartTimestamp {
		months = int((e.ExpirationTimestamp - e.StartTimestamp) / stakeTimeUnitSec)
	}
	return StakeEntry{
		Id:                  e.Id.String(),
		Amount:              amt,
		StartTimestamp:      e.StartTimestamp,
		ExpirationTimestamp: e.ExpirationTimestamp,
		DurationMonths:      months,
		IsMatured:           nowUnix >= e.ExpirationTimestamp,
	}
}

// GetStakeList returns the active address's stakes with derived maturity.
func (s *NomService) GetStakeList() (StakeInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return StakeInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return StakeInfo{}, errLocked
	}
	list, err := client.StakeApi.GetEntriesByAddress(addr, 0, 50)
	if err != nil {
		return StakeInfo{}, err
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		return StakeInfo{}, err
	}
	now := frontierUnix(m) // unix seconds of the frontier momentum (see note)
	total := "0"
	if list.TotalAmount != nil {
		total = list.TotalAmount.String()
	}
	out := StakeInfo{TotalAmount: total, Entries: []StakeEntry{}}
	for _, e := range list.List {
		out.Entries = append(out.Entries, stakeEntryDTO(e, now))
	}
	return out, nil
}

// GetUncollectedReward returns the active address's uncollected stake reward.
func (s *NomService) GetUncollectedReward() (RewardInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return RewardInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return RewardInfo{}, errLocked
	}
	r, err := client.StakeApi.GetUncollectedReward(addr)
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
Add a small `frontierUnix` helper that extracts unix seconds from the SDK frontier momentum. **Confirm the exact field/type by reading the type returned by `client.LedgerApi.GetFrontierMomentum()`** (the same call NodeService already uses for `Height`/`ChainIdentifier`): the momentum carries a timestamp — if it is a `time.Time` (or `*time.Time`) field named `Timestamp`, return `m.Timestamp.Unix()`; if it is already a unix `int64`, return it directly. Implement `frontierUnix(m)` accordingly (one line). Do not guess — grep the SDK/go-zenon momentum type to get it right.

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run TestStakeEntryDTO -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/dto.go app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService stake reads (entries w/ maturity, uncollected reward)"
```

---

## Task 2: NomService stake actions

**Files:** Modify `app/nom_service.go`, `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.StakeApi.Stake(durationInSec int64, amt *big.Int)`/`Cancel(id types.Hash)`/`CollectReward()`, `tx.prepareCall`, `callExpect`, `types.{StakeContract, ZnnTokenStandard, ParseAddress, HexToHash}`, `formatBaseAmount`, `stakeTimeUnitSec`, `embedded.NewStakeApi`.
- Produces: `PrepareStake(amountZnn, durationMonths string) (CallPreview, error)`; `PrepareCancelStake(id string) (CallPreview, error)`; `PrepareCollectReward() (CallPreview, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/nom_service_test.go`:
```go
func TestPrepareStakeValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// amount below 1 ZNN min, non-numeric amount, and bad duration are rejected before any node use.
	if _, err := s.PrepareStake("50000000", "3"); err == nil { // 0.5 ZNN < 1 ZNN min
		t.Fatal("expected below-min amount to be rejected")
	}
	if _, err := s.PrepareStake("abc", "3"); err == nil {
		t.Fatal("expected non-numeric amount to be rejected")
	}
	if _, err := s.PrepareStake("100000000", "0"); err == nil {
		t.Fatal("expected duration 0 to be rejected")
	}
	if _, err := s.PrepareStake("100000000", "13"); err == nil {
		t.Fatal("expected duration 13 to be rejected")
	}
	if _, err := s.PrepareCancelStake("not-a-hash"); err == nil {
		t.Fatal("expected bad id to be rejected")
	}
}

func TestStakeTemplateTokenStandards(t *testing.T) {
	api := embedded.NewStakeApi(nil) // builders construct blocks from args; no client deref
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	for name, b := range map[string]*nom.AccountBlock{
		"stake":   api.Stake(stakeTimeUnitSec, big.NewInt(100_000_000)),
		"cancel":  api.Cancel(id),
		"collect": api.CollectReward(),
	} {
		if b.ToAddress != types.StakeContract {
			t.Fatalf("%s: ToAddress=%v want StakeContract", name, b.ToAddress)
		}
		if b.TokenStandard != types.ZnnTokenStandard {
			t.Fatalf("%s: TokenStandard=%v want ZNN", name, b.TokenStandard)
		}
	}
}
```
(Confirm `embedded.NewStakeApi` exists and its `Stake`/`Cancel`/`CollectReward` don't deref the client — mirror 5a's `NewPlasmaApi(nil)` test; if the constructor name differs, adjust. Ensure `nom` is imported in the test.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestPrepareStake|TestStakeTemplate' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go`:
```go
// PrepareStake builds a Stake template (ZNN for durationMonths*30 days) and hands
// it to TxService. Inputs validated before any node use.
func (s *NomService) PrepareStake(amountZnn, durationMonths string) (CallPreview, error) {
	amt, ok := new(big.Int).SetString(amountZnn, 10)
	if !ok || amt.Cmp(big.NewInt(100_000_000)) < 0 { // StakeMinAmount = 1 ZNN
		return CallPreview{}, errors.New("stake amount must be at least 1 ZNN")
	}
	months, err := strconv.Atoi(durationMonths)
	if err != nil || months < 1 || months > 12 {
		return CallPreview{}, errors.New("stake duration must be 1 to 12 months")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.StakeApi.Stake(int64(months)*stakeTimeUnitSec, amt)
	return s.tx.prepareCall(template,
		callExpect{to: types.StakeContract, zts: types.ZnnTokenStandard, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Stake %s ZNN for %d months", formatBaseAmount(amountZnn, 8), months))
}

// PrepareCancelStake builds a Cancel template for a matured stake id.
func (s *NomService) PrepareCancelStake(id string) (CallPreview, error) {
	hash, err := types.HexToHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid stake id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.StakeApi.Cancel(hash)
	return s.tx.prepareCall(template,
		callExpect{to: types.StakeContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Cancel stake %s", id))
}

// PrepareCollectReward builds a CollectReward template (claims accrued QSR).
func (s *NomService) PrepareCollectReward() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.StakeApi.CollectReward()
	return s.tx.prepareCall(template,
		callExpect{to: types.StakeContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Collect staking rewards")
}
```
Add imports `"strconv"` (and ensure `"fmt"`, `"math/big"`, `nom "github.com/zenon-network/go-zenon/chain/nom"` available where needed; `nom` is only needed in the test).

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run 'TestPrepareStake|TestStakeTemplate' -v && go build ./... && go vet ./app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService Stake/Cancel/CollectReward actions via prepareCall"
```

---

## Task 3: Bindings + stake store + nav

**Files:** Modify `frontend/wailsjs/` (generated), `frontend/src/lib/stores/nav.ts`; Create `frontend/src/lib/stores/stake.ts`.

**Interfaces:**
- Consumes: bound `NomService.GetStakeList`/`GetUncollectedReward`/`PrepareStake`/`PrepareCancelStake`/`PrepareCollectReward`.
- Produces: `stake` store (`stakeInfo` + `reward` writables) + `refreshStake()`; `nav` `'stake'` view.

- [ ] **Step 1: Regenerate bindings**

```bash
"$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/ 2>/dev/null || true
ls frontend/wailsjs/go/app/NomService.d.ts   # GetStakeList/GetUncollectedReward/PrepareStake/PrepareCancelStake/PrepareCollectReward present; StakeInfo/StakeEntry/RewardInfo in models.ts
```
Revert any `frontend/wailsjs/runtime/*` churn.

- [ ] **Step 2: Add the stake store + nav view**

`frontend/src/lib/stores/stake.ts`:
```ts
import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'

export type StakeEntry = { id: string; amount: string; startTimestamp: number; expirationTimestamp: number; durationMonths: number; isMatured: boolean }
export type StakeInfo = { totalAmount: string; entries: StakeEntry[] }
export type RewardInfo = { znn: string; qsr: string }

export const stakeInfo = writable<StakeInfo | null>(null)
export const reward = writable<RewardInfo | null>(null)

export async function refreshStake(): Promise<void> {
  try {
    stakeInfo.set((await Nom.GetStakeList()) as StakeInfo)
    reward.set((await Nom.GetUncollectedReward()) as RewardInfo)
  } catch { /* not connected / locked — leave as-is */ }
}
```
Add `'stake'` to the `View` union in `frontend/src/lib/stores/nav.ts`.

- [ ] **Step 3: Build to verify**

Run: `cd frontend && pnpm run build`
Expected: clean (clean `pnpm install` first if node_modules is stale).

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs frontend/src/lib/stores/stake.ts frontend/src/lib/stores/nav.ts
git commit -m "feat(frontend): stake bindings + store + nav view"
```

---

## Task 4: Stake route UI

**Files:** Create `frontend/src/routes/Stake.svelte`, `frontend/src/routes/Stake.test.ts`; Modify `frontend/src/routes/Dashboard.svelte`, `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: `stake` store, `NomService.PrepareStake`/`PrepareCancelStake`/`PrepareCollectReward`, the `tx` store + `awaitConfirm`/`TxModal`/`TxResult`, `formatAmount`, `nav`.
- Produces: Stake route (stake form + stakes list + reward/Collect) + dashboard link + App route.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/Stake.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetStakeList: vi.fn().mockResolvedValue({ totalAmount: '500000000', entries: [
    { id: 'abc', amount: '500000000', startTimestamp: 1, expirationTimestamp: 2, durationMonths: 3, isMatured: false },
  ] }),
  GetUncollectedReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  PrepareStake: vi.fn(), PrepareCancelStake: vi.fn(), PrepareCollectReward: vi.fn(),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Stake from './Stake.svelte'

describe('Stake', () => {
  it('disables Cancel for a non-matured stake', async () => {
    render(Stake)
    const btn = await screen.findByRole('button', { name: /cancel stake/i })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Stake.test.ts`
Expected: FAIL — cannot resolve `./Stake.svelte`.

- [ ] **Step 3: Implement**

READ `frontend/src/routes/Plasma.svelte` first and mirror its exact tx-flow wiring (`awaitConfirm`, `$tx.status`, `TxModal`/`TxResult`, `toBase`, refresh-on-done). `frontend/src/routes/Stake.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { stakeInfo, reward, refreshStake } from '../lib/stores/stake'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let amount = ''
  let months = '1'
  let error = ''

  onMount(refreshStake)
  $: if ($tx.status === 'done') refreshStake()
  $: rewardZero = !$reward || ($reward.znn === '0' && $reward.qsr === '0')

  // ZNN has 8 decimals.
  function toBase(v: string): string {
    const [whole, frac = ''] = v.trim().split('.')
    const f = (frac + '00000000').slice(0, 8)
    try { return (BigInt(whole || '0') * BigInt(100000000) + BigInt(f || '0')).toString() } catch { return '0' }
  }
  async function stake() {
    error = ''
    try { await awaitConfirm(await Nom.PrepareStake(toBase(amount), months)) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function cancel(id: string) {
    error = ''
    try { await awaitConfirm(await Nom.PrepareCancelStake(id)) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function collect() {
    error = ''
    try { await awaitConfirm(await Nom.PrepareCollectReward()) } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Staking</h1>
    <button class="text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  {#if $stakeInfo}<p class="text-sm text-muted">Total staked {formatAmount($stakeInfo.totalAmount, 8)} ZNN</p>{/if}

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Uncollected reward</h2>
    {#if $reward}<p class="text-sm">{formatAmount($reward.znn, 8)} ZNN · {formatAmount($reward.qsr, 8)} QSR</p>{/if}
    <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-40" disabled={rewardZero} on:click={collect}>Collect</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Stake ZNN</h2>
    <input class="w-full rounded bg-bg px-3 py-2" placeholder="ZNN amount (min 1)" bind:value={amount} aria-label="znn amount" />
    <label class="block text-sm text-muted">Duration
      <select class="mt-1 w-full rounded bg-bg px-3 py-2" bind:value={months} aria-label="duration months">
        {#each Array(12) as _, i}<option value={String(i + 1)}>{i + 1} month{i ? 's' : ''}</option>{/each}
      </select>
    </label>
    <button class="rounded bg-accent px-3 py-1 text-bg" on:click={stake}>Stake</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Your stakes</h2>
    {#each ($stakeInfo?.entries ?? []) as e}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">{formatAmount(e.amount, 8)} ZNN · {e.durationMonths}mo</span>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs disabled:opacity-40" disabled={!e.isMatured} on:click={() => cancel(e.id)} aria-label="cancel stake">Cancel</button>
      </div>
    {/each}
    {#if !$stakeInfo || $stakeInfo.entries.length === 0}<p class="text-xs text-muted">No active stakes.</p>{/if}
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done' || $tx.status === 'error'}<TxResult />{/if}
</div>
```
(If Plasma.svelte wires `TxModal`/`TxResult` or the tx status values differently, match Plasma exactly — it is the working reference.)

In `Dashboard.svelte` add a "Staking" button → `view.set('stake')`. In `App.svelte` (unlocked branch) route `$view === 'stake'` → `Stake`.

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS; clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Stake.svelte frontend/src/routes/Stake.test.ts frontend/src/routes/Dashboard.svelte frontend/src/App.svelte
git commit -m "feat(frontend): Stake route (stake form + stakes list + collect)"
```

---

## Task 5: Verification + acceptance

**Files:** Create `docs/phase5b-acceptance.md`.

- [ ] **Step 1: Full automated verification**

```bash
find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} + 2>/dev/null
go test ./...
cd frontend && pnpm test && pnpm run build && cd ..
xattr -cr build/bin 2>/dev/null; "$(go env GOPATH)/bin/wails" build
```
Expected: backend green, frontend green, app builds.

- [ ] **Step 2: Manual acceptance (Phase 5b gate)**

1. On testnet, open the Staking route → see total staked + uncollected reward + stakes.
2. Stake ≥1 ZNN for N months → TxModal shows "Stake X ZNN for N months" from the built block → Confirm → after a momentum the stake appears.
3. Collect rewards (when uncollected QSR > 0) → QSR arrives.
4. Cancel a matured stake → ZNN returns; the entry disappears.
5. Mainnet guard: with `AllowMainnetSend` false on a mainnet node, PrepareStake is blocked.

- [ ] **Step 3: Record the result**

`docs/phase5b-acceptance.md`: automated results + the manual checks (stake/collect/cancel, confirm-modal correctness, mainnet-gated), with testnet tx hashes.

- [ ] **Step 4: Commit**

```bash
git add docs/phase5b-acceptance.md
git commit -m "docs: Phase 5b acceptance record"
```

---

## Self-Review

**Spec coverage:** stake reads + DTOs + `IsMatured`/`DurationMonths` (T1); Stake/Cancel/Collect actions + validation + token-standard regression test (T2); bindings + store + nav (T3); Stake route UI with reward/Collect + stakes list + Cancel gating (T4); verification + manual acceptance (T5). All spec sections mapped.

**Placeholder scan:** No TBD/TODO in product code. The `frontierUnix` helper (T1 Step 3) is the one runtime detail isolated to a single line with an explicit "grep the momentum type, don't guess" instruction (the testable logic — `stakeEntryDTO` — is fully specified and unit-tested via an injected `nowUnix`). Integration is an opt-in documented skip (same as 5a). Bindings regen is environment-run with the revert caution.

**Type consistency:** `StakeInfo`/`StakeEntry`/`RewardInfo` Go fields ↔ camelCase TS (`totalAmount/entries`, `id/amount/startTimestamp/expirationTimestamp/durationMonths/isMatured`, `znn/qsr`). `PrepareStake`/`PrepareCancelStake`/`PrepareCollectReward` match the TS store/route calls. `callExpect{to: StakeContract, zts: ZnnTokenStandard, amount, data}` matches the real SDK templates (T2 regression test locks it). `stakeTimeUnitSec` consistent T1↔T2. Reuses `prepareCall`/`assertMatches`/`awaitConfirm`/`formatAmount`/`formatBaseAmount` from 5a unchanged.

**Known follow-up (not 5b):** reward-history list; tokens/pillars/sentinels/accelerator reuse the same path in later sub-phases; deeper ABI `Data` semantic decode remains the Phase-5/7 hardening note.
