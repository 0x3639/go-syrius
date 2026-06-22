# Phase 5c — Pillar Delegation (Delegate / Undelegate / Collect) Design

**Date:** 2026-06-21
**Status:** Approved
**Scope:** Third sub-phase of Phase 5 (NoM features). Pillar delegation: view available pillars + the active address's current delegation + uncollected delegation rewards, delegate to a pillar, undelegate, and collect accrued rewards. Reuses the shared embedded-contract-call path established in 5a/5b (NomService + `TxService.prepareCall`).

## Goal

Let the user delegate their ZNN weight to a pillar to earn rewards, see the pillar list + their current delegation + uncollected rewards, change/undelegate, and collect rewards — all through the existing confirm-what-you-sign / PoW / chain-guard pipeline.

## Locked decisions (brainstorming 2026-06-21)

- **Actions:** Delegate + Undelegate + Collect rewards (the everyday delegator flow).
- **Out of scope (operator features, deferred):** pillar Register / UpdatePillar / Revoke / DepositQsr / WithdrawQsr — the "optional pillar registration for operators" items from plan.md §3.
- **Pillar picker:** a searchable, rank-sorted list showing per-pillar stats (name, rank, weight, delegate-reward %), each row delegatable. (Not a bare dropdown; not type-the-name.)
- **Reward UI:** uncollected total (ZNN/QSR) + a Collect button; reward-history list deferred.
- **No SDK / no go-zenon forks:** templates via `client.PillarApi.Delegate/Undelegate/CollectReward`; publish via the shared `prepareCall`.

## Context (verified against go-zenon @ v0.0.8-alphanet / SDK @ v0.1.16)

- `client.PillarApi.GetAll(pageIndex, pageSize uint32) (*PillarInfoList{Count int; List []*PillarInfo}, error)`; `PillarInfo{Name string; Rank int32; Type int32; OwnerAddress, ProducerAddress, WithdrawAddress types.Address; GiveMomentumRewardPercentage, GiveDelegateRewardPercentage int32; IsRevocable bool; RevokeCooldown, RevokeTimestamp int64; CurrentStats *PillarEpochStats; Weight *big.Int}`.
- `client.PillarApi.GetDelegatedPillar(address) (*DelegationInfo{Name string; Status int32; Weight *big.Int}, error)` — returns the active address's current delegation. May be nil / empty Name when the address has not delegated.
- `client.PillarApi.GetUncollectedReward(address) (*UncollectedReward{Address; ZnnAmount, QsrAmount *big.Int}, error)` — same `UncollectedReward` type used by Stake (5b).
- `client.PillarApi.Delegate(name string) *nom.AccountBlock` — `{ToAddress: PillarContract, TokenStandard: ZnnTokenStandard, Amount: Big0, Data: ABIPillars.Pack(Delegate, name)}`.
- `client.PillarApi.Undelegate() *nom.AccountBlock` — `{PillarContract, ZnnTokenStandard, Amount: Big0, Data: ABIPillars.Pack(Undelegate)}`.
- `client.PillarApi.CollectReward() *nom.AccountBlock` — `{PillarContract, ZnnTokenStandard, Amount: Big0, Data: ABIPillars.Pack(CollectReward)}`.
- Constants: `types.PillarContract`, `types.ZnnTokenStandard`. All three delegation calls move **0 ZNN** (no funds leave the account; delegation is by weight = the account's ZNN balance).
- All three calls use **ZnnTokenStandard** — verified against the SDK builders; a regression test locks this (mirrors the 5a/5b token-standard guards).

5a/5b provide the shared path: `TxService.prepareCall(template, callExpect{to, zts, amount, data}, summary)` runs guard→`zenon.PrepareBlock`(PoW)→hold; `ConfirmPublish` re-asserts the built block's `ToAddress`/`TokenStandard`/`Amount`/`Data` against the held `callExpect` (confirm-what-you-sign), mainnet-gated. NomService holds no key material.

## Architecture

Identical shape to 5a plasma / 5b staking. NomService gains pillar reads + three action builders; each action builds a `PillarApi` template and delegates to `prepareCall`. The frontend Pillars route drives the shared `tx` flow (`awaitConfirm` → `TxModal` → `ConfirmPublish` → `TxResult`).

## Components

### NomService additions (`app/nom_service.go`)

Reads (active address via WalletService, client via NodeService):
- `GetPillarList() ([]PillarSummary, error)` — `PillarApi.GetAll` paginated until all `Count` entries are fetched (page size 100); map each to `PillarSummary` and sort by `Rank` ascending. Pure mapper `pillarSummaryDTO(p *embedded.PillarInfo) PillarSummary` is unit-tested.
- `GetDelegation() (DelegationInfo, error)` — `PillarApi.GetDelegatedPillar(active)`; map to `{Name, Status, Weight string}`. A nil result or empty `Name` ⇒ `DelegationInfo{}` (the frontend treats empty Name as "not delegated").
- `GetPillarReward() (RewardInfo, error)` — `PillarApi.GetUncollectedReward(active)` → reuse the existing `RewardInfo{Znn, Qsr string}` DTO (base-unit decimal strings).

Actions (build template → `prepareCall`; all ZNN; pass `data: append([]byte(nil), template.Data...)` like 5a/5b):
- `PrepareDelegate(name string) (CallPreview, error)` — validate `name != ""` (trimmed) **before** any node use; `PillarApi.Delegate(name)` → `prepareCall(callExpect{to: PillarContract, zts: ZnnTokenStandard, amount: Big0, data}, "Delegate to <name>")`.
- `PrepareUndelegate() (CallPreview, error)` — `PillarApi.Undelegate()` → `prepareCall(callExpect{PillarContract, ZnnTokenStandard, 0, data}, "Undelegate from current pillar")`.
- `PrepareCollectPillarReward() (CallPreview, error)` — `PillarApi.CollectReward()` → `prepareCall(callExpect{PillarContract, ZnnTokenStandard, 0, data}, "Collect delegation rewards")`.

### DTOs (`app/dto.go`)

- `PillarSummary{ Name string; Rank int; Weight string; DelegateRewardPercent int; ProducerAddress string }` (Weight base-unit decimal string; DelegateRewardPercent from `GiveDelegateRewardPercentage`).
- `DelegationInfo{ Name string; Status int; Weight string }`.
- Reuse `RewardInfo{ Znn, Qsr string }` from 5b for pillar rewards.

## Frontend

- **Pillars route** (`/pillars`, dashboard link + `'pillars'` nav view):
  - **Header / current delegation:** if delegated (non-empty Name), show the pillar name + delegated weight, an **Undelegate** button, and an uncollected-reward line (ZNN/QSR via `formatAmount(_,8)`) with a **Collect** button (disabled when both reward amounts are 0). If not delegated, show a "Not delegated" note (Undelegate hidden).
  - **Pillar list:** a **search box** (case-insensitive filter on name) over a rank-sorted list; each row shows name, rank, weight (`formatAmount(_,8)` ZNN), and delegate-reward %, with a **Delegate** button → `PrepareDelegate(name)` → reuse **TxModal**/**TxResult**. The row matching the current delegation is visually marked (e.g. "current").
- **Stores:** a `pillar` store (`pillars` list + `delegation` + `reward`) refreshed on mount and after publish; reuse the `tx` store/`awaitConfirm` bridge, `TxModal`/`TxResult`, `formatAmount`.

## Error handling

- Empty/whitespace pillar name → inline form error; Delegate disabled.
- Undelegate offered only when currently delegated; a node rejection surfaces on the result.
- Collect disabled when uncollected ZNN and QSR are both 0; node rejection surfaces.
- Mainnet attempt while `AllowMainnetSend` off → the guard's "mainnet sending is disabled" error.
- Publish failure → `TxResult` error; held block retained for retry/cancel (existing behavior).
- Reads tolerate a not-connected / locked node by leaving the store as-is (same pattern as plasma/stake stores).

## Testing

- **Backend (Go, offline):** `pillarSummaryDTO` mapper (rank/weight/percent mapping); `GetPillarList` sort-by-rank ordering (pure helper over a fixed slice); `PrepareDelegate` rejects empty name before any node use; a regression test that builds the **real** SDK `Delegate`/`Undelegate`/`CollectReward` templates (via `embedded.NewPillarApi(nil)`, offline) and asserts each `TokenStandard == ZnnTokenStandard` and `ToAddress == PillarContract` (mirrors 5a/5b guards); mainnet guard blocks unless `AllowMainnetSend`. **Integration (`//go:build integration`, opt-in):** documented skip (manual acceptance is the gate).
- **Frontend (Vitest, mocked bindings):** pillar list renders + sorts by rank + filters on search; Undelegate hidden when `delegation.name` is empty; Collect disabled when reward is 0; the prepare→confirm→publish store flow.
- **Acceptance (manual):** testnet — view pillars + current delegation + uncollected reward; delegate to a pillar (see it marked current); collect rewards (when > 0); undelegate; confirm-modal shows the real built block + action summary; mainnet-gated.

## Security

- Reuses the one audited prepare/confirm/publish path (binds to/zts/amount **and** ABI `Data`); mainnet gated by `AllowMainnetSend`; no key material in NomService.
- Each delegation call's token standard verified against the SDK template (all ZNN, Amount 0); confirm-what-you-sign shows the action summary + built block.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded — `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering, including the delegated pillar name) but does not human-decode arbitrary params; tracked as Phase-5/7 hardening.

## Exit criteria (5c → 5d)

- View pillars + current delegation + uncollected reward; delegate; undelegate; collect — all through confirm-what-you-sign, mainnet-gated.
- `go test ./...` (offline) + frontend unit tests pass; the opt-in integration test compiles.

## Out of scope (deferred)

- Pillar operator features: Register / UpdatePillar / Revoke / DepositQsr / WithdrawQsr.
- Reward-history list (`GetFrontierRewardByPage`); pillar epoch/momentum stats display.
- Tokens / sentinels / accelerator (later sub-phases).
