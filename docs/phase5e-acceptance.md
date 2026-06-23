# Phase 5e — Tokens / ZTS Acceptance

> Status: **Automated + live read gates PASSED.** Token reads (`GetByOwner` / `GetByZts`) succeed
> against the testnet node, exercising the exact `TokenApi` calls `NomService` uses. Manual GUI
> acceptance (the on-testnet write flow: issue/mint/burn/update) remains PENDING a user run.

## Automated verification (2026-06-22) — PASSED
- Stray-duplicate cleanup (`find … -name '* 2.*' … -exec rm -rf {} +`) — ran clean (exit 0).
- `GOWORK=off go test ./...` — green across all packages: `app`, `internal/compat`,
  `internal/embeddednode`, `internal/sdksmoke`, `internal/version` (root has no test files).
  `GOWORK=off` is required: a parent `/Users/dfriestedt/Github/go.work` references a missing
  sibling module, so bare `go test ./...` fails to load the workspace. Local-env artifact.
- `app` package: Phase 5e cases `TestTokenInfoDTO`, `TestPrepareIssueTokenValidatesInput`,
  `TestTokenTemplateTokenStandards` — PASS (alongside the 5a–5d template/DTO regression tests,
  which still pass).
- `GOWORK=off go build -tags integration ./...` — the opt-in integration test (incl. the new
  `TestReadOnlyTokens`) compiles; exit 0.
- Frontend `pnpm test` — 37/37 across 18 files (incl. `src/routes/Tokens.test.ts`, 2 tests).
- `pnpm run check` — `svelte-check found 0 errors, 0 warnings, and 3 hints` (the 3 hints are
  cosmetic unused-import `fireEvent` in pre-existing test files).
- `pnpm run build` — clean (126 modules transformed).
- `xattr -cr build/bin; GOWORK=off wails build` — compiles + packages + self-signs (darwin/arm64);
  built in ~10s. No binding drift (`git status` shows only the integration test modified;
  bindings regenerated identically).

Covered by tests (offline):
- Token reads: `TokenInfo` → DTO maps name/symbol/domain/tokenStandard/owner/totalSupply/
  maxSupply/decimals/isMintable/isBurnable/isUtility; nil-supply mapping handled
  (`TestTokenInfoDTO`).
- IssueToken: full on-chain field validation — name/symbol/domain format + length, supply/decimals
  bounds, flag coherence (`TestPrepareIssueTokenValidatesInput`).
- Token standard: Issue / Mint / Update use `TokenContract` + `ZnnTokenStandard` (Issue carries the
  1 ZNN issuance fee), while **Burn uses the token's own ZTS** with the burn amount — regression-
  locked against the real `embedded.NewTokenApi(nil)` builders (`TestTokenTemplateTokenStandards`).
  This is the 5e analogue of the 5a/5b/5d standard-mismatch lesson: confirm-what-you-sign renders
  the effect from the built block, so Burn must show the burned token, not ZNN.

## Live read-only smoke (2026-06-22) — PASSED
Run against the testnet node `ws://172.245.236.40:35998` (opt-in integration test
`internal/spike.TestReadOnlyTokens`, `-tags integration`), address
`z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg`.

Observed output (verbatim):
```
=== RUN   TestReadOnlyTokens
    readonly_integration_test.go:167: owned tokens: count=0 returned=0
    readonly_integration_test.go:180: ZNN token: ZNN (ZNN) decimals=8 totalSupply=45687200000000
--- PASS: TestReadOnlyTokens (0.27s)
PASS
ok  	github.com/0x3639/go-syrius/internal/spike	0.786s
```

- `TokenApi.GetByOwner(addr, 0, 50)` succeeded — count=0 (this address owns no tokens; the call
  **succeeding** is the verification, and proves the node exposes the `embedded` RPC namespace).
- `TokenApi.GetByZts(ZnnTokenStandard)` succeeded — returned the well-known ZNN token:
  `ZNN (ZNN) decimals=8 totalSupply=45687200000000`, proving the single-token read path.
- No SDK errors: 5e ships against `znn-sdk-go v0.1.17` (which fixed the `GetDepositedQsr` pointer
  bug surfaced in 5d). The token reads use pointer-correct calls, so they pass cleanly.

Repro:
```
ZNN_NODE_URL="ws://172.245.236.40:35998" ZNN_TEST_ADDR="z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg" \
  GOWORK=off go test -tags integration ./internal/spike/ -run TestReadOnlyTokens -v -count=1
```

## Manual acceptance (Phase 5e gate) — PENDING (user GUI run)
> Node prerequisite (carried from 5b/5c/5d): the connected node must expose the `embedded` RPC
> namespace. A `ledger`-only node returns `embedded.* does not exist` for reads and
> `embedded.* not available` for every PoW-requiring action (issue/mint/burn/update all require
> PoW). Not a wallet bug.

> The on-testnet write flow below has NOT been run by the user for 5e. Each item is PENDING and no
> tx hashes are recorded.

On a testnet node with the `embedded` namespace enabled (Tokens route):
1. [PENDING] Open the Tokens route → see your owned tokens (or "No tokens owned").
2. [PENDING] Issue a token → TxModal shows "Issue token <SYMBOL>" (zts = ZNN, amount 1 ZNN
   issuance fee) → Confirm → after a momentum the token appears in "My tokens".
3. [PENDING] Mint a mintable token to an address → TxModal shows the mint summary
   (zts = ZNN, amount 0) → Confirm → total supply increases.
4. [PENDING] Look up a token by ZTS → info renders; Burn (if burnable) → TxModal shows
   **zts = the token, amount = the burn amount** (NOT ZNN) → Confirm → supply decreases.
5. [PENDING] Update a token → transfer ownership and/or disable a flag → Confirm → change
   reflected.
6. [PENDING] Mainnet guard: with `AllowMainnetSend` false on a mainnet node, the actions are
   blocked (mainnet-gated).

Confirm-modal token-standard check to verify during the manual run (confirm-what-you-sign): the
modal must render the effect from the **built block**, not raw form inputs — **Burn = the token's
own ZTS** (with the burn amount), while **Issue / Mint / Update = ZNN** (Issue carrying the 1 ZNN
issuance fee). The offline regression test `TestTokenTemplateTokenStandards` already locks these
against the SDK builders; `TestPrepareIssueTokenValidatesInput` locks the issue-form validation.

## Security recap
- Reuses the one audited prepare/confirm/publish path; confirm-what-you-sign re-asserts the built
  block's to/zts/amount AND ABI `Data`; mainnet gated by `AllowMainnetSend`; no key material in
  NomService.
- All token call templates verified against the SDK builders (`TestTokenTemplateTokenStandards`) —
  Issue/Mint/Update = ZNN (Issue = 1 ZNN fee), Burn = the burned token's ZTS.
- IssueToken input is fully re-validated server-side (`TestPrepareIssueTokenValidatesInput`) — never
  trust frontend validation.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded —
  `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does
  not human-decode arbitrary params; tracked as Phase-5/7 hardening. Global all-tokens browser
  (`GetAll`) + token-holdings list for burn discovery are tracked follow-ups (not 5e).
