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

### Result (fill in after the manual run)
- view stakes + uncollected reward: PASS / FAIL
- stake X ZNN for N months + appears: PASS / FAIL
- collect rewards (QSR arrives): PASS / FAIL
- cancel matured stake returns ZNN: PASS / FAIL
- confirm-modal renders built block + human ZNN amount + summary: PASS / FAIL
- mainnet-gated: PASS / FAIL
- testnet tx hashes observed: ____

## Security recap
- Reuses the one audited prepare/confirm/publish path; confirm-what-you-sign re-asserts the built block's to/zts/amount AND ABI `Data`; mainnet gated by AllowMainnetSend; no key material in NomService.
- Each stake call's token standard verified against the SDK template (all ZNN) — applying the 5a Cancel-token lesson.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded — `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does not human-decode arbitrary params; tracked as Phase-5/7 hardening.
