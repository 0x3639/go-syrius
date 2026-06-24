# UI Home Redesign — Plan B (tab panels) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax. Use **superpowers:frontend-design** for the visual adaptation; the established Plan A design language (`ui/` primitives + theme tokens) is the source of truth, plus the nom-ui screenshots.

**Goal:** Replace the six placeholder tabs on the Home with native restyled panels (Rewards is new; Plasma/Pillar/Staking/Sentinels/Accelerator are adapted from the current route bodies), add a Settings entry point to the Home top bar, centralize the tx-confirm UI, and delete the now-dead routes.

**Architecture:** Each existing `routes/X.svelte` becomes `lib/components/panels/XPanel.svelte` — same store/binding/`Prepare*` logic, page chrome stripped, controls restyled with the `ui/` primitives. The write/confirm flow is centralized: `Home` renders one `TxModal`/`TxResult` and resets `tx` on tab change (the panels just call `Prepare*` + `awaitConfirm`). Presentation-only; no Go/binding changes.

**Tech Stack:** Svelte 3, Tailwind 3.4, Vitest 0.34, the Plan A `ui/` components + theme.

## Global Constraints

- **Presentation-only.** No Go file, `wailsjs` binding, or store-logic change. Reuse the existing `Nom.*`/`Tx.*` bindings, the `tx` store (`awaitConfirm`/`prepare`/`resetTx`), and `TxModal`/`TxResult` exactly.
- **Design language:** use the Plan A `ui/` primitives (`Card`, `Button`, `Input`, `Field`, `Tabs`) and theme tokens (`accent`=green, `qsr`=blue, `bg`/`surface`/`elevated`/`border`/`muted`, radius 0.375rem, mono for amounts). No raw `bg-surface` form controls where a `ui/` primitive fits.
- **tx flow in the single page:** `tx` resets on `$view` change, but switching tabs does NOT change `$view`. So: `Home` resets `tx` (`resetTx()`) whenever the active tab changes, and renders a single `TxModal`/`TxResult` for all panels. Panels MUST NOT render their own `TxModal`/`TxResult` (drop those blocks when adapting).
- **Adaptation recipe (per existing route → panel):** create `lib/components/panels/XPanel.svelte`; copy the `<script>` from `routes/X.svelte` but remove the `view` import + any `view.set('dashboard')`/Back handling; replace the outer `<div class="mx-auto mt-8 w-[…]">` + the `<h1>…/Back` header with a plain `<div class="space-y-4 p-4">`; swap raw `<input>`/`<select>`/`<button>` for `ui/` `Input`/`Field`/`Button` (keep all `aria-label`s and the `on:click`/binding wiring); **delete the per-panel `{#if $tx.status …}<TxModal/>…<TxResult/>` blocks** (Home owns them). **Preserve test coverage:** if `routes/X.test.ts` exists, port it to `panels/XPanel.test.ts` — adapt the import path + mock paths (now under `panels/`, so `../../...` depth) and drop any assertion about the removed Back button / page header; keep the behavioral assertions. (Task 8 deletes the old route test, so the ported panel test is where that coverage lives.)
- **Keep** `routes/Tokens.svelte` (token management) and `routes/Settings.svelte` — they are reached from the Home (Tokens "Manage" + Settings button); their full restyle is the follow-up.
- **Delete in the final task** (now dead): `routes/{Dashboard,Send,Plasma,Stake,Pillars,Sentinels,Accelerator}.svelte` + `lib/components/StatusBar.svelte` and their `*.test.ts`. (Tokens.svelte + Settings.svelte stay.)
- Frontend commands (in `frontend/`): `pnpm test`, `pnpm run check`, `pnpm run build`. Visual: `GOWORK=off ~/go/bin/wails dev` (controller/user).
- Commits GPG-signed: **implementers STAGE only**, controller commits.

## File Structure

- Create: `lib/components/panels/{PlasmaPanel,PillarPanel,StakingPanel,SentinelsPanel,AcceleratorPanel,RewardsPanel}.svelte` (+ tests where logic warrants).
- Modify: `routes/Home.svelte` (Settings button, centralized TxModal + reset-on-tab-change, wire the 7 panels, Tokens "Manage" link).
- Modify: `lib/components/panels/TokensPanel.svelte` (add a "Manage tokens" button → `view.set('tokens')`).
- Delete (final task): the dead routes + `StatusBar.svelte` + their tests.

---

## Task 1: Home — Settings button, centralized tx UI, reset-on-tab-change

**Files:** Modify `frontend/src/routes/Home.svelte`; update `frontend/src/routes/Home.test.ts`.

**Interfaces:**
- Consumes: `view` (`stores/nav`), `tx`/`resetTx` (`stores/tx`), `TxModal`, `TxResult`, `Button`.
- Produces: a Settings button (`view.set('settings')`); a single `TxModal`/`TxResult` rendered at Home; `tx` reset whenever `active` changes.

- [ ] **Step 1: Add the Settings button to the top bar** — in `Home.svelte`, next to the Lock button:

```svelte
  import { view } from '../lib/stores/nav'
  ...
  <Button variant="ghost" on:click={() => view.set('settings')} aria-label="Settings">Settings</Button>
  <Button variant="ghost" on:click={lock}>Lock</Button>
```

(Implementer: a gear icon is a nice touch via frontend-design; keep `aria-label="Settings"`.)

- [ ] **Step 2: Centralize the tx-confirm UI + reset on tab change** — in `Home.svelte` add the imports and a reactive reset, and render the shared modal once at the end of the page:

```svelte
  import { tx, resetTx } from '../lib/stores/tx'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'
  ...
  let prevTab = active
  $: if (active !== prevTab) { prevTab = active; resetTx() }
```

At the bottom of the page markup (after the tab panel, alongside the Send/Receive modals):

```svelte
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
```

- [ ] **Step 3: Update `Home.test.ts`** — add a case for the Settings button:

```ts
  it('exposes a Settings entry point', () => {
    render(Home)
    expect(screen.getByRole('button', { name: 'Settings' })).toBeTruthy()
  })
```

- [ ] **Step 4: Verify**

Run: `cd frontend && pnpm test -- src/routes/Home && pnpm run check`
Expected: Home tests pass (incl. Settings); svelte-check 0.

- [ ] **Step 5: Stage** — `git add frontend/src/routes/Home.svelte frontend/src/routes/Home.test.ts`

---

## Tasks 2–6: Adapt the five feature routes into panels

Each task follows the **Adaptation recipe** in Global Constraints. The implementer reads the named route file, creates the panel, and verifies. Drop the per-panel `TxModal`/`TxResult` (Home owns them). Keep every `aria-label` and the exact `Nom.Prepare*`/`awaitConfirm` calls.

### Task 2: PlasmaPanel

**Files:** Create `frontend/src/lib/components/panels/PlasmaPanel.svelte` from `frontend/src/routes/Plasma.svelte`.

- [ ] **Step 1:** Read `routes/Plasma.svelte`. Create `panels/PlasmaPanel.svelte` per the recipe: keep the plasma store (`plasmaInfo`/`fusionEntries`/`refreshPlasma`), the fuse/cancel `Nom.PrepareFuse`/`PrepareCancelFuse` + `awaitConfirm` wiring, and the beneficiary/amount fields (now `ui/ Field`+`Input`, the fuse action a green `ui/ Button`). Strip the page wrapper/Back/header; drop the `TxModal`/`TxResult` blocks.
- [ ] **Step 2:** `cd frontend && pnpm run check` → 0 errors; `pnpm test` → still green.
- [ ] **Step 3:** Stage `git add frontend/src/lib/components/panels/PlasmaPanel.svelte`

### Task 3: PillarPanel

**Files:** Create `frontend/src/lib/components/panels/PillarPanel.svelte` from `frontend/src/routes/Pillars.svelte`.

- [ ] **Step 1:** Read `routes/Pillars.svelte`. Create `PillarPanel.svelte` per the recipe: pillar list (`pillars` store, search/sort), delegate/undelegate (`Nom.PrepareDelegate`/`PrepareUndelegate` + `awaitConfirm`), delegation status. Pillar rows styled like the nom-ui Pillar screenshot (rank · name · APR · weight · green Delegate button). Strip chrome; drop tx blocks.
- [ ] **Step 2:** check + test green.
- [ ] **Step 3:** Stage `git add frontend/src/lib/components/panels/PillarPanel.svelte`

### Task 4: StakingPanel

**Files:** Create `frontend/src/lib/components/panels/StakingPanel.svelte` from `frontend/src/routes/Stake.svelte`.

- [ ] **Step 1:** Read `routes/Stake.svelte`. Create `StakingPanel.svelte` per the recipe: amount + duration selector (`ui/ Field`/`Input`/`<select>`), active stakes list + cancel, `Nom.PrepareStake`/`PrepareCancelStake` + `awaitConfirm`. Green "Stake ZNN" `ui/ Button` (matches the screenshot). Strip chrome; drop tx blocks.
- [ ] **Step 2:** check + test green.
- [ ] **Step 3:** Stage `git add frontend/src/lib/components/panels/StakingPanel.svelte`

### Task 5: SentinelsPanel

**Files:** Create `frontend/src/lib/components/panels/SentinelsPanel.svelte` from `frontend/src/routes/Sentinels.svelte`.

- [ ] **Step 1:** Read `routes/Sentinels.svelte`. Create `SentinelsPanel.svelte` per the recipe: sentinel status + deposit/register/revoke/collect (`Nom.PrepareDepositQsr`/`PrepareRegisterSentinel`/`PrepareRevokeSentinel` + `awaitConfirm`). Strip chrome; drop tx blocks.
- [ ] **Step 2:** check + test green.
- [ ] **Step 3:** Stage `git add frontend/src/lib/components/panels/SentinelsPanel.svelte`

### Task 6: AcceleratorPanel

**Files:** Create `frontend/src/lib/components/panels/AcceleratorPanel.svelte` from `frontend/src/routes/Accelerator.svelte`.

- [ ] **Step 1:** Read `routes/Accelerator.svelte`. Create `AcceleratorPanel.svelte` per the recipe: project list/browse, donate, vote (gated on `votablePillars`), create/manage `<details>` — all the existing `accelerator` store + `Nom.Prepare*` wiring. Strip chrome; drop tx blocks. Keep every aria-label (`vote target id`, `donate amount/token`, `project id`, etc.).
- [ ] **Step 2:** check + test green.
- [ ] **Step 3:** Stage `git add frontend/src/lib/components/panels/AcceleratorPanel.svelte`

---

## Task 7: RewardsPanel (new aggregation)

**Files:** Create `frontend/src/lib/components/panels/RewardsPanel.svelte`; test `panels/RewardsPanel.test.ts`.

**Interfaces:**
- Consumes: `Nom.GetUncollectedReward` (stake), `Nom.GetPillarReward` (delegation), `Nom.GetSentinelReward` (sentinel) — each returns `{ znn, qsr }` base-unit strings; `Nom.PrepareCollectReward`/`PrepareCollectPillarReward`/`PrepareCollectSentinelReward` → `CallPreview`; `awaitConfirm` (`stores/tx`); `formatAmount`; `ui/ Button`, `ui/ Card`.

- [ ] **Step 1: Write `RewardsPanel.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../../wailsjs/go/app/NomService'
  import { tx, awaitConfirm } from '../../stores/tx'
  import { formatAmount } from '../../format'
  type R = { znn: string; qsr: string }
  const SOURCES = [
    { key: 'delegation', label: 'Delegation', get: Nom.GetPillarReward, collect: Nom.PrepareCollectPillarReward },
    { key: 'staking', label: 'Staking', get: Nom.GetUncollectedReward, collect: Nom.PrepareCollectReward },
    { key: 'sentinel', label: 'Sentinel', get: Nom.GetSentinelReward, collect: Nom.PrepareCollectSentinelReward },
  ]
  let rewards: Record<string, R> = {}
  let error = ''
  function has(r?: R) { return r && (r.znn !== '0' || r.qsr !== '0') }
  async function load() {
    for (const s of SOURCES) {
      try { rewards[s.key] = (await s.get()) as unknown as R } catch { /* locked/not-connected */ }
    }
    rewards = rewards
  }
  onMount(load)
  $: if ($tx.status === 'done') load()
  async function collect(s: typeof SOURCES[number]) {
    error = ''
    try { awaitConfirm((await s.collect()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>
<div class="space-y-3 p-4">
  {#each SOURCES as s}
    <div class="flex items-center justify-between rounded border border-border bg-surface px-4 py-3">
      <div>
        <div class="font-medium">{s.label}</div>
        <div class="font-mono text-sm text-muted">
          {formatAmount(rewards[s.key]?.znn ?? '0', 8)} ZNN · {formatAmount(rewards[s.key]?.qsr ?? '0', 8)} QSR
        </div>
      </div>
      <button class="rounded bg-accent px-3 py-1 text-sm text-accent-fg disabled:opacity-50"
        disabled={!has(rewards[s.key])} aria-label={`collect ${s.label}`} on:click={() => collect(s)}>Collect</button>
    </div>
  {/each}
  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}
</div>
```

- [ ] **Step 2: Write `panels/RewardsPanel.test.ts`**

```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
const mocks = vi.hoisted(() => ({
  GetPillarReward: vi.fn().mockResolvedValue({ znn: '100000000', qsr: '0' }),
  GetUncollectedReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  GetSentinelReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  PrepareCollectPillarReward: vi.fn(), PrepareCollectReward: vi.fn(), PrepareCollectSentinelReward: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/NomService', () => mocks)
vi.mock('../../stores/tx', () => ({ tx: { subscribe: (f: any) => { f({ status: 'idle' }); return () => {} } }, awaitConfirm: vi.fn() }))
import RewardsPanel from './RewardsPanel.svelte'
describe('RewardsPanel', () => {
  it('lists the three sources and enables Collect only where reward > 0', async () => {
    render(RewardsPanel)
    expect(await screen.findByText('Delegation')).toBeTruthy()
    expect(screen.getByText('Staking')).toBeTruthy()
    expect(screen.getByText('Sentinel')).toBeTruthy()
    const delg = screen.getByRole('button', { name: 'collect Delegation' }) as HTMLButtonElement
    const stk = screen.getByRole('button', { name: 'collect Staking' }) as HTMLButtonElement
    expect(delg.disabled).toBe(false)   // 1 ZNN pending
    expect(stk.disabled).toBe(true)     // nothing pending
  })
})
```

- [ ] **Step 3: Verify** — `cd frontend && pnpm test -- src/lib/components/panels/RewardsPanel && pnpm run check` → pass; svelte-check 0.

- [ ] **Step 4: Stage** — `git add frontend/src/lib/components/panels/RewardsPanel.svelte frontend/src/lib/components/panels/RewardsPanel.test.ts`

---

## Task 8: Wire panels into Home, Tokens "Manage" link, delete dead routes

**Files:** Modify `frontend/src/routes/Home.svelte`, `frontend/src/lib/components/panels/TokensPanel.svelte`; delete `routes/{Dashboard,Send,Plasma,Stake,Pillars,Sentinels,Accelerator}.svelte` + `lib/components/StatusBar.svelte` + their `*.test.ts`; update `frontend/src/lib/stores/nav.ts`.

- [ ] **Step 1: Wire the panels** — in `Home.svelte`, import the six panels and replace the `{:else}<PanelPlaceholder …/>` branch with the real panels:

```svelte
  import RewardsPanel from '../lib/components/panels/RewardsPanel.svelte'
  import PlasmaPanel from '../lib/components/panels/PlasmaPanel.svelte'
  import PillarPanel from '../lib/components/panels/PillarPanel.svelte'
  import StakingPanel from '../lib/components/panels/StakingPanel.svelte'
  import SentinelsPanel from '../lib/components/panels/SentinelsPanel.svelte'
  import AcceleratorPanel from '../lib/components/panels/AcceleratorPanel.svelte'
  ...
  {#if active === 'Tokens'}<TokensPanel />
  {:else if active === 'Rewards'}<RewardsPanel />
  {:else if active === 'Plasma'}<PlasmaPanel />
  {:else if active === 'Pillar'}<PillarPanel />
  {:else if active === 'Staking'}<StakingPanel />
  {:else if active === 'Sentinels'}<SentinelsPanel />
  {:else if active === 'Accelerator'}<AcceleratorPanel />{/if}
```

Remove the now-unused `PanelPlaceholder` import.

- [ ] **Step 2: Tokens "Manage" link** — in `TokensPanel.svelte`, add a button (top-right of the panel) that opens the token-management screen (kept for the follow-up):

```svelte
  import { view } from '../../stores/nav'
  ...
  <div class="flex items-center justify-between">
    <Input bind:value={q} placeholder="Search tokens…" ariaLabel="search tokens" />
    <button class="ml-2 shrink-0 rounded border border-border px-3 py-2 text-sm text-muted hover:text-text" on:click={() => view.set('tokens')}>Manage</button>
  </div>
```

- [ ] **Step 3: Delete the dead routes + StatusBar + their tests**

```bash
cd frontend
git rm src/routes/Dashboard.svelte src/routes/Dashboard.test.ts \
       src/routes/Send.svelte src/routes/Send.test.ts \
       src/routes/Plasma.svelte src/routes/Plasma.test.ts \
       src/routes/Stake.svelte src/routes/Stake.test.ts \
       src/routes/Pillars.svelte src/routes/Pillars.test.ts \
       src/routes/Sentinels.svelte src/routes/Sentinels.test.ts \
       src/routes/Accelerator.svelte src/routes/Accelerator.test.ts \
       src/lib/components/StatusBar.svelte
```

(`routes/Tokens.svelte` + `routes/Settings.svelte` stay. If any deleted test asserted shared store behavior still worth keeping, move that assertion into a panel test instead of losing it.)

- [ ] **Step 4: Clean the nav `View` union** — in `stores/nav.ts`, keep the views still used (`dashboard` default, `create`, `import`, `unlock`, `settings`, `tokens`) and drop the ones whose routes were deleted (`send`, `plasma`, `stake`, `pillars`, `sentinels`, `accelerator`). Verify nothing references the removed members (`grep -rn "view.set('plasma'" src` etc. → none).

- [ ] **Step 5: Full verification**

Run: `cd frontend && pnpm test && pnpm run check && pnpm run build`
Expected: full suite green (no references to deleted files); svelte-check 0; build succeeds.

- [ ] **Step 6: Stage** — `git add -A frontend/src` (includes the deletions, Home, TokensPanel, nav.ts).

- [ ] **Step 7: Visual verification (controller/user)** — `GOWORK=off ~/go/bin/wails dev`: every tab now shows a native restyled panel (no "being restyled" placeholders); Settings button works; Tokens "Manage" opens management; the write flows (fuse/stake/delegate/donate/collect) still confirm+publish via the shared TxModal.

---

## Self-Review / Verification (end-to-end)

- `cd frontend && pnpm test` — all green (panels + Rewards + Home + ui); `pnpm run check` 0; `pnpm run build` ok.
- No Go/binding changes; funds path unchanged (panels call the same `Prepare*` + the shared `TxModal`/`ConfirmPublish`).
- No dead route files remain (Tokens-management + Settings intentionally kept); nav `View` union has no dangling members.
- Visual: all 7 tabs are native nom-ui-styled panels; Settings + Tokens-Manage reachable.

## Hand-off (follow-up sub-project)

Restyle the kept `routes/{Tokens,Settings}.svelte` (token management + settings) and the entry screens (Unlock/Create/Import) into the same language; optionally fold token management into a modal so the last full-page routes go away.
