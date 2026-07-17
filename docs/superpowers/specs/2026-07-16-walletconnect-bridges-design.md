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
