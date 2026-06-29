# go-syrius

A reimplementation of the Zenon **syrius** wallet as a **Go + [Wails v2](https://wails.io)** desktop app (Vue 3 + TypeScript frontend). It reuses the proven Go crypto/node stack — [`znn-sdk-go`](https://github.com/0x3639/znn-sdk-go) and `go-zenon` — so wallet files and transactions interoperate with the original syrius.

Working today: read-only wallet, send/receive, wallet lifecycle (create/import/manage), all three node modes (remote/local/embedded), the full Network-of-Momentum feature set (plasma, staking, pillars, sentinels, tokens, accelerator), and an experimental **Governance** module (browse / vote / propose / execute) gated to testnet.

---

## ⚠️ This release is for TESTNET testing only

> **`v0.1.0-testnet` is an unsigned pre-release.** Use a **throwaway wallet** and **testnet funds only — never real/mainnet funds or seed phrases.** The builds are not code-signed or notarized, so your OS will warn you the first time you open them (steps below). The **Governance** feature is gated to testnet and is hidden + blocked on mainnet.

---

## Testnet network (use these exact settings)

| Setting | Value |
|---|---|
| **RPC endpoint** | `ws://172.245.236.40:35998` |
| **Network / Chain ID** | `73404` |

> Mainnet is Chain ID `1`; on mainnet the Governance tab is hidden and the governance write methods are blocked. You **must** be on the testnet RPC + Chain ID `73404` to test governance.

---

## 1. Download

From the release page: **https://github.com/0x3639/go-syrius/releases/tag/v0.1.0-testnet**

| OS | File |
|---|---|
| **macOS** (Apple Silicon) | `go-syrius-v0.1.0-testnet-macos.zip` |
| **Windows** (x64) | `go-syrius-v0.1.0-testnet-windows-amd64.exe` |
| **Linux** (x64) | `go-syrius-v0.1.0-testnet-linux-amd64.tar.gz` |

### Verify your download (recommended)

Download `SHA256SUMS` from the same release and check it:

```bash
# macOS
shasum -a 256 go-syrius-v0.1.0-testnet-macos.zip
# Linux
sha256sum go-syrius-v0.1.0-testnet-linux-amd64.tar.gz
# Windows (PowerShell)
Get-FileHash go-syrius-v0.1.0-testnet-windows-amd64.exe -Algorithm SHA256
```

The output hash must match the line for your file in `SHA256SUMS`.

## 2. Install & open (unsigned build)

Because the build isn't signed, the OS blocks it on first launch. This is expected.

- **macOS:** unzip → drag `syrius.app` to Applications (optional) → **right-click the app → Open → Open**. If it still refuses:
  ```bash
  xattr -dr com.apple.quarantine /path/to/syrius.app
  ```
- **Windows:** run the `.exe`. On the SmartScreen prompt → **More info → Run anyway**.
- **Linux:** `tar -xzf go-syrius-v0.1.0-testnet-linux-amd64.tar.gz` → `chmod +x syrius` → `./syrius`. Needs `libgtk-3` and `libwebkit2gtk-4.1` installed.

## 3. Create or import a wallet

On first run, **create a new wallet** (or import one) — use a **fresh testnet wallet**, not anything holding real funds. Save the mnemonic somewhere safe; it's shown once.

## 4. Connect to the testnet node

**Settings → Node:**
1. Select **Remote**.
2. Set the URL to **`ws://172.245.236.40:35998`**.
3. Click **Apply node** and wait until the status reads **Connected**.

**Settings → Network Configuration:**
1. It shows **"Connected node chain: 73404"**.
2. Set **Chain ID** to **`73404`** to match, then **Apply network**.

> If "Configured Chain ID differs from the connected node's chain" appears, your Chain ID and the node don't match — fix it here, or sends/votes will be rejected.

## 5. Get testnet funds & plasma

You need a small amount of testnet **ZNN/QSR** to do anything that writes on-chain:
- **Voting / executing** needs **plasma** (fuse some QSR via the **Plasma** tab, or generate PoW — the wallet does this automatically when plasma is low).
- **Proposing** a governance action costs **1 ZNN** (non-refundable) plus plasma.

Use your usual testnet faucet / source to fund the wallet address (shown on the Receive card).

## 6. Enable Governance

Governance is off by default. **Settings → Testnet features → check "Show Governance."** A **Governance** tab then appears in the top navigation.

> The tab only appears while connected to testnet (Chain ID ≠ 1). If you don't see it, re-check steps 4–6.

## 7. Use Governance

The Governance tab has three sub-tabs:

- **Vote** — appears only if the active wallet account **owns a pillar**. Lists open actions; pick Yes / No / Abstain. (Votes target the action's current round automatically.)
- **Actions** — browse all governance actions (filter by status, paginate, expand for details). An **Execute** button appears on an approved-but-unexecuted action (uncommon — approval usually auto-executes).
- **Propose** — submit a new action (costs **1 ZNN**). For an easy end-to-end test, pick **`Spork — Create`**: fill the action name / description / URL, plus a spork name (5–40 chars) and description, then **Propose**. Bridge and Liquidity action kinds are also available (these need bridge/liquidity admin state and will typically be rejected at execution on a normal testnet — the proposal still posts and is votable).

**Suggested test flow:** Propose a Spork → switch to a pillar-owning account → **Vote** on it → watch the tally update in **Actions**.

## What to report

Please report:
- Any console/runtime errors (open the dev console if available, or note the on-screen error).
- Whether votes land and the tally increments.
- Anything confusing in the propose → vote → execute flow.
- Platform + OS version + the build you used.

Open an issue: https://github.com/0x3639/go-syrius/issues

## Troubleshooting

| Symptom | Fix |
|---|---|
| Governance tab missing | Connect to the testnet RPC, set Chain ID `73404` (not `1`), then enable it under Settings → Testnet features. |
| "mainnet sending is disabled" / governance blocked | You're connected to mainnet (Chain ID 1). Governance is testnet-only; connect to the testnet node. |
| Sends/votes rejected for chain mismatch | Make the configured Chain ID match the connected node (`73404`). |
| Vote view shows a "pillar operators" note | The active account doesn't own a pillar; switch to a pillar-owning account. |
| macOS "app is damaged / can't be opened" | `xattr -dr com.apple.quarantine /path/to/syrius.app`, then right-click → Open. |
| Propose fails | You need ≥1 testnet ZNN + plasma; check the connected chain is testnet. |

## Build from source (developers)

Requires Go 1.25.x, Node 22 + pnpm 10.17.1, and the Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`).

```bash
# run (a parent go.work on some machines requires GOWORK=off)
GOWORK=off wails dev
# package
GOWORK=off wails build           # Linux: add -tags webkit2_41 (+ libgtk-3-dev libwebkit2gtk-4.1-dev)
```

Frontend (in `frontend/`): `pnpm install --frozen-lockfile`, `pnpm run typecheck`, `pnpm test`, `pnpm run build`.

## Releases

Releases are cut from a version tag via `.github/workflows/release.yml` (build matrix → packaged per-platform assets + `SHA256SUMS` → GitHub Release; pre-release when the tag carries a `-suffix`). Current builds are **unsigned**; code signing + notarization are planned.

---

*Testnet only. Not affiliated with or audited for mainnet use. Use at your own risk.*
