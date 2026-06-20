# Rebuilding `s y r i u s` in Wails — Complete Development Plan

A start-to-finish plan to reimplement the Zenon `syrius` wallet (currently Flutter/Dart) as a Go + Wails desktop application.

---

## 0. Executive summary

`syrius` is a cross-platform non-custodial wallet for Zenon's Network of Momentum. The original is ~97% Dart (Flutter), with native crypto and node integration wired in through Dart FFI: Argon2 (KDF), PoW links (feeless tx), an embedded go-zenon full node, and Ledger hardware support.

The key insight that shapes this entire plan: **the hard backend already exists in Go.** Your own `github.com/0x3639/znn-sdk-go` is ~98% complete and provides, in pure/idiomatic Go:

- BIP39/BIP44 HD wallets with keystore encryption
- Pure-Go PoW generation (sync + async, context-cancellable)
- All 11 embedded contract APIs (Pillar, Sentinel, Token, Plasma, Stake, Accelerator, Bridge, Liquidity, HTLC, Swap, Spork)
- Enhanced WebSocket RPC client with auto-reconnect
- Crypto primitives (Ed25519, SHA3, Argon2) via go-zenon's own type system

Because Wails uses a Go backend, this SDK becomes a direct import rather than something accessed across an FFI boundary. The embedded full node (go-zenon) is *also* Go, so embedded-node mode becomes a native import too. The project therefore reduces from "reimplement a wallet's cryptography and node plumbing" to **"build a web frontend and a thin Wails binding layer over an SDK you already own."**

This is one of the cases where Wails is a genuinely strong fit, not merely a possible one.

### Recommended stack

| Layer | Choice | Rationale |
|---|---|---|
| Shell | **Wails v2** (stable) | v3 is alpha; a wallet handling real funds should not sit on an alpha framework |
| Backend | Go 1.22+, `znn-sdk-go`, `go-zenon` | Already-built, tested SDK + native node |
| Frontend | **Svelte + TypeScript + Vite** | Light, reactive, small bundle; good for a data-dense wallet UI. (React is a fine alternative if you prefer it.) |
| Styling | Tailwind CSS | Fast iteration; matches a modern wallet aesthetic |
| State | Svelte stores (or Zustand if React) | Simple reactive state synced to Go events |
| Build/CI | Wails CLI + GitHub Actions matrix | Mirrors syrius's existing release pipeline |

---

## 1. Architecture

### 1.1 Process & data-flow model

```
┌──────────────────────────────────────────────────────────┐
│  Wails App (single binary)                                │
│                                                            │
│  ┌──────────────────┐         ┌───────────────────────┐  │
│  │  Frontend (WebView)│  bind  │  Go Backend            │  │
│  │  Svelte + TS       │◄──────►│  app/ (Wails-bound)    │  │
│  │  - routes/screens  │ events │   ├── WalletService    │  │
│  │  - stores          │        │   ├── NodeService      │  │
│  │  - components       │        │   ├── TxService        │  │
│  └──────────────────┘         │   ├── LedgerService    │  │
│                                │   └── ConfigService    │  │
│                                │         │              │  │
│                                │         ▼              │  │
│                                │  znn-sdk-go            │  │
│                                │   (rpc, wallet, pow,   │  │
│                                │    embedded contracts) │  │
│                                │         │              │  │
│                                │         ▼              │  │
│                                │  go-zenon (embedded    │  │
│                                │   node, opt-in)        │  │
│                                └───────────────────────┘  │
└──────────────────────────────────────────────────────────┘
                  │ WebSocket (ws://… or wss://…)
                  ▼
        Local node / Remote node / Embedded node
```

### 1.2 The three node modes (must match syrius)

1. **Embedded Node** — bundle go-zenon, run it in-process/in-goroutine, connect locally. Heaviest, most "full node" trust model.
2. **Local Node** — connect to a user-run `znnd` at `ws://127.0.0.1:35998`.
3. **Remote Node** — connect to a third-party node over `wss://`.

`NodeService` abstracts these behind one interface so the frontend only ever sees "connected / syncing / height / mode."

### 1.3 Binding boundary principle

The frontend must **never** see a private key, mnemonic seed, or decrypted keystore. All signing happens in Go. The frontend sends *intent* ("send X ZNN to address Y"); the backend builds, PoWs, signs, and publishes. Mnemonics are shown exactly once at creation and on explicit reveal (password-gated).

---

## 2. Compatibility: the things that must match exactly

These are the correctness-critical invariants. Getting any of them wrong means either incompatible wallet files or invalid transactions.

| Concern | Requirement | Where it lives |
|---|---|---|
| **Keystore format** | Must read/write existing syrius `.dat` keystores: same Argon2 variant + params (memory, iterations, parallelism), same AES mode, same file layout | go-zenon `wallet/keyfile.go`; mirrored in `znn-sdk-go/wallet` |
| **Address derivation** | BIP39 mnemonic → BIP44 path → Ed25519 → `z1…` address must match syrius byte-for-byte | `znn-sdk-go/wallet` |
| **PoW links** | Nonce algorithm + difficulty must match go-zenon's verifier exactly | `znn-sdk-go/pow` |
| **Tx hashing & signing** | AccountBlock hash construction + Ed25519 signature must verify on-chain | `znn-sdk-go` + go-zenon types |
| **ABI encoding** | Embedded-contract call encoding must match | `znn-sdk-go/abi` |

> Because `znn-sdk-go` already imports `go-zenon`'s own `common/types` and crypto, most of these are inherited rather than reimplemented — which is exactly why this is tractable. **Phase 0 exists to prove this empirically before any UI work.**

---

## 3. Phased delivery

Each phase is independently shippable/testable and ordered by risk. Don't start UI-heavy work until Phase 0 + 1 prove the foundation.

### Phase 0 — De-risking spike (foundation proof) ⏱ ~3–5 days

**Goal:** prove wallet-file and transaction compatibility *before* committing to UI.

- [ ] New Go module; import `znn-sdk-go`.
- [ ] **Keystore round-trip:** open a *real* existing syrius `.dat` file with the SDK, decrypt with its password, derive address index 0, and confirm the `z1…` address matches what syrius shows. This single test de-risks the trickiest compatibility concern.
- [ ] **Read-only RPC:** connect to a known public/remote node, fetch frontier momentum and an account's balances.
- [ ] **Testnet transaction:** on testnet, build → autofill → PoW → sign → publish a send, and confirm it lands. This proves PoW + signing end-to-end.
- [ ] Write down the exact Argon2 params and keystore layout discovered, as a compatibility note for the repo.

**Exit criteria:** existing wallet opens with correct address; a testnet tx confirms. If both pass, the rest is "normal software."

---

### Phase 1 — Wails skeleton + read-only wallet ⏱ ~1–2 weeks

**Goal:** a real window that opens an existing wallet and shows balances/history. No sending yet.

- [ ] `wails init` (Svelte-TS template). Establish repo layout (§4).
- [ ] `ConfigService`: app data dir, settings persistence (node mode, selected node URL, theme).
- [ ] `WalletService`: list wallets in data dir, unlock by password, lock, current address(es), switch account index.
- [ ] `NodeService`: Remote-node mode only for now; connection lifecycle + status events to frontend.
- [ ] Frontend: unlock screen → dashboard. Dashboard shows ZNN/QSR/ZTS balances, address w/ copy + QR, recent transactions, sync/connection status.
- [ ] Wire SDK subscriptions → Wails events → Svelte stores (live momentum height, connection state).

**Exit criteria:** unlock a real wallet, see correct balances and history, live-updating connection status. Read-only, safe.

---

### Phase 2 — Transactions (send / receive) ⏱ ~2–3 weeks

**Goal:** the correctness-critical milestone — move funds reliably.

- [ ] `TxService.Send(toAddress, tokenStandard, amount, data)`: template → autofill → PoW *or* plasma → sign → publish, with progress events (esp. PoW, which can take seconds).
- [ ] Cancellable PoW via context (SDK already supports `GeneratePowAsync` + cancel) wired to a "Cancel" button.
- [ ] Receive flow: detect unreceived blocks (`ToUnreceivedAccountBlocksByAddress`), present them, build + sign + publish receive blocks. Optional auto-receive toggle.
- [ ] Send UI: recipient validation (`z1…` checksum), amount + token selector, fee/plasma vs PoW indicator, confirm modal showing exactly what will be signed, success/failure states with tx hash.
- [ ] Address book (local, encrypted-at-rest or plaintext-in-data-dir — decide).
- [ ] Robust error surfaces: insufficient plasma, insufficient balance, node rejection, timeout.

**Exit criteria:** repeated reliable send + receive on testnet; then a small mainnet validation with trivial amounts. Get the signing/PoW/hashing path **reviewed** before mainnet (see §7).

---

### Phase 3 — Wallet lifecycle (create / import / manage) ⏱ ~1–2 weeks

**Goal:** full key management, syrius-compatible.

- [ ] Create new wallet: generate mnemonic, **forced backup confirmation** flow (show once, verify N random words), set password, write keystore.
- [ ] Import from mnemonic; import from existing keystore file.
- [ ] Multi-account: derive/manage multiple addresses from one seed (account index UI).
- [ ] Change password (re-encrypt keystore), export/reveal mnemonic (password-gated, with warnings).
- [ ] Confirm files written are byte-compatible with syrius (open them in syrius as the acceptance test).

**Exit criteria:** wallet created here opens in syrius and vice-versa.

---

### Phase 4 — Embedded & local node modes ⏱ ~2–4 weeks

**Goal:** the feature where Wails materially beats the Flutter original — go-zenon as a native import, not FFI.

- [ ] `NodeService` gains Local + Embedded modes.
- [ ] Embedded: vendor/import go-zenon, run node in-process (goroutine) with managed lifecycle (start/stop, data dir, genesis/seeders config). Surface sync progress to UI.
- [ ] Node management UI: mode switcher, sync %, peer count, height, resync/reset controls.
- [ ] Handle the heavy lifting: initial sync UX, disk usage warnings, clean shutdown on app exit.

**Exit criteria:** app runs a full embedded node, syncs, and the wallet operates against it identically to remote mode.

---

### Phase 5 — Network of Momentum features ⏱ ~3–5 weeks

**Goal:** parity on the NoM-specific functionality syrius exposes. Each maps to an SDK embedded-contract API you already have.

- [ ] **Plasma / Fusion:** fuse QSR for an address, list/cancel fusions, show current plasma.
- [ ] **Staking:** stake ZNN (duration selector), list active stakes, cancel, collect rewards.
- [ ] **Pillars:** list pillars, delegate/undelegate, show delegation + rewards; (optional) pillar registration for operators.
- [ ] **Sentinels:** register/revoke, collect.
- [ ] **Tokens (ZTS):** issue, mint, burn, transfer ownership, view token info.
- [ ] **Accelerator-Z:** browse projects/phases, donate, (optional) voting for pillar operators.
- [ ] (Optional/advanced) **Bridge / Liquidity / HTLC** screens.

**Exit criteria:** staking, delegation, plasma, and token ops all work end-to-end against mainnet with small amounts.

---

### Phase 6 — Ledger hardware wallet ⏱ ~2–4 weeks (highest unknown)

**Goal:** hardware-signing parity. This is the messiest area because the original uses a Rust FFI lib (`ledger_ffi_rs`) and there's no first-class Go Zenon-Ledger binding.

- [ ] Choose an approach:
  - **(a)** Pure-Go HID + APDU: talk to the Zenon Ledger app directly using a Go HID library (e.g. `karalabe/hid`) and implement the app's APDU protocol. Most self-contained; most protocol work.
  - **(b)** cgo binding to `ledger_ffi_rs` / hidapi: reuse the existing Rust/C work. Less protocol re-derivation; reintroduces a native build dependency (the thing Wails otherwise lets you avoid).
- [ ] Implement: device detection, address derivation on-device, display-and-confirm signing, plug into `TxService` as an alternate signer.
- [ ] Linux: ship the udev rules (mirror syrius's `udev/` directory) so non-root users can access the HID device.

**Exit criteria:** a transaction signed on-device confirms on-chain.

> If Ledger support is not essential for v1, defer this phase — it carries the most schedule risk and is cleanly separable.

---

### Phase 7 — Hardening, packaging, release ⏱ ~2–3 weeks

- [ ] Cross-platform builds: Windows, macOS (universal), Linux — Wails matrix in GitHub Actions (mirror `syrius_builder.yml`).
- [ ] Code signing + notarization (macOS), signing (Windows). Linux: AppImage/deb + udev rules.
- [ ] Auto-update strategy (or signed-release + manual, matching syrius's cadence).
- [ ] Accessibility, keyboard nav, error telemetry (opt-in/none, given it's a wallet).
- [ ] Security pass (§7), threat-model review, dependency audit (`govulncheck`, `gosec`).
- [ ] Docs: build instructions, CONTRIBUTING, threat model, compatibility notes.

**Exit criteria:** signed installers for all three OSes; reproducible CI builds; security review closed.

---

## 4. Suggested repository layout

```
syrius-wails/
├── wails.json
├── go.mod
├── main.go                      # Wails app bootstrap
├── app/                         # Wails-bound services (the binding boundary)
│   ├── app.go                   # App struct, lifecycle (startup/shutdown)
│   ├── wallet_service.go        # unlock/lock/list/accounts
│   ├── node_service.go          # remote/local/embedded modes + status events
│   ├── tx_service.go            # send/receive: build→pow→sign→publish
│   ├── nom_service.go           # plasma/stake/pillar/sentinel/token/az
│   ├── ledger_service.go        # hardware signing (Phase 6)
│   ├── config_service.go        # settings + data dir
│   └── events.go                # typed event names emitted to frontend
├── internal/
│   ├── signer/                  # signing abstraction (software | ledger)
│   ├── powmgr/                  # cancellable PoW orchestration
│   └── compat/                  # keystore compatibility helpers + tests
├── frontend/
│   ├── src/
│   │   ├── routes/              # unlock, dashboard, send, receive, stake, …
│   │   ├── lib/
│   │   │   ├── stores/          # wallet, node, tx stores
│   │   │   ├── components/      # AddressInput, AmountInput, TxModal, QR, …
│   │   │   └── bindings/        # generated Wails bindings (wailsjs)
│   │   └── app.css              # Tailwind entry
│   ├── index.html
│   ├── package.json
│   └── vite.config.ts
├── build/                       # platform build assets, icons, installers
│   ├── windows/  ├── darwin/  └── linux/   (incl. udev rules)
└── .github/workflows/
    └── build-release.yml        # cross-platform matrix
```

---

## 5. The Wails binding contract (frontend ⇄ Go)

Define this interface early and keep it stable; both sides build against it.

**Methods (Go → exposed to JS):**

```
// Wallet
ListWallets() []WalletMeta
Unlock(name, password string) error
Lock() error
CurrentAccounts() []AccountInfo
SelectAccount(index int) error
CreateWallet(password string) (mnemonic string, err error)   // reveal once
ImportMnemonic(mnemonic, password, name string) error
RevealMnemonic(password string) (string, error)
ChangePassword(old, new string) error

// Node
SetNodeMode(mode string, url string) error                   // remote|local|embedded
NodeStatus() NodeStatus                                      // mode, connected, height, peers, syncing
EmbeddedStart() error / EmbeddedStop() error

// Balances & history
GetBalances() []TokenBalance
GetTransactions(page, count int) []TxRecord
GetUnreceived() []UnreceivedBlock

// Transactions
Send(req SendRequest) (txHash string, err error)             // emits pow/sign/publish progress events
ReceiveAll() error
CancelPow(jobId string) error

// NoM
Fuse / CancelFusion / Stake / CancelStake / CollectRewards
Delegate / Undelegate / RegisterSentinel / …
IssueToken / MintToken / BurnToken / …
ListProjects / Donate / …
```

**Events (Go → frontend, via `runtime.EventsEmit`):**

`node:status`, `node:sync`, `wallet:locked`, `tx:pow-progress`, `tx:signed`, `tx:published`, `tx:received`, `balance:updated`.

**Hard rules:**
- No method ever returns a private key, seed, or decrypted keystore (mnemonic only via explicit, password-gated `RevealMnemonic` / one-time `CreateWallet`).
- Every state-changing method validates inputs in Go; never trust frontend-side validation.
- Long operations emit progress events rather than blocking.

---

## 6. Key technical decisions to lock early

1. **Wallet-file compatibility: yes.** Read & write syrius-compatible keystores so users migrate seamlessly and can run both wallets. (Phase 0 proves it.)
2. **Frontend framework: Svelte-TS** for a lean, reactive, data-dense UI. Switch to React only if your team's familiarity outweighs bundle size.
3. **Wails v2, not v3.** Stability over features for a funds-handling app.
4. **SDK as a vendored dependency you control.** Since `znn-sdk-go` is yours, pin it and evolve it alongside the app; fixes flow both ways.
5. **Ledger: defer to post-v1** unless hardware support is a launch requirement. It's the single biggest schedule risk and cleanly separable.
6. **Signing lives only in Go**, behind a `Signer` interface with `software` and (later) `ledger` implementations.

---

## 7. Security plan (non-negotiable for a non-custodial wallet)

- **Threat model up front:** keys at rest (keystore encryption), keys in memory (zeroization where feasible), keys in transit to frontend (never), malicious/remote node (validate everything client-side that's validatable), supply chain (pin deps, `go.sum`, `govulncheck`/`gosec` in CI).
- **Confirm-what-you-sign:** the confirm modal must render the exact effect (recipient, token, amount, contract call) derived from the *built block*, not from the raw form inputs.
- **No secrets to the WebView.** Treat the frontend as untrusted for key material.
- **Memory hygiene:** minimize lifetime of decrypted seeds; avoid logging anything sensitive; scrub buffers after use where Go allows.
- **Independent review of the crypto-critical path** (keystore, derivation, hashing, signing, PoW) before mainnet. Even though the SDK is tested, the *integration* is new.
- **Exhaustive testnet testing** before any mainnet path is enabled in a build.
- **Reproducible, signed releases** so users can trust binaries.

---

## 8. Risk register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Keystore params mismatch → can't open existing wallets | Low (SDK inherits go-zenon) | High | Phase 0 round-trip test against a real file |
| PoW/signing mismatch → invalid txs | Low | Critical | Phase 0 testnet tx; reviewed signing path |
| Ledger protocol work underestimated | High | Medium | Defer to post-v1; pick pure-Go HID vs cgo deliberately |
| SDK gaps vs Dart SDK surface | Medium | Medium | You own the SDK — fill gaps directly; it's already 98% |
| Embedded node lifecycle/resource issues | Medium | Medium | Treat as its own phase; robust start/stop, disk warnings |
| Frontend rebuild scope (full UI) | High | Medium | Phase ordering; ship read-only first, expand outward |
| Mainnet incident from a subtle bug | Low | Critical | Testnet-gate, security review, small-amount mainnet validation |

---

## 9. Rough timeline (single experienced dev)

| Phase | Scope | Estimate |
|---|---|---|
| 0 | De-risking spike | 3–5 days |
| 1 | Skeleton + read-only | 1–2 weeks |
| 2 | Send/receive | 2–3 weeks |
| 3 | Wallet lifecycle | 1–2 weeks |
| 4 | Embedded/local node | 2–4 weeks |
| 5 | NoM features | 3–5 weeks |
| 6 | Ledger (optional v1) | 2–4 weeks |
| 7 | Hardening + release | 2–3 weeks |
| **Total** | **incl. Ledger** | **~14–23 weeks** |
| **Total** | **v1 without Ledger** | **~11–18 weeks** |

Parallelizing frontend and backend work, or reusing existing UI patterns, compresses this meaningfully.

---

## 10. Definition of done for v1

- Opens existing syrius wallets; wallets created here open in syrius.
- Send/receive ZNN, QSR, and ZTS reliably on mainnet.
- Plasma/fusion, staking, and pillar delegation functional.
- All three node modes work (remote, local, embedded).
- Signed installers for Windows, macOS, Linux from reproducible CI.
- Crypto-critical path independently reviewed; no secrets ever cross into the WebView.
- (Stretch) Ledger hardware signing.

---

### Appendix: why this is tractable

The original syrius spends most of its complexity budget on bridging Dart to native code (Argon2, PoW, node, Ledger) through FFI. In a Wails/Go world, three of those four are *native Go already*: the SDK (yours, 98% done) covers crypto/PoW/wallet/RPC/contracts, and go-zenon covers the embedded node as a plain import. Only Ledger remains a genuine native-integration problem — and it's optional for v1. What's left is mostly a frontend rebuild plus a disciplined, security-reviewed binding layer.