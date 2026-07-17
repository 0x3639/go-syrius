# WalletConnect bridge integration design

**Date:** 2026-07-16
**Status:** approved implementation direction

## Goal

Allow go-syrius to pair with both the deployed `0x3639/bridge-dapp` and the new `nom-bridge` without bridge-specific code, while preserving the binding-boundary and confirm-what-you-sign invariants.

## Shared protocol

Both dapps use WalletConnect v2 with the same CAIP contract:

- namespace: `zenon`
- chain: `zenon:1`
- account: `zenon:1:<z1 address>`
- methods: `znn_info`, `znn_sign`, `znn_send`
- events: `chainIdChange`, `addressChange`

WalletConnect SignClient 2.23.x may normalize the unregistered custom `zenon` namespace from a dapp's required namespace into the wallet proposal's `optionalNamespaces`, leaving `requiredNamespaces` empty. The wallet accepts the exact frozen Zenon contract from either proposal field, omits unrelated optional namespaces, and still rejects every unsupported required namespace.

`znn_info` returns `{ address, chainId, nodeUrl? }`. It reads `NodeService.NodeStatus()` for every handshake rather than trusting the event-driven Pinia snapshot, so a reconnect cannot reject a valid session using stale chain state. A configured node URL containing username/password, query-string, or fragment credentials is omitted from the response. Automatic request failures are also retained in the WalletConnect screen because dapps may collapse multiple protocol codes into a generic rejection message. `znn_send` supplies `{ fromAddress, accountBlock }`, where `accountBlock` is the JSON output of the TypeScript SDK's `AccountBlockTemplate.toJson()`.

## Trust boundary

WalletConnect transport and session state run in the Vue WebView. No key, mnemonic, decrypted keystore, signing key, or signing primitive crosses into it.

For `znn_send`, Go accepts only the immutable intent fields (`chainIdentifier`, `blockType`, `address`, `toAddress`, `amount`, `tokenStandard`, and base64 `data`). Contract templates may carry the zero address; any populated sender must match the active account. Go rejects the wrong chain, wrong sender, non-user-send block type, unknown destinations, malformed values, and non-canonical ABI payloads. It reconstructs a clean block and never trusts dapp-supplied hash, frontier, height, plasma, nonce, public key, or signature fields.

The first release permits only calls to the embedded Bridge contract. `WrapToken` must attach a positive amount. `Redeem` must attach zero ZNN, matching the canonical SDK template; a funded or non-ZNN redeem is rejected before confirmation. That is sufficient for token bridging in both dapps and avoids granting a connected website a generic transaction surface. Additional embedded contracts can be added deliberately later using the same exact ABI decode and preview path.

Go derives the confirmation preview from the reconstructed block and byte-exact decoded ABI call. Only one backend block may await confirmation; a racing WalletConnect or first-party prepare is refused rather than replacing the block already displayed. After the user confirms, the held-block gate rechecks the wallet session, active account, mainnet opt-in, connected chain, destination, token, amount, and call data before frontier fill, PoW, signing, and publish.

`znn_sign` is advertised for compatibility but arbitrary payload signing is not enabled in this phase. Requests fail explicitly until a domain-separated signing format and user-facing semantic preview are specified.

## Mainnet policy

Chain ID 1 and a mainnet remote node can be configured today, but writes are fail-closed behind `AllowMainnetSend`. Settings gains an explicit, warning-backed mainnet-transactions toggle. WalletConnect uses the same guard; pairing never bypasses it.

## UX

The WalletConnect screen provides:

1. a `wc:` URI field and pair action;
2. a session proposal showing dapp name/URL, requested chain, methods, and the account to expose;
3. connected-session cards with disconnect;
4. a request approval dialog showing the Go-derived Bridge method, exact decoded fields, amount/token, sender, destination, and PoW status.

Pairing requires `VITE_WALLETCONNECT_PROJECT_ID`, a project id owned by the wallet application. It is never a secret, but production builds must configure it rather than reuse either dapp's identity.

## Compatibility and lifecycle

The WalletConnect client uses the same v2 SignClient protocol version exercised by both bridges. Sessions persist using the WalletConnect client storage and are reconciled to the active account after unlock. Wallet lock rejects requests that have not started publication with wallet-locked code `9000`; it never pre-empts an in-flight publish. User rejection uses code `5000`. Account changes cancel an awaiting preview before updating the session and emitting `addressChange`. Session deletion/expiry releases an awaiting backend hold without attempting a response on the ended topic.

Publication and WalletConnect result delivery are separate states. Once Go returns a published block, the wallet records its result/hash and can only retry delivering that success result. A relay failure, session end, account change, or wallet lock can never convert an already-published request into an error response; the UI warns that the transaction moved and must not be submitted again. If publication fails after the session ends, the exact retained hold is cancelled before local request state is dropped.

## Verification

- Go unit tests cover malformed input, wrong sender/chain/block type, non-Bridge destinations, canonical Wrap/Redeem funding, non-canonical ABI, occupied and mismatched held-block identity, all WalletConnect prepare gates, and returned published JSON.
- Frontend tests cover namespace validation, authoritative `znn_info`, credential-safe node URL disclosure, locked request rejection, rejection/cancellation ordering, result-delivery failure after publication, lock during publication, session deletion, duplicate request serialization, account reconciliation, SignClient initialization retry, and the consent-state behavior of the mainnet toggle.
- Typecheck, Vitest, Go tests, and frontend production build must pass.
- Manual acceptance pairs each bridge independently, reads `znn_info`, previews a Bridge request, rejects one request, and publishes one small mainnet request after the explicit opt-in.

### Acceptance progress (2026-07-16)

- `bridge.0x3639.com`: live WalletConnect pairing approved successfully and go-syrius displayed the deployed Zenon Bridge session with the expected `zenon:1:<active-address>` account.
- No transaction was requested or published during this connection-only check.
- The deployed WalletConnect modal's copy control did not populate the in-app browser clipboard; decoding the displayed QR into the same `wc:` URI and pasting it into go-syrius paired successfully. This did not affect the WalletConnect protocol handshake.
- `nom-bridge`: live WalletConnect pairing approved successfully against the local dapp and go-syrius displayed the NoM Bridge session with the expected `zenon:1:<active-address>` account. SignClient delivered its custom namespace under `optionalNamespaces`; compatibility is covered by the frozen-namespace tests. Transaction preview/rejection/publication acceptance remains to be run with a deliberately small mainnet amount.
- A follow-up lifecycle/security review was applied locally: published-result delivery is terminal, lock/session/account races are fail-safe, racing prepares cannot replace a held block, canonical method funding is enforced, initialization is retryable, and the test claims above now correspond to implemented tests. No real-funds transaction was submitted during this review.

### Audit remediation (2026-07-17)

All eight findings of `docs/walletconnect-audit-2026-07-17.md` are remediated:

- **WC-01** — publication is durable and idempotent: a per-request journal in
  the backend data directory persists the signed block before broadcast,
  classifies broadcast errors as *unknown* (reconciled by hash query or exact
  rebroadcast, never a rebuilt block), and replays journaled outcomes for
  redelivered requests. See
  `docs/superpowers/specs/2026-07-17-walletconnect-durable-publication.md`.
- **WC-02** — `session_request_expire` cancels preparing/awaiting requests
  without a response; `expiryTimestamp` is re-checked at approval; expiry
  during publication never invents a rejection.
- **WC-03** — custom-token decimals must resolve or preparation fails; the
  confirmation always shows the held block's exact base-unit amount.
- **WC-04** — the mainnet opt-in is re-read immediately before broadcast,
  keyed off the built block's chain identifier.
- **WC-05** — the SignClient Verify context is surfaced on proposals and
  requests; scam-flagged peers cannot be approved; unverified origins carry an
  explicit warning.
- **WC-06** — `znn_info` discloses only bare `ws(s)://host[:port]` origins;
  URLs with userinfo, query, fragment, or any path are omitted.
- **WC-07** — peer metadata icons are never fetched; a local placeholder
  renders instead. (A production CSP remains a deliberate follow-up: it must
  be validated live against the Wails runtime, the WalletConnect relay, and
  the pre-existing lottie-web `eval` dependency.)
- **WC-08** — `proposal_expire` clears only the matching proposal; approval
  re-checks the proposal deadline.

Verification after remediation: Go tests (including new journal/reconcile/
expiry/guard-ordering coverage), `go vet`, Vitest (46 WalletConnect-specific
tests), vue-tsc, and the production build all pass. The mainnet acceptance run
(pair, `znn_info`, preview, reject, one small publish) is still outstanding.

#### Round-2 review fixes (2026-07-17)

A follow-up review of the remediation commit surfaced six findings, all fixed:

1. Journaled requests now resolve **before** the locked-wallet/chain/node
   gates (sender-independent intent validation), so a known outcome is never
   answered with an ordinary rejection; the frontend consults the backend for
   `znn_send` even while locked and maps only *fresh* requests to code 9000.
2. The journal cap no longer evicts: every retained record is duplicate
   protection, so a full journal refuses **new** writes (and therefore new
   broadcasts) while existing records stay updatable for reconciliation.
3. Custom-token decimals are resolved exactly once — missing token metadata is
   an error, and the checked value is stamped into the confirmation preview
   (no second fail-open lookup).
4. Reconciliation serializes under the publication mutex and requires the
   connected node to be on the journaled block's chain before a "not found"
   query result counts as evidence or a rebroadcast is attempted.
5. The Verify known-scam block now runs before *any* method dispatch,
   including `znn_info`.
6. A failed delivery of a replayed published result keeps the standard
   retryable delivery-error state (with the journaled result) instead of a
   dead global error.

#### Round-3 review fixes (2026-07-17)

A third review pass surfaced one remaining P1 and two P2 edge cases, all fixed:

1. **[P1] Journal replay now precedes the frontend policy gates.** A new
   journal-only `Tx.LookupWalletConnectPublication` (no wallet/node gate, never
   creates a hold) resolves a redelivered `znn_send` before the scam,
   existing-request, and busy-`tx.status` gates, so a published/unknown outcome
   is never turned into code 5000 or `-32000`. A fresh (unjournaled) request
   still passes through all those gates and `PrepareWalletConnectSend`; a reused
   id with a different intent fails closed.
2. **[P2] Reconciliation reads the client and chain in one snapshot.**
   `NodeService.connectionSnapshot()` returns both under a single read lock, so
   a node transition can't pair an old client with the new chain identifier
   between the two accessor calls.
3. **[P2] Session end during replay delivery is preserved.** A
   `session_delete`/expire that lands while a replayed result's `respond()` is
   in flight now carries `sessionEnded` into the retained delivery-error state
   (with the ended-session message), so no retry button targets a dead session.

#### Round-4 review fixes (2026-07-17)

A fourth review pass found three race/classification edge cases in the
round-3 replay path, all fixed:

1. **[P1] Lookup no longer hijacks the shared `preparingRequest` slot.** The
   journal lookup runs without mutating `preparingRequest`, so an earlier
   in-flight preparation keeps receiving its own `session_delete` / expiry
   events. Replay delivery claims the slot only when it is free (to track a
   session that ends during its `respond()`), and never while another request
   is in flight.
2. **[P1] A journal-read failure is unknown, not a rejection.** Reused-id /
   different-intent is now a resolved Go `conflict` outcome (frontend →
   code 5000, a safe refusal of the never-approved new intent); a genuine
   lookup throw (journal read / IPC failure) leaves the true outcome unknown
   and answers with a retryable `-32000`, never a definite 5000.
3. **[P2] Replays never clobber a displayed request.** An unknown replay or a
   failed replayed delivery arriving while another request occupies the modal
   no longer overwrites it (which would orphan that request's backend hold):
   the journal record survives for a later redelivery and the condition is
   surfaced non-destructively.

#### Round-5 review fixes (2026-07-17)

A fifth pass found two P1 lifecycle gaps in the round-4 lookup path plus a
retry improvement:

1. **[P1] The looked-up request now tracks its own lifecycle.** A keyed
   `lookupMarkers` collection (separate from the shared `preparingRequest`
   slot) marks the exact request under lookup, so a `session_request_expire` /
   `session_delete` arriving while `LookupWalletConnectPublication` is awaiting
   is observed — an expired request aborts instead of falling through to a
   fresh, approvable hold.
2. **[P1] A journal-read failure no longer answers the dapp at all.** A lookup
   throw leaves the outcome unknown (the block may be published), so sending
   any JSON-RPC response — even `-32000` — is removed: a terminal error could
   make the dapp retry under a NEW id and bypass the journal identity. The
   failure is kept local and retryable; a same-id redelivery re-runs the
   idempotent lookup.
3. **[P2] A pending-replay queue** surfaces an unknown replay or a
   delivery-failed published result promptly when the modal/preparation slot
   clears, instead of relying on the dapp to redeliver. The journal remains the
   durable source of truth; the queue is drained on every request-clearing path
   and purged on session end/expiry.

#### Round-6 review fixes (2026-07-17)

A sixth pass found the round-5 lookup-failure handling and replay queue still
had gaps:

1. **[P1] Failed journal lookups are now actively retried.** SignClient
   suppresses same-id `session_request` re-emission for a client's lifetime, so
   "leave it unanswered and let the dapp redeliver" would not actually retry —
   the request could expire and the dapp reissue under a NEW id (bypassing the
   journal identity) while the original block may have published. The znn_send
   flow is extracted into a shared `resolveZnnSend`; a lookup throw now retains
   the request and schedules bounded backoff retries of the SAME-id lookup,
   stopping at the request's expiry or a max attempt count. A resolved retry
   delivers the original outcome (published → deliver, unknown → reconcile,
   none → fresh) under the original id.
2. **[P2] Slot release and queue drain are centralized.** `drainPendingReplays`
   now loops so multiple queued published results all deliver; the
   fresh-preparation `finally` drains after releasing its marker (so a replay
   queued behind a preparation that ends without a modal still surfaces); and
   approval-time expiry drains too. The delivery retain-vs-queue decision now
   accounts for an in-flight preparation (not just a displayed request), and
   marker identity is compared by token rather than object identity (Pinia
   wraps stored markers in a reactive proxy).
