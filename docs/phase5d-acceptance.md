# Phase 5d — Sentinel Lifecycle Acceptance

> Status: **Automated + live read gates PASSED.** The live read smoke initially surfaced a
> real upstream-SDK defect (`GetDepositedQsr` passed its result by value, not pointer); the SDK
> was patched and re-pinned, and the smoke now passes including `GetDepositedQsr`. Manual GUI
> acceptance (the on-testnet write flow) remains PENDING a user run.

## Automated verification (2026-06-22) — PASSED (against the patched SDK)
- Stray-duplicate cleanup (`find … -name '* 2.*' … -exec rm -rf {} +`) — ran clean.
- `GOWORK=off go test ./...` — green across all packages: `app`, `internal/compat`,
  `internal/embeddednode`, `internal/sdksmoke`, `internal/version` (root has no test files).
  `GOWORK=off` is required: a parent `/Users/dfriestedt/Github/go.work` references a missing
  sibling module, so bare `go test ./...` fails to load the workspace. Local-env artifact.
- `app` package: Phase 5d cases `TestSentinelTemplateTokenStandards`, `TestSentinelDTO` — PASS.
- `GOWORK=off go build -tags integration ./...` — the opt-in integration test (incl. the new
  `TestReadOnlySentinels`) compiles; exit 0.
- Frontend `pnpm test` — 33/33 across 17 files (incl. `src/routes/Sentinels.test.ts`, 3 tests).
- `pnpm run check` — `svelte-check found 0 errors, 0 warnings, and 3 hints` (the 3 hints are
  cosmetic unused-import `fireEvent` in pre-existing test files).
- `pnpm run build` — clean (124 modules transformed).
- `xattr -cr build/bin; GOWORK=off wails build` — compiles + packages + self-signs (darwin/arm64);
  no binding drift (only `go.mod`/`go.sum` modified, for the SDK re-pin).

Covered by tests (offline):
- Sentinel reads: `SentinelInfo` → DTO maps owner/registrationTimestamp/isRevocable/
  revokeCooldown/active; empty-owner → "no sentinel" mapping (`TestSentinelDTO`).
- Token standard: Deposit uses `SentinelContract` + `QsrTokenStandard`; Register/Collect/Revoke/
  Withdraw use the correct standards/amounts — regression-locked against the real
  `embedded.NewSentinelApi(nil)` builders (`TestSentinelTemplateTokenStandards`); Register amount
  locked to 5,000 ZNN from the template (Deposit=QSR vs Register=5,000 ZNN, the 5a/5b lesson).

## Upstream SDK fix (2026-06-22) — GetDepositedQsr pointer bug
The first live smoke run failed `GetDepositedQsr` for every address with
`call result parameter must be pointer or nil interface`. Root cause in `znn-sdk-go v0.1.16`:
`SentinelApi.GetDepositedQsr`, `PillarApi.GetDepositedQsr`, and `PillarApi.GetQsrRegistrationCost`
passed the result target **by value** (`Call(ans, …)`) instead of by pointer (`Call(&ans, …)`),
so they always errored — data-independent, not specific to the zero-deposit case. The sibling
reads (`GetByOwner`/`GetUncollectedReward`) pass a `new(...)` pointer and worked. 5c never
exercised `PillarApi.GetDepositedQsr`, so the defect surfaced only with this sentinel smoke.

Fix: patched all three call sites to `Call(&ans, …)` in `znn-sdk-go`, fast-forwarded `master`, and
tagged/pushed **`v0.1.17`** (commit `8a52da8`, = v0.1.16 + the pointer fix; no other changes).
go-syrius now pins `github.com/0x3639/znn-sdk-go v0.1.17` in `go.mod` — the interim local `replace`
directive has been removed. This also fixes the latent `PillarApi.GetDepositedQsr` /
`GetQsrRegistrationCost` bugs for future phases. Re-pin **finalized** — no follow-up needed.

Verified against the tagged `v0.1.17`: `go build ./...`, `go test ./...`, and the live smoke
(`GetDepositedQsr` returns `0` cleanly) all pass.

## Live read-only smoke (2026-06-22) — PASSED (against the patched SDK)
Run against the testnet node `ws://172.245.236.40:35998` (opt-in integration test
`internal/spike.TestReadOnlySentinels`, `-tags integration`), three testnet addresses. None has a
sentinel registered (expected — same read-probe addresses used in 5c).

Per-address observed result (all three reads PASS):

| Address | GetByOwner | GetDepositedQsr | GetUncollectedReward |
|---|---|---|---|
| `z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg` | regTs=0 active=false isRevocable=false cooldown=0 (no sentinel) | **0** | znn=0 qsr=0 |
| `z1qzu5wkg93qlsk24w5cjkg7w9y0q42e5g7dvgpn` | regTs=0 active=false isRevocable=false cooldown=0 | **0** | znn=0 qsr=0 |
| `z1qqsfews4dyjghnqh4l5jp6y7qz70j4a6d4a8ec` | regTs=0 active=false isRevocable=false cooldown=0 | **0** | znn=0 qsr=0 |

`GetByOwner` succeeding proves the node exposes the `embedded` RPC namespace. `GetDepositedQsr`
now returns `0` cleanly (previously errored) — the pointer fix is confirmed live. The addresses
hold no deposit so the value is 0; the call **succeeding** is the verification.

Repro:
```
ZNN_NODE_URL=ws://172.245.236.40:35998 ZNN_TEST_ADDR=<z1…> \
  GOWORK=off go test -tags integration ./internal/spike -run TestReadOnlySentinels -v -count=1
```

## Manual acceptance (Phase 5d gate) — PENDING (user GUI run)
> Node prerequisite (carried from 5b/5c): the connected node must expose the `embedded` RPC
> namespace. A `ledger`-only node returns `embedded.* does not exist` for reads and
> `embedded.* not available` for every PoW-requiring action (deposit/register/collect/revoke/
> withdraw all require PoW). Not a wallet bug.

> The on-testnet write flow below has NOT been run by the user for 5d. Each item is PENDING and no
> tx hashes are recorded. (The SDK `GetDepositedQsr` defect that previously blocked items 2/6 is
> now fixed, so the deposited-total display / Register-threshold gating renders correctly.)

On a testnet node with the `embedded` namespace enabled (Sentinels route):
1. [PENDING] Open the Sentinels route → see "Register a Sentinel" (or your active sentinel) +
   escrowed QSR + uncollected reward.
2. [PENDING] Deposit QSR → TxModal shows "Deposit … QSR for sentinel" (zts = QSR) → Confirm → after
   a momentum the deposited total advances; once ≥ 50,000 the Register button appears.
3. [PENDING] Register → TxModal shows "Register sentinel (5,000 ZNN)" (zts = ZNN, amount 5,000) →
   Confirm → the sentinel shows as active.
4. [PENDING] Collect rewards (when uncollected > 0) → reward arrives.
5. [PENDING] Revoke (after the 27-day cooldown, when IsRevocable) → collateral returns.
6. [PENDING] WithdrawQsr (escrowed-but-unregistered) → QSR returns.
7. [PENDING] Mainnet guard: with `AllowMainnetSend` false on a mainnet node, the actions are
   blocked (mainnet-gated).

Confirm-modal token-standard check to verify during the manual run (confirm-what-you-sign): the
modal must render the effect from the **built block**, not raw form inputs — **Deposit = QSR**,
while **Register / Collect / Revoke / Withdraw = ZNN**. The offline regression test
`TestSentinelTemplateTokenStandards` already locks these against the SDK builders.

## Security recap
- Reuses the one audited prepare/confirm/publish path; confirm-what-you-sign re-asserts the built
  block's to/zts/amount AND ABI `Data`; mainnet gated by `AllowMainnetSend`; no key material in
  NomService.
- All five sentinel call templates verified against the SDK builders
  (`TestSentinelTemplateTokenStandards`) — Deposit=QSR, Register=5,000 ZNN, etc.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded —
  `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does
  not human-decode arbitrary params; tracked as Phase-5/7 hardening.
