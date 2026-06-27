# Pillar Registration — Design Spec

**Date:** 2026-06-27
**Branch:** `ui-ux-fixes` (or a follow-on)
**Status:** Approved design, pre-implementation
**Models the existing:** Sentinel launch wizard (`SentinelLaunch`/`SentinelActive`/`SentinelsPanel`)

## 1. Goal

Add the ability to **register a Pillar** from the wallet — a capability that does not exist today (the Pillar tab currently only supports delegation: delegate / undelegate / collect). The flow mirrors the Sentinel launch wizard in design and style (stepped wizard, on-chain polling/"clearing" states, escape hatches) but accounts for the additional resources and inputs a Pillar requires.

Scope (confirmed): **registration wizard + a minimal owned-pillar management view** (status / collect reward / revoke). `UpdatePillar` (editing reward config) is **out of scope** for this pass.

## 2. Background — the three resources (verified against go-zenon + znn-sdk-go v0.1.19)

Registering a regular Pillar requires three distinct resources, which the UI must surface clearly and never conflate:

| Resource | Amount | Fate | Source / SDK |
|---|---|---|---|
| **Plasma** | A `Register` block costs **105,000 plasma** (`2 * EmbeddedSimplePlasma`, `2 * 52,500`). | consumed; regenerates over time | Fuse QSR. 1 QSR fused → 2,100 plasma (`PlasmaPerFusionUnit`). Recommend fusing **500 QSR → 1,050,000 plasma** (~10× buffer). In-app fusion already exists (`PrepareFuse`/`GetPlasmaInfo`/`EstimatePlasma`). |
| **QSR registration cost** | Dynamic: `PillarApi.GetQsrRegistrationCost()` — base 150,000 QSR + 10,000 per existing normal pillar. | **Deposited, then permanently BURNED at registration.** Reclaimable via `WithdrawQsr` only if not yet consumed. | User's QSR balance, deposited via `PillarApi.DepositQsr(amount)`. **Not** carried in the Register block. |
| **ZNN collateral** | **15,000 ZNN** (`PillarStakeAmount`, baked into the SDK `Register` template `Amount`). | **Locked, refundable** — returned on `Revoke` (time-gated: 83-day lock, 7-day revoke window per cycle). | User's ZNN balance; sent as the Register block amount. |

### go-zenon validation rules (authoritative; backend re-validates all of these)
- **Pillar name:** non-empty, max 40 chars (`PillarNameLengthMax`), regex `^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$` (alphanumerics with single `-`/`.`/`_` only *between* alphanumerics — no leading/trailing/consecutive separators). Must be unique (`CheckNameAvailability`).
- **Reward percentages:** `giveBlockRewardPercentage` and `giveDelegateRewardPercentage`, each a `uint8` in **0–100** inclusive.
- **Producer address:** must be a valid `z1…` address. (syrius additionally rejects wallet default-seed addresses; we surface a note but default to the active address per the UX decision below.)

### SDK surface (znn-sdk-go v0.1.19 `PillarApi`)
- Reads: `GetDepositedQsr(addr)`, `GetQsrRegistrationCost()`, `GetByOwner(addr)`, `GetByName(name)`, `CheckNameAvailability(name)`, `GetUncollectedReward(addr)`.
- Templates: `Register(name, producerAddress, rewardAddress, blockProducingPercentage, delegationPercentage uint8)` (Amount = 15,000 ZNN, ToAddress = `PillarContract`, auto-set), `DepositQsr(amount)`, `WithdrawQsr()`, `Revoke(name)`, `CollectReward()`.

## 3. Decisions (from brainstorming)

1. **Producer address:** editable field, **defaulting to the active address**, with a note that it should be the pillar node's block-producing address.
2. **Insufficient plasma:** wizard offers an **inline "Fuse 500 QSR" step** (reusing `PrepareFuse`) so the user never leaves the flow.
3. **Scope:** registration wizard **+ minimal owned-pillar view** (status / collect / revoke). No `UpdatePillar`.
4. **Percentages:** number inputs (0–100), not sliders (matches the wallet's minimal style).
5. **Plasma gate threshold:** the real go-zenon `Register` cost, **105,000 plasma** (syrius uses a conservative 252,000; we use the verified actual and recommend fusing 500 QSR for margin).

## 4. Architecture

### 4.1 Backend — new `NomService` methods (`app/nom_service.go`)

Every method uses the existing guard pattern (`currentClient()` nil → "not connected"; `activeAddress()` → `errLocked`) and routes writes through `s.tx.prepareCall(template, callExpect{to: types.PillarContract, ...}, summary)`. **Amounts are read from the SDK template, never hardcoded.** All inputs are re-validated server-side (never trust the frontend).

Reads:
- `GetMyPillar() (OwnedPillarInfo, error)` — `client.PillarApi.GetByOwner(addr)`; returns the first owned pillar mapped to a DTO, or an empty DTO (empty `Name`/`Owner` = "none").
- `GetPillarDepositedQsr() (string, error)` — `client.PillarApi.GetDepositedQsr(addr)` → base-unit string, "0" if nil.
- `GetPillarQsrCost() (string, error)` — `client.PillarApi.GetQsrRegistrationCost()` → base-unit string.
- `CheckPillarName(name string) (bool, error)` — validates the name statically (regex + length) then `client.PillarApi.CheckNameAvailability(name)`.

Writes (all return `CallPreview`):
- `PreparePillarDepositQsr(qsr string) (CallPreview, error)` — validate amount > 0; `client.PillarApi.DepositQsr(amt)`; `callExpect{to: PillarContract, zts: QsrTokenStandard, amount: amt, data: template.Data}`; summary `"Deposit %s QSR for pillar (will be burned on registration)"`.
- `PreparePillarWithdrawQsr() (CallPreview, error)` — `client.PillarApi.WithdrawQsr()`; amount 0; summary `"Withdraw deposited pillar QSR"`.
- `PrepareRegisterPillar(name, producer, reward string, blockPct, delegatePct uint8) (CallPreview, error)` — validate: name (regex + length), `producer`/`reward` parse as addresses, `blockPct`/`delegatePct` ≤ 100. Build `client.PillarApi.Register(name, producerAddr, rewardAddr, blockPct, delegatePct)`; `callExpect{to: PillarContract, zts: ZnnTokenStandard, amount: template.Amount, data: template.Data}`; summary `"Register pillar %q (15,000 ZNN)"`.
- `PrepareRevokePillar(name string) (CallPreview, error)` — `client.PillarApi.Revoke(name)`; amount 0; summary `"Revoke pillar %q"`.

Reuse (already present): `PrepareCollectPillarReward()` and `GetPillarReward()` collect/report the address's uncollected reward — used by the owned-pillar view as-is. Reuse `GetPlasmaInfo()`, `EstimatePlasma(qsr)`, `PrepareFuse(beneficiary, qsr)` for the plasma step.

### 4.2 DTO (`app/dto.go`)

```go
type OwnedPillarInfo struct {
    Name              string
    OwnerAddress      string
    ProducerAddress   string
    RewardAddress     string
    GiveBlockRewardPct uint8
    GiveDelegateRewardPct uint8
    IsRevocable       bool
    RevokeCooldown    int64
    // plus any rank/weight/active fields exposed by SDK PillarInfo as needed for the status pill
}
```
Empty `Name` ⇒ the address owns no pillar. Regenerate Wails bindings (`wails generate module` / `wails build`).

### 4.3 Store (`frontend/src/stores/pillar.ts`)

Extend the existing store (keep delegation state untouched) with registration state mirroring `sentinel.ts`:
- State: `myPillar: app.OwnedPillarInfo | null`, `depositedQsr: '0'`, `qsrCost: '0'`, `plasma: app.PlasmaInfo | null`, `pendingStep: 'plasma' | 'deposit' | 'register' | null`, `pollCount: number`, `pollHandle: number | null`.
- Getters: `ownsPillar` (= `!!myPillar && myPillar.name !== ''`), `qsrCleared` (= `BigInt(depositedQsr) >= BigInt(qsrCost)`), `plasmaCleared` (= `BigInt(plasma.currentPlasma) >= PILLAR_PLASMA_REQUIRED`).
- Constants: `PILLAR_PLASMA_REQUIRED = 105000n`, `FUSE_RECOMMENDED_QSR = '500'`, `POLL_INTERVAL_MS = 3000`, `SLOW_AFTER_POLLS = 6`.
- Actions: `refreshRegistration()` (reads `GetMyPillar`, `GetPillarDepositedQsr`, `GetPillarQsrCost`, `GetPlasmaInfo`, `GetPillarReward`; swallows locked/disconnected errors), `beginPending(step)`, `settleCheck()` (stops when `plasma`→`plasmaCleared`, `deposit`→`qsrCleared`, `register`→`ownsPillar`), `stopPolling()`.

### 4.4 Components (`frontend/src/components/panels/`)

- **`StepHeader.vue`** — parameterize: add optional `steps?: { n: number; label: string }[]` prop, defaulting to the current Sentinel labels for backward compatibility. Pillar passes its own three labels.
- **`PillarLaunch.vue`** — the 3-step wizard (detailed in §5). Reuses the `lastAction` module variable + `watch(tx.status)` settle-watcher + `beginPending`/`stopPolling` pattern verbatim from `SentinelLaunch`.
- **`PillarActive.vue`** — owned-pillar view: status pill, name, producer/reward addresses, percentages, uncollected reward; **Collect reward** (disabled when reward zero, reuses `PrepareCollectPillarReward`) and **Revoke** (disabled unless `isRevocable`, with cooldown note, calls `PrepareRevokePillar(name)`).
- **Pillar tab container** — restructure the Pillar tab into a container with two nom-ui `Tabs` sub-sections:
  - **"Delegate"** — the existing delegation UI (current `PillarPanel` content), untouched.
  - **"Run a Pillar"** — shows `PillarActive` if `ownsPillar`, else `PillarLaunch`. Refreshes registration state on mount, stops polling on unmount.

  Implementation note: keep the existing delegation markup in a `PillarDelegate.vue` (extracted from current `PillarPanel.vue`) and make `PillarPanel.vue` the container, OR add the sub-tabs inside the current `PillarPanel.vue`. Prefer extraction for file focus.

### 4.5 Helpers (`frontend/src/lib/format.ts` or a new validation module)

- `isValidPillarName(name: string): boolean` — mirrors the go-zenon regex + length for instant client-side feedback. Backend + `CheckNameAvailability` remain authoritative.

## 5. The wizard flow (`PillarLaunch.vue`)

`<StepHeader :steps="PILLAR_STEPS" :current="currentStep" />` where `PILLAR_STEPS = ['Fuse plasma', 'Deposit QSR', 'Configure & register']`. Step is **derived from chain state**, never stored: `currentStep = plasmaCleared ? (qsrCleared ? 3 : 2) : 1`. A single `clearing` state (`pendingStep !== null`) shows a spinner + contextual message; when `slow` (`pollCount >= SLOW_AFTER_POLLS`), show a "network may be busy" note + **Refresh** / **Stop waiting** buttons (as in Sentinel).

- **Step 1 — Fuse plasma.** If `plasmaCleared`, auto-advances (shows "✓ sufficient plasma"). Else: explainer ("A pillar registration needs ~105,000 plasma. We recommend fusing 500 QSR."), current plasma readout, and a **Fuse 500 QSR** button → `tx.awaitConfirm(await Nom.PrepareFuse(activeAddr, '500'))`, `lastAction = 'plasma'`. After publish, `beginPending('plasma')` polls until plasma lands.
- **Step 2 — Deposit QSR.** Reads `qsrCost` + `depositedQsr`. Shows "Deposited X / {cost} QSR". **⚠ Warning:** "Deposited QSR is **burned and unrecoverable** when the pillar is registered." Button **Deposit {shortfall} QSR** (top-up = `cost − deposited`) → `PreparePillarDepositQsr(shortfall)`, `lastAction = 'deposit'`. Outline escape hatch **Withdraw deposited QSR** → `PreparePillarWithdrawQsr()` (`lastAction = null`, no polling, just refresh). Gates Step 3 until `qsrCleared`.
- **Step 3 — Configure & register.** Form:
  - Pillar name (live `isValidPillarName` + async `CheckPillarName` for uniqueness; show inline validity/availability).
  - Producer address (default active address, editable; note "your pillar node's block-producing address").
  - Reward address (default active address, editable).
  - Momentum (block) reward % (0–100) and Delegate reward % (0–100).
  - **Deposit 15,000 ZNN & Register Pillar** button → `PrepareRegisterPillar(name, producer, reward, blockPct, delegatePct)`, `lastAction = 'register'`. After publish, `beginPending('register')` polls until `ownsPillar`, then the container swaps to `PillarActive`.

Settle watcher (verbatim Sentinel pattern):
```ts
watch(() => tx.status, (s) => {
  if (s === 'idle' || s === 'error') { lastAction = null; return }
  if (s !== 'done') return
  if (lastAction === 'plasma' || lastAction === 'deposit' || lastAction === 'register')
    pillarStore.beginPending(lastAction)
  else pillarStore.refreshRegistration()
  lastAction = null
})
```

### Edge case — cost drift
The QSR cost can rise (+10,000) if another pillar registers between deposit and register, causing `Register` to fail with `ErrNotEnoughDepositedQsr`. Mitigation: Step 3 re-reads `qsrCost` and keeps the register button gated on `qsrCleared`; if a register fails for this reason, surface a clear error and the wizard naturally drops back to Step 2 for a top-up.

## 6. Security / compatibility invariants (per CLAUDE.md)

- No secrets cross into the WebView; the frontend sends intent, Go builds → PoWs → signs → publishes.
- **Confirm-what-you-sign:** the NoM confirm dialog renders the effect from the built block (`callExpect`/`assertMatches`), not raw form inputs — already enforced by `prepareCall`/`ConfirmPublish`.
- Backend re-validates name (regex+length), percentages (≤100), and address parsing independently of frontend validation.
- Register block amount comes from the SDK template (15,000 ZNN), never hardcoded.
- Testnet-gated; `prepareCall` guards mainnet.

## 7. Testing

Mirror Sentinel coverage:
- `frontend/src/components/panels/PillarLaunch.test.ts` — step derivation; fuse action + begins polling; deposit top-up amount + burn warning visible; withdraw escape (no polling); register forwards correct args; clearing hides actions; slow → "Stop waiting" calls `stopPolling`; name validation blocks register when invalid/taken.
- `frontend/src/components/panels/PillarActive.test.ts` — Collect disabled when reward zero; Revoke disabled + cooldown note when not revocable; collect/revoke forward to `tx.awaitConfirm`.
- Container test — renders Delegate vs Run-a-Pillar sub-tabs; shows `PillarActive` when owned else `PillarLaunch`; stops polling on unmount.
- `frontend/src/stores/pillar.test.ts` — `beginPending('plasma'|'deposit'|'register')` clears at the right threshold; `stopPolling` resets state.
- Backend `app/nom_service_test.go` — name regex (valid/invalid/too-long), percentage range, address parse failure, deposit amount > 0, withdraw/register template shape (`callExpect` to PillarContract, correct token/amount).

## 8. Out of scope

- `UpdatePillar` (edit reward config / addresses).
- `RegisterLegacy` (legacy-swap pillars; requires public key + signature).
- Any change to the existing delegation flow beyond extracting it into `PillarDelegate.vue`.
