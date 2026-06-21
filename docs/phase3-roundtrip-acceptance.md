# Phase 3 — Round-trip Acceptance (wallet lifecycle)

## Automated verification (2026-06-21) — PASSED
- `go test ./...` — green (app + internal/*).
- Frontend `pnpm test` — 17/17 across 12 files.
- `wails build` — compiles, packages, self-signs; `build/bin/syrius.app` (arm64) produced.

Covered by Go unit tests (offline, go-zenon-verified):
- GenerateMnemonic → 24-word BIP-39, distinct.
- ImportMnemonic round-trip: written keystore re-read via go-zenon `ReadKeyFile`→`Decrypt`→ index-0 address matches; overwrite refused; invalid mnemonic rejected.
- ChangePassword: new password decrypts, old fails afterward; atomic temp+rename.
- RevealMnemonic: correct password returns the mnemonic; wrong/locked error.
- Account labels persist and surface in CurrentAccounts.

## Manual round-trip acceptance (Phase 3 Gate) — PENDING user GUI run
Requires a real syrius install (cannot be done headless):
1. Launch `build/bin/syrius.app` → Unlock screen → "Create new wallet".
2. Record the 24 words; pass the 3-word backup verify (3 randomly chosen positions); set a name + password; land on the dashboard (zero balances).
3. Locate the keystore at `~/Library/Application Support/go-syrius/wallets/<name>`.
4. **Open that keystore in real syrius** with the same password → confirm syrius shows the SAME index-0 `z1…` address. (THE GATE.)
5. In the app: change the password, re-unlock with the new one; reveal the mnemonic (matches what was recorded); set an account label and confirm it persists across lock/unlock.

Reverse direction (syrius keystore opens in go-syrius) is already proven in Phase 0.

### Result (fill in after the manual run)
- syrius version tested: ____
- created-here address (go-syrius): ____
- same keystore in syrius shows: ____  (must match)
- created-here-opens-in-syrius: PASS / FAIL
- change-password / reveal / labels: PASS / FAIL

If the created keystore does NOT open in syrius, capture the exact error and the `argon2Params` field diff between a go-syrius-written keystore and a syrius-written one (`pillar.json`) — that would indicate syrius needs the full Argon2 param set rather than go-zenon's salt-only, to be addressed before Phase 3 closes.
