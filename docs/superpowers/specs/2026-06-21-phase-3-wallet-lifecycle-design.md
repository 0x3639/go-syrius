# Phase 3 — Wallet Lifecycle Design

**Date:** 2026-06-21
**Status:** Approved
**Scope:** Phase 3 of the syrius-wails roadmap. Full key management: create a new wallet (with forced backup), import from mnemonic, change password, reveal mnemonic, and per-account labels. Builds on Phases 1–2 (merged). Acceptance: a wallet created here opens in real syrius and vice-versa.

## Goal

Create/import/manage wallets with syrius-compatible keystores, the mnemonic shown exactly once at creation (after a forced backup check) and otherwise only via an explicit password-gated reveal.

## Locked decisions (brainstorming 2026-06-21)

- **New-wallet mnemonic:** 24 words (256-bit), matching syrius.
- **Multi-account:** add per-account labels (persisted in settings); account derivation/switching already exists from Phase 1.
- **Forced backup:** show the mnemonic once, then verify 3 random word positions before the keystore is written.
- **No SDK / no dependency forks:** all keystore work uses go-zenon's `wallet` package (the SDK cannot read syrius keystores — Phase 0). Mnemonic generation uses BIP-39 directly. No custom crypto.

## Context

Phase 1 gave WalletService (`ListWallets`/`ImportKeystore`/`Unlock`/`Lock`/`CurrentAccounts`/`SelectAccount`/`signingKeyPair`) holding an unlocked go-zenon `*wallet.KeyStore` (fields `Entropy`/`Seed`/`Mnemonic`/`BaseAddress` are exported), and a dashboard + account switcher. Phase 2 added send/receive. Phase 0 established the keystore crypto: raw BIP-39 entropy encrypted AES-256-GCM, AAD `"zenon"`, Argon2id fixed params `(1, 64*1024, 4, 32)`, 12-byte nonce, JSON `{baseAddress, crypto{cipherName:"aes-256-gcm", kdf:"argon2.IDKey", cipherData, nonce, argon2Params:{salt}}, version:1, timestamp}`.

go-zenon wallet facts (verified): `KeyStore` has exported fields and public `Encrypt(password) (*KeyFile, error)`, `DeriveForIndexPath`, `Zero`; `KeyFile` has exported `Path` + public `Write()`, `ReadKeyFile`, `Decrypt`. There is **no** public create-from-entropy, but the exported `KeyStore` struct lets us assemble one. BIP-39 is `github.com/tyler-smith/go-bip39` (already in the dependency graph via the SDK/go-zenon).

## Architecture

### Creation = generate → backup-verify → import

A single keystore-writing path, `ImportMnemonic`, serves both "create new" and "import existing." Creation is a frontend wizard that calls `GenerateMnemonic` (no persistence), gates on a 3-word backup check, then calls `ImportMnemonic`.

```
GenerateMnemonic() ──▶ (frontend shows 24 words, verifies 3 random positions)
                          │
                          ▼
ImportMnemonic(name, password, mnemonic)  ── assemble KeyStore ▶ Encrypt(password) ▶ KeyFile.Write()
```

`ImportMnemonic` assembles a go-zenon `wallet.KeyStore{Entropy, Seed, Mnemonic}` (entropy from the mnemonic, seed = `bip39.NewSeed(mnemonic, "")`), derives index-0 to set `BaseAddress`, then `Encrypt`/`Write` — go-zenon's canonical crypto, no custom code.

### Secret boundary (the two deliberate exceptions)

Per the roadmap binding-boundary rule, the mnemonic crosses to the WebView only here:
- `GenerateMnemonic()` returns the new mnemonic once, for backup.
- `RevealMnemonic(password)` returns the active wallet's mnemonic, **password-gated** (re-decrypts the active keystore file to verify the password before returning).

No other method returns a key/seed/mnemonic; the mnemonic is never logged.

### Keystore compatibility

The written keystore is go-zenon's canonical format. go-zenon and syrius use the **same fixed** Argon2 params, so a `argon2Params:{salt}`-only file is readable by syrius (it defaults the rest). Verified locally by reading the written file back via `ReadKeyFile`→`Decrypt`→ index-0 address; the authoritative check is the manual "opens in real syrius" round-trip (Phase 3 Gate).

## Components

### WalletService (`app/wallet_service.go`, all via go-zenon)

- `GenerateMnemonic() (string, error)` — `crypto/rand` 32-byte entropy → `bip39.NewMnemonic`; persists nothing.
- `ImportMnemonic(name, password, mnemonic string) (WalletMeta, error)` — `bip39.IsMnemonicValid` (reject invalid); `entropy = bip39.EntropyFromMnemonic`; `seed = bip39.NewSeed(mnemonic, "")`; build `&wallet.KeyStore{Entropy, Seed, Mnemonic}`; `DeriveForIndexPath(0)` → set `BaseAddress`; `Encrypt(password)` → set `KeyFile.Path = <walletsDir>/<name>` → `Write()`. Refuse overwrite (stat + the writer must not clobber). Return `WalletMeta{Name, BaseAddress}`.
- `ChangePassword(name, oldPassword, newPassword string) error` — `ReadKeyFile(path)` → `Decrypt(old)` (→ "incorrect password" on failure) → `Encrypt(new)` → write **atomically**: write to `<path>.tmp` then `os.Rename` over `<path>` (a failure leaves the original intact). `defer ks.Zero()`.
- `RevealMnemonic(password string) (string, error)` — require unlocked (`activeAddress` ok); re-verify by `ReadKeyFile(<walletsDir>/<activeWalletName>).Decrypt(password)`; on success return the in-memory `keystore.Mnemonic`; wrong password → error. Never logged. (Requires WalletService to record the active wallet **name** on `Unlock` — add an unexported `activeWallet string` field set in `Unlock` and cleared in `Lock`, if not already tracked.)
- `SetAccountLabel(index int, label string) error` — persist via ConfigService; validate index in range.
- `AccountInfo` gains `Label string`, populated in `CurrentAccounts` from settings.

(`ImportMnemonic`/`ChangePassword` operate under the existing WalletService mutex; creating/changing does not require the wallet to be unlocked except `RevealMnemonic`, which does.)

### ConfigService

- `Settings.AccountLabels map[string]string` (key `"<walletName>:<index>"`), default empty (nil-safe access).

### DTOs

- Extend `AccountInfo` with `Label string` (camelCase `label`).
- No new event names (create/import/change/reveal are request/response; existing `wallet:locked` unchanged).

## Frontend

- **Unlock screen**: add "Create wallet" and "Import" actions beside the existing "Import keystore…".
- **Create wizard** (`/create`): (1) `GenerateMnemonic` → display 24 words (copy button, "write these down / never share" warnings); (2) verify 3 random word positions (inputs/selects) — Continue disabled until all correct; (3) wallet name + password + confirm-password → `ImportMnemonic` → unlock into the dashboard.
- **Import mnemonic** (`/import-mnemonic`): mnemonic textarea with live BIP-39 validity, name, password+confirm → `ImportMnemonic`.
- **Settings** (new surface reachable from the dashboard): **Change password** (old/new/confirm → `ChangePassword`, success/error states) and **Reveal mnemonic** (password prompt → `RevealMnemonic` → show with warnings + copy, hidden again on close).
- **Account labels**: inline-editable label in the account switcher / accounts list → `SetAccountLabel`; labels shown alongside `Account N`.
- Stores: extend the `wallet` store with `generateMnemonic`/`importMnemonic`/`changePassword`/`revealMnemonic`/`setLabel`; small `nav` additions for `/create` and `/import-mnemonic`.

## Error handling

- Invalid mnemonic (import) → inline validation error; submit disabled.
- Duplicate wallet name → "a wallet named X already exists".
- Backup verification mismatch → step 2 cannot be completed; clear prompt.
- Wrong old password (change) / wrong password (reveal) → clear "incorrect password".
- Password/confirm mismatch → inline error.
- Atomic re-encrypt: on any failure the original keystore is preserved (temp-file + rename).

## Testing

- **Backend (Go, go-zenon-verified, offline):**
  - `GenerateMnemonic` returns a 24-word BIP-39-valid mnemonic; distinct across calls.
  - `ImportMnemonic` round-trips: written file `ReadKeyFile`→`Decrypt`→ index-0 address equals the mnemonic's derived address; refuses overwrite; rejects an invalid mnemonic; the written file's `baseAddress` matches.
  - `ChangePassword`: new password decrypts; old password fails afterward; a simulated rename/write failure leaves the original file decryptable with the old password (atomicity).
  - `RevealMnemonic`: correct password returns the exact mnemonic; wrong password errors; errors when locked.
  - Account-label set/get persists across a ConfigService reload.
  - No secrets committed; tests use temp data dirs.
- **Frontend (Vitest, mocked bindings):** create-wizard gating (cannot finish until the 3 words match), import validation, change-password form (mismatch + success), reveal flow (shows on correct password), label edit.
- **Acceptance (Phase 3 Gate):** a wallet **created in go-syrius opens in real syrius** (manual GUI round-trip), and a syrius keystore opens in go-syrius (already proven Phase 0). Optional automated cross-check: a go-syrius-created keystore decrypts via go-zenon to the expected address (covered by the round-trip unit test).

## Security

- Mnemonic reaches the WebView only via `GenerateMnemonic` (once) and `RevealMnemonic` (password-gated); never logged or returned elsewhere.
- `ChangePassword` re-encrypt is atomic (temp + rename) — never corrupts the keystore.
- Decrypted `KeyStore` instances used for create/change are `Zero()`-ed after use; `Lock()` continues to zero the active keystore.
- Treat the WebView as untrusted for key material; every state-changing method validates inputs in Go.

## Exit criteria (Phase 3 → Phase 4)

- Create a new wallet (24-word, forced 3-word backup verify), import from mnemonic, change password, reveal mnemonic — all working; account labels persist.
- A wallet created in go-syrius opens in real syrius, and vice-versa.
- `go test ./...` (offline) and frontend unit tests pass.

## Out of scope (deferred)

- Local/embedded node modes (Phase 4); NoM contract features (Phase 5); Ledger (Phase 6); packaging/signing (Phase 7).
- Hardware-backed key storage; multiple simultaneously-unlocked wallets; mnemonic passphrase (BIP-39 25th word) — single empty passphrase only, matching syrius default.
