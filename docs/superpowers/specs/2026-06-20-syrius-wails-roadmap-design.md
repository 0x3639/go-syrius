# syrius-wails — Roadmap Execution Design

**Date:** 2026-06-20
**Status:** Approved
**Scope:** A single higher-level execution plan spanning all phases of `plan.md`. This is the *how-we-run-it* layer on top of the *what-we-build* roadmap in `plan.md`; each phase later gets its own detailed implementation plan.

## Context

Reimplement the Zenon `syrius` wallet (Flutter/Dart) as a Go + Wails v2 desktop app. The hard backend already exists in Go (`znn-sdk-go` + `go-zenon`), so the work is a frontend rebuild plus a security-reviewed Wails binding layer. Full feature roadmap, architecture, compatibility invariants, and security plan live in `plan.md` and are not duplicated here.

### Decisions locked for this plan

- **Planning scope:** whole roadmap, at execution-strategy altitude (not deep per-phase specs).
- **Team / parallelism:** two tracks — backend (Go services) and frontend (Svelte UI) — run concurrently with **backend leading**.
- **Coordination model:** **backend-leads.** Backend implements phase N's services and runs `wails generate module` to emit TypeScript bindings; frontend consumes those *real* generated bindings to build phase N's UI while backend moves to phase N+1. No mock backend. The binding contract (`plan.md` §5) is authored by backend and is the living synchronization artifact.
- **Ledger:** deferred to post-v1 (stretch). Phase 6 is not on the v1 critical path.

### Prerequisites status (gates Phase 0)

In hand:
- `znn-sdk-go` access (importable).
- Mainnet remote node URL (for read-only RPC).

**Not** in hand — must be acquired before/within Phase 0:
- A real syrius `.dat` keystore + its password (for the round-trip address-match test).
- Testnet node URL + funded testnet address (for the end-to-end tx test).

## Execution model

### Backend-leads cadence

Steady state: frontend trails backend by ~one phase. While backend builds phase N+1 services, frontend builds phase N screens against freshly generated bindings. The binding contract (method signatures, DTO shapes, event names) is backend's output and frontend's input — every contract change regenerates bindings and the frontend reacts.

### Hard gates between phases

These are correctness fences, not coordination points. No phase overlap is allowed across them.

- **Gate 0 → 1:** an existing syrius wallet opens with a byte-correct index-0 `z1…` address AND a testnet tx confirms. Until this passes, no *product* UI work starts — no screens, routes, or binding-consuming components. Non-product frontend scaffolding (the Wails/Svelte/Vite/Tailwind toolchain and an empty component/design-system skeleton) is explicitly permitted before the gate, since it consumes no bindings and proves nothing about the foundation; it is the frontend track's Phase 0 prep (see the phase table).
- **Gate 2 → mainnet:** the crypto-critical path (keystore, derivation, hashing, signing, PoW) is independently reviewed and exhaustive testnet testing passes — explicitly including **at least one end-to-end PoW send** (an unfused address → `requiredDifficulty > 0`), since Phase 0's confirmed testnet tx used the plasma path and did not exercise PoW at the integration level. Mainnet send stays behind a build flag until this clears.
- **Gate 3:** a wallet created by this app opens in real syrius and vice-versa (round-trip interop).

### Cross-cutting tracks (continuous from day one — not phases)

- Security discipline (`plan.md` §7): no secrets to the WebView, confirm-what-you-sign, memory hygiene.
- CI: `govulncheck` / `gosec` and a build-matrix stub.
- Testnet-gate: any mainnet code path stays behind a build flag until its phase gate clears.

## Phase 0 adaptation

Phase 0 as written in `plan.md` assumes a real `.dat` and testnet funds, neither of which we have. Two prerequisite tasks are prepended:

- **P0-a — Acquire a reference keystore.** Install the real syrius, create a throwaway wallet, record its password and its displayed index-0 `z1…` address. That file + address becomes the round-trip test vector committed to `internal/compat/testdata/`. Throwaway funds only.
- **P0-b — Acquire testnet access.** Get a testnet node URL and fund the throwaway address via faucet.

The rest of Phase 0 proceeds as written: keystore round-trip → read-only RPC against the mainnet node → testnet build→PoW→sign→publish → record exact Argon2 params and keystore layout as a compatibility note in the repo.

## Phase-by-phase sequence

Legend: **B** = backend track (leads). **F** = frontend track (trails ~one phase, consumes generated bindings).

### Phase 0 — De-risking spike
- **B:** new Go module, import SDK; P0-a/P0-b prereqs; keystore round-trip; read-only RPC (mainnet node); testnet tx end-to-end; compat note.
- **F:** no bindings yet — bootstrap Wails + Svelte + TypeScript + Vite + Tailwind tooling and a small component/design system so the team is ready when Phase 1 bindings land.
- **Gate:** 0 → 1.

### Phase 1 — Wails skeleton + read-only wallet
- **B:** `wails init`; `ConfigService` (data dir, settings); `WalletService` (unlock/lock/list/accounts); `NodeService` (remote mode only); SDK subscriptions → Wails events; **emit the binding contract** (`plan.md` §5).
- **F:** unlock screen → dashboard (ZNN/QSR/ZTS balances, address with copy + QR, recent transactions, live sync/connection status) against Phase 1 bindings.
- **Gate:** unlock a real wallet; correct balances and history; live-updating status. Read-only.

### Phase 2 — Transactions (send / receive)
- **B:** `TxService.Send` (template → autofill → PoW *or* plasma → sign → publish) with progress events; cancellable PoW (context); receive flow (`ToUnreceivedAccountBlocksByAddress` → build/sign/publish receive blocks); robust error surfaces (insufficient plasma/balance, node rejection, timeout).
- **F:** send UI (recipient `z1…` checksum validation, token/amount selector, plasma-vs-PoW indicator, **confirm-what-you-sign modal**, progress, success/failure with tx hash); receive UI with optional auto-receive; address book.
- **Gate:** Gate 2 → mainnet (independent crypto-path review + exhaustive testnet).

### Phase 3 — Wallet lifecycle (create / import / manage)
- **B:** create wallet (mnemonic generation, write keystore); import from mnemonic and from keystore file; multi-account derivation; change password (re-encrypt); reveal mnemonic (password-gated).
- **F:** create flow with forced backup confirmation (show once, verify N random words); import flows; account-index switcher; change-password and reveal-mnemonic UIs with warnings.
- **Gate:** Gate 3 (syrius round-trip interop).

### Phase 4 — Embedded & local node modes
- **B:** `NodeService` gains local mode and embedded mode (import go-zenon, run in-goroutine with managed lifecycle: start/stop, data dir, genesis/seeders config, sync progress).
- **F:** node management UI (mode switcher, sync %, peer count, height, resync/reset controls); initial-sync UX, disk-usage warnings, clean shutdown.
- **Gate:** embedded node syncs; wallet operates against it identically to remote mode.

### Phase 5 — Network of Momentum features
- **B:** `NomService` per embedded-contract API — plasma/fusion, staking, pillars (delegate/undelegate), sentinels, tokens (ZTS issue/mint/burn/transfer), Accelerator-Z. Bridge / Liquidity / HTLC optional.
- **F:** one screen per feature, reusing the confirm-what-you-sign modal pattern.
- **Gate:** staking, delegation, plasma, and token ops work end-to-end on mainnet with small amounts.

### Phase 6 — Ledger hardware wallet (deferred post-v1)
- Stretch. Pure-Go HID+APDU vs cgo binding to `ledger_ffi_rs` decided when started; plug into `TxService` as an alternate `Signer`; ship Linux udev rules.
- **Gate:** a transaction signed on-device confirms on-chain.

### Phase 7 — Hardening, packaging, release
- **B:** GitHub Actions cross-platform matrix; code signing + notarization (macOS), signing (Windows), AppImage/deb + udev rules (Linux); dependency audit; threat-model review.
- **F:** accessibility, keyboard navigation, polish.
- **Gate:** signed installers for Windows/macOS/Linux from reproducible CI; security review closed.

## Cross-cutting tracks (detail)

- **Binding contract (`plan.md` §5):** backend keeps method signatures, DTO shapes, and event names as the single source of truth; every change regenerates bindings and the frontend reacts. Enforced in code review: no secrets ever returned to JS; every state-changing method re-validates inputs in Go; long operations emit progress events rather than blocking.
- **Security (`plan.md` §7):** threat model written during Phase 1; confirm-what-you-sign rendered from the *built block* (not form inputs) from the first Phase 2 send; memory hygiene and no-sensitive-logging as standing review checklist items; independent crypto-path review is the Phase 2 → mainnet gate.
- **CI/CD:** `govulncheck` + `gosec` wired in Phase 1 (cheap early); cross-platform build matrix stubbed in Phase 1, hardened to signed/notarized releases in Phase 7; deps pinned (`go.sum`), `znn-sdk-go` vendored/pinned.
- **Testnet-gate:** any mainnet code path stays behind a build flag until its phase gate clears.

## Definition of done for v1

Per `plan.md` §10: opens existing syrius wallets and produces syrius-readable wallets; reliable mainnet send/receive of ZNN/QSR/ZTS; plasma/fusion, staking, and pillar delegation functional; all three node modes work; signed installers for all three OSes from reproducible CI; crypto-critical path independently reviewed; no secrets ever cross into the WebView. Ledger is a stretch goal, not a v1 requirement.

## Out of scope for this plan

- Detailed per-phase implementation plans (each phase gets its own spec → plan cycle).
- Ledger protocol design (deferred with the phase).
- Final UI visual design / brand decisions.
