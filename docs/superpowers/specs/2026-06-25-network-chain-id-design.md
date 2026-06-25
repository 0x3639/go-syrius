# Network Configuration (Chain ID) for RPC — design

**Date:** 2026-06-25
**Branch:** `network-chain-id` (off `main` `bb7ee9c`)
**Type:** Backend feature (Go) + a small Settings UI addition. The first post-Vue-migration feature.

## Context

The Vue migration is merged. Today the wallet's chain identifier is **derived from the connected node** — the SDK (`zenon/utils.go:141`) fills a block's `ChainIdentifier` from the node's frontier momentum only when the block leaves it at `0`. So transacting on testnet currently requires pointing at a testnet RPC node and relying on that derivation.

This feature adds an **explicit, user-configurable Chain ID** (per the nom/syrius wallet's Network Configuration), so the wallet **builds and signs blocks for the configured chain** over an RPC connection — `1` = mainnet, `73404` = (a) testnet. The **embedded node stays mainnet-only** (unchanged). Only **Chain ID** is exposed: `znnd` maps the chain id to the network id internally, so no Network ID field is needed.

## Goal & non-goals

- **Goal:** a `chainId` setting that the wallet uses when building every transaction, with a Settings UI field + a connected-node mismatch warning. Set `73404` + connect to a testnet RPC → testnet transactions.
- **Non-goals:** embedded-node testnet (embedded stays mainnet); a Network ID field; changing the keystore/derivation/PoW/signing crypto (the chain id only changes which `ChainIdentifier` is committed in the block — the SDK already supports this).

## Technical background (verified)

- The Zenon `AccountBlock` commits `ChainIdentifier` in its hash/signature (8 bytes, big-endian) — so a block built for chain `73404` is valid only on testnet, and vice-versa.
- `znn-sdk-go` `PrepareBlock` (`zenon/utils.go:141`): `if transaction.ChainIdentifier == 0 { transaction.ChainIdentifier = momentum.ChainIdentifier }` — it derives from the node **only when unset**; a pre-set chain id is kept.
- All block-building flows go through **three** sites in `app/tx_service.go`: `PrepareSend` (send, ~L144), `prepareCall` (the shared helper for **every** NoM + token op, ~L245), and `Receive` (~L285). Setting the chain id at these three points covers the whole funds surface.
- `tx_service.go` already validates the built block's chain against the node: `if b.ChainIdentifier != t.node.currentChainID()` (~L210) → "connected node chain differs". This stays as the safety gate.
- `mainnetChainID = 1`; the mainnet-send guard checks `currentChainID() == mainnetChainID` (the node's chain).

## Design

### Backend

1. **`Settings.ChainID uint64`** (`app/dto.go`), JSON `chainId`, **default `1`**. Migration: settings loaded without the field (or `0`) are normalized to `1` (mainnet) on read — so existing mainnet users are unaffected.
2. **`TxService` applies the configured chain id to every built block.** Add a small helper `func (t *TxService) configuredChainID() uint64` reading `ConfigService` settings (normalizing `0`→`1`). At each of the three block-building sites, set `template.ChainIdentifier = t.configuredChainID()` **before** `PrepareBlock`/`Send` (so the SDK keeps it instead of deriving). One helper, three call sites.
3. **Validation stays + clearer message.** The existing `b.ChainIdentifier != node.currentChainID()` check now compares the *configured* chain against the *connected node's* chain; on mismatch return a clear error: "configured Chain ID (N) does not match the connected node's chain (M); set the correct Chain ID in Settings or connect to a matching node."
4. **Expose the node's chain id to the frontend.** Add `ChainID uint64` to the `NodeStatus` DTO (already emitted via the `node:status` event + returned by `NodeStatus()`), set from `currentChainID()`. This lets Settings show the connected node's chain + compute the mismatch warning. No new binding needed beyond the DTO field.
5. **Mainnet-send guard:** unchanged (keys off `currentChainID()`, the node's chain). Since the configured chain id is validated to equal the node's chain before publish, the guard remains consistent.
6. **Embedded node:** unchanged — mainnet only.

### Frontend (Vue)

7. **`Settings.vue` → a "Network Configuration" section** (under/near the Node section), matching the nom wallet: a single editable **Chain ID** field bound to `settings.chainId` (via the existing `ConfigService.GetSettings`/`SetSettings`). Show the **connected node's chain id** (from `node.chainId` / the status) and a **mismatch warning** when the configured chain ≠ the node's chain ("You're configured for chain N but connected to a chain-M node; sends will be rejected until these match."). Apply persists via `SetSettings`. Default shows `1`.
8. The frontend `node` store surfaces `chainId` from the `node:status` event (add to the status type).

### Data flow

Set Chain ID in Settings → persisted in `ConfigService` → `TxService` reads it and stamps every built block's `ChainIdentifier` → `PrepareBlock` keeps it → the block is signed for that chain → validated against the connected node before publish → confirm-what-you-sign modal shows the built block (unchanged) → publish.

## Funds-safety

- The chain id is committed in the **signed block**; a wrong value can't silently send on the wrong chain — the node-match validation rejects a mismatch **before** publish with a clear error. The confirm-what-you-sign modal already renders the built block.
- No new key-material exposure; the crypto path (keystore/derivation/hash/sign/PoW) is untouched — only which `ChainIdentifier` integer is set on the block.
- Default `1` (mainnet) preserves today's mainnet behavior. The mainnet-send guard is unchanged.

## Testing

- **Backend:** `configuredChainID()` normalizes `0`/unset → `1`; a built send block carries `template.ChainIdentifier == settings.chainId` (test `PrepareSend` + `prepareCall` + `Receive` stamp the configured chain); the validation rejects a configured-vs-node mismatch with the clear error; `NodeStatus.ChainID` reflects `currentChainID()`. (Unit tests + the existing integration tests behind `//go:build integration` where a live node is needed.)
- **Frontend:** the Settings Chain ID field loads/persists via `ConfigService`; the mismatch warning shows when `settings.chainId !== node.chainId`.
- **Gates:** `go test ./...`, `go vet`, `gosec`/`govulncheck`; `pnpm test`/`typecheck`/`build`; controller live `wails dev` on testnet — set Chain ID `73404`, connect to a testnet RPC, send a small tx end-to-end (confirm modal → publish), and verify a deliberately-wrong Chain ID is rejected with the clear error.

## Risks

- **Behavior change for zero-config testnet:** today, connecting to a testnet RPC "just works" (derive-from-node). With an explicit default of `1`, a testnet user must now set Chain ID `73404` (else the mismatch validation blocks sends with the clear error). This is the intended explicit-control model; document it. (Alternative considered: default `0`=auto/derive — rejected because the nom-wallet UI shows an explicit `1` and the user wants explicit control.)
- **All three block sites must be covered** — missing one (e.g. `Receive`) would build that block with a derived chain id, inconsistent with the configured one. The plan covers all three + tests each.
- **`NodeStatus` DTO change** ripples to the frontend `node` store + any status consumers — small, but verify the status event/type on both sides.
- This is the first **backend** change since the migration; CI's `build-test` matrix + security gates apply (no `GOWORK` in CI).
