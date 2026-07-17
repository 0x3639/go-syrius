# WalletConnect branch audit — 2026-07-17

This is a security-focused review of `codex/walletconnect-bridges` at
`1f14487` against `main` at `e12b943`. It is an implementation handoff for
Claude or another engineer. No finding below asserts that a real-funds loss
occurred; no real-funds transaction was submitted during this audit.

## Outcome

The branch gets the most important transaction-content checks right: Go
reconstructs account blocks from a narrow intent DTO, restricts WalletConnect
to the Bridge contract's user operations, byte-round-trips decoded ABI data,
binds previews to a held-block identity and sender session, and rechecks the
material block effect after PoW.

Eight issues remain. WC-01 through WC-03 are merge blockers for a mainnet
release because they can produce duplicate publication or an inaccurate
confirmation for custom tokens.

| ID | Priority | Area | Mainnet blocker |
|---|---:|---|---|
| WC-01 | P1 | Publication durability / idempotency | Yes |
| WC-02 | P1 | Request expiry lifecycle | Yes |
| WC-03 | P1 | Custom-token amount confirmation | Yes |
| WC-04 | P2 | Mainnet opt-out race | Recommended before release |
| WC-05 | P2 | Dapp identity verification | Recommended before release |
| WC-06 | P2 | Node URL credential disclosure | Recommended before release |
| WC-07 | P2 | Untrusted metadata image fetches | Recommended before release |
| WC-08 | P2 | Proposal expiry lifecycle | No |

## Non-negotiable constraints

- Preserve the binding boundary: no key, mnemonic, decrypted keystore, or
  signing primitive may cross into the WebView.
- Preserve confirm-what-you-sign: confirmation must describe the exact held
  block, including its base-unit amount, rather than only a friendly rendering.
- Treat WalletConnect transport, peer metadata, and request data as untrusted.
- Do not turn an uncertain publication outcome into a definite rejection.
- Do not rebuild a fresh block for a WalletConnect request that might already
  have published; retry only the exact signed block or return its stored result.
- Keep the single pending-slot, hold-ID, sender/session, chain, and ABI checks.

## WC-01 — Make publication durable and idempotent across errors and restarts

**Priority:** P1

**Primary locations:**

- `app/tx_service.go:311-340`, especially the error path at `331-334`
- `app/walletconnect.go:129-149`
- `frontend/src/stores/walletconnect.ts:378-429`

### Problem

`PublishRawTransaction` returning an error does not prove that the node did not
accept the block. A connection can fail after the node accepted the request.
The backend nevertheless clears the only copy of the finalized block and
returns an ordinary error. The frontend then exposes a retryable error/reject
path as though nothing moved.

Successful publication state is also kept only in the in-memory Pinia request
object. If the block publishes and the WalletConnect result cannot be relayed,
or the app exits between publication and `publishedResult` assignment, a
restart loses the result while SignClient restores the pending session request
from its persistent store. The request can then be prepared and published as a
new block.

Concrete failure sequence:

1. Go signs block H and the node accepts it.
2. The RPC response or WalletConnect relay response is lost.
3. `pending` is cleared, or the app exits before the frontend retains the
   success result.
4. The same restored/requested intent is presented again.
5. A new block is built against the advanced frontier and can move the funds a
   second time.

The frontend's current “Do not submit it again” state protects only a live
process after `ConfirmWalletConnectPublish` returned successfully. It is not a
durable exactly-once boundary.

### Required behavior

- Associate a WalletConnect hold with a stable request identity (topic + JSON-RPC
  id) and a canonical intent hash in Go. The current backend method never
  receives that identity.
- Persist the finalized signed block and its result/hash before attempting
  publication, then persist the terminal publication state.
- On an RPC error, classify the outcome as unknown. Query by block hash and/or
  rebroadcast only the exact same signed block until the outcome is known.
- A restored request with the same identity and intent must return/re-deliver
  the stored result; it must never build a fresh block.
- A reused identity with different intent must fail closed.
- Persist this state in the backend data directory, not only localStorage or
  Pinia, because the WebView is not the funds-safety authority.
- Bound retention and define cleanup only after result delivery or an explicit,
  safely reconciled terminal state.

### Acceptance tests

- Inject a publisher that records acceptance and then returns a transport
  error. Assert the wallet enters an unknown/published state and cannot prepare
  a replacement block for that request.
- Restart the service after signed-block persistence but before publish, after
  node acceptance but before terminal persistence, and after publish but before
  WalletConnect response delivery. In every case, the same request must resolve
  to H or rebroadcast H only.
- Restore the same topic/id with altered intent and assert rejection.
- Prove a relay failure followed by an app restart re-delivers the stored result
  without calling `PrepareBlock` or producing another account-block hash.

## WC-02 — Cancel or quarantine expired session requests

**Priority:** P1

**Primary locations:**

- `frontend/src/stores/walletconnect.ts:173-202` (`installListeners`)
- `frontend/src/stores/walletconnect.ts:269-377` (`handleRequest`)
- `frontend/src/stores/walletconnect.ts:378-403` (`approveRequest`)

### Problem

SignClient 2.23.9 emits `session_request_expire` with the expired request id and
removes it from the pending-request store. The wallet does not install that
listener or retain the request expiry timestamp.

An approval dialog and backend hold can therefore outlive the dapp's request.
If the user approves it after expiry, Go can publish real funds, after which
`respond()` fails because the request no longer exists. The dapp may already
have timed out and retried elsewhere, creating a duplicate-publication risk.
Even without approval, the orphan hold blocks every other first-party or
WalletConnect prepare.

### Required behavior

- Listen for `session_request_expire` and match the id against both preparing
  and displayed requests.
- Before publication begins, cancel the exact backend hold and remove the local
  request without attempting a response to the expired request.
- Retain and check `params.request.expiryTimestamp` at approval time as defense
  in depth; reject approval when the deadline has passed even if an event was
  delayed.
- If expiry occurs after publication starts, follow the same terminal-state
  rule as session deletion: do not invent a rejection, but persist/reconcile the
  actual publication outcome under WC-01.

### Acceptance tests

- Expire a request while backend preparation is in flight; when preparation
  returns, its exact hold must be cancelled and no modal may appear.
- Expire an awaiting request; assert its exact hold is cancelled and approval
  cannot call `ConfirmWalletConnectPublish`.
- Expire during publication; assert no error response races the publication and
  the durable final result is retained.
- Advance a fake clock beyond `expiryTimestamp` without emitting the event and
  assert approval still fails closed.

## WC-03 — Do not silently guess custom-token decimals in confirmation

**Priority:** P1

**Primary locations:**

- `app/decimals.go:18-37`
- `app/tx_service.go:384-395`
- `frontend/src/components/WalletConnectRequest.vue:34-40`

### Problem

For a custom ZTS, `resolveDecimals` silently falls back to 8 if token metadata
is missing, malformed, or unavailable. A remote node can also report false
metadata. The WalletConnect confirmation displays only the human-formatted
amount using that value; it does not display the held block's raw base-unit
amount.

For example, a held amount of `100000000` can be shown as `1` with an 8-decimal
fallback even when the token actually has 2 decimals (`1000000`). Conversely,
a malicious node can make a large transfer appear tiny. The ZTS is shown, but
the amount itself is not an exact rendering of the block. Custom tokens are a
core Bridge use case, so this violates confirm-what-you-sign.

### Required behavior

- Always show the exact base-unit integer from the held block in the confirmation.
- For custom tokens, fail preparation when decimals cannot be resolved, or show
  a prominent “metadata unavailable” state rather than silently assuming 8.
- Label human formatting as using the reported decimals and continue to show
  the full ZTS. Do not treat node-supplied symbol/decimals as trusted identity.
- Prefer changing the confirmation-specific resolver to return an error/source
  instead of changing unrelated list rendering that intentionally tolerates
  missing metadata.

### Acceptance tests

- Custom token with 2 decimals renders both the correct human amount and exact
  base units.
- Metadata lookup failure never renders an 8-decimal amount as authoritative.
- A node-reported decimals value cannot hide the raw base-unit amount or ZTS.
- ZNN and QSR continue to use their protocol-fixed 8 decimals.

## WC-04 — Recheck the mainnet opt-in immediately before broadcast

**Priority:** P2

**Primary location:** `app/tx_service.go:236-334`

### Problem

The mainnet guard runs before the pending block snapshot and before PoW, which
can take seconds. After PoW the code rechecks wallet session, exact effect, and
connected chain, but not `AllowMainnetSend`.

If the user disables mainnet transactions while PoW is running, the already
started call still reaches `PublishRawTransaction` even though the authoritative
setting is now false. The existing `guard()` also keys off a mutable node chain
snapshot rather than the held/built block's chain, leaving an avoidable
check/use race during node transitions.

### Required behavior

- Immediately before `PublishRawTransaction`, re-read the mainnet setting and
  enforce it based on `built.ChainIdentifier`.
- Keep the earlier guard for fast failure, but make the final block-based check
  authoritative.
- Treat opt-out before broadcast as a definite non-publication and clear or
  retain the hold according to an explicit retry policy.

### Acceptance tests

- Pause after `PrepareBlock`, disable mainnet sends, resume, and assert the
  publisher is never called.
- Race a testnet-to-mainnet node transition around the early guard and assert a
  chain-1 block cannot bypass `AllowMainnetSend`.

## WC-05 — Surface WalletConnect Verify identity instead of trusting metadata

**Priority:** P2

**Primary locations:**

- `frontend/src/stores/walletconnect.ts:176-197`
- `frontend/src/stores/walletconnect.ts:269-377`
- `frontend/src/views/WalletConnect.vue:60-77`
- `frontend/src/components/WalletConnectRequest.vue:22-24`

### Problem

Both `session_proposal` and `session_request` carry a SignClient
`verifyContext`, but the store discards it and presents the peer-controlled
metadata name and URL as the dapp identity. A malicious pairing can label itself
as either trusted bridge. The request dialog makes this worse by showing only
the spoofable name, not even the claimed or verified origin.

Exact ABI confirmation limits the damage, but an impersonating bridge can still
ask the user to wrap to an attacker-controlled external address and rely on the
trusted-looking identity to obtain approval.

### Required behavior

- Retain and display the verified origin, validation state, and scam signal from
  `verifyContext` for proposals and requests.
- Hard-block known-scam results. Show a prominent warning for invalid or unknown
  origin validation; do not style claimed metadata as verified identity.
- Show the verified/claimed origin in the transaction approval dialog, not only
  on the pairing page.
- Define a conservative fallback for Verify service outages.

### Acceptance tests

- Spoofed metadata with an invalid verified origin is visibly untrusted in both
  proposal and request UI.
- A scam-marked context cannot be approved.
- A valid bridge origin is displayed separately from its friendly name.

## WC-06 — Do not disclose credentials embedded in a node URL path

**Priority:** P2

**Primary locations:**

- `frontend/src/stores/walletconnect.ts:100-111`
- `frontend/src/stores/walletconnect.ts:283-312`

### Problem

`publicWalletConnectNodeURL` removes URLs with userinfo, query strings, or
fragments, but returns URLs with arbitrary paths. Hosted WebSocket providers
commonly put project/API tokens in the path, for example
`wss://node.example/v1/secret-project-token`. Every approved dapp can recover
that credential through `znn_info`.

### Required behavior

- Prefer omitting `nodeUrl` entirely unless interoperability strictly requires
  it.
- Otherwise disclose only explicitly public endpoints, with an allowlist or a
  root-path-only rule, and require `ws:`/`wss:`.
- Never infer that a path is non-secret.

### Acceptance tests

- Root public `wss://host[:port]/` may pass under the chosen policy.
- Non-root paths, percent-encoded paths, userinfo, queries, and fragments are
  omitted.

## WC-07 — Stop fetching arbitrary peer-provided icon URLs in the wallet WebView

**Priority:** P2

**Primary locations:**

- `frontend/src/views/WalletConnect.vue:60-63`
- `frontend/src/views/WalletConnect.vue:86-88`

### Problem

WalletConnect peer metadata is untrusted, but its first icon URL is bound
directly to `<img src>`. Merely receiving a proposal or rendering a restored
session can therefore make the privileged desktop WebView issue an attacker-
chosen GET. This leaks the user's IP and proposal/session timing, and can target
loopback or LAN HTTP services. The app currently has no restrictive CSP in
`frontend/index.html` to constrain image or connection origins.

### Required behavior

- Do not load arbitrary remote metadata images. The safest option is a local
  generic dapp icon.
- If icons are retained, fetch through a constrained path that enforces HTTPS,
  blocks loopback/private/link-local destinations after DNS resolution and
  redirects, bounds size, validates image content, and caches a local copy.
- Add a production CSP tailored to the app and WalletConnect relay requirements;
  do not use CSP as the sole SSRF control.

### Acceptance tests

- `http://127.0.0.1`, `http://[::1]`, private/LAN, link-local, non-HTTP schemes,
  redirects to those targets, and oversized/non-image responses are never
  fetched by the WebView.
- Pairing and restored-session rendering work with the local fallback icon.

## WC-08 — Clear expired proposals from the single proposal slot

**Priority:** P2

**Primary locations:**

- `frontend/src/stores/walletconnect.ts:173-202`
- `frontend/src/stores/walletconnect.ts:218-251`

### Problem

SignClient emits `proposal_expire`, but no listener clears the proposal. The UI
continues offering Approve/Reject for an id SignClient has already deleted; both
operations fail and the stale card remains. With only one proposal slot, an
expired proposal can also obscure lifecycle state for later pairing attempts.

### Required behavior

- Listen for `proposal_expire` and clear only the matching proposal id.
- Disable actions after the proposal deadline as defense in depth.
- Do not let an old expiry event clear a newer proposal that replaced it.

### Acceptance tests

- Expiring the displayed proposal removes it and disables approval.
- A delayed expiry for proposal A cannot clear a newer proposal B.

## Verification performed

The following passed on the audited worktree:

- `GOWORK=off GOTOOLCHAIN=auto go test ./...`
- `GOWORK=off GOTOOLCHAIN=auto go test -race ./app`
- `GOWORK=off GOTOOLCHAIN=auto go vet ./...`
- `pnpm run typecheck`
- `pnpm test` — 69 files, 326 tests
- `pnpm run build`
- `pnpm audit --prod` — no known vulnerabilities reported by npm
- `git diff --check main...HEAD`

The frontend build still reports the pre-existing `lottie-web` `eval` warning
and a large main chunk; neither is counted above as a WalletConnect correctness
finding, but the `eval` usage must be considered when adding the production CSP.

`govulncheck` and `gosec` were not installed in this local environment, so
`scripts/govulncheck-gate.sh` and `gosec -conf .gosec.json ./...` could not run.
CI should run both before merge.
