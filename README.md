# go-syrius

A reimplementation of the Zenon **syrius** wallet as a **Go + [Wails v2](https://wails.io)** desktop app (Vue 3 + TypeScript frontend). It reuses the proven Go crypto/node stack — [`znn-sdk-go`](https://github.com/0x3639/znn-sdk-go) and `go-zenon` — so wallet files and transactions interoperate with the original syrius.

**Working today:** send/receive, wallet lifecycle (create / import / manage / auto-lock), all three node modes (remote / local / embedded full node), the full Network-of-Momentum feature set (plasma, staking, pillars, sentinels, tokens, accelerator), and **WalletConnect v2** for bridge dApps. Mainnet-capable since `v0.3.0`.

> The experimental **Governance** module is **temporarily disabled** pending an SDK update; it will return in a future release.

---

## Security model (read this)

- **Keys never leave the Go backend.** The UI sends *intent*; Go builds, PoWs, signs, and publishes. The mnemonic surfaces exactly once at creation and via an explicit password-gated reveal.
- **Confirm what you sign.** Every confirmation dialog renders the effect decoded from the **actual built transaction bytes** — destination, token, amount, and for contract calls the decoded method and parameters — not from form input.
- **Mainnet is opt-in.** Sending on mainnet (Chain ID 1) requires explicitly enabling **Settings → Enable mainnet transactions**, and the opt-in is re-checked against the built block immediately before broadcast.
- **Auto-lock.** The wallet locks itself after 5 minutes of inactivity by default (configurable: 1 / 5 / 15 / 30 minutes / Never), enforced by the backend.
- **WalletConnect is tightly scoped.** Sessions are restricted to Zenon mainnet bridge operations (`WrapToken` / `RedeemUnwrap` to the bridge contract only); dApps cannot request arbitrary transactions.
- **Independently audited.** A comprehensive security audit ([2026-07-20](docs/security-audit-2026-07-20.md)) found **no Critical or High severity vulnerability** in the funds-critical code; its code-level remediations shipped in `v0.3.3`. This is not a claim the application is "secure" — read the report's scope and residual-risk statement before trusting it with significant funds.

### ⚠️ Builds are currently unsigned

Releases are **not yet code-signed or notarized** (planned — Phase 7c), so your OS will warn on first launch (bypass steps below). **Always verify your download against `SHA256SUMS`.** Until signed builds ship, treat this as beta software: prefer modest balances, and keep your seed phrase backed up offline.

---

## 1. Download

Latest release: **https://github.com/0x3639/go-syrius/releases/latest**

| OS | Asset |
|---|---|
| **macOS** (Apple Silicon) | `go-syrius-<version>-macos.zip` |
| **Windows** (x64) | `go-syrius-<version>-windows-amd64.exe` |
| **Linux** (x64) | `go-syrius-<version>-linux-amd64.tar.gz` |

### Verify your download (strongly recommended)

Download `SHA256SUMS` from the same release and compare:

```bash
# macOS
shasum -a 256 go-syrius-<version>-macos.zip
# Linux
sha256sum go-syrius-<version>-linux-amd64.tar.gz
# Windows (PowerShell)
Get-FileHash go-syrius-<version>-windows-amd64.exe -Algorithm SHA256
```

The hash must match the line for your file in `SHA256SUMS`.

## 2. Install & open (unsigned build)

- **macOS:** unzip → drag `syrius.app` to Applications (optional) → **right-click the app → Open → Open**. If it still refuses:
  ```bash
  xattr -dr com.apple.quarantine /path/to/syrius.app
  ```
- **Windows:** run the `.exe` → SmartScreen → **More info → Run anyway**.
- **Linux:** `tar -xzf go-syrius-<version>-linux-amd64.tar.gz` → `chmod +x syrius` → `./syrius`. Needs `libgtk-3` and `libwebkit2gtk-4.1`.

## 3. Create or import a wallet

On first run, create a new wallet or import an existing mnemonic / syrius keystore (`.dat` files are byte-compatible both ways). The mnemonic is shown **once** at creation — write it down offline.

## 4. Choose a node

**Settings → Node** offers three modes:

| Mode | What it is |
|---|---|
| **Remote** (default) | A third-party public node over `wss://` — zero setup. |
| **Local** | Your own `znnd` at `ws://127.0.0.1:35998`. |
| **Embedded** | A full go-zenon node running *inside* the wallet — no separate install; expect an initial sync (progress is shown live). |

## 5. Enable mainnet sending (deliberate step)

Out of the box the wallet will not sign mainnet transactions. When you're ready: **Settings → Network Configuration → Enable mainnet transactions**, and confirm the warning. Verify the connected node's chain matches the configured Chain ID (`1` for mainnet) — mismatches are rejected at signing.

## 6. WalletConnect (bridge dApps)

**WalletConnect** in the sidebar pairs the wallet with Zenon bridge dApps (paste a `wc:` URI or scan). Sessions can only request bridge wrap/redeem operations, every request shows a full decoded confirmation before signing, and duplicate/replayed requests are blocked by a persistent journal that survives crashes and restarts.

## Testnet

To use a testnet instead: point **Settings → Node** at your testnet RPC (e.g. `ws://172.245.236.40:35998`) and set **Chain ID** to the testnet's id (`73404` for the public testnet). Testnet-gated experimental features surface only while connected to a non-mainnet chain.

## Troubleshooting

| Symptom | Fix |
|---|---|
| macOS "app is damaged / can't be opened" | `xattr -dr com.apple.quarantine /path/to/syrius.app`, then right-click → Open. |
| "mainnet sending is disabled" | Enable it explicitly: Settings → Network Configuration → Enable mainnet transactions. |
| Sends rejected for chain mismatch | Make the configured Chain ID match the connected node's chain (shown in Settings). |
| Embedded node syncing slowly | Initial sync downloads the full ledger; the sidebar height tracks live progress. Remote mode needs no sync. |
| WalletConnect screen says not configured | Only affects self-built binaries: set `VITE_WALLETCONNECT_PROJECT_ID` (see `frontend/.env.example`) and rebuild. Official releases ≥ `v0.3.2` are configured. |
| Wallet locked itself | Inactivity auto-lock (default 5 min). Adjust in Settings → Auto-lock. |

Report issues: https://github.com/0x3639/go-syrius/issues — include platform, OS version, and the release you used.

## Build from source (developers)

Requires Go 1.25.x, Node 22 + pnpm 10.17.1, and the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`).

```bash
# run (a parent go.work on some machines requires GOWORK=off)
GOWORK=off wails dev
# package
GOWORK=off wails build           # Linux: add -tags webkit2_41 (+ libgtk-3-dev libwebkit2gtk-4.1-dev)
```

Frontend (in `frontend/`): `pnpm install --frozen-lockfile`, `pnpm run typecheck`, `pnpm test`, `pnpm run build`. For a working WalletConnect in local builds, copy `frontend/.env.example` to `frontend/.env.local` and set your own WalletConnect project id.

## Releases

Releases are cut from a version tag via `.github/workflows/release.yml` (cross-platform build matrix → per-platform assets + `SHA256SUMS` → GitHub Release; a `-suffix` in the tag marks a pre-release). The workflow's actions are pinned to immutable commit SHAs with a CI guard against unpinned refs. Current builds are **unsigned**; code signing + notarization are planned (Phase 7c).

---

*Unsigned beta builds — use at your own risk. See the [security audit](docs/security-audit-2026-07-20.md) for the wallet's verified guarantees and their limits.*
