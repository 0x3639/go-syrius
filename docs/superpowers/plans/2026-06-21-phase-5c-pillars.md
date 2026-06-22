# Phase 5c — Pillar Delegation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** View pillars + current delegation + uncollected delegation rewards, delegate to a pillar, undelegate, and collect rewards — reusing the 5a/5b shared contract-call path.

**Architecture:** `NomService` gains pillar reads (`GetPillarList`/`GetDelegation`/`GetPillarReward`) + three action builders (`PrepareDelegate`/`PrepareUndelegate`/`PrepareCollectPillarReward`) that build `PillarApi` templates and delegate to `TxService.prepareCall` (confirm-what-you-sign, mainnet-gated). A Pillars route drives the shared `tx` flow. No new backend pipeline — 5a's `prepareCall`/`assertMatches` is reused unchanged.

**Tech Stack:** Go 1.24+, `znn-sdk-go` (`PillarApi`), `go-zenon/common/types`, Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- Templates via `client.PillarApi.Delegate(name)` / `Undelegate()` / `CollectReward()`; publish via the existing `prepareCall`. No SDK/go-zenon forks.
- All three pillar calls use `types.ZnnTokenStandard` and `types.PillarContract`, **Amount 0** (delegation moves no funds; weight = the account's ZNN balance) — verified; a regression test locks this against the real SDK templates.
- Pillar name: non-empty (trimmed) validated in Go **before** any node use. `GetDelegatedPillar` may return nil / empty `Name` when not delegated → map to `DelegationInfo{}`.
- `prepareCall` binds to/zts/amount AND ABI `Data`; mainnet stays behind `AllowMainnetSend`; no key material in NomService; inputs validated in Go before any node use.
- `GOWORK=off go test ./...` offline (a parent `/Users/dfriestedt/Github/go.work` references a missing module; bare `go test` fails to load the workspace — always prefix Go/wails commands with `GOWORK=off`). Frontend `pnpm test` + `pnpm run build` pass.
- **Node prerequisite for manual acceptance:** the connected node must expose the `embedded` RPC namespace (go-zenon `RPC.Endpoints` whitelist). A node serving only `ledger` returns `embedded.* does not exist/is not available` for every read and PoW-requiring action. Automated tests are offline and unaffected.
- ENV HAZARD (iCloud repo, if applicable): `" 2"` collision copies break builds (`find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} +`); stale `node_modules` (`rm -rf frontend/node_modules && pnpm install`); codesign xattrs (`xattr -cr build/bin`). Commits GPG-signed.

## File structure

```
app/dto.go                 # MOD: PillarSummary, DelegationInfo DTOs (reuse RewardInfo from 5b)
app/nom_service.go         # MOD: pillar reads + actions + pure pillarSummaryDTO mapper + sort
app/nom_service_test.go    # MOD: mapper + sort + validation + template-token-standard tests
frontend/wailsjs/...       # regenerated bindings
frontend/src/lib/stores/pillar.ts   # NEW: pillar store + refresh
frontend/src/lib/stores/nav.ts      # MOD: add 'pillars' view
frontend/src/routes/Pillars.svelte  # NEW: current delegation + reward/collect + searchable pillar list
frontend/src/routes/Pillars.test.ts # NEW
frontend/src/routes/Dashboard.svelte # MOD: link to Pillars
frontend/src/App.svelte             # MOD: route 'pillars'
```

---

## Task 1: NomService pillar reads + DTOs

**Files:** Modify `app/dto.go`, `app/nom_service.go`; Test `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.PillarApi.GetAll/GetDelegatedPillar/GetUncollectedReward`, `node.currentClient()`, `wallet.activeAddress()`, `errLocked`, existing `RewardInfo` (5b).
- Produces: DTOs `PillarSummary{Name string; Rank int; Weight string; DelegateRewardPercent int; ProducerAddress string}`, `DelegationInfo{Name string; Status int; Weight string}`; `GetPillarList() ([]PillarSummary, error)`; `GetDelegation() (DelegationInfo, error)`; `GetPillarReward() (RewardInfo, error)`; pure `pillarSummaryDTO(p *embedded.PillarInfo) PillarSummary`.

- [ ] **Step 1: Write the failing test**

In `app/dto.go` add:
```go
// PillarSummary is one pillar in the delegation picker list.
type PillarSummary struct {
	Name                  string `json:"name"`
	Rank                  int    `json:"rank"`
	Weight                string `json:"weight"`
	DelegateRewardPercent int    `json:"delegateRewardPercent"`
	ProducerAddress       string `json:"producerAddress"`
}

// DelegationInfo is the active address's current pillar delegation.
// An empty Name means the address is not delegated.
type DelegationInfo struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
	Weight string `json:"weight"`
}
```
Add to `app/nom_service_test.go`:
```go
func TestPillarSummaryDTO(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	p := &embedded.PillarInfo{
		Name:                         "Pillar-A",
		Rank:                         3,
		GiveDelegateRewardPercentage: 90,
		ProducerAddress:              owner,
		Weight:                       big.NewInt(1_500_000_000_000),
	}
	d := pillarSummaryDTO(p)
	if d.Name != "Pillar-A" || d.Rank != 3 || d.DelegateRewardPercent != 90 {
		t.Fatalf("bad mapping: %+v", d)
	}
	if d.Weight != "1500000000000" || d.ProducerAddress != owner.String() {
		t.Fatalf("bad weight/producer: %+v", d)
	}
	// nil Weight → "0"
	if pillarSummaryDTO(&embedded.PillarInfo{Name: "B"}).Weight != "0" {
		t.Fatal("nil weight should map to 0")
	}
}

func TestSortPillarsByRank(t *testing.T) {
	in := []PillarSummary{{Name: "c", Rank: 5}, {Name: "a", Rank: 1}, {Name: "b", Rank: 3}}
	sortPillarsByRank(in)
	if in[0].Name != "a" || in[1].Name != "b" || in[2].Name != "c" {
		t.Fatalf("not sorted by rank: %+v", in)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `GOWORK=off go test ./app/ -run 'TestPillarSummaryDTO|TestSortPillarsByRank' -v`
Expected: FAIL — `pillarSummaryDTO` / `sortPillarsByRank` undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go` (the `embedded` import already exists from 5a; add `"sort"` to the import block):
```go
// pillarSummaryDTO maps an SDK PillarInfo to the delegation-picker summary.
func pillarSummaryDTO(p *embedded.PillarInfo) PillarSummary {
	weight := "0"
	if p.Weight != nil {
		weight = p.Weight.String()
	}
	return PillarSummary{
		Name:                  p.Name,
		Rank:                  int(p.Rank),
		Weight:                weight,
		DelegateRewardPercent: int(p.GiveDelegateRewardPercentage),
		ProducerAddress:       p.ProducerAddress.String(),
	}
}

// sortPillarsByRank orders pillars by ascending rank (in place).
func sortPillarsByRank(ps []PillarSummary) {
	sort.Slice(ps, func(i, j int) bool { return ps[i].Rank < ps[j].Rank })
}

// GetPillarList returns all pillars (rank-sorted) for the delegation picker.
func (s *NomService) GetPillarList() ([]PillarSummary, error) {
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	out := []PillarSummary{}
	var pageIndex uint32 = 0
	const pageSize uint32 = 100
	for {
		list, err := client.PillarApi.GetAll(pageIndex, pageSize)
		if err != nil {
			return nil, err
		}
		for _, p := range list.List {
			out = append(out, pillarSummaryDTO(p))
		}
		if len(out) >= list.Count || len(list.List) == 0 {
			break
		}
		pageIndex++
	}
	sortPillarsByRank(out)
	return out, nil
}

// GetDelegation returns the active address's current pillar delegation.
// An empty Name means not delegated.
func (s *NomService) GetDelegation() (DelegationInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return DelegationInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return DelegationInfo{}, errLocked
	}
	d, err := client.PillarApi.GetDelegatedPillar(addr)
	if err != nil {
		return DelegationInfo{}, err
	}
	if d == nil {
		return DelegationInfo{}, nil
	}
	weight := "0"
	if d.Weight != nil {
		weight = d.Weight.String()
	}
	return DelegationInfo{Name: d.Name, Status: int(d.Status), Weight: weight}, nil
}

// GetPillarReward returns the active address's uncollected delegation reward.
func (s *NomService) GetPillarReward() (RewardInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return RewardInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return RewardInfo{}, errLocked
	}
	r, err := client.PillarApi.GetUncollectedReward(addr)
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

Run: `GOWORK=off go test ./app/ -run 'TestPillarSummaryDTO|TestSortPillarsByRank' -v && GOWORK=off go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/dto.go app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService pillar reads (list w/ rank sort, delegation, reward)"
```

---

## Task 2: NomService pillar actions

**Files:** Modify `app/nom_service.go`, `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.PillarApi.Delegate(name)/Undelegate()/CollectReward()`, `tx.prepareCall`, `callExpect`, `types.{PillarContract, ZnnTokenStandard}`, `embedded.NewPillarApi`, `nom.AccountBlock`.
- Produces: `PrepareDelegate(name string) (CallPreview, error)`; `PrepareUndelegate() (CallPreview, error)`; `PrepareCollectPillarReward() (CallPreview, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/nom_service_test.go` (the `nom` import was added in 5b; reuse it):
```go
func TestPrepareDelegateValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// empty / whitespace name rejected before any node use.
	if _, err := s.PrepareDelegate(""); err == nil {
		t.Fatal("expected empty name to be rejected")
	}
	if _, err := s.PrepareDelegate("   "); err == nil {
		t.Fatal("expected whitespace name to be rejected")
	}
}

func TestPillarTemplateTokenStandards(t *testing.T) {
	api := embedded.NewPillarApi(nil) // builders construct blocks from args; no client deref
	for name, b := range map[string]*nom.AccountBlock{
		"delegate":   api.Delegate("Pillar-A"),
		"undelegate": api.Undelegate(),
		"collect":    api.CollectReward(),
	} {
		if b.ToAddress != types.PillarContract {
			t.Fatalf("%s: ToAddress=%v want PillarContract", name, b.ToAddress)
		}
		if b.TokenStandard != types.ZnnTokenStandard {
			t.Fatalf("%s: TokenStandard=%v want ZNN", name, b.TokenStandard)
		}
		if b.Amount == nil || b.Amount.Sign() != 0 {
			t.Fatalf("%s: Amount=%v want 0", name, b.Amount)
		}
	}
}
```
(Confirm `embedded.NewPillarApi(nil)`'s `Delegate`/`Undelegate`/`CollectReward` don't deref the client — mirrors 5a/5b. They construct `*nom.AccountBlock` from args only.)

- [ ] **Step 2: Run to verify failure**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareDelegate|TestPillarTemplate' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go` (`"strings"` already imported from 5a; `"fmt"`, `"math/big"` available):
```go
// PrepareDelegate builds a Delegate template (delegates the account's ZNN weight
// to the named pillar; no funds move) and hands it to TxService. Name validated
// before any node use.
func (s *NomService) PrepareDelegate(name string) (CallPreview, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return CallPreview{}, errors.New("pillar name is required")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.Delegate(name)
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Delegate to %s", name))
}

// PrepareUndelegate builds an Undelegate template (removes the current delegation).
func (s *NomService) PrepareUndelegate() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.Undelegate()
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Undelegate from current pillar")
}

// PrepareCollectPillarReward builds a CollectReward template (claims accrued
// delegation rewards).
func (s *NomService) PrepareCollectPillarReward() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.CollectReward()
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Collect delegation rewards")
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareDelegate|TestPillarTemplate' -v && GOWORK=off go build ./... && GOWORK=off go vet ./app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService Delegate/Undelegate/CollectReward actions via prepareCall"
```

---

## Task 3: Bindings + pillar store + nav

**Files:** Modify `frontend/wailsjs/` (generated), `frontend/src/lib/stores/nav.ts`; Create `frontend/src/lib/stores/pillar.ts`.

**Interfaces:**
- Consumes: bound `NomService.GetPillarList`/`GetDelegation`/`GetPillarReward`/`PrepareDelegate`/`PrepareUndelegate`/`PrepareCollectPillarReward`.
- Produces: `pillar` store (`pillars` + `delegation` + `pillarReward` writables) + `refreshPillars()`; `nav` `'pillars'` view.

- [ ] **Step 1: Regenerate bindings**

```bash
GOWORK=off "$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/ 2>/dev/null || true
ls frontend/wailsjs/go/app/NomService.d.ts   # GetPillarList/GetDelegation/GetPillarReward/PrepareDelegate/PrepareUndelegate/PrepareCollectPillarReward present; PillarSummary/DelegationInfo in models.ts
```
Revert any `frontend/wailsjs/runtime/*` churn.

- [ ] **Step 2: Add the pillar store + nav view**

`frontend/src/lib/stores/pillar.ts`:
```ts
import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'

export type PillarSummary = { name: string; rank: number; weight: string; delegateRewardPercent: number; producerAddress: string }
export type DelegationInfo = { name: string; status: number; weight: string }
export type RewardInfo = { znn: string; qsr: string }

export const pillars = writable<PillarSummary[]>([])
export const delegation = writable<DelegationInfo | null>(null)
export const pillarReward = writable<RewardInfo | null>(null)

export async function refreshPillars(): Promise<void> {
  try {
    pillars.set((await Nom.GetPillarList()) as PillarSummary[])
    delegation.set((await Nom.GetDelegation()) as DelegationInfo)
    pillarReward.set((await Nom.GetPillarReward()) as RewardInfo)
  } catch { /* not connected / locked — leave as-is */ }
}
```
Add `'pillars'` to the `View` union in `frontend/src/lib/stores/nav.ts` (currently `'dashboard' | 'send' | 'create' | 'import' | 'unlock' | 'settings' | 'plasma' | 'stake'`).

- [ ] **Step 3: Build to verify**

Run: `cd frontend && pnpm run build`
Expected: clean (run `rm -rf node_modules && pnpm install` first if node_modules is stale).

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs frontend/src/lib/stores/pillar.ts frontend/src/lib/stores/nav.ts
git commit -m "feat(frontend): pillar bindings + store + nav view"
```

---

## Task 4: Pillars route UI

**Files:** Create `frontend/src/routes/Pillars.svelte`, `frontend/src/routes/Pillars.test.ts`; Modify `frontend/src/routes/Dashboard.svelte`, `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: `pillar` store, `NomService.PrepareDelegate`/`PrepareUndelegate`/`PrepareCollectPillarReward`, the `tx` store + `awaitConfirm`/`TxModal`/`TxResult`, `formatAmount`, `nav`.
- Produces: Pillars route (current delegation + reward/Collect + Undelegate + searchable pillar list w/ Delegate) + dashboard link + App route.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/Pillars.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetPillarList: vi.fn().mockResolvedValue([
    { name: 'Alpha', rank: 1, weight: '100000000000', delegateRewardPercent: 90, producerAddress: 'z1a' },
    { name: 'Beta', rank: 2, weight: '50000000000', delegateRewardPercent: 80, producerAddress: 'z1b' },
  ]),
  GetDelegation: vi.fn().mockResolvedValue({ name: '', status: 0, weight: '0' }),
  GetPillarReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  PrepareDelegate: vi.fn(), PrepareUndelegate: vi.fn(), PrepareCollectPillarReward: vi.fn(),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Pillars from './Pillars.svelte'

describe('Pillars', () => {
  it('filters the pillar list by search text', async () => {
    render(Pillars)
    expect(await screen.findByText('Alpha')).toBeTruthy()
    expect(screen.getByText('Beta')).toBeTruthy()
    const search = screen.getByLabelText('search pillars') as HTMLInputElement
    await fireEvent.input(search, { target: { value: 'alph' } })
    expect(screen.getByText('Alpha')).toBeTruthy()
    expect(screen.queryByText('Beta')).toBeNull()
  })

  it('hides Undelegate when not delegated', async () => {
    render(Pillars)
    await screen.findByText('Alpha')
    expect(screen.queryByRole('button', { name: /undelegate/i })).toBeNull()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Pillars.test.ts`
Expected: FAIL — cannot resolve `./Pillars.svelte`.

- [ ] **Step 3: Implement**

READ `frontend/src/routes/Stake.svelte` first and mirror its exact tx-flow wiring (`awaitConfirm`, `$tx.status` blocks, `TxModal`/`TxResult`, refresh-on-done). `frontend/src/routes/Pillars.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { pillars, delegation, pillarReward, refreshPillars } from '../lib/stores/pillar'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let search = ''
  let error = ''

  onMount(refreshPillars)
  $: rewardZero = !$pillarReward || ($pillarReward.znn === '0' && $pillarReward.qsr === '0')
  $: delegated = !!$delegation && $delegation.name !== ''
  $: filtered = ($pillars ?? []).filter((p) => p.name.toLowerCase().includes(search.trim().toLowerCase()))
  $: if ($tx.status === 'done') refreshPillars()

  async function delegate(name: string) {
    error = ''
    try { awaitConfirm((await Nom.PrepareDelegate(name)) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function undelegate() {
    error = ''
    try { awaitConfirm((await Nom.PrepareUndelegate()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function collect() {
    error = ''
    try { awaitConfirm((await Nom.PrepareCollectPillarReward()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Pillars</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Your delegation</h2>
    {#if delegated}
      <p class="text-sm">Delegated to <span class="font-mono">{$delegation.name}</span> · weight {formatAmount($delegation.weight, 8)} ZNN</p>
      <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={undelegate}>Undelegate</button>
    {:else}
      <p class="text-xs text-muted">Not delegated.</p>
    {/if}
    {#if $pillarReward}<p class="text-sm">Uncollected reward {formatAmount($pillarReward.znn, 8)} ZNN · {formatAmount($pillarReward.qsr, 8)} QSR</p>{/if}
    <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-40" disabled={rewardZero} on:click={collect}>Collect</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Pillars</h2>
    <input class="w-full rounded bg-bg px-3 py-2" placeholder="search pillars" bind:value={search} aria-label="search pillars" />
    {#each filtered as p}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">#{p.rank} {p.name} · {formatAmount(p.weight, 8)} ZNN · {p.delegateRewardPercent}%{#if p.name === $delegation?.name} · current{/if}</span>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => delegate(p.name)} aria-label={`delegate to ${p.name}`}>Delegate</button>
      </div>
    {/each}
    {#if filtered.length === 0}<p class="text-xs text-muted">No pillars.</p>{/if}
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
```
(If Stake.svelte wires `TxModal`/`TxResult` or the tx status values differently, match Stake exactly — it is the working reference.)

In `Dashboard.svelte` add a "Pillars" button → `view.set('pillars')` (next to the existing "Staking" button). In `App.svelte` import `Pillars` and add an `{:else if $view === 'pillars'}<Pillars />` branch (next to the `'stake'` branch).

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS; clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Pillars.svelte frontend/src/routes/Pillars.test.ts frontend/src/routes/Dashboard.svelte frontend/src/App.svelte
git commit -m "feat(frontend): Pillars route (delegation + reward/collect + searchable pillar list)"
```

---

## Task 5: Verification + acceptance

**Files:** Create `docs/phase5c-acceptance.md`.

- [ ] **Step 1: Full automated verification**

```bash
find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} + 2>/dev/null
GOWORK=off go test ./...
GOWORK=off go build -tags integration ./...   # opt-in integration test still compiles
cd frontend && pnpm test && pnpm run build && cd ..
xattr -cr build/bin 2>/dev/null; GOWORK=off "$(go env GOPATH)/bin/wails" build
```
Expected: backend green, frontend green, app builds.

- [ ] **Step 2: Manual acceptance (Phase 5c gate)**

On a testnet node **with the `embedded` namespace enabled** (Pillars route):
1. Open the Pillars route → see the rank-sorted pillar list + your current delegation (or "Not delegated") + uncollected reward.
2. Search filters the list by name.
3. Delegate to a pillar → TxModal shows "Delegate to <name>" from the built block → Confirm → after a momentum the delegation shows as current.
4. Collect rewards (when uncollected > 0) → reward arrives.
5. Undelegate → delegation clears.
6. Mainnet guard: with `AllowMainnetSend` false on a mainnet node, PrepareDelegate is blocked.

- [ ] **Step 3: Record the result**

`docs/phase5c-acceptance.md`: automated results + the manual checks (delegate/collect/undelegate, search, confirm-modal correctness, mainnet-gated), with testnet tx hashes. Note the `embedded`-namespace node prerequisite (as in 5b).

- [ ] **Step 4: Commit**

```bash
git add docs/phase5c-acceptance.md
git commit -m "docs: Phase 5c acceptance record"
```

---

## Self-Review

**Spec coverage:** pillar reads + DTOs + rank sort (T1); Delegate/Undelegate/Collect actions + name validation + token-standard/Amount-0 regression test (T2); bindings + store + nav (T3); Pillars route UI with current-delegation + reward/Collect + Undelegate + searchable pillar list + Delegate (T4); verification + manual acceptance (T5). All spec sections mapped.

**Placeholder scan:** No TBD/TODO in product code. All steps carry full code. Integration is an opt-in documented skip (same as 5a/5b). Bindings regen is environment-run with the revert caution.

**Type consistency:** `PillarSummary`/`DelegationInfo` Go fields ↔ camelCase TS (`name/rank/weight/delegateRewardPercent/producerAddress`, `name/status/weight`). Reuses `RewardInfo` (`znn/qsr`) from 5b. `PrepareDelegate(name)`/`PrepareUndelegate()`/`PrepareCollectPillarReward()` match the TS store/route calls. `callExpect{to: PillarContract, zts: ZnnTokenStandard, amount: 0, data}` matches the real SDK templates (T2 regression test locks it). Reuses `prepareCall`/`assertMatches`/`awaitConfirm`/`formatAmount` from 5a/5b unchanged.

**Known follow-up (not 5c):** pillar operator features (Register/Update/Revoke/DepositQsr/WithdrawQsr); reward-history list; pillar epoch/momentum stats; tokens/sentinels/accelerator in later sub-phases; deeper ABI `Data` semantic decode remains the Phase-5/7 hardening note.
