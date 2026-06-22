# Phase 5b — Staking (Stake / Cancel / Collect) Design

**Date:** 2026-06-21
**Status:** Approved
**Scope:** Second sub-phase of Phase 5 (NoM features). Staking: view stakes + uncollected rewards, stake ZNN for a duration, cancel a matured stake, and collect accrued QSR rewards. Reuses the shared embedded-contract-call path established in 5a (NomService + `TxService.prepareCall`).

## Goal

Let the user stake ZNN (1–12 months) to earn QSR, see their stakes and uncollected rewards, cancel matured stakes (ZNN returns), and collect rewards — all through the existing confirm-what-you-sign / PoW / chain-guard pipeline.

## Locked decisions (brainstorming 2026-06-21)

- **Actions:** Stake + Cancel + Collect rewards (full feature).
- **Reward UI:** uncollected total (ZNN/QSR) + a Collect button; reward-history list deferred.
- **Duration:** 1–12 months in 30-day units (`StakeTimeUnitSec = 30 days`); min stake 1 ZNN.
- **No SDK / no go-zenon forks:** templates via `client.StakeApi.Stake/Cancel/CollectReward`; publish via the shared `prepareCall`.

## Context (verified against go-zenon @ v0.0.8-alphanet / SDK @ v0.1.16)

- `client.StakeApi.GetEntriesByAddress(addr, 0, 50) (*StakeList{TotalAmount, Count, List []*StakeEntry}, error)`; `StakeEntry{Amount, WeightedAmount *big.Int; StartTimestamp, ExpirationTimestamp int64; Address types.Address; Id types.Hash}` — **no IsMatured field**; maturity is derived from `ExpirationTimestamp` vs chain time.
- `client.StakeApi.GetUncollectedReward(addr) (*UncollectedReward{Address; ZnnAmount, QsrAmount *big.Int}, error)`.
- `client.StakeApi.Stake(durationInSec int64, amount *big.Int) *nom.AccountBlock` — `{ToAddress: StakeContract, TokenStandard: ZnnTokenStandard, Amount: amount, Data: ABIStake.Pack(Stake, durationInSec)}`.
- `client.StakeApi.Cancel(id types.Hash) *nom.AccountBlock` — `{StakeContract, ZnnTokenStandard, Amount: Big0, Data: ABIStake.Pack(CancelStake, id)}`.
- `client.StakeApi.CollectReward() *nom.AccountBlock` — `{StakeContract, ZnnTokenStandard, Amount: Big0, Data: ABIStake.Pack(CollectReward)}`.
- Constants (`go-zenon/vm/constants`): `StakeTimeUnitSec = 30*SecsInDay`; `StakeTimeMinSec = 1 unit`, `StakeTimeMaxSec = 12 units`; `StakeMinAmount = 1 ZNN (1*Decimals)`. `types.StakeContract`, `types.ZnnTokenStandard`.
- All three stake calls use **ZnnTokenStandard** (Stake moves ZNN; Cancel/Collect move 0 ZNN) — verified, applying the 5a Cancel-token lesson.

5a provides the shared path: `TxService.prepareCall(template, callExpect{to, zts, amount, data}, summary)` runs guard→`zenon.PrepareBlock`(PoW)→hold; `ConfirmPublish` re-asserts the built block's `ToAddress`/`TokenStandard`/`Amount`/`Data` against the held `callExpect` (confirm-what-you-sign), mainnet-gated. NomService holds no key material.

## Architecture

Identical shape to 5a plasma. NomService gains stake reads + three action builders; each action builds a `StakeApi` template and delegates to `prepareCall`. The frontend Stake route drives the shared `tx` flow (`awaitConfirm` → `TxModal` → `ConfirmPublish` → `TxResult`).

## Components

### NomService additions (`app/nom_service.go`)

Reads (active address via WalletService, client via NodeService, frontier time via `client.LedgerApi.GetFrontierMomentum().Timestamp`):
- `GetStakeList() (StakeInfo, error)` — `StakeApi.GetEntriesByAddress(active, 0, 50)`; map to `{TotalAmount string; Entries []StakeEntry}` where each `StakeEntry` is `{Id, Amount, StartTimestamp, ExpirationTimestamp, IsMatured}` and `IsMatured = frontierTimestamp >= ExpirationTimestamp`.
- `GetUncollectedReward() (RewardInfo, error)` — `StakeApi.GetUncollectedReward(active)` → `{Znn, Qsr string}` (base-unit decimal strings).

Actions (build template → `prepareCall`; all ZNN; pass `data: append([]byte(nil), template.Data...)` like 5a):
- `PrepareStake(amountZnn, durationMonths string) (CallPreview, error)` — parse amount (≥ `StakeMinAmount`) + months (integer in 1..12) **before** any node use; `durationInSec = months * StakeTimeUnitSec`; `StakeApi.Stake(durationInSec, amt)` → `prepareCall(callExpect{to: StakeContract, zts: ZnnTokenStandard, amount: amt, data}, "Stake <amountZnn> ZNN for <months> months")` (amount formatted human in the summary via the existing `formatBaseAmount`).
- `PrepareCancelStake(id string) (CallPreview, error)` — `types.HexToHash(id)` → `StakeApi.Cancel(hash)` → `prepareCall(callExpect{StakeContract, ZnnTokenStandard, 0, data}, "Cancel stake <id>")`.
- `PrepareCollectReward() (CallPreview, error)` — `StakeApi.CollectReward()` → `prepareCall(callExpect{StakeContract, ZnnTokenStandard, 0, data}, "Collect staking rewards")`.

### DTOs (`app/dto.go`)

- `StakeInfo{ TotalAmount string; Entries []StakeEntry }`.
- `StakeEntry{ Id, Amount string; StartTimestamp, ExpirationTimestamp int64; DurationMonths int; IsMatured bool }` (DurationMonths derived: `round((Expiration-Start)/StakeTimeUnitSec)` for display).
- `RewardInfo{ Znn, Qsr string }`.

## Frontend

- **Stake route** (`/stake`, dashboard link + `'stake'` nav view): a header with total ZNN staked + uncollected reward (ZNN/QSR via `formatAmount(_,8)`) and a **Collect** button (disabled when both reward amounts are 0); a **Stake** form (ZNN amount with `≥ 1 ZNN` validation + `toBase`, a duration **1–12 months** dropdown) → `PrepareStake` → reuse **TxModal**/**TxResult**; a **stakes list** (amount, duration, expiration date) with per-entry **Cancel** enabled only when `IsMatured`.
- **Stores:** a `stake` store (`stakeInfo` + `reward`) refreshed on mount, `momentum:tick`, and after publish; reuse the `tx` store/`awaitConfirm` bridge, `TxModal`/`TxResult`, `formatAmount`, and the `toBase` helper pattern from Plasma/Send.

## Error handling

- Amount < 1 ZNN, non-numeric amount, or duration outside 1–12 → inline form error; submit disabled.
- Cancel only offered/enabled for `IsMatured` entries; a node rejection (e.g. not yet matured) surfaces on the result.
- Collect disabled when uncollected ZNN and QSR are both 0; node rejection surfaces.
- Mainnet attempt while `AllowMainnetSend` off → the guard's "mainnet sending is disabled" error.
- Publish failure → `TxResult` error; held block retained for retry/cancel (existing behavior).

## Testing

- **Backend (Go, offline):** DTO/`IsMatured` mappers (frontier-timestamp derivation; `DurationMonths` rounding); `PrepareStake`/`PrepareCancelStake`/`PrepareCollectReward` validate inputs before any node use (amount ≥ min, months 1–12, bad id rejected); a regression test that builds the **real** SDK `Stake`/`Cancel`/`CollectReward` templates (via `embedded.NewStakeApi(nil)`, offline) and asserts each `TokenStandard == ZnnTokenStandard` and `ToAddress == StakeContract` (mirrors 5a's plasma token-standard guard); mainnet guard blocks unless `AllowMainnetSend`. **Integration (`//go:build integration`, opt-in):** documented skip (manual acceptance is the gate).
- **Frontend (Vitest, mocked bindings):** stake form validation (min amount, duration bounds), Cancel gating on `IsMatured`, Collect disabled when reward is 0, the prepare→confirm→publish store flow.
- **Acceptance (manual):** testnet — stake ZNN (see it appear with duration), collect rewards (QSR arrives), cancel a matured stake (ZNN returns); confirm-modal shows the real built block + human ZNN amount + action summary.

## Security

- Reuses the one audited prepare/confirm/publish path (binds to/zts/amount **and** ABI `Data`); mainnet gated by `AllowMainnetSend`; no key material in NomService.
- Each stake call's token standard verified against the SDK template (all ZNN); amounts shown human-formatted in the summary so confirm-what-you-sign is legible.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded — `assertMatches` binds the exact `Data` bytes the template produced, which prevents tampering, but does not human-decode arbitrary params; tracked as Phase-5/7 hardening.

## Exit criteria (5b → 5c)

- View stakes + uncollected reward; stake ZNN (1–12 months); cancel a matured stake; collect rewards — all through confirm-what-you-sign, mainnet-gated.
- `go test ./...` (offline) + frontend unit tests pass; the opt-in integration test compiles.

## Out of scope (deferred)

- Reward-history list (`GetFrontierRewardByPage`); tokens/pillars/sentinels/accelerator (later sub-phases).
- Auto-restake; partial cancel; multi-account staking beyond the active account.
