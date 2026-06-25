# Vue migration — Sub-project B, Phase B3: NoM panels — design

**Date:** 2026-06-24
**Branch:** `frontend-vue-migration` (continues after B2, `4ea632e`)
**Parent:** `docs/superpowers/specs/2026-06-24-frontend-vue-migration-design.md`. Sub-project B is decomposed B1–B4; **this spec covers B3**. B4 (Settings + Tokens-management + parity/merge) gets its own spec.

## Context

B2 delivered the Home with the Tokens tab and the Send/Receive funds flows, leaving the other 6 tabs as `PanelPlaceholder`s. **B3 fills those 6 tabs** — Rewards, Plasma, Pillar, Staking, Sentinels, Accelerator — porting the merged Svelte panels (`main:frontend/src/lib/components/panels/*.svelte`) onto Vue + nom-ui, plus their NomService-backed Pinia stores. Each panel performs **NoM embedded-contract calls** (fuse plasma, stake, delegate, register sentinel, donate/vote, collect rewards).

The Go backend + Wails bindings are unchanged. B3 reuses B2's confirm-what-you-sign machinery for every NoM call.

## The NoM-call confirm pattern (the core of B3)

Every NoM action follows the pattern the Svelte panels established (e.g. `PlasmaPanel.svelte`):

1. The panel calls `NomService.PrepareX(...)` — Go **builds the embedded-contract call block** and returns a `CallPreview` (a superset of `SendPreview`: it adds a human-readable `summary`, e.g. "Fuse 50 QSR to z1…").
2. The panel calls `tx.awaitConfirm(preview)` (the B2 `tx` store action) — seating the built block's preview into the shared `tx` flow.
3. A **global `TxModal`** (in Home) renders the preview — the `summary`, the exact amount via `formatAmountExact`, the fee, the hash — **confirm-what-you-sign**. The user confirms → `tx.confirm()` → `ConfirmPublish()` publishes the Go-held block. Cancel → `CancelPending()`.
4. The panel refreshes its store when `tx.status === 'done'`.

**Funds-safety:** identical to B2 — the frontend sends intent (the Prepare call's args), Go builds/signs/publishes, and the user confirms the **built block** (not the form). No NoM call bypasses `tx.confirm`→`ConfirmPublish`.

## Wiring added to Home + the B2-minor fix

- **Global `TxModal`/`TxResult` in `Home.vue`:** add them gated `tx.status` + `!sendOpen && !receiveOpen` (the Svelte Home pattern) so panel confirms render without clashing with the Send/Receive dialogs. (B2's Home omitted these because the panels were placeholders.)
- **Reset `tx` on route change** (the carried-forward B2 review minor): a router `afterEach` (or App-level watch on `route`) calls `useTxStore().reset()`, so a half-built NoM block can't surface on another screen. B2 only reset on tab change; B3 makes it route-wide before NoM panels can produce pending blocks.

## Stores (`src/stores/`, Pinia)

- **Extend `pillar`** (B2 had `delegation` only): add `pillars: PillarSummary[]` (`GetPillarList`), `reward: RewardInfo` (`GetPillarReward`); keep `delegation` (`GetDelegation`). `refresh()` loads all three.
- **Extend `plasma`** (B2 had `info` only): add `fusionEntries: FusionEntry[]` (`GetFusionEntries`) and an `estimate(qsr)` action (`EstimatePlasma`).
- **New `stake`:** `stakeInfo` (`GetStakeList`), `reward` (`GetUncollectedReward`); `refresh()`.
- **New `sentinel`:** `sentinel` (`GetSentinel`), `depositedQsr` (`GetDepositedQsr`), `reward` (`GetSentinelReward`); `refresh()`.
- **New `accelerator`:** `projects` (`GetProjects(page,20)`), `selectedProject` (`GetProject(id)`), `votablePillars` (`GetVotablePillars`), `error`; `loadProjects(page)`, `openProject(id)`, `loadVotablePillars()`.

Read stores follow the B2 try/catch→leave-as-is convention; the panels surface action errors via the `tx` store's `error`.

## The 6 panels (`src/components/panels/`, faithful ports, nom-ui presentation)

Each is a tab component; reads its store (refresh on mount), runs actions via `Nom.PrepareX → tx.awaitConfirm`, refreshes on `tx.status==='done'`, shows `tx.status==='preparing'/'error'`. Amounts via `formatAmount`/`formatAmountExact`; `toBase` for amount inputs. nom-ui `Field`/`Input`/`Button` + `Address`/`TokenIcon` where they fit.

- **RewardsPanel** — uncollected stake/pillar/sentinel rewards + Collect buttons (`PrepareCollectReward`/`PrepareCollectPillarReward`/`PrepareCollectSentinelReward`).
- **PlasmaPanel** — current/max plasma + QSR fused; Fuse (`PrepareFuse(beneficiary, toBase(qsr))`, with `EstimatePlasma`) + fusion list with Cancel (`PrepareCancelFuse(id)`, `isRevocable`).
- **PillarPanel** — pillar list + current delegation + reward; Delegate (`PrepareDelegate(name)`), Undelegate (`PrepareUndelegate`), Collect (`PrepareCollectPillarReward`).
- **StakingPanel** — stake list + reward; Stake (`PrepareStake(amountZnn, durationMonths)`), Cancel (`PrepareCancelStake(id)`), Collect (`PrepareCollectReward`).
- **SentinelsPanel** — sentinel status + deposited QSR + reward; Deposit (`PrepareDepositQsr(qsr)`), Register (`PrepareRegisterSentinel`), Collect (`PrepareCollectSentinelReward`), Revoke (`PrepareRevokeSentinel`), Withdraw (`PrepareWithdrawQsr`).
- **AcceleratorPanel** (most complex) — browse projects (`GetProjects` paged) + project detail (`GetProject`); Donate (`PrepareDonate(amount, token)`), Vote (`PrepareVote(id, pillarName, vote)` with `GetVotablePillars`), and create/manage (`PrepareCreateProject`, `PrepareAddPhase`, `PrepareUpdatePhase`). Port the Svelte AcceleratorPanel's sub-views faithfully.

`Home.vue` swaps the 6 `PanelPlaceholder`s for these real panels in the `Tabs`.

## Error handling

- NoM action errors surface through the `tx` store (`status==='error'`, `error`) — rendered by the panel + the global TxModal; never swallowed.
- Read failures (not connected / locked) leave store state as-is (B2 convention).
- The `tx` reset on route change prevents a stale built block from being confirmed on an unrelated screen.

## Testing

Vitest + @vue/test-utils, mocking `NomService.*` bindings + the `tx` store:
- **Per panel:** an action (e.g. PlasmaPanel Fuse) calls `NomService.PrepareFuse` with the right args and then `tx.awaitConfirm(preview)`; a `tx.status==='done'` transition triggers the store refresh. (Assert the binding call + the awaitConfirm, not vacuous renders.)
- **Stores:** each `refresh()`/`load*()` sets state from the mocked bindings.
- **tx route-reset:** navigating between routes calls `tx.reset()`.
- **Home:** the global TxModal renders when `tx.status==='awaiting'` and a panel (not Send/Receive) triggered it.
- Gates: `pnpm test`, `pnpm run typecheck` (vue-tsc), `pnpm run build`; controller live `wails dev` gate — exercise one action per panel on testnet (fuse plasma, stake, delegate, register/deposit sentinel, donate/vote, collect a reward), each confirming the built-block summary in the global TxModal before publish.

## Out of scope (→ B4)

- Settings (node modes, data dir, auto-receive default, reveal-mnemonic, change-password), the Tokens-management route (issue/mint/burn/update), account rename UI beyond AccountSwitcher, the branch merge.

## Risks

- **Accelerator complexity** — the most involved panel (paged browse + project detail + create/manage sub-forms). Port the Svelte AcceleratorPanel's structure faithfully; consider splitting its plan task (browse/donate/vote vs create/manage) if large.
- **CallPreview vs SendPreview shape** — `tx.awaitConfirm` already accepts the `CallPreview` superset (B2); verify `summary` renders in TxModal (B2 TxModal already renders `preview.summary`).
- **nom-ui field/input/button props** — verify against `node_modules/nom-ui/src` (A/B1/B2 discipline); the `Field` wrapper is our local `src/components/Field.vue` (B1).
- **Global TxModal vs SendModal's inline TxModal** — gate the global one with `!sendOpen && !receiveOpen` so the Send confirm isn't double-rendered.
- **Per-panel data refresh on momentum tick** — Home's `refresh()` (B2) reloads balances/txs/unreceived/plasma/pillar-delegation on tick; B3 panels additionally refresh their own store on mount + on `tx.done`. Avoid heavy per-tick NoM reads (panels refresh on mount/done, not every tick).
