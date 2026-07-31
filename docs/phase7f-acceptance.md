# Phase 7f acceptance — additive user entropy (wallet creation)

Spec: `docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md`

## Automated gates (record command output dates/results when run)

- [x] `GOWORK=off GOTOOLCHAIN=auto go test ./...` / `go vet ./...` / `go build ./...`
      — 2026-07-31: all three exit 0 (only the known gopsutil/IOKit cgo
      deprecation warning; 7 packages `ok`, including `app` and `internal/compat`).
- [x] `bash scripts/govulncheck-gate.sh` and `gosec -conf .gosec.json ./...`
      — 2026-07-31: gate exit 0, "only allowlisted vulnerabilities present"
      (GO-2026-4314/4315/4507/4508/4511, the deferred embedded-node-only
      go-ethereum p2p set); gosec exit 0, 26 files / 7594 lines / **0 issues**.
- [x] `pnpm test`, `pnpm run typecheck`, `pnpm run build` (frontend/)
      — 2026-07-31: 72 test files / 432 tests passed, `vue-tsc --noEmit` clean,
      Vite build succeeded.

## Manual acceptance (spec §11.4 — user-run, per release-target WebView: macOS, Windows, Linux)

- [ ] 1. Open Create Wallet: no phrase is shown or generated before an explicit action.
- [ ] 2. Complete pointer collection and create a wallet.
- [ ] 3. Complete keyboard-only collection and create a wallet.
- [ ] 4. Use the skip-interaction path and create a wallet.
- [ ] 5. All three paths produce 24 valid BIP-39 words.
- [ ] 6. Each created keystore unlocks and derives the displayed base address.
- [ ] 7. At least one enhanced wallet opens in original Syrius with the same address.
- [ ] 8. Navigate away mid-collection; on return no listeners or stale progress remain.
- [ ] 9. Simulate Web Crypto unavailability in a dev build; only the explicit
      `Generate using backend randomness` action generates.
- [ ] 10. Inspect logs and network activity: no contribution, digest, mnemonic,
      or final entropy appears anywhere.
