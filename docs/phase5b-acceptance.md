# Phase 5b — Staking Acceptance

## Automated verification (2026-06-21) — PASSED
- `GOWORK=off go test ./...` — green incl. `app` (stakeEntryDTO maturity/duration derivation, PrepareStake/PrepareCancelStake validation, Stake/Cancel/Collect template token-standard regression test). `GOWORK=off` is required: a parent `/Users/dfriestedt/Github/go.work` references a missing sibling module, so bare `go test ./...` fails to load the workspace. This is a local-env artifact, not a repo issue.
- `GOWORK=off go build -tags integration ./...` + `GOWORK=off go vet -tags integration ./app/` — the opt-in integration test compiles.
- Frontend `pnpm test` — 26/26 across 14 files (incl. Stake Cancel-gating on `isMatured`).
- `pnpm run build` — clean.
- `GOWORK=off wails build` — compiles + packages with NomService bound (darwin/arm64); self-signed cleanly after `xattr -cr build/bin` (iCloud xattr environment artifact).

Covered by tests (offline):
- Stake reads: `stakeEntryDTO` maps amount/id/timestamps; `DurationMonths = (Expiration-Start)/StakeTimeUnitSec`; `IsMatured = frontierUnix >= ExpirationTimestamp`. Maturity boundary tested before/at/after expiration.
- `frontierUnix` uses `api.Momentum.TimestampUnix` (the wire field); the `*time.Time Timestamp` is `json:"-"` and nil over RPC — verified against go-zenon `nom.Momentum` to avoid a nil-deref.
- Actions: PrepareStake (amount ≥ 1 ZNN, duration 1–12), PrepareCancelStake (bad id), PrepareCollectReward validate before any node use.
- Token standard: all three Stake/Cancel/CollectReward SDK templates use `StakeContract` + `ZnnTokenStandard` (Stake moves ZNN; Cancel/Collect move 0 ZNN) — regression-locked against the real `embedded.NewStakeApi(nil)` builders.

## Manual acceptance (Phase 5b gate) — PENDING user GUI run
On testnet (Staking route):
1. Open the Staking route → see total staked + uncollected reward (ZNN/QSR) + stakes list.
2. Stake ≥1 ZNN for N months → TxModal shows "Stake X ZNN for N months" from the built block → Confirm → after a momentum the stake appears.
3. Collect rewards (when uncollected QSR > 0) → QSR arrives.
4. Cancel a matured stake → ZNN returns; the entry disappears.
5. Mainnet guard: with AllowMainnetSend false on a mainnet node, PrepareStake is blocked.

### Result (manual run — 2026-06-21, testnet node ws://172.245.236.40:35998, chainId 73404)
- view stakes + uncollected reward: PASS
- stake X ZNN for N months + appears: PASS
- collect rewards (QSR arrives): PENDING — rewards not yet accrued; retest once uncollected QSR > 0
- cancel matured stake returns ZNN: PENDING — requires a stake past its 30-day expiration
- confirm-modal renders built block + human ZNN amount + summary: PASS
- mainnet-gated: not retested this run (guard unchanged from 5a; AllowMainnetSend=false)
- testnet tx hashes observed: ____ (record on next run)

> Node prerequisite discovered during acceptance: the connected node must expose the
> `embedded` RPC namespace (whitelisted via go-zenon `RPC.Endpoints`). A node serving only
> `ledger` returns `embedded.plasma.getRequiredPoWForAccountBlock does not exist/is not
> available` for every PoW-requiring action (fuse, stake, send-without-plasma). Verified the
> fix end-to-end: with `embedded` enabled, both Fuse and Stake price real PoW (reqDiff≈78.75M
> on a 0-plasma account) and publish successfully. Not a wallet bug.

## Security recap
- Reuses the one audited prepare/confirm/publish path; confirm-what-you-sign re-asserts the built block's to/zts/amount AND ABI `Data`; mainnet gated by AllowMainnetSend; no key material in NomService.
- Each stake call's token standard verified against the SDK template (all ZNN) — applying the 5a Cancel-token lesson.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded — `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does not human-decode arbitrary params; tracked as Phase-5/7 hardening.
