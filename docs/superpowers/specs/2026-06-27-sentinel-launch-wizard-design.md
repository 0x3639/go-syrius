# Sentinel launch wizard

**Date:** 2026-06-27
**Branch:** `ui-ux-fixes`
**Status:** Approved (brainstorming) — ready for implementation plan

## Goal

Replace the single-panel Sentinel registration with a **stepped wizard** that
makes the two-stage launch (deposit 50,000 QSR → register with 5,000 ZNN) legible,
shows explicit "waiting for the network to confirm" states between stages, offers
a withdraw escape hatch after the QSR clears, and ends in a clear "active" state.
Also give the active-sentinel management view a matching visual refresh.

## Background — how a Sentinel launches

A Sentinel needs **50,000 QSR + 5,000 ZNN** collateral. The launch is two
on-chain calls to the Sentinel contract, each with a settle lag:

1. **`DepositQsr`** — sends QSR to the contract. The contract must *receive and
   process* the deposit block before `GetDepositedQsr()` reflects it.
2. **`Register`** — sends 5,000 ZNN; activates the Sentinel once processed
   (`GetSentinel().active`). There is no separate "deposit ZNN" call — `Register`
   *is* the ZNN collateral send.

The current `SentinelsPanel.vue` swaps a "Deposit QSR" button for a "Register"
button based on `GetDepositedQsr()`, refreshes once per completed tx, and shows
**no waiting indication** for the settle lag — which is the confusing part.

### Existing backend (unchanged by this work)

`NomService` already exposes everything needed (`app/nom_service.go`):
`PrepareDepositQsr(qsr)`, `PrepareRegisterSentinel()`, `PrepareWithdrawQsr()`,
`PrepareCollectSentinelReward()`, `PrepareRevokeSentinel()`, `GetSentinel()`,
`GetDepositedQsr()`, `GetSentinelReward()`. `SentinelInfo` carries `owner`,
`active`, `isRevocable`, `revokeCooldown`. No Go changes required.

## Constants

- `QSR_REQUIRED = 5000000000000n` (50,000 QSR at 1e8 base units) — already in the panel.
- `ZNN_COLLATERAL` display value: 5,000 ZNN (sent by `Register`; not separately queried).
- `POLL_INTERVAL_MS = 3000` — poll cadence while a step is clearing.
- `SLOW_AFTER_POLLS = 6` — after this many polls without clearing, show the "taking
  longer than usual" hint (keep polling).

## States (derived from chain state + a `pendingStep` flag)

The wizard step is computed from chain state so it resumes correctly across app
restarts; `pendingStep` only drives the transient "clearing" sub-states within a
session.

Inputs: `deposited = BigInt(depositedQsr)`, `active = !!sentinel && sentinel.owner !== ''`,
`pendingStep ∈ {'deposit','register',null}`.

| State | Condition | Body |
|---|---|---|
| **Step 1 — deposit** | `!active && deposited < QSR_REQUIRED && pendingStep !== 'deposit'` | Progress `deposited / 50,000 QSR`; **"Deposit 50,000 QSR"** button (deposits the shortfall) |
| **Step 1 — clearing** | `pendingStep === 'deposit' && deposited < QSR_REQUIRED` | spinner + "Your QSR deposit is on-chain. Waiting for the Sentinel contract to credit it — usually a few momentums." No actions; auto-advances |
| **Step 2 — register** | `!active && deposited >= QSR_REQUIRED && pendingStep !== 'register'` | "✓ 50,000 QSR cleared." + **"Deposit 5,000 ZNN & Launch Sentinel"** button + escape hatch **"Changed your mind? Withdraw your 50,000 QSR"** |
| **Step 2 — clearing** | `pendingStep === 'register' && !active` | spinner + "Launching your Sentinel — waiting for activation…" No actions; auto-advances |
| **Step 3 — active** | `active` | ✓ success banner, then the active-management view |

`pendingStep` self-clears (see Data flow) once the chain reflects the step, so the
"clearing" rows are transient. On restart `pendingStep` is `null`, so the step is
read straight from chain (deposited / active).

## Components

Split `SentinelsPanel.vue` (currently ~190 lines doing both flows) into focused units:

- **`components/panels/SentinelsPanel.vue`** (modified, container)
  - Reads the sentinel store; renders `SentinelActive` when `active`, else `SentinelLaunch`.
  - Keeps the `onMounted(refresh)` + "refresh on tx done" wiring.
- **`components/panels/SentinelLaunch.vue`** (new)
  - The stepper: `StepHeader` + the five state bodies above.
  - Actions: `depositQsr()`, `register()`, `withdrawQsr()` via `tx.awaitConfirm(await Nom.Prepare…)`
    (the existing NoM-confirm pattern — no modal of its own).
  - On a completed action (`tx.status` transitions to `done` after the user's deposit/
    register), sets `sentinelStore.beginPending('deposit'|'register')`.
- **`components/panels/SentinelActive.vue`** (new)
  - Refreshed management view: status, uncollected reward, **Collect** (disabled when
    reward is zero), **Revoke** (disabled with `cooldown {revokeCooldown}s` when
    `!isRevocable`). Same actions as today, matching the wizard's card styling.
- **`components/panels/StepHeader.vue`** (new, small)
  - Props: `current: 1|2|3`. Renders the three-dot progress with completed steps
    checked and the current step highlighted. Pure presentational.

## Store changes — `stores/sentinel.ts`

Add the pending/poll machinery (kept in the store so polling survives panel
re-renders and is unit-testable):

- State: `pendingStep: 'deposit' | 'register' | null` (default `null`), `pollCount: number`
  (default `0`); a private poll handle.
- `beginPending(step)`: set `pendingStep = step`, `pollCount = 0`, then start an
  interval (`POLL_INTERVAL_MS`) that increments `pollCount`, calls `refresh()`, then
  `settleCheck()`.
- `settleCheck()`: clear `pendingStep` (and reset `pollCount = 0`) + stop the interval
  when the chain reflects the step — `deposited ≥ QSR_REQUIRED` for `'deposit'`,
  `active` for `'register'`.
- `stopPolling()`: clears the interval (called on `settle`, and on panel unmount).
- `refresh()` stays as-is (pulls `GetSentinel` / `GetDepositedQsr` / `GetSentinelReward`).

The "slow" hint is shown by the component when `pendingStep !== null && pollCount >= SLOW_AFTER_POLLS`.

## Data flow

```
Step 1 (deposit):
  user clicks Deposit → tx.awaitConfirm(PrepareDepositQsr(shortfall))
  → NomConfirm → publish → tx.status 'done'
  → SentinelLaunch sees done → sentinelStore.beginPending('deposit')
  → store polls refresh() every 3s → when deposited ≥ 50,000:
      settleCheck clears pendingStep → wizard shows Step 2

Step 2 (register):
  user clicks Launch → tx.awaitConfirm(PrepareRegisterSentinel())
  → publish → 'done' → beginPending('register')
  → poll until sentinel.active → settle → Step 3 (active)

Withdraw escape hatch (only in Step 2, not while clearing):
  user clicks Withdraw → tx.awaitConfirm(PrepareWithdrawQsr())
  → publish → 'done' → beginPending('deposit') is NOT set; just refresh()
  → deposited drops below 50,000 → wizard returns to Step 1
```

## Error handling

- Prepare/publish failures surface through the existing `tx.status === 'error'` /
  local `error` ref (kept from the current panel).
- **Slow settle:** after `SLOW_AFTER_POLLS` polls without clearing, show "taking
  longer than usual — the network may be busy" plus a manual **Refresh** button;
  polling continues.
- **Restart mid-clear:** `pendingStep` is not persisted; on reload the step derives
  from chain. If a deposit hadn't yet credited, the user sees Step 1 again — the QSR
  is in the contract's pending queue and will credit; re-depositing the shortfall is
  guarded because the button deposits only `QSR_REQUIRED - deposited`.
- **Withdraw** is offered only in the Step 2 "register" body, never during a
  clearing state, so the user can't withdraw a deposit that isn't credited yet.

## Testing

Vitest + @vue/test-utils, mocking `NomService` and/or the store (no live node):

- **`SentinelLaunch.test.ts`**
  - Step 1 body when `deposited = 0`, not active.
  - Step 2 body (Register + Withdraw) when `deposited ≥ 50,000`, not active.
  - "clearing" body (spinner, no action buttons) when `pendingStep` is set and chain
    not yet reflecting it.
  - Clicking Deposit/Register calls `tx.awaitConfirm` with the right prepared call.
  - On `tx.status` → `done` after an action, `beginPending` is called with the right step.
- **`sentinel.store.test.ts`**
  - `beginPending('deposit')` sets `pendingStep`; after a `refresh` that returns
    `deposited ≥ 50,000`, `settleCheck` clears it. Same for `'register'` + `active`
    (use fake timers / inject a stub `refresh`).
- **`SentinelActive.test.ts`**
  - Collect disabled when reward is zero; Revoke disabled with cooldown text when
    `!isRevocable`.
- **`StepHeader.test.ts`**
  - `current` prop marks earlier steps complete and the current one active.

## Out of scope

- No Go/backend changes (all NomService methods already exist).
- Pillar registration (a separate, similar flow) — not touched here.
- Persisting `pendingStep` across restarts (chain-derived state covers resume).

## Acceptance

1. With 0 QSR deposited, the Sentinels tab shows Step 1 with a "Deposit 50,000 QSR"
   action and the 3-step header on step 1.
2. After the deposit publishes, the wizard shows the Step 1 "clearing" state with an
   explanatory message and auto-advances to Step 2 once `GetDepositedQsr ≥ 50,000`.
3. Step 2 offers "Deposit 5,000 ZNN & Launch" and a "Withdraw your 50,000 QSR"
   escape hatch; withdrawing returns to Step 1.
4. After Register publishes, the wizard shows the Step 2 "clearing" state and
   advances to the active view once `GetSentinel().active`.
5. The active view shows status/reward with Collect/Revoke, matching the wizard styling.
6. `pnpm run typecheck`, `pnpm test`, and `pnpm run build` pass.
