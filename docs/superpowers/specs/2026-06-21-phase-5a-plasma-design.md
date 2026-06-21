# Phase 5a — Plasma (Fuse / Cancel) Design

**Date:** 2026-06-21
**Status:** Approved
**Scope:** First sub-phase of Phase 5 (NoM features). Plasma: view plasma/fused QSR, fuse QSR for a beneficiary, and cancel fusion entries. Establishes the **shared embedded-contract-call architecture** (a new `NomService` + a generic contract-call path in TxService) that the remaining Phase-5 subsystems (staking, tokens, pillars, sentinels, accelerator) reuse.

## Goal

Let the user fuse QSR to generate plasma (feeless transactions) for any beneficiary and cancel their own fusion entries, through the existing confirm-what-you-sign / PoW / chain-guard pipeline — with one audited contract-call path the rest of Phase 5 builds on.

## Locked decisions (brainstorming 2026-06-21)

- **Phase-5 decomposition:** six sub-phases, build order 5a Plasma → 5b Staking → 5c Tokens → 5d Pillars → 5e Sentinels → 5f Accelerator. Each is its own spec→plan→implement cycle. Per subsystem: read + core actions.
- **Shared architecture:** a new bound `NomService` builds embedded-contract templates + exposes reads; state-changing actions reuse the existing TxService prepare→confirm→publish (confirm-what-you-sign, PoW, chain-id/mainnet guard, signing). No keys in NomService.
- **Confirm-what-you-sign scope:** the modal renders the built block (to/amount/token/hash) + an action summary; `ConfirmPublish` re-asserts to/zts/amount of the built block. Full ABI-decode re-assertion of the contract `Data` (e.g. fuse beneficiary) is flagged as future hardening, not silently skipped.
- **No SDK / no go-zenon forks:** templates via `client.PlasmaApi.Fuse/Cancel`; publish via the `zenon` facade.

## Context (verified against go-zenon @ v0.0.8-alphanet / SDK @ v0.1.16)

- `client.PlasmaApi.Get(addr) (*PlasmaInfo{CurrentPlasma, MaxPlasma uint64, QsrAmount *big.Int}, error)`.
- `client.PlasmaApi.GetEntriesByAddress(addr, pageIndex, pageSize) (*FusionEntryList, error)`; each `FusionEntry{QsrAmount *big.Int, Beneficiary types.Address, ExpirationHeight uint64, Id types.Hash}` — **no `IsRevocable` field**; revocability is derived from `ExpirationHeight` vs the current momentum height.
- `client.PlasmaApi.GetPlasmaByQsr(qsr *big.Int) *big.Int` (plasma estimate for a QSR amount).
- `client.PlasmaApi.Fuse(beneficiary types.Address, amount *big.Int) *nom.AccountBlock` and `Cancel(id types.Hash) *nom.AccountBlock` — unsigned templates.
- `types.PlasmaContract` (the embedded plasma contract address); `types.QsrTokenStandard`.
- `zenon.PrepareBlock(template, kp)` autofills→PoW→signs ANY `*nom.AccountBlock` without publishing; `RequiresPoW`; `PublishRawTransaction` to broadcast. TxService (Phase 2) already holds a single pending built block and runs prepare→hold→`ConfirmPublish` (which re-asserts to/zts/amount) / `CancelPending`, with the chain-id/mainnet guard and `tx:pow-progress`/`tx:published` events.

## Architecture

### Shared contract-call path

```
NomService.PrepareFuse(beneficiary, qsr) ─ build PlasmaApi.Fuse template ─▶ TxService.prepareCall(template, expect, summary)
                                                                              │ guard → zenon.PrepareBlock (PoW) → hold(block, expect) → CallPreview
TxService.ConfirmPublish()  ── re-assert built block.{To,Zts,Amount}==expect ─▶ PublishRawTransaction ─▶ tx:published
TxService.CancelPending()   ── discard held block
```

NomService owns contract knowledge (addresses, template builders, read mappers); TxService owns the single audited prepare/confirm/publish. The frontend action flow mirrors Send.

### TxService generic call path (`app/tx_service.go`)

- Unexported `callExpect{ to types.Address; zts types.ZenonTokenStandard; amount *big.Int }`.
- `prepareCall(template *nom.AccountBlock, expect callExpect, summary string) (CallPreview, error)` — run the existing `guard()` (chain-id/mainnet) → `zenon.PrepareBlock(template, signingKeyPair())` (emits `tx:pow-progress`) → hold the built block + `expect` (same single-pending-block slot as Send; a new prepare/cancel/publish/lock clears it) → return a `CallPreview` rendered from the built block.
- Extend `ConfirmPublish()` so that for a held *call* it re-asserts the built block's `ToAddress`/`TokenStandard`/`Amount` equal the held `expect` (Send's existing per-request re-assertion is the same shape — unify on `expect`). Same mainnet/chain-match checks and `tx:published` emit.
- `CancelPending()` already discards the held block.

### NomService (`app/nom_service.go`, new, bound)

Depends on NodeService (RPC client + current frontier height) and WalletService (active address); calls TxService for actions.

- `GetPlasmaInfo() (PlasmaInfo, error)` — `PlasmaApi.Get(active)` → `{QsrFused, CurrentPlasma, MaxPlasma}`.
- `GetFusionEntries() ([]FusionEntry, error)` — `PlasmaApi.GetEntriesByAddress(active, 0, 50)` → map each to `{Id, Beneficiary, QsrAmount, ExpirationHeight, IsRevocable}` where `IsRevocable = currentFrontierHeight >= ExpirationHeight`.
- `EstimatePlasma(qsr string) (uint64, error)` — `PlasmaApi.GetPlasmaByQsr(parse(qsr))`.
- `PrepareFuse(beneficiary, qsrAmount string) (CallPreview, error)` — validate beneficiary (`types.ParseAddress`) + amount (>0, ≥ network fuse minimum) → `PlasmaApi.Fuse(addr, amt)` → `tx.prepareCall(template, callExpect{to: types.PlasmaContract, zts: types.QsrTokenStandard, amount: amt}, "Fuse <amt> QSR for <beneficiary>")`.
- `PrepareCancelFuse(id string) (CallPreview, error)` — parse `types.HexToHash` → `PlasmaApi.Cancel(id)` → `tx.prepareCall(template, callExpect{to: types.PlasmaContract, zts: types.QsrTokenStandard, amount: big.NewInt(0)}, "Cancel fusion <id>")`.

### DTOs (`app/dto.go`)

- `PlasmaInfo{ QsrFused string; CurrentPlasma, MaxPlasma uint64 }` (camelCase; QsrFused = base-unit decimal string).
- `FusionEntry{ Id, Beneficiary, QsrAmount string; ExpirationHeight uint64; IsRevocable bool }`.
- `CallPreview{ ToAddress, Zts, Symbol, Amount, Hash, Summary string; UsedPlasma uint64; Difficulty uint64; NeedsPoW bool }` — the contract-call analogue of `SendPreview`, rendered from the built block + action summary.

### Events

Reuse `tx:pow-progress` / `tx:published` (and `wallet:locked` clears the pending call). No new events; the frontend refreshes plasma/entries after publish.

## Frontend

- **Plasma route** (`/plasma`, reachable from dashboard/nav): a header showing CurrentPlasma / MaxPlasma + QSR fused; a **Fuse** form (beneficiary input defaulting to the active address, z1 checksum validation; QSR amount with a live "≈ N plasma" estimate from `EstimatePlasma`) → `PrepareFuse` → reuse **TxModal** (renders the built block + "Fuse …" summary) → `ConfirmPublish` → **TxResult**; a **fusion-entries list** (beneficiary, QSR, expiration) with a per-entry **Cancel** button enabled only when `IsRevocable`.
- **Stores:** a `plasma` store (info + entries) refreshed on mount, on `momentum:tick`, and after publish; reuse the Phase-2 `tx` store + `TxModal`/`TxResult`/`AmountInput`/`AddressDisplay`.
- **nav:** add a `'plasma'` view + a dashboard entry point.

## Error handling

- Invalid beneficiary (z1 checksum) or zero/under-minimum QSR → inline form error; submit disabled.
- Cancel on a non-revocable entry → button disabled (UI) and `PrepareCancelFuse` is only offered for revocable entries; a node rejection surfaces on the result.
- Mainnet attempt while `AllowMainnetSend` off → the guard's "mainnet sending is disabled" error.
- Publish failure → `TxResult` error; held block retained for retry or cancel (existing behavior).

## Testing

- **Backend (Go, offline):** DTO mappers (`PlasmaInfo`, `FusionEntry` incl. `IsRevocable` from height); `PrepareFuse` builds a template with `ToAddress == types.PlasmaContract`, `TokenStandard == QsrTokenStandard`, `Amount ==` requested, and holds it; `prepareCall`/`ConfirmPublish` re-assertion rejects a built block whose to/zts/amount diverge from `expect`; mainnet guard blocks unless `AllowMainnetSend`; invalid beneficiary / zero amount rejected. **Integration (`//go:build integration`, opt-in, testnet):** fuse QSR → confirms on-chain; cancel a revocable entry → confirms.
- **Frontend (Vitest, mocked bindings):** Fuse form validation (bad address, zero/under-min), plasma estimate display, fusion-list Cancel gating (disabled when not revocable), the prepare→confirm→publish store flow (reuses the Phase-2 tx store/modal).
- **Acceptance (manual):** testnet fuse (self + another beneficiary), see plasma rise; cancel a revocable entry, see QSR return; confirm-modal shows the real built block.

## Security

- One audited prepare/confirm/publish path; `ConfirmPublish` re-asserts the built block's to/zts/amount against `expect` before broadcast.
- Mainnet stays behind `AllowMainnetSend` (default false); chain-id guard prevents wrong-network broadcast. Plasma is QSR-only (no ZNN), but the same gating applies.
- No key material in NomService; the mnemonic/keypair stay backend-only in WalletService/TxService; nothing sensitive logged.
- Residual: the contract `Data` (fuse beneficiary) is shown in the summary but not yet ABI-decode-re-asserted at publish — recorded as Phase-5 hardening, applies to every subsystem.

## Exit criteria (5a → 5b)

- View plasma + fused QSR + fusion entries; fuse QSR for self/another beneficiary; cancel a revocable entry — all through confirm-what-you-sign, mainnet-gated.
- The generic `prepareCall` path is in place and reused by NomService (ready for 5b staking).
- `go test ./...` (offline) + frontend unit tests pass; the opt-in testnet integration test passes when run.

## Out of scope (deferred)

- Staking/tokens/pillars/sentinels/accelerator (later Phase-5 sub-phases).
- Full ABI-decode re-assertion of contract `Data` at publish (Phase-5 hardening / Phase 7).
- Plasma auto-top-up; multi-account plasma management beyond the active account.
