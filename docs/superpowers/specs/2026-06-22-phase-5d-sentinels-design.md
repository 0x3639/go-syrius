# Phase 5d — Sentinels (Register / Deposit / Collect / Revoke / Withdraw) Design

**Date:** 2026-06-22
**Status:** Approved
**Scope:** Fourth sub-phase of Phase 5 (NoM features). Full sentinel lifecycle: view the active address's sentinel status + escrowed QSR + uncollected rewards; deposit the QSR collateral; register a sentinel; collect rewards; revoke; and withdraw escrowed-but-unused QSR. Reuses the shared embedded-contract-call path established in 5a/5b/5c (NomService + `TxService.prepareCall`).

## Goal

Let the user create, manage, and exit a sentinel entirely from the wallet — through the existing confirm-what-you-sign / PoW / chain-guard pipeline — handling the two-transaction registration (deposit 50,000 QSR, then register with 5,000 ZNN) with a guided flow that prevents an out-of-order register.

## Locked decisions (brainstorming 2026-06-22)

- **Actions:** full lifecycle — `DepositQsr` + `Register` + `CollectReward` + `Revoke` + `WithdrawQsr`.
- **Register UX:** guided / state-driven. One register card keyed on escrowed QSR vs the 50,000 QSR threshold: under threshold shows **Deposit** (prefilled to the shortfall) plus a **Withdraw deposited QSR** escape hatch when escrowed `> 0`; at/over threshold shows **Register (5,000 ZNN)**. Each step is its own confirm-what-you-sign tx; the card re-reads and advances after each.
- **Register amount:** taken from the **real SDK template** (`SentinelZnnRegisterAmount`, 5,000 ZNN) — never hardcoded in our code.
- **Reward UI:** uncollected total (ZNN/QSR) + a Collect button; reward-history list deferred.
- **No SDK / no go-zenon forks:** templates via `client.SentinelApi.*`; publish via the shared `prepareCall`.

## Context (verified against go-zenon @ v0.0.8-alphanet / SDK @ v0.1.16)

- `client.SentinelApi.GetByOwner(address) (*SentinelInfo{Owner types.Address; RegistrationTimestamp int64; IsRevocable bool; RevokeCooldown int64; Active bool}, error)` — the active address's sentinel. A node with no sentinel for the address returns a zero/empty `Owner`.
- `client.SentinelApi.GetDepositedQsr(address) (*big.Int, error)` — QSR escrowed toward registration (drives the guided flow). Once registration consumes the collateral this returns 0.
- `client.SentinelApi.GetUncollectedReward(address) (*UncollectedReward{Address; ZnnAmount, QsrAmount *big.Int}, error)` — same `UncollectedReward` type used by Stake (5b) / Pillars (5c).
- `client.SentinelApi.Register() *nom.AccountBlock` — `{ToAddress: SentinelContract, TokenStandard: ZnnTokenStandard, Amount: constants.SentinelZnnRegisterAmount (5,000 ZNN), Data: ABISentinel.Pack(Register)}`.
- `client.SentinelApi.DepositQsr(amount *big.Int) *nom.AccountBlock` — `{SentinelContract, **QsrTokenStandard**, Amount: amount, Data: ABISentinel.Pack(DepositQsr)}` — the **only** QSR-token call.
- `client.SentinelApi.Revoke() *nom.AccountBlock` — `{SentinelContract, ZnnTokenStandard, Amount: Big0, Data: ABISentinel.Pack(Revoke)}`.
- `client.SentinelApi.WithdrawQsr() *nom.AccountBlock` — `{SentinelContract, ZnnTokenStandard, Amount: Big0, Data: ABISentinel.Pack(WithdrawQsr)}`.
- `client.SentinelApi.CollectReward() *nom.AccountBlock` — `{SentinelContract, ZnnTokenStandard, Amount: Big0, Data: ABISentinel.Pack(CollectReward)}`.
- Constants: `types.SentinelContract`, `types.ZnnTokenStandard`, `types.QsrTokenStandard`; `SentinelZnnRegisterAmount = 5,000 ZNN`, `SentinelQsrDepositAmount = 50,000 QSR`, `SentinelLockTimeWindow = 27 days` (cooldown before revocable). Decimals = 1e8 for both ZNN and QSR.
- **Token-standard split is the correctness-critical detail (the 5a "Cancel=ZNN" lesson):** `DepositQsr` is QSR; `Register` / `Revoke` / `WithdrawQsr` / `CollectReward` are all ZNN. A regression test locks each call against the real SDK builder.

5a/5b/5c provide the shared path: `TxService.prepareCall(template, callExpect{to, zts, amount, data}, summary)` runs guard→`zenon.PrepareBlock`(PoW)→hold; `ConfirmPublish` re-asserts the built block's `ToAddress`/`TokenStandard`/`Amount`/`Data` against the held `callExpect` (confirm-what-you-sign), mainnet-gated. NomService holds no key material.

## Architecture

Identical shape to 5a/5b/5c. NomService gains sentinel reads + five action builders; each action builds a `SentinelApi` template and delegates to `prepareCall`. The frontend Sentinels route drives the shared `tx` flow (`awaitConfirm` → `TxModal` → `ConfirmPublish` → `TxResult`). No new backend pipeline.

## Components

### NomService additions (`app/nom_service.go`)

Reads (active address via WalletService, client via NodeService; not-connected → error, locked → `errLocked`):
- `GetSentinel() (SentinelInfo, error)` — `SentinelApi.GetByOwner(active)`; map to the DTO. A nil result or zero/empty `Owner` ⇒ `SentinelInfo{}` (the frontend treats empty `Owner` as "no sentinel"). Pure mapper `sentinelDTO(s *embedded.SentinelInfo) SentinelInfo` is unit-tested.
- `GetDepositedQsr() (string, error)` — `SentinelApi.GetDepositedQsr(active)` → base-unit decimal string; nil → "0".
- `GetSentinelReward() (RewardInfo, error)` — `SentinelApi.GetUncollectedReward(active)` → reuse the existing `RewardInfo{Znn, Qsr string}` DTO (base-unit decimal strings).

Actions (build template → `prepareCall`; pass `data: append([]byte(nil), template.Data...)` like 5a/5b/5c; amount snapshotted from the template where fixed):
- `PrepareDepositQsr(qsr string) (CallPreview, error)` — `qsr` is a **base-unit** decimal string (consistent with `PrepareFuse`/`PrepareStake`); parse to big.Int, validate `> 0` **before** any node use; `SentinelApi.DepositQsr(amt)` → `prepareCall(callExpect{to: SentinelContract, zts: **QsrTokenStandard**, amount: amt, data}, summary)`. The summary renders the amount as human-formatted whole QSR (the 5a QSR-units fix), e.g. "Deposit 50,000 QSR for sentinel".
- `PrepareRegisterSentinel() (CallPreview, error)` — `SentinelApi.Register()` → `prepareCall(callExpect{SentinelContract, ZnnTokenStandard, amount: template.Amount, data}, "Register sentinel (5,000 ZNN)")` — amount read from the template, not hardcoded.
- `PrepareCollectSentinelReward() (CallPreview, error)` — `SentinelApi.CollectReward()` → `prepareCall(callExpect{SentinelContract, ZnnTokenStandard, 0, data}, "Collect sentinel rewards")`.
- `PrepareRevokeSentinel() (CallPreview, error)` — `SentinelApi.Revoke()` → `prepareCall(callExpect{SentinelContract, ZnnTokenStandard, 0, data}, "Revoke sentinel")`.
- `PrepareWithdrawQsr() (CallPreview, error)` — `SentinelApi.WithdrawQsr()` → `prepareCall(callExpect{SentinelContract, ZnnTokenStandard, 0, data}, "Withdraw deposited QSR")`.

### DTOs (`app/dto.go`)

- `SentinelInfo{ Owner string; RegistrationTimestamp int64; IsRevocable bool; RevokeCooldown int64; Active bool }` (Owner is the `z1…` string; empty ⇒ no sentinel).
- Reuse `RewardInfo{ Znn, Qsr string }` from 5b for sentinel rewards.

## Frontend

- **Sentinels route** (`/sentinels`, dashboard link + `'sentinels'` nav view), state-driven on the `sentinel` store:
  - **No active sentinel (empty Owner):** a guided **Register a Sentinel** card keyed on `depositedQsr` vs 50,000 QSR:
    - `depositedQsr < 50,000`: **Deposit QSR** button → `PrepareDepositQsr(shortfall)` where `shortfall = 50,000·1e8 − depositedQsr` (prefilled, shown in whole QSR); plus a **Withdraw deposited QSR** button → `PrepareWithdrawQsr()` shown only when `depositedQsr > 0`.
    - `depositedQsr ≥ 50,000`: **Register Sentinel (5,000 ZNN)** button → `PrepareRegisterSentinel()`.
  - **Active sentinel:** status block — registered date (from `RegistrationTimestamp`), `Active`, and remaining cooldown (`RevokeCooldown`); an uncollected-reward line (ZNN/QSR via `formatAmount(_,8)`) with a **Collect** button (disabled when both reward amounts are 0); a **Revoke** button (disabled until `IsRevocable`, showing the cooldown remaining).
  - Drives the shared `awaitConfirm` → `TxModal` → `TxResult` flow; refresh-on-done — mirrors `Pillars.svelte` exactly.
- **Stores:** a `sentinel` store (`sentinel` + `depositedQsr` + `sentinelReward`, refreshed on mount and after publish) using the generated `models.ts` types (the standard set in the post-5c cleanup); reuse the `tx` store/`awaitConfirm` bridge, `TxModal`/`TxResult`, `formatAmount`.
- Dashboard "Sentinels" button + `App.svelte` `'sentinels'` route branch, in the existing style.

## Error handling

- Non-positive / unparseable deposit amount → rejected in Go before any node use; inline form error.
- Register offered only at/over the 50,000 QSR threshold (guided card); a node rejection (e.g. insufficient ZNN) surfaces on the result.
- Collect disabled when uncollected ZNN and QSR are both 0; node rejection surfaces.
- Revoke disabled until `IsRevocable` (27-day cooldown); node rejection surfaces.
- WithdrawQsr offered only when escrowed `> 0` and no active sentinel; node rejection surfaces.
- Mainnet attempt while `AllowMainnetSend` off → the guard's "mainnet sending is disabled" error.
- Publish failure → `TxResult` error; held block retained for retry/cancel (existing behavior).
- Reads tolerate a not-connected / locked node by leaving the store as-is (same pattern as plasma/stake/pillar stores).

## Testing

- **Backend (Go, offline):** `sentinelDTO` mapper (owner/timestamps/flags mapping, empty-Owner → `SentinelInfo{}`); `PrepareDepositQsr` rejects `<= 0` / unparseable before any node use; a regression test that builds the **real** SDK `Register`/`DepositQsr`/`Revoke`/`WithdrawQsr`/`CollectReward` templates (via `embedded.NewSentinelApi(nil)`, offline — confirm the builders construct blocks from args/constants only and don't deref the nil client) and asserts each `ToAddress == SentinelContract`, the correct `TokenStandard` per call (QSR only for DepositQsr; ZNN for the rest), and `Amount` (5,000 ZNN for Register; 0 for Revoke/WithdrawQsr/CollectReward); mainnet guard blocks unless `AllowMainnetSend`. **Integration (`//go:build integration`, opt-in):** extend `internal/spike` read smoke with GetByOwner/GetDepositedQsr/GetUncollectedReward.
- **Frontend (Vitest, mocked bindings):** the guided card shows **Deposit** (with Withdraw escape hatch) when `depositedQsr < 50,000` and **Register** when `≥ 50,000`; active-sentinel view shows status + Collect (disabled at 0 reward) + Revoke (disabled until `IsRevocable`); the prepare→confirm→publish store flow.
- **Acceptance (manual + live read smoke):** testnet — view sentinel status + escrowed QSR + uncollected reward; deposit QSR → register → see it active; collect rewards (when > 0); revoke (after cooldown) → withdraw QSR; confirm-modal shows the real built block + action summary (note Deposit = QSR, others = ZNN); mainnet-gated. Live read smoke against the testnet node records GetByOwner/GetDepositedQsr/GetUncollectedReward output.

## Security

- Reuses the one audited prepare/confirm/publish path (binds to/zts/amount **and** ABI `Data`); mainnet gated by `AllowMainnetSend`; no key material in NomService.
- Each call's token standard verified against the SDK template — **DepositQsr = QSR, all others = ZNN** — and Register's 5,000 ZNN amount read from the template, not our code; confirm-what-you-sign shows the action summary + built block.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded — `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does not human-decode arbitrary params; tracked as Phase-5/7 hardening.

## Exit criteria (5d → 5e)

- View sentinel status + escrowed QSR + uncollected reward; deposit QSR; register; collect; revoke; withdraw QSR — all through confirm-what-you-sign, mainnet-gated.
- `go test ./...` (offline) + frontend unit tests + `svelte-check` pass; the opt-in integration test compiles; live read smoke passes against the testnet node.

## Out of scope (deferred)

- Reward-history list (`GetFrontierRewardByPage`); active-sentinel browser (`GetAllActive`).
- Tokens (ZTS) / Accelerator-Z (later sub-phases).
- Deeper ABI `Data` semantic decode (Phase-5/7 hardening).
