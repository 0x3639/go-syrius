# Phase 2 — Send / Receive Design

**Date:** 2026-06-20
**Status:** Approved (spec); implementation deferred until Phase 1 merges
**Scope:** Phase 2 of the syrius-wails roadmap. Move funds reliably: send (with confirm-what-you-sign) and receive, on testnet by default. The correctness-critical milestone. Builds on Phase 1 (WalletService/NodeService/ConfigService + dashboard).

## Goal

Repeated, reliable send + receive on testnet (plasma and PoW paths), with the confirm-what-you-sign guarantee, then a small mainnet validation only after the Gate 2 review and behind a mainnet flag.

## Locked decisions (brainstorming 2026-06-20)

- **PoW:** progress events only, no mid-PoW cancel (the SDK `zenon.PrepareBlock` runs `pow.GeneratePowBytes` synchronously; we do not modify the SDK).
- **Mainnet gating:** testnet-only by default; mainnet send disabled behind a flag until Gate 2→mainnet clears.
- **Receive:** manual receive of unreceived blocks + an optional auto-receive toggle (default off).
- **Address book:** deferred to a later phase; recipient entry is plain input with z1 checksum validation.
- **No SDK modification:** use the `zenon` facade (`PrepareBlock`/`RequiresPoW`/`PublishRawTransaction`); keystore/keypair via go-zenon + the Phase-0 mnemonic→SDK-keypair bridge.

## Context

Phase 1 provides: unlocked go-zenon keystore held in WalletService (with `Mnemonic`), a connected `rpc_client.RpcClient` in NodeService, balances/history reads, and the dashboard. Phase 0 established `zenon.NewZenon(client).Send/PrepareBlock`, the chain-id guard pattern, and the go-zenon→SDK keypair bridge (derive SDK keypair from the mnemonic, assert its address equals the go-zenon active address).

SDK facts (verified): `zenon.PrepareBlock(template, kp) (*nom.AccountBlock, error)` does autofill → required-PoW → PoW → sign and returns the finalized block **without publishing**; `Send` = PrepareBlock + publish; `Zenon.PowCallback func(pow.PowStatus)` reports Generating/Done; `RequiresPoW(template, kp) (bool, error)`. PoW is **not** context-cancellable via the facade.

## Architecture

### Send pipeline — prepare-then-publish (confirm-what-you-sign)

The confirm modal renders from the **built, signed block** (not raw form inputs), and nothing is broadcast until explicit confirm:

1. **Frontend validation:** recipient via `types.ParseAddress` (z1 checksum), amount > 0 and ≤ balance, token chosen from current balances. A `RequiresPoW` pre-check drives a "feeless (plasma)" vs "PoW (~seconds)" indicator.
2. **`TxService.PrepareSend(req)`:** chain-id guard (reject unless the node's chainId is permitted — mainnet only if the mainnet flag is set) → `LedgerApi.SendTemplate(to, zts, amount, nil)` → `zenon.PrepareBlock` (autofill → PoW → sign, emitting `tx:pow-progress`) → **hold** the finalized block in TxService → return a `SendPreview` rendered from that block (toAddress, symbol, amount, usedPlasma, difficulty, hash). No broadcast.
3. **Confirm modal:** displays the `SendPreview` (the real signed effect + hash).
4. **`TxService.ConfirmPublish()`:** re-assert the held block's `ToAddress`/`Amount`/`TokenStandard`/`Data` equal the originating request (defense-in-depth) → `LedgerApi.PublishRawTransaction` → emit `tx:published{hash}` → clear held block. Returns the tx hash.
5. **`TxService.CancelPending()`:** discard the held block.

TxService holds at most one pending prepared block; a new `PrepareSend`, `CancelPending`, a successful publish, or `wallet:locked` clears it.

Trade-off (accepted): PoW runs during `PrepareSend`, before the confirm modal — a cancelled PoW send wastes that computation. Acceptable given the no-cancel decision; plasma-covered sends do no PoW; the modal showing the real hash is the stronger security property.

### Keypair bridge

`zenon.PrepareBlock` needs an SDK `*wallet.KeyPair`; WalletService holds a go-zenon keystore. WalletService gains an unexported `signingKeyPair() (*sdkwallet.KeyPair, error)`: derive the SDK keystore from the unlocked mnemonic (`sdkwallet.NewKeyStoreFromMnemonic`), `GetKeyPair(activeIndex)`, and **assert** its address equals the go-zenon active address before returning. The mnemonic and SDK keypair stay backend-only, are never logged, and are transient.

## Components

### TxService (`app/tx_service.go`, bound)

Depends on WalletService (`signingKeyPair`, active address) and NodeService (RPC client + active chainId).

- `PrepareSend(req SendRequest) (SendPreview, error)`
- `ConfirmPublish() (string, error)` — returns tx hash
- `CancelPending() error`
- `RequiresPoW(req SendRequest) (bool, error)`
- `Receive(fromHash string) (string, error)` — `ReceiveTemplate(fromHash)` → `zenon.Send` (receive blocks publish directly, no confirm modal) → emit `tx:received{hash}`; returns the hash

Internal: `holdBlock *nom.AccountBlock` + the originating `SendRequest`; cleared on publish/cancel/lock. The chain-id guard reads NodeService's current frontier `ChainIdentifier` and `ConfigService` `AllowMainnetSend`.

### NodeService additions

- `GetUnreceived() ([]UnreceivedBlock, error)` — `GetUnreceivedBlocksByAddress(active, 0, 50)`.
- Auto-receive: when the `AutoReceive` setting is on, subscribe to `SubscriberApi.ToUnreceivedAccountBlocksByAddress(ctx, active)` and receive each arrival; toggle off unsubscribes. Wiring: `App.New` injects a receive callback (`func(fromHash string) error` bound to `TxService.Receive`) into NodeService at construction, avoiding a hard NodeService→TxService dependency.

### DTOs (secret-free)

- `SendRequest{ ToAddress string; Zts string; Amount string }` (amount = base-unit decimal string)
- `SendPreview{ ToAddress, Symbol, Zts, Amount string; UsedPlasma uint64; Difficulty uint64; Hash string; NeedsPoW bool }`
- `UnreceivedBlock{ FromHash, FromAddress, Token, Amount string }`

### Settings additions (ConfigService)

- `AllowMainnetSend bool` (default false) — gates mainnet in the chain-id guard.
- `AutoReceive bool` (default false).

### Events

`tx:pow-progress` (`{state:"generating"|"done"}`), `tx:published` (`{hash}`), `tx:received` (`{hash}`). Existing `wallet:locked` additionally clears any pending prepared block.

## Frontend

- **Send route** (`/send`): `SendForm` (recipient with live z1 validation, token selector from balances, `AmountInput` with max button), a plasma-vs-PoW indicator (from `RequiresPoW`); Send → `PrepareSend` (PoW progress shown if needed) → **`TxModal`** rendered from `SendPreview` (recipient, token, amount, hash, plasma/difficulty) with Confirm/Cancel → Confirm → `ConfirmPublish` → `TxResult` success (tx hash + copy) or error.
- **Receive:** `UnreceivedPanel` on the dashboard (count + per-block Receive + Receive-All); auto-receive toggle in settings.
- **Stores:** `tx` store (pending preview; status idle/preparing/awaiting-confirm/publishing/done/error; last hash) wired to `tx:*` events; `unreceived` store. Reuse `format` util and the confirm-modal pattern from Phase 1.
- **Components:** `SendForm`, `AmountInput`, `TxModal`, `UnreceivedPanel`, `TxResult`.

## Error handling

- Invalid recipient (z1 checksum) → inline form error, Send disabled.
- Amount ≤ 0 or > balance → inline error.
- Insufficient plasma without PoW capability, node rejection, timeout → surfaced on the result/modal with the node's message.
- Mainnet send attempted while flag off → clear "mainnet sending is disabled" error from the chain-id guard.
- Publish failure → `TxResult` error state; held block retained so the user can retry publish or cancel.

## Testing

- **Backend (Go):** chain-id guard (mainnet blocked unless flag; testnet allowed); `ConfirmPublish` built-block-matches-request assertion (and refusal if no pending block); `signingKeyPair` address-match; DTO mappers (`UnreceivedBlock`, `SendPreview`). Integration (`//go:build integration`, gitignored `secrets/`, skip if absent): testnet send via PrepareSend→ConfirmPublish confirms on-chain; receive of an unreceived block; **an unfused-address PoW send (`requiredDifficulty > 0`)** — the Gate 2 carry-forward. Reuse the chain-id-guard test pattern from Phase 0.
- **Frontend (Vitest, mocked bindings):** SendForm validation (bad address, zero/over-balance), TxModal renders preview fields incl. hash, the prepare→confirm→publish store flow, cancel discards pending, UnreceivedPanel receive flow.
- **Manual acceptance:** testnet send (plasma + PoW), receive, confirm-modal correctness — on a real GUI run.

## Security (Gate 2 — non-negotiable)

- Confirm modal renders only from the built, signed block + hash; re-assert built block == request before publish.
- Mnemonic and SDK keypair backend-only and transient; never logged; `Lock()` already zeroes the keystore and clears pending.
- Mainnet send stays behind `AllowMainnetSend` (default false) until the **Gate 2→mainnet** review passes: independent review of the crypto-critical path (keystore, derivation, hashing, signing, PoW) **and** exhaustive testnet testing including the end-to-end PoW send. Only then enable the flag and do a small-amount mainnet validation.
- Testnet-gate everything; the chain-id guard prevents broadcasting a funded-wallet tx against the wrong network.

## Exit criteria (Phase 2 → Phase 3)

- Repeated reliable testnet send + receive across both plasma and PoW paths.
- Confirm-what-you-sign verified (modal from built block; re-assert before publish).
- Crypto-critical path independently reviewed.
- Mainnet send remains flag-gated; a small mainnet validation completed only after the flag is enabled.

## Out of scope (deferred)

- Address book (later phase); wallet creation / import-from-mnemonic / multi-seed / password change / mnemonic reveal (Phase 3); local/embedded node modes (Phase 4); NoM contract features (Phase 5); Ledger (Phase 6); packaging/signing (Phase 7).
- Mid-PoW cancellation (constrained by the unmodified SDK facade).
