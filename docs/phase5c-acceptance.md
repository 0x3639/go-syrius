# Phase 5c — Pillar Delegation Acceptance

## Automated verification (2026-06-22) — PASSED
- `GOWORK=off go test ./...` — green across all packages: `app`, `internal/compat`, `internal/embeddednode`, `internal/sdksmoke`, `internal/version` (root has no test files). `GOWORK=off` is required: a parent `/Users/dfriestedt/Github/go.work` references a missing sibling module, so bare `go test ./...` fails to load the workspace. This is a local-env artifact, not a repo issue.
- `app` package, fresh run (`-count=1`): `ok` in 0.945s. Phase 5c cases verified: `TestPillarSummaryDTO`, `TestSortPillarsByRank`, `TestPrepareDelegateValidatesInput`, `TestPillarTemplateTokenStandards` — all PASS.
- `GOWORK=off go build -tags integration ./...` — the opt-in integration test compiles (only the unrelated gopsutil cgo `kIOMasterPortDefault` deprecation warning is emitted).
- Frontend `pnpm test` — 28/28 across 15 files (incl. `src/routes/Pillars.test.ts`, 2 tests).
- `pnpm run build` — clean (122 modules transformed; `dist/assets/index.*.js` 107.14 KiB).
- `GOWORK=off wails build` — compiles + packages with NomService bound (darwin/arm64); self-signed cleanly after `xattr -cr build/bin` (iCloud xattr environment artifact). Wails-generated bindings match the committed tree (no working-tree drift after build).

Covered by tests (offline):
- Pillar reads: `PillarSummaryDTO` maps name/rank/weight/delegateRewardPercent/producerAddress; pillar list sorted by rank (`TestSortPillarsByRank`).
- Actions: `PrepareDelegate(name)` validates input before any node use (`TestPrepareDelegateValidatesInput`); `PrepareUndelegate()` / `PrepareCollectPillarReward()` follow the same shared prepare path.
- Token standard: Delegate/Undelegate/CollectReward SDK templates use `PillarContract` + `ZnnTokenStandard` with `amount: 0` — regression-locked against the real `embedded.NewPillarApi(nil)` builders (`TestPillarTemplateTokenStandards`).

## Live read-only smoke (2026-06-22) — PASSED (embedded namespace confirmed)
Run against the testnet node `ws://172.245.236.40:35998` (opt-in integration test
`internal/spike.TestReadOnlyPillars`, `-tags integration`):
- `PillarApi.GetAll(0,50)` — **PASSED**: `count=7`, returned 7 pillars (ATSocy.com, Testnet-P1..P4, …),
  each with rank/weight/giveReward%. This proves the node exposes the `embedded` RPC namespace
  (a `ledger`-only node would return `embedded.* does not exist`) and that NomService's exact read
  path works end-to-end against a live node. Read-only — no PoW, no signing.
- `PillarApi.GetDelegatedPillar` / `GetUncollectedReward` (per-address): **PASSED** against three
  testnet addresses — both code paths exercised on live data:
  - `z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg` → delegated: `name="Testnet-P1" status=1 weight=5000000000000` (50,000 ZNN); uncollected reward `znn=0 qsr=0`.
  - `z1qzu5wkg93qlsk24w5cjkg7w9y0q42e5g7dvgpn` → **not delegated** (empty-Name → `DelegationInfo{}` mapping confirmed live); reward `znn=0 qsr=0`.
  - `z1qqsfews4dyjghnqh4l5jp6y7qz70j4a6d4a8ec` → **not delegated**; reward `znn=0 qsr=0`.

  Repro: `ZNN_NODE_URL=ws://172.245.236.40:35998 ZNN_TEST_ADDR=<z1…> GOWORK=off go test -tags integration ./internal/spike -run TestReadOnlyPillars -v -count=1`.

## Post-merge carry-forward cleanup (2026-06-22) — DONE (commit 5dbfce7)
From the whole-branch review's tracked follow-ups:
- Shared `tx` store now resets on navigation (`resetTx` + `view` subscription in `tx.ts`) — a
  finished/errored/awaiting modal from one route no longer leaks onto another (Send/Plasma/Stake/Pillars).
  Covered by `tx.test.ts` (resetTx + reset-on-nav). Frontend suite 30/30; `svelte-check` 0 errors.
- `pillar.ts`/`plasma.ts`/`stake.ts` now import generated `models.ts` types instead of hand-redeclaring
  them — removes the silent-drift risk if a Go JSON tag changes.

## Manual acceptance (Phase 5c gate) — user-confirmed
On a testnet node **with the `embedded` RPC namespace enabled** (Pillars route):
1. Open the Pillars route → see the rank-sorted pillar list + your current delegation (or "Not delegated") + uncollected reward.
2. Search filters the list by name.
3. Delegate to a pillar → TxModal shows "Delegate to <name>" from the built block → Confirm → after a momentum the delegation shows as current.
4. Collect rewards (when uncollected > 0) → reward arrives.
5. Undelegate → delegation clears.
6. Mainnet guard: with `AllowMainnetSend` false on a mainnet node, `PrepareDelegate` is blocked.

### Result (manual run — testnet)
User confirmed acceptance of all local testing on 2026-06-22 (node `ws://172.245.236.40:35998`):
- view pillar list + current delegation + uncollected reward: ACCEPTED (user-confirmed)
- search filters list by name: ACCEPTED (user-confirmed)
- delegate to pillar (TxModal "Delegate to <name>" from built block) + shows as current: ACCEPTED (user-confirmed)
- collect rewards (reward arrives): ACCEPTED (user-confirmed)
- undelegate (delegation clears): ACCEPTED (user-confirmed)
- mainnet-gated (AllowMainnetSend=false blocks PrepareDelegate): ACCEPTED (user-confirmed)
- testnet tx hashes observed: not recorded (user-confirmed acceptance without per-tx hash capture)

> Node prerequisite (carried from 5b): the connected node must expose the `embedded` RPC
> namespace (whitelisted via go-zenon `RPC.Endpoints`). A node serving only `ledger` returns
> `embedded.plasma.getRequiredPoWForAccountBlock does not exist/is not available` for every
> PoW-requiring action (delegate/undelegate/collect all require PoW). Not a wallet bug.

## Security recap
- Reuses the one audited prepare/confirm/publish path; confirm-what-you-sign re-asserts the built block's to/zts/amount AND ABI `Data`; mainnet gated by `AllowMainnetSend`; no key material in NomService.
- All three pillar call templates verified against the SDK builders (`PillarContract` + `ZnnTokenStandard`, amount 0) — applying the 5a/5b token-standard lesson.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded — `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does not human-decode arbitrary params; tracked as Phase-5/7 hardening.
