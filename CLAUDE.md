# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

This is a **greenfield repository**. As of this writing the only artifact is `plan.md` — a complete development plan. No Go module, frontend, or build tooling exists yet. `plan.md` is the authoritative spec; read it before substantial work, and keep it in sync as decisions change.

## What this project is

A reimplementation of the Zenon `syrius` wallet (originally Flutter/Dart) as a **Go + Wails v2** desktop app. The core insight driving the whole design: the hard cryptographic/node backend already exists in Go — `github.com/0x3639/znn-sdk-go` (BIP39/BIP44 HD wallets, keystore encryption, pure-Go PoW, all 11 embedded-contract APIs, WebSocket RPC) and `go-zenon` (the full node). Because Wails runs a Go backend, these are direct imports rather than FFI boundaries. The work is therefore mostly a **web frontend rebuild plus a thin, security-reviewed binding layer** over an SDK the author owns.

## Stack (locked decisions — see plan.md §6)

- **Wails v2** (not v3 — stability for a funds-handling app)
- **Go 1.22+**, importing `znn-sdk-go` (vendored/pinned, author-controlled) and `go-zenon`
- **Svelte + TypeScript + Vite**, **Tailwind CSS**, Svelte stores for state
- Build via Wails CLI + GitHub Actions cross-platform matrix

## Architecture

### Binding boundary (the central invariant)

The frontend (WebView) must **never** receive a private key, mnemonic seed, or decrypted keystore. The frontend sends *intent* ("send X ZNN to Y"); Go builds → PoWs → signs → publishes. Mnemonics surface exactly once at creation and via an explicit, password-gated `RevealMnemonic`. Every state-changing Go method re-validates its inputs — never trust frontend validation. Long operations (especially PoW, which takes seconds) emit progress events instead of blocking.

### Service layout (planned — see plan.md §4)

Wails-bound services live under `app/`, each a clear seam: `WalletService` (unlock/lock/accounts), `NodeService` (node modes + status events), `TxService` (build→pow→sign→publish), `NomService` (plasma/stake/pillar/sentinel/token/accelerator), `LedgerService` (Phase 6), `ConfigService` (settings/data dir). Non-bound internals under `internal/`: `signer/` (software | ledger abstraction), `powmgr/` (cancellable PoW), `compat/` (keystore compatibility + tests). Frontend under `frontend/src/` (routes, lib/stores, lib/components, lib/bindings).

### Three node modes

`NodeService` abstracts all three behind one interface so the frontend only sees "mode / connected / syncing / height / peers":
1. **Remote** — `wss://` third-party node (built first)
2. **Local** — user-run `znnd` at `ws://127.0.0.1:35998`
3. **Embedded** — `go-zenon` imported and run in-process (goroutine); the feature where Wails materially beats the Flutter original

### Frontend ⇄ Go contract

Methods and events are enumerated in plan.md §5. Events flow Go→frontend via `runtime.EventsEmit`: `node:status`, `node:sync`, `wallet:locked`, `tx:pow-progress`, `tx:signed`, `tx:published`, `tx:received`, `balance:updated`. Define this contract early and keep it stable; both sides build against it.

## Correctness-critical compatibility (plan.md §2)

These invariants determine whether wallet files and transactions interoperate with the original syrius. Most are *inherited* from `go-zenon`'s `common/types` and crypto via the SDK rather than reimplemented — that is why the project is tractable. Do not diverge from them:

- **Keystore format** — must read/write existing syrius `.dat` keystores byte-compatibly: same Argon2 variant + params (memory/iterations/parallelism), same AES mode, same layout.
- **Address derivation** — BIP39 → BIP44 → Ed25519 → `z1…` must match syrius byte-for-byte.
- **PoW links** — nonce algorithm + difficulty must match go-zenon's verifier exactly.
- **Tx hashing & signing** — AccountBlock hash + Ed25519 signature must verify on-chain.
- **ABI encoding** — embedded-contract call encoding must match.

The acceptance test for compatibility: a wallet created here opens in syrius and vice-versa.

## Working order (phases — plan.md §3)

Ordered by risk; do not start UI-heavy work before the foundation is proven.
- **Phase 0** — de-risking spike: keystore round-trip against a *real* `.dat`, read-only RPC, one testnet tx end-to-end. Proves compatibility before any UI.
- **Phase 1** — Wails skeleton + read-only wallet (remote node only)
- **Phase 2** — send/receive (the correctness-critical milestone)
- **Phase 3** — wallet lifecycle (create/import/manage)
- **Phase 4** — embedded & local node modes
- **Phase 5** — NoM features (plasma, staking, pillars, sentinels, tokens, accelerator)
- **Phase 6** — Ledger (optional for v1; highest unknown — pure-Go HID vs cgo to `ledger_ffi_rs`)
- **Phase 7** — hardening, packaging, signed releases

## Security rules (non-negotiable — plan.md §7)

- No secrets ever cross into the WebView; treat the frontend as untrusted for key material.
- **Confirm-what-you-sign:** the confirm modal renders the effect derived from the *built block*, not from raw form inputs.
- Minimize decrypted-seed lifetime; never log anything sensitive.
- Testnet-gate everything; the crypto-critical path (keystore, derivation, hashing, signing, PoW) gets independent review before any mainnet path ships.
- CI runs `govulncheck` and `gosec`; deps pinned with `go.sum`.

## Commands

No build/test/lint commands exist yet (no code). Once scaffolded with `wails init` (Svelte-TS template), the standard toolchain will be `wails dev` (run), `wails build` (package), `go test ./...` (backend tests), and the Vite/frontend scripts in `frontend/package.json`. Update this section with the real commands once they exist.
