# Phase 1 — Wails Skeleton + Read-Only Wallet Design

**Date:** 2026-06-20
**Status:** Approved
**Scope:** Phase 1 of the syrius-wails roadmap (see `docs/superpowers/specs/2026-06-20-syrius-wails-roadmap-design.md`). A real Wails desktop window that unlocks an existing wallet and shows balances, address, history, and live connection/sync status. **Strictly read-only plus keystore import** — no sending (Phase 2), no wallet creation (Phase 3).

## Goal

Unlock a real wallet, see correct balances and transaction history, with live-updating connection/sync status. Read-only and safe.

## Locked decisions (from brainstorming 2026-06-20)

- **Wallet source:** go-syrius keeps its **own** wallet directory; the user imports a keystore (copy a file in). No dependency on a syrius install.
- **Default node:** ship a sensible default remote URL (`wss://my.hc1node.com:35998`, mainnet), user-editable in settings.
- **Design:** functional + clean foundation — Tailwind with a small token set and reusable components; full polish deferred.
- **Scope:** read-only **+ import keystore**. No create/import-from-mnemonic, no send.
- **Live updates:** backend-driven Wails events (not frontend polling).
- **Keystore handling:** go-zenon's `wallet` package directly (the SDK cannot read syrius keystores — see `docs/compatibility-notes.md`). SDK used for RPC reads. No SDK modification.

## Context

Builds on Phase 0 (merged): module `github.com/0x3639/go-syrius`, `znn-sdk-go v0.1.16` pinned (unmodified), `go-zenon` direct dependency, proven keystore-read + read-only RPC. Phase 0's `internal/` (compat/integration tests, `version`) is retained.

## Architecture

### Process & data-flow

Single Wails v2 binary. Svelte frontend (WebView) ⇄ Go backend over Wails bindings + events. Backend owns the RPC client and all keystore material; the frontend holds none.

- **Pull** (frontend calls Go): balances and history — on unlock and on each `momentum:tick`.
- **Push** (Go emits events): connection/sync status and momentum height.

### Project structure & Wails integration

Scaffold-then-merge: run `wails init -n syrius -t svelte-ts` in a temp dir, move `main.go`, `wails.json`, `frontend/`, `build/` into the repo, and reconcile `go.mod` (keep our module path + deps, add the Wails dep). Phase 0 `internal/` is unchanged.

```
main.go               # Wails bootstrap; registers bound services
wails.json
app/                  # Wails-bound services (the binding boundary)
  app.go              # App struct; startup/shutdown lifecycle; holds services
  config_service.go   # data dir + settings persistence
  wallet_service.go   # list/import/unlock/lock/accounts (go-zenon wallet)
  node_service.go     # remote connection lifecycle, status events, reads
  dto.go              # DTOs crossing the boundary (no secrets)
  events.go           # typed event-name constants
internal/             # Phase 0 compat/integration tests, version, helpers
frontend/
  src/
    routes/           # unlock, dashboard
    lib/
      stores/         # wallet, node, balances, txs
      components/     # WalletPicker, PasswordInput, AddressDisplay, BalanceList, TxHistory, StatusBar, AccountSwitcher
      bindings/       # generated wailsjs
    app.css           # Tailwind entry + tokens
  index.html
  package.json
  vite.config.ts
build/                # platform assets, icons
```

Each `app/*_service.go` has one responsibility and a small, well-defined method set; DTOs and event names are shared, stable artifacts both sides build against.

## Components

### ConfigService

- `GetSettings() (Settings, error)` · `SetSettings(Settings) error`
- Data dir: `os.UserConfigDir()/go-syrius` (created on first run); settings persisted as JSON (`settings.json`); wallets under `<dataDir>/wallets/`.
- `Settings{ NodeURL string; Theme string; LastWallet string; ActiveAccount int }` with defaults (NodeURL = the shipped default, Theme = "dark", ActiveAccount = 0).

### WalletService (go-zenon `wallet`)

- `ListWallets() ([]WalletMeta, error)` — list keystore files in the wallets dir, reading `baseAddress` from each (no decryption).
- `ImportKeystore(srcPath string) (WalletMeta, error)` — validate via `wallet.ReadKeyFile` (rejects non-keystores / unsupported version/cipher/kdf), copy into the wallets dir under a derived name; refuse to overwrite.
- `Unlock(name, password string) error` — `ReadKeyFile().Decrypt(password)`; hold the decrypted `*KeyStore` in memory on the service; map decrypt failure to a clear "incorrect password" error.
- `Lock() error` — call `KeyStore.Zero()` and drop the reference; emit `wallet:locked`.
- `CurrentAccounts() ([]AccountInfo, error)` — derive a small fixed range (e.g. indices 0–9) via `DeriveForIndexPath`, returning `{Index, Address}`.
- `SelectAccount(index int) error` — validate range; set active index; persist via ConfigService. The frontend re-pulls balances/history after the call returns (no backend event needed for this user-initiated change).
- **Never** returns a key, seed, or mnemonic.

### NodeService (remote only)

- `SetNode(url string) error` — open `rpc_client.NewRpcClient(url)`, verify reachability (`GetFrontierMomentum`), subscribe to momentums, persist URL, emit `node:status`. Replaces any existing connection cleanly.
- `Disconnect() error` · `NodeStatus() NodeStatus`.
- On momentum subscription tick: emit `momentum:tick{height}` and an updated `node:status`.
- Reads (active address, from WalletService): `GetBalances() ([]TokenBalance, error)` (`GetAccountInfoByAddress` → BalanceInfoMap), `GetTransactions(page, count int) ([]TxRecord, error)` (`GetAccountBlocksByPage`).
- Connection drops surface as `node:status{Connected:false}` (the SDK client auto-reconnects).

### DTOs (secret-free)

- `WalletMeta{ Name string; BaseAddress string }`
- `AccountInfo{ Index int; Address string }`
- `Settings{ NodeURL, Theme, LastWallet string; ActiveAccount int }`
- `NodeStatus{ Mode string; Connected bool; Syncing bool; Height uint64; Peers int }`
- `TokenBalance{ Zts string; Symbol string; Decimals int; Amount string }` (Amount as base-unit decimal string)
- `TxRecord{ Hash string; Direction string; Counterparty string; Token string; Amount string; MomentumHeight uint64; Confirmed bool; Timestamp int64 }`

### Events (Go → frontend)

`node:status`, `momentum:tick`, `wallet:locked`. (Phase 2 will add `tx:*`.)

## Frontend

- **Routes:** `/unlock` (wallet picker + password + import button; empty state with import CTA when no wallets) → `/dashboard`.
- **Dashboard:** active address with copy + QR; ZNN/QSR/ZTS balances; recent transaction history (paged); StatusBar with connection/sync/height/peers; AccountSwitcher.
- **Stores:** `wallet` (locked state, accounts, active index), `node` (status, fed by `node:status`/`momentum:tick`), `balances`, `txs`. Stores call bindings and subscribe to events via `runtime.EventsOn`.
- **Components:** `WalletPicker`, `PasswordInput`, `AddressDisplay` (copy + QR via the `qrcode` package), `BalanceList`, `TxHistory`, `StatusBar`, `AccountSwitcher`.
- **Design tokens:** Tailwind config with a small palette (bg / surface / text / accent / success / warn / error), one spacing + type scale, a display font plus **monospace for addresses and amounts**. Dark default; light optional via the theme setting.
- On `momentum:tick` the dashboard refreshes balances + history for the active address.

## Error handling

- Wrong password → clear inline error on the unlock screen; no stack/secret leakage.
- No wallet present → empty state with an Import CTA.
- Invalid keystore import → validation error naming the problem (bad version/cipher/kdf or unreadable).
- Node unreachable / dropped → StatusBar shows disconnected; reads return a surfaced error; auto-reconnect continues in the background.

## Testing

- **Backend (Go):** ConfigService (settings round-trip, data-dir resolution, defaults); WalletService (import validation incl. rejecting non-keystores, list reads baseAddress without decrypting, unlock against the gitignored `secrets/` keystore — skip if absent, `Lock()` zeroes state); DTO mapping (balances/tx records). NodeService status mapping is unit-tested; live connect is `//go:build integration`. No secrets committed; reuse the `secrets/` skip pattern.
- **Frontend:** Vitest tests for store logic and the unlock/dashboard flows against **mocked bindings** (unlock success/failure, dashboard render, status event updates). Kept light.
- **Manual acceptance:** import the Phase 0 keystore, unlock, see correct balances/history against the default node, observe live height updates.

## Security

- No private key, seed, mnemonic, or decrypted keystore ever crosses the binding boundary.
- Decrypted keystore held only in WalletService memory; `Lock()` zeroes it; minimize lifetime.
- Never log secrets. Treat the frontend as untrusted for key material.
- Wallets dir and any keystores stay gitignored (Phase 0 `.gitignore` covers `secrets/`, `*.dat`, keystore-name patterns).

## Exit criteria (Phase 1 → Phase 2)

- Unlock a real wallet; see correct balances and transaction history; live-updating connection/sync status.
- Read-only: no send path, no wallet creation, exists in the binary.
- `go test ./...` (offline) and frontend unit tests pass; `wails build` produces a runnable binary.

## Out of scope (deferred)

- Sending / receiving (Phase 2); wallet creation, import-from-mnemonic, multi-seed management, password change, mnemonic reveal (Phase 3); local/embedded node modes (Phase 4); NoM features (Phase 5); Ledger (Phase 6); packaging/signing (Phase 7).
- Bespoke visual identity and motion.
