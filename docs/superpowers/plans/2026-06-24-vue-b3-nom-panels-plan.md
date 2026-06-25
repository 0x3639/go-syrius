# Vue B3 — NoM Panels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Fill the 6 NoM tabs (Rewards/Plasma/Pillar/Staking/Sentinels/Accelerator) with real panels + their NomService Pinia stores, each NoM action confirmed through the B2 confirm-what-you-sign flow.

**Architecture:** Each panel reads a Pinia store (refresh on mount), runs actions via `NomService.PrepareX(...) → tx.awaitConfirm(preview)`, a **global TxModal** (in Home, in a nom-ui Dialog gated `!sendOpen && !receiveOpen`) renders the `CallPreview` (summary + exact amount) for confirm → `ConfirmPublish`, and the panel refreshes on `tx.status==='done'`. The `tx` store resets on route change.

**Tech Stack:** Vue 3.4 + Pinia + vue-router, Tailwind 4 + nom-ui, Vitest + @vue/test-utils.

## Global Constraints

- **Branch `frontend-vue-migration`** (after B2 `4ea632e`); not merged until B4.
- **Frontend-only:** NO `app/*.go`/`internal/*` changes; NomService bindings consumed as-is.
- **Faithful port** of the merged Svelte panels (`main:frontend/src/lib/components/panels/*.svelte`) + stores (`main:frontend/src/lib/stores/{stake,sentinel,accelerator,plasma,pillar}.ts`): same fields, validation, copy, actions.
- **Funds-safety:** every NoM action goes `Nom.PrepareX` (Go builds the call block) → `tx.awaitConfirm(preview)` → global TxModal (confirm-what-you-sign: renders `preview.summary` + `formatAmountExact(preview.amount,8)`) → `tx.confirm()`→`ConfirmPublish()`. NO NoM call may bypass `tx.confirm`. Amount inputs → `toBase` (from `src/lib/format.ts`).
- **Amounts:** `formatAmount` (display) / `formatAmountExact` (confirm). Never nom-ui `Amount`.
- **nom-ui:** verify each component vs `node_modules/nom-ui/src` before use; `Field` = our local `src/components/Field.vue`. Theme map (from B2): `text-text→text-foreground`, `text-muted→text-muted-foreground`, `bg-surface→bg-card`, green→`primary`, blue/qsr→`info`, `border-border` stays, `text-error→text-destructive`, `text-success→text-primary`.
- **`tx` store** (B2): state `status`/`preview`/`hash`/`error`; actions `prepare`/`awaitConfirm(preview)`/`confirm`/`cancel`/`reset`. `awaitConfirm` already accepts the `CallPreview` superset.
- Commands in `frontend/`: `pnpm test`/`pnpm run typecheck`/`pnpm run build`. wails=`~/go/bin/wails`. Commits GPG-signed: implementers STAGE only; keep `go.mod` 2.12.0 churn out.

## File Structure

- `src/stores/`: extend `pillar.ts`, `plasma.ts`; new `stake.ts`, `sentinel.ts`, `accelerator.ts`.
- `src/components/panels/`: `RewardsPanel.vue`, `PlasmaPanel.vue`, `PillarPanel.vue`, `StakingPanel.vue`, `SentinelsPanel.vue`, `AcceleratorPanel.vue` (+ tests).
- `src/views/Home.vue` (global TxModal Dialog + swap placeholders), `src/router/index.ts` (tx route-reset).
- `src/components/NomConfirm.vue` (the global confirm Dialog wrapper).

---

## Task 1: Stores (extend pillar/plasma; new stake/sentinel/accelerator)

**Files:** Modify `src/stores/pillar.ts`, `src/stores/plasma.ts`; Create `src/stores/{stake,sentinel,accelerator}.ts`; tests `src/stores/nom-stores.test.ts`.

**Interfaces (Pinia stores; port the Svelte writables):**
- `usePillarStore` (extend): add `pillars: PillarSummary[]`, `reward: RewardInfo|null`; `refresh()` → `GetPillarList`+`GetDelegation`+`GetPillarReward` (keep `delegation`/`refreshDelegation`).
- `usePlasmaStore` (extend): add `fusionEntries: FusionEntry[]`; `refresh()` → `GetPlasmaInfo`+`GetFusionEntries`; `estimate(qsr)` → `EstimatePlasma` (returns number, 0 on error).
- `useStakeStore` (new): `stakeInfo`, `reward`; `refresh()` → `GetStakeList`+`GetUncollectedReward`.
- `useSentinelStore` (new): `sentinel`, `depositedQsr`, `reward`; `refresh()` → `GetSentinel`+`GetDepositedQsr`+`GetSentinelReward`.
- `useAcceleratorStore` (new): `projects`, `selectedProject`, `votablePillars`, `error`; `loadProjects(page=0)`→`GetProjects(page,20)` (use `.list ?? []`), `openProject(id)`→`GetProject(id)`, `loadVotablePillars()`→`GetVotablePillars`.

- [ ] **Step 1: Extend `src/stores/pillar.ts`** — add the pillars/reward state + a `refresh()` loading all three (`import type { app } from '../../wailsjs/go/models'` for the types):

```ts
// state adds: pillars: [] as app.PillarSummary[], reward: null as app.RewardInfo | null
    async refresh() {
      try {
        this.pillars = await Nom.GetPillarList()
        this.delegation = await Nom.GetDelegation()
        this.reward = await Nom.GetPillarReward()
      } catch { /* not connected / locked */ }
    },
```

- [ ] **Step 2: Extend `src/stores/plasma.ts`** — add `fusionEntries`, extend `refresh()`, add `estimate`:

```ts
// state adds: fusionEntries: [] as app.FusionEntry[]
    async refresh() {
      try { this.info = await Nom.GetPlasmaInfo(); this.fusionEntries = await Nom.GetFusionEntries() } catch {}
    },
    async estimate(qsr: string): Promise<number> {
      try { return await Nom.EstimatePlasma(qsr) } catch { return 0 }
    },
```

- [ ] **Step 3: Create `src/stores/stake.ts`, `sentinel.ts`, `accelerator.ts`** — Pinia ports of the Svelte stores (read each via `git show main:frontend/src/lib/stores/<name>.ts`), e.g.:

```ts
// stake.ts
import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'
export const useStakeStore = defineStore('stake', {
  state: () => ({ stakeInfo: null as app.StakeInfo | null, reward: null as app.RewardInfo | null }),
  actions: {
    async refresh() {
      try { this.stakeInfo = await Nom.GetStakeList(); this.reward = await Nom.GetUncollectedReward() } catch {}
    },
  },
})
```
(`sentinel.ts` and `accelerator.ts` follow the Svelte `sentinel.ts`/`accelerator.ts` field-for-field; accelerator keeps the per-action `error` surfacing.)

- [ ] **Step 4: Write `src/stores/nom-stores.test.ts`** — for each store, `refresh()`/`load*()` sets state from mocked `NomService` bindings (vi.hoisted + vi.mock). e.g. accelerator `loadProjects` sets `projects` from `GetProjects().list`; stake `refresh` sets `stakeInfo`.
- [ ] **Step 5: Run** `pnpm test -- src/stores && pnpm run typecheck` → pass + clean. **Stage** the 5 store files + test. No commit.

---

## Task 2: Global NoM confirm Dialog + tx route-reset

**Files:** Create `src/components/NomConfirm.vue`; Modify `src/views/Home.vue`, `src/router/index.ts`; test `src/components/NomConfirm.test.ts`.

**Interfaces:** Consumes `useTxStore`, nom-ui `Dialog`, the B2 `TxModal`/`TxResult`. Produces `NomConfirm.vue` — a Dialog open when `tx.status` is `awaiting`/`done`, showing `TxModal`/`TxResult`; closing → `tx.cancel()`/`tx.reset()`.

- [ ] **Step 1: Write `src/components/NomConfirm.vue`** — a nom-ui `Dialog` whose `open` = `tx.status==='awaiting' || tx.status==='done'`; body = `<TxModal/>` when `awaiting`, `<TxResult/>` when `done`; on close → if `awaiting` `tx.cancel()` else `tx.reset()`.

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from 'nom-ui'
import { useTxStore } from '../stores/tx'
import TxModal from './TxModal.vue'
import TxResult from './TxResult.vue'
const tx = useTxStore()
const open = computed({
  get: () => tx.status === 'awaiting' || tx.status === 'done',
  set: (v: boolean) => { if (!v) { tx.status === 'awaiting' ? tx.cancel() : tx.reset() } },
})
</script>
<template>
  <Dialog v-model:open="open">
    <DialogContent>
      <DialogHeader><DialogTitle>Confirm</DialogTitle></DialogHeader>
      <TxModal v-if="tx.status === 'awaiting'" />
      <TxResult v-else-if="tx.status === 'done'" />
    </DialogContent>
  </Dialog>
</template>
```

- [ ] **Step 2: Mount it in `Home.vue`** — render `<NomConfirm v-if="!sendOpen && !receiveOpen" />` after the Send/Receive modals (so panel-triggered confirms show, but never while the Send/Receive dialogs own the tx flow). Import + place it.
- [ ] **Step 3: tx route-reset in `src/router/index.ts`** — after the router is created, add:

```ts
router.afterEach(() => {
  // Discard any half-built/finished tx when navigating between screens so a
  // stale block never surfaces on an unrelated route.
  useTxStore().reset()
})
```
(Import `useTxStore`; it runs after the guard, with Pinia active.)

- [ ] **Step 4: Tests** — `NomConfirm.test.ts`: with `tx.status='awaiting'` + a preview, the dialog is open and renders the TxModal (stub nom-ui Dialog + TxModal/TxResult); setting open=false while awaiting calls `tx.cancel`. A `router/index.test.ts` addition: navigating calls `tx.reset` (spy).
- [ ] **Step 5: Run** `pnpm test && pnpm run typecheck` → pass + clean. **Stage** NomConfirm.vue, Home.vue, router/index.ts, tests. No commit.

---

## Tasks 3–8: the panels (one per task)

Each panel task: **read the Svelte original** (`git show main:frontend/src/lib/components/panels/<Name>Panel.svelte`) and **port it to `src/components/panels/<Name>Panel.vue`**, applying these mappings, then write the test. Common mappings for ALL panels:
- Svelte store imports → the Pinia stores (Task 1): `$store` → `store.field`; `refreshX()` → `store.refresh()` on mount (`onMounted`).
- `import { tx, awaitConfirm } from '../../stores/tx'` → `const tx = useTxStore()`; `awaitConfirm(preview)` → `tx.awaitConfirm(preview)`. The action body stays: `const preview = await Nom.PrepareX(...); tx.awaitConfirm(preview)`.
- `$: if ($tx.status === 'done') store.refresh()` → `watch(() => tx.status, (s) => { if (s === 'done') store.refresh() })`.
- `ui/Field`/`ui/Input`/`ui/Button` → local `Field.vue` + nom-ui `Input`/`Button`.
- amounts → `formatAmount`/`formatAmountExact`; amount inputs → `toBase`.
- `on:click`→`@click`, Svelte reactivity→Vue `computed`/`ref`/`watch`; theme map above.
- `{#if $tx.status==='preparing'}`/`'error'` blocks → `v-if="tx.status==='preparing'"`/`'error'` (the global NomConfirm shows the awaiting/done modal; the panel shows preparing/error inline, as in Svelte).

Each panel test mocks the relevant `NomService.Prepare*` + read bindings and the `tx` store; asserts the primary action calls `Nom.PrepareX(args)` then `tx.awaitConfirm(preview)`, and that a `tx.status='done'` transition triggers `store.refresh()`.

- [ ] **Task 3: `RewardsPanel.vue`** ← `main:.../RewardsPanel.svelte`. Reads stake/pillar/sentinel rewards (useStakeStore/usePillarStore/useSentinelStore); Collect buttons → `Nom.PrepareCollectReward`/`PrepareCollectPillarReward`/`PrepareCollectSentinelReward` → `tx.awaitConfirm`. Test: clicking a Collect calls the right Prepare + awaitConfirm. Run `pnpm test -- src/components/panels/RewardsPanel && pnpm run typecheck`. Stage. No commit.
- [ ] **Task 4: `PlasmaPanel.vue`** ← `main:.../PlasmaPanel.svelte`. usePlasmaStore (info/fusionEntries/estimate); Fuse → `Nom.PrepareFuse(beneficiary, toBase(amount))`→awaitConfirm; Cancel(id) → `Nom.PrepareCancelFuse(id)`→awaitConfirm; live `estimate` on amount. Test: Fuse calls PrepareFuse(base-units) + awaitConfirm. Run, stage, no commit.
- [ ] **Task 5: `PillarPanel.vue`** ← `main:.../PillarPanel.svelte`. usePillarStore (pillars/delegation/reward); Delegate(name)→`PrepareDelegate`, Undelegate→`PrepareUndelegate`, Collect→`PrepareCollectPillarReward`, each →awaitConfirm. Test + run + stage. No commit.
- [ ] **Task 6: `StakingPanel.vue`** ← `main:.../StakingPanel.svelte`. useStakeStore; Stake→`PrepareStake(toBase(amountZnn), durationMonths)`, Cancel(id)→`PrepareCancelStake`, Collect→`PrepareCollectReward`. Test + run + stage. No commit.
- [ ] **Task 7: `SentinelsPanel.vue`** ← `main:.../SentinelsPanel.svelte`. useSentinelStore; Deposit→`PrepareDepositQsr(toBase(qsr))`, Register→`PrepareRegisterSentinel`, Collect→`PrepareCollectSentinelReward`, Revoke→`PrepareRevokeSentinel`, Withdraw→`PrepareWithdrawQsr`. Test + run + stage. No commit.
- [ ] **Task 8: `AcceleratorPanel.vue`** ← `main:.../AcceleratorPanel.svelte` (the largest, ~150 lines). useAcceleratorStore (projects/selectedProject/votablePillars/error); browse (`loadProjects`/`openProject`), Donate→`PrepareDonate(toBase(amount), token)`, Vote→`PrepareVote(id, pillarName, vote)` (with `loadVotablePillars`), create/manage→`PrepareCreateProject`/`PrepareAddPhase`/`PrepareUpdatePhase` (amounts via `toBase`). Each action →awaitConfirm. Port the Svelte sub-views faithfully. Test: Donate calls PrepareDonate + awaitConfirm; Vote calls PrepareVote + awaitConfirm. Run, stage. No commit.

---

## Task 9: Wire panels into Home + integration gate

**Files:** Modify `src/views/Home.vue` (swap the 6 `PanelPlaceholder`s for the real panels); verify.

- [ ] **Step 1: Swap placeholders in `Home.vue`** — replace each `<PanelPlaceholder name="X"/>` in the `Tabs` with the real panel: `<TabsContent value="Rewards"><RewardsPanel/></TabsContent>` … through Accelerator. Remove the now-unused `PanelPlaceholder` import (keep the component file; B4 may not need it).
- [ ] **Step 2: Full gate** — `cd frontend && pnpm test && pnpm run typecheck && pnpm run build` → ALL pass + clean + build OK.
- [ ] **Step 3: Go sanity** — `cd /Users/dfriestedt/Github/go-syrius && GOWORK=off go build ./...` → compiles.
- [ ] **Step 4: Stage** Home.vue. No commit.

---

## Self-Review / Verification (B3)

- `pnpm test` green (stores + NomConfirm + 6 panels + B1/B2/A); `pnpm run typecheck` clean; `pnpm run build` OK.
- **Live `wails dev` gate (controller):** on testnet, exercise one action per panel — Fuse plasma, Stake, Delegate, Register/Deposit sentinel, Donate/Vote accelerator, Collect a reward — each showing the **CallPreview summary + exact amount in the global confirm Dialog** before publish; the panel refreshes after.
- **Funds-safety:** every NoM action routes through `tx.awaitConfirm`→TxModal→`ConfirmPublish` (no bypass); the `tx` store resets on route change.
- No `app/*.go`/`internal/*` changes; `go.mod` 2.12.0 churn not committed.

## Hand-off to B4

B4: real Settings (node modes/data-dir/auto-receive default/reveal-mnemonic/change-password) replacing the placeholder `settings` route; the Tokens-management route (issue/mint/burn/update) replacing the `/tokens` placeholder; final parity pass over the A-review + B-review minors; CI green; then the branch MERGES to main, replacing the Svelte frontend.
