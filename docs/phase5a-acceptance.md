# Phase 5a — Plasma Acceptance

## Automated verification (2026-06-21) — PASSED
- `go test ./...` — green incl. `app` (assertMatches, fusionEntryDTO revocability, Fuse/Cancel validation, Fuse=QSR/Cancel=ZNN template token-standard regression test).
- Frontend `pnpm test` — 22/22 across 13 files (incl. Plasma Cancel-gating).
- `pnpm run build` — clean.
- `wails build` — compiles + packages with NomService bound; self-sign tripped on iCloud xattrs, signed cleanly after `xattr -cr build/bin/syrius.app && codesign --force --deep -s -` (environment artifact).

Covered by tests (offline):
- Shared contract-call path: `assertMatches` re-asserts built block to/zts/amount; `prepareCall` mirrors Send's guarded PoW/hold path.
- Plasma reads: PlasmaInfo/FusionEntry mappers; `IsRevocable = frontier >= expirationHeight`.
- Actions: PrepareFuse/PrepareCancelFuse validate inputs before any node use; Cancel's callExpect uses ZNN (matches SDK template) — regression-locked.

## Manual acceptance (Phase 5a gate) — PENDING user GUI run
On testnet (Plasma route):
1. View current plasma + QSR fused + fusion entries.
2. Fuse a small QSR amount for self → TxModal shows "Fuse … QSR for z1…" from the built block → Confirm → after a momentum, plasma rises + a fusion entry appears.
3. Fuse for a different beneficiary → confirms.
4. Once a fusion entry is revocable (expiration height passed), Cancel → QSR returns; entry disappears.
5. Mainnet guard: with AllowMainnetSend false on a mainnet node, PrepareFuse is blocked.

### Result (fill in after the manual run)
- fuse self/other + plasma rise: PASS / FAIL
- cancel returns QSR: PASS / FAIL
- confirm-modal renders built block: PASS / FAIL
- mainnet-gated: PASS / FAIL
- testnet tx hashes observed: ____

## Security recap
- One audited prepare/confirm/publish path; confirm-what-you-sign re-asserts the built block; mainnet gated by AllowMainnetSend; no key material in NomService.
- Residual (flagged): contract Data (fuse beneficiary) shown in summary but not ABI-decode-re-asserted at publish — Phase-5 hardening.
- Process note: the SDK's Cancel block uses ZnnTokenStandard (not QSR); each future sub-phase must verify its contract call's real TokenStandard against the SDK before setting callExpect.
