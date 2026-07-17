# WalletConnect durable publication design (WC-01)

**Date:** 2026-07-17
**Status:** approved implementation direction
**Remediates:** WC-01 in `docs/walletconnect-audit-2026-07-17.md`

## Problem

`PublishRawTransaction` returning an error does not prove the node rejected the
block, and a successful publication is recorded only in the in-memory Pinia
request object. Either failure mode (lost RPC response, app exit between
publication and result retention) lets the same restored WalletConnect request
be prepared and published as a **new** block against the advanced frontier —
a duplicate mainnet transfer.

## Design

### Request identity and intent hash

`WalletConnectSendRequest` gains `topic` (session topic) and `requestId`
(JSON-RPC id), supplied by the frontend from the `session_request` event.
Go computes an **intent hash**: SHA-256 over the *validated, reconstructed*
template fields (`chainIdentifier|blockType|address|toAddress|amount|zts|data`)
— never over raw dapp JSON.

### Publication journal

A JSON journal `walletconnect-publications.json` lives in the backend data
directory (same atomic temp-file + rename pattern as `settings.json`; the
WebView is never the funds-safety authority). One record per `topic/requestId`:

```
{ intentHash, state: "signed" | "published", blockJson, hash, createdAt }
```

- `signed` — the finalized signed block was persisted **before** the first
  broadcast attempt. A publish error leaves the record in `signed`: the outcome
  is *unknown*, never "definitely failed".
- `published` — the node accepted the broadcast, or reconciliation found the
  block on chain. The stored `blockJson` is the exact value the WalletConnect
  response must deliver.

Records are deleted when the frontend acknowledges result delivery
(`AckWalletConnectResult`) and capped at 32 entries (oldest evicted) as a
retention bound. The signed block contains only public material (signature,
public key) — persisting it leaks no secrets.

### Flow

- **Prepare** (`PrepareWalletConnectSend`, now returning
  `WalletConnectPrepareResult{preview?, published?, publishedHash, outcome}`):
  - journal hit with **matching** intent hash:
    - `published` → return the stored block JSON; the frontend responds with it
      and acks. No new block is ever built.
    - `signed` → `outcome: "unknown"`; the frontend enters the reconcile flow.
  - journal hit with **different** intent hash → fail closed (reused id).
  - no record → normal validation/hold flow; the hold additionally records the
    WalletConnect identity + intent hash (cleared with the hold).
- **Confirm** (`ConfirmWalletConnectPublish`): after PoW/sign and all existing
  re-checks, the signed block is journaled (`signed`) *before* broadcast; on
  broadcast success the record moves to `published`. On broadcast error the
  hold is cleared (the journal now owns the block) and a
  `publication outcome unknown` error is returned — distinguishable from
  definite pre-broadcast failures, which continue to report normally.
- **Reconcile** (`ReconcileWalletConnectPublication(topic, requestId)`):
  query the node by block hash; found → `published`. Not found → rebroadcast
  the **exact stored signed block** (never a rebuilt one); success →
  `published`. Still failing → remains `signed`/unknown and the error is
  reported; reconcile is retryable.
- **Ack** (`AckWalletConnectResult(topic, requestId)`): called by the frontend
  after `respond()` delivers the result; deletes the record.

### Frontend

`handleRequest` passes `topic`/`requestId` to prepare and short-circuits
journal hits: `published` → respond + ack (no modal); `unknown` → a terminal
"outcome unknown" dialog whose only actions are **Check outcome** (reconcile →
respond + ack on success) and **Close locally**. A confirm error carrying the
unknown-outcome marker enters the same dialog instead of the retryable error
state. Publication-unknown state is never converted into a dapp rejection.

### Test seam

`TxService` gains injectable `prepareBlockFn` / `publishFn` / `blockByHashFn`
(defaulting to the real SDK calls). This enables the WC-01 acceptance tests
(transport-error-after-acceptance, restart-shaped replays, altered-intent
rejection, reconcile-by-query and reconcile-by-rebroadcast) and the WC-04
ordering test (opt-out during PoW never reaches the publisher) without a live
node.

### Out of scope

First-party sends keep their existing semantics (the hold either publishes or
reports the error; no journal). Extending durability to them can reuse the same
journal later.
