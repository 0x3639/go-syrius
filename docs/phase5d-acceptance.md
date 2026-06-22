# Phase 5d — Sentinel Lifecycle Acceptance

> Status: **BLOCKED on the live-read gate.** Automated verification (offline) is green
> and the app builds, but the live read smoke surfaced a real upstream-SDK defect that
> also affects the product read path (`NomService.GetDepositedQsr`). See
> "Live read-only smoke" below. This doc records observed truth — it is **not** a
> passing sign-off. Manual GUI acceptance remains PENDING.

## Automated verification (2026-06-22) — PASSED
- Stray-duplicate cleanup (`find … -name '* 2.*' … -exec rm -rf {} +`) — ran clean (exit 0).
- `GOWORK=off go test ./...` — green across all packages: `app`, `internal/compat`,
  `internal/embeddednode`, `internal/sdksmoke`, `internal/version` (root has no test files).
  `GOWORK=off` is required: a parent `/Users/dfriestedt/Github/go.work` references a missing
  sibling module, so bare `go test ./...` fails to load the workspace. Local-env artifact,
  not a repo issue.
- `app` package, fresh run (`-count=1`): `ok` in 0.732s. Phase 5d cases verified:
  `TestSentinelTemplateTokenStandards`, `TestSentinelDTO` — both PASS.
- `GOWORK=off go build -tags integration ./...` — the opt-in integration test (including the
  new `TestReadOnlySentinels`) compiles; exit 0 (only the unrelated gopsutil cgo
  `kIOMasterPortDefault` deprecation warning is emitted).
- Frontend `pnpm test` — 33/33 across 17 files (incl. `src/routes/Sentinels.test.ts`, 3 tests).
- `pnpm run check` — `svelte-check found 0 errors, 0 warnings, and 3 hints` (the 3 hints are
  cosmetic unused-import `fireEvent` in pre-existing test files).
- `pnpm run build` — clean (124 modules transformed; `dist/assets/index.b4200372.js` 115.32 KiB).
- `xattr -cr build/bin; GOWORK=off wails build` — compiles + packages with NomService bound
  (darwin/arm64); self-signed cleanly. Wails-generated bindings match the committed tree
  (no working-tree drift after build — only the new test file shows as modified).

Covered by tests (offline):
- Sentinel reads: `SentinelInfo` → DTO maps owner/registrationTimestamp/isRevocable/
  revokeCooldown/active; empty-owner → "no sentinel" mapping (`TestSentinelDTO`).
- Token standard: Deposit uses `SentinelContract` + `QsrTokenStandard`; Register/Collect/
  Revoke/Withdraw use the correct standards/amounts — regression-locked against the real
  `embedded.NewSentinelApi(nil)` builders (`TestSentinelTemplateTokenStandards`), applying the
  5a/5b token-standard lesson (Deposit=QSR vs Register=5,000 ZNN).

## Live read-only smoke (2026-06-22) — PARTIAL FAIL (upstream SDK defect)
Run against the testnet node `ws://172.245.236.40:35998` (opt-in integration test
`internal/spike.TestReadOnlySentinels`, `-tags integration`), three testnet addresses.
None of the three has a sentinel registered (expected — these are the same read-probe
addresses used in 5c).

Per-address observed result:

| Address | GetByOwner | GetDepositedQsr | GetUncollectedReward |
|---|---|---|---|
| `z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg` | PASS — regTs=0 active=false isRevocable=false cooldown=0 (empty owner → no sentinel) | **ERR** | PASS — znn=0 qsr=0 |
| `z1qzu5wkg93qlsk24w5cjkg7w9y0q42e5g7dvgpn` | PASS — regTs=0 active=false isRevocable=false cooldown=0 | **ERR** | PASS — znn=0 qsr=0 |
| `z1qqsfews4dyjghnqh4l5jp6y7qz70j4a6d4a8ec` | PASS — regTs=0 active=false isRevocable=false cooldown=0 | **ERR** | PASS — znn=0 qsr=0 |

`GetByOwner` succeeding proves the node exposes the `embedded` RPC namespace (a `ledger`-only
node would return `embedded.* does not exist`). `GetUncollectedReward` also succeeds.

`GetDepositedQsr` fails for **every** address with:
```
GetDepositedQsr: call result parameter must be pointer or nil interface:
```

### Root cause (upstream SDK bug — affects the product path)
In `znn-sdk-go v0.1.16` (the pinned version), `SentinelApi.GetDepositedQsr` passes the
result target **by value** instead of by pointer:
```go
func (sa *SentinelApi) GetDepositedQsr(address types.Address) (*big.Int, error) {
    var ans string
    if err := sa.client.Call(ans, "embedded.sentinel.getDepositedQsr", address); err != nil { // BUG: should be &ans
        return nil, err
    }
    return common.StringToBigInt(ans), nil
}
```
The underlying JSON-RPC client requires the result arg to be a pointer, so this call **always**
fails — it is data-independent (not specific to the zero-deposit case). The sibling
`SentinelApi.GetByOwner` / `GetUncollectedReward` correctly pass a `new(...)` pointer and work.
`PillarApi.GetDepositedQsr` has the **identical** `Call(ans, …)` bug; 5c's pillar smoke never
exercised `GetDepositedQsr` (it ran GetAll/GetDelegatedPillar/GetUncollectedReward only), so the
defect was not surfaced until this sentinel smoke.

This is **not** a wallet/test defect, but it does propagate into the product read path:
`app/nom_service.go` `GetSentinelDepositedQsr` (line ~486) calls the same SDK method and returns
the error verbatim. Its `if q == nil { return "0" }` guard at line ~490 is unreachable because the
SDK errors before returning. The guided register flow (escrowed-QSR threshold display) reads this
value, so the Sentinels route would surface the error instead of "0 QSR deposited" for any account.

Repro:
```
ZNN_NODE_URL=ws://172.245.236.40:35998 ZNN_TEST_ADDR=<z1…> \
  GOWORK=off go test -tags integration ./internal/spike -run TestReadOnlySentinels -v -count=1
```

### Recommended fix (out of scope for Task 5 — verification/acceptance only)
Bump `znn-sdk-go` to a version that passes `&ans` in `GetDepositedQsr` (sentinel + pillar), or
patch the pinned SDK. After the fix, re-run the live smoke; `GetDepositedQsr` should return `0`
for unregistered addresses and the deposited base-unit total otherwise.

## Manual acceptance (Phase 5d gate) — PENDING (user GUI run)
> Node prerequisite (carried from 5b/5c): the connected node must expose the `embedded` RPC
> namespace (whitelisted via go-zenon `RPC.Endpoints`). A node serving only `ledger` returns
> `embedded.* does not exist` for the reads and `embedded.plasma.getRequiredPoWForAccountBlock
> does not available` for every PoW-requiring action (deposit/register/collect/revoke/withdraw all
> require PoW). Not a wallet bug.

> The on-testnet write flow below has NOT been run by the user for 5d. Each item is PENDING and
> no tx hashes are recorded. Do not treat as accepted until the user confirms.
> NOTE: items 2 and 6 (anything reading deposited QSR) are currently blocked by the SDK
> `GetDepositedQsr` defect above — the deposited-total display / Register-threshold gating cannot
> render correctly until the SDK is fixed.

On a testnet node with the `embedded` namespace enabled (Sentinels route):
1. [PENDING] Open the Sentinels route → see "Register a Sentinel" (or your active sentinel) +
   escrowed QSR + uncollected reward.
2. [PENDING / BLOCKED by SDK GetDepositedQsr defect] Deposit QSR → TxModal shows
   "Deposit … QSR for sentinel" (zts = QSR) → Confirm → after a momentum the deposited total
   advances; once ≥ 50,000 the Register button appears.
3. [PENDING] Register → TxModal shows "Register sentinel (5,000 ZNN)" (zts = ZNN, amount 5,000) →
   Confirm → the sentinel shows as active.
4. [PENDING] Collect rewards (when uncollected > 0) → reward arrives.
5. [PENDING] Revoke (after the 27-day cooldown, when IsRevocable) → collateral returns.
6. [PENDING / BLOCKED by SDK GetDepositedQsr defect] WithdrawQsr (escrowed-but-unregistered) →
   QSR returns.
7. [PENDING] Mainnet guard: with `AllowMainnetSend` false on a mainnet node, the actions are
   blocked (mainnet-gated).

Confirm-modal token-standard check to verify during the manual run (confirm-what-you-sign):
the modal must render the effect from the **built block**, not raw form inputs —
**Deposit = QSR**, while **Register / Collect / Revoke / Withdraw = ZNN** standard. The offline
regression test `TestSentinelTemplateTokenStandards` already locks these against the SDK builders.

## Security recap
- Reuses the one audited prepare/confirm/publish path; confirm-what-you-sign re-asserts the built
  block's to/zts/amount AND ABI `Data`; mainnet gated by `AllowMainnetSend`; no key material in
  NomService.
- All five sentinel call templates verified against the SDK builders
  (`TestSentinelTemplateTokenStandards`) — Deposit=QSR, Register=5,000 ZNN, etc.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded —
  `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does
  not human-decode arbitrary params; tracked as Phase-5/7 hardening.
- **New (this phase):** the upstream `GetDepositedQsr` pointer defect (sentinel + pillar) must be
  fixed before the Sentinels route's deposited-total / register-threshold UI is usable on a live
  node.
