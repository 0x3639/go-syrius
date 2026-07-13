# Peer-review remediation handoff — 2026-07-13

This document records the remaining findings from the re-review of branch
`codex-review-fixes` against `main` at `93047be`. It is written as an
implementation handoff for Claude or another engineer.

## Outcome

The previous remediation fixed most of the original review findings. Six
issues remain:

| ID | Priority | Area | Merge blocker |
|---|---:|---|---|
| PR-01 | P1 | Account selection consistency | Yes |
| PR-02 | P1 | Governance confirmation effects | Yes |
| PR-03 | P2 | Subscription setup failure status | No, but fix in this pass |
| PR-04 | P2 | Closed subscription status | No, but fix in this pass |
| PR-05 | P2 | Concurrent node-mode transitions | No, but fix in this pass |
| PR-06 | P2 | Accelerator confirmation metadata | No, but fix in this pass |

Do not merge until PR-01 and PR-02 are resolved. Prefer resolving all six
together because PR-03 through PR-05 share node connection lifecycle code.

## Non-negotiable constraints

- Preserve the binding boundary: no key, mnemonic, or decrypted-keystore data
  may cross into the WebView.
- Preserve confirm-what-you-sign: the confirmation must describe the exact
  effect encoded by the backend-held block, not merely echo frontend state.
- Keep the frontend untrusted. All concurrency and input invariants must hold
  for direct, overlapping Wails calls, even if the current UI normally makes
  those calls unlikely.
- Do not weaken the sender/session checks, hold identity, mainnet guard,
  governance testnet policy, or send/receive publication mutex.
- Use `GOWORK=off` for local Go commands as documented in `AGENTS.md`.

## PR-01 — Serialize account selection end-to-end

**Priority:** P1

**Primary locations:**

- `app/wallet_service.go`, `WalletService.SelectAccount`, currently around
  lines 419–473
- `frontend/src/stores/wallet.ts`, `select`
- `frontend/src/components/AccountSlotPicker.vue`, `pick`

### Problem

`SelectAccount` changes `w.active` under `w.mu`, releases that lock, and then
persists `ActiveAccount` under the separate config mutex. Two calls can
therefore complete with three different notions of the active account:

1. Call A sets backend account A and pauses before persistence.
2. Call B sets backend account B and persists B.
3. Call A persists A and returns last.
4. The frontend processes A last and displays A, while the backend signer still
   uses B.

The current sender binding prevents an already-held transaction from silently
changing sender, but it does not make the wallet's displayed account, receive
address, balances, persisted selection, and backend signer consistent.

### Required behavior

- Account changes must have a single, well-defined order.
- When a successful `SelectAccount(index)` call resolves, the backend active
  index and persisted active index must match the resolved selection unless a
  strictly newer operation has superseded it.
- The frontend must not commit an older selection after a newer selection has
  completed.
- Every actual account transition must continue to increment the wallet
  session generation and invalidate the held transaction.

### Suggested implementation

Serialize the complete backend selection operation, including validation,
`w.active` mutation, session invalidation, and persistence. A dedicated
selection/operation mutex is preferable to holding `w.mu` across config I/O,
because other wallet methods should not be blocked on a disk write while
holding the keystore lock.

An operation-generation/latest-intent design is also acceptable, but it must
ensure that a superseded operation cannot persist stale settings or report
success in a way that causes the frontend to display stale state. Consider
returning the authoritative active index/address rather than making the
frontend assume the requested index won.

The frontend should additionally await selection and suppress or supersede
overlapping picker actions. This is defense in depth, not a replacement for
backend serialization.

### Acceptance tests

- Add a deterministic backend test that pauses two `SelectAccount` operations
  at the persistence boundary and forces the adverse interleaving.
- Assert after both calls finish that backend active address, persisted
  `ActiveAccount`, and the last successful selection agree.
- Assert the wallet generation increments for each real transition but not for
  a no-op selection of the already-active index.
- Add a frontend test with deferred promises resolved out of order; the picker
  must finish on the latest user intent and must not expose stale account data.
- Run the new backend test under `go test -race` as well as ordinary tests.

## PR-02 — Show the decoded governance action in confirmation

**Priority:** P1

**Primary locations:**

- `app/governance_propose.go`, `PrepareProposeAction`, currently around lines
  600–639
- `app/nom_governance.go`, `PrepareExecuteAction`, currently around lines
  139–170
- `app/dto.go`, `CallPreview`
- `frontend/src/components/TxModal.vue`

### Problem

The proposal confirmation currently shows the proposal name, action label, fee,
and destination contract, but omits the parameters encoded in
`payload.Data`. Examples of hidden material effects include:

- the new administrator in `changeAdministrator`;
- guardian addresses in `nominateGuardians`;
- token standards, percentages, and minimum amounts in `setTokenTuple`;
- ZNN/QSR amounts in funding and reward actions;
- the boolean target state in halt/unhalt actions.

`PrepareExecuteAction` exposes opaque base64 ABI bytes. Those bytes are exact,
but they are not a human-verifiable description of the action.

This violates the project's confirm-what-you-sign invariant: knowing only the
destination and method label is insufficient to approve the exact governance
effect.

### Required behavior

- Proposal confirmation must show all proposal metadata and every material,
  typed action parameter.
- Execution confirmation must show the decoded effect of the on-chain action
  being executed, not only base64 data.
- The displayed effect must be derived from the constructed or fetched ABI
  payload. It must not trust a parallel summary assembled solely from raw form
  inputs.
- Unknown or undecodable methods must fail closed rather than showing an
  incomplete friendly summary.
- Full addresses, token standards, integer/base-unit amounts, percentages,
  booleans, and lists must remain unambiguous and must not be truncated.

### Suggested implementation

Introduce a structured effect on `CallPreview`, for example a typed action name
plus ordered label/value fields. Build it by decoding the exact destination and
ABI data placed in the held proposal, or the exact destination/data fetched for
an executable action. Render those structured fields in `TxModal.vue`.

Avoid a second hand-maintained encoder that simply reformats `params`; that can
drift from the SDK payload helpers. If the SDK does not expose a decoder, add a
small, exhaustively tested decoder/mapping at the backend boundary using the
same ABI definitions. Retaining the raw payload as an advanced detail is fine,
but it is not a substitute for decoded fields.

### Acceptance tests

- Table-test every value returned by `proposeKinds()`.
- For each action kind, build its payload, decode the exact returned bytes, and
  assert every supplied parameter appears in the structured effect with the
  correct type/value.
- Include list-heavy cases and large integer amounts.
- Assert malformed, unknown, or destination/method-mismatched payloads fail
  closed.
- Test `PrepareExecuteAction` with fetched actions for representative bridge,
  liquidity, and accelerator/governance destinations.
- Add frontend rendering tests proving full values are displayed without
  truncation and opaque payload data alone is not treated as confirmation.

## PR-03 — Emit disconnected status when subscription setup fails

**Priority:** P2

**Primary location:** `app/node_service.go`, `SetNode`, currently around lines
103–116

### Problem

`SetNode` installs the client and emits `Connected: true` before starting the
momentum subscription. If subscription setup fails, it calls
`disconnectLocked()`, which increments `connGen`. The subsequent
`stillCurrent()` check compares against the old generation and is therefore
always false, so `emitStatus(false)` is skipped.

The method returns an error and removes the client, but listeners retain the
earlier connected status until some later pull or event happens.

### Required behavior

- If the installed connection cannot establish its required subscription, the
  authoritative status must become disconnected.
- A failure from an old generation must never overwrite the status of a newer
  successful connection.
- Status, client ownership, URL, height, and chain ID must describe the same
  connection generation.

### Suggested implementation

Capture whether the failing generation still owned the connection while under
`n.mu`, perform the generation-safe teardown, and publish a status snapshot
that is still valid when emitted. A generation-tagged status event or a helper
that snapshots current state under the mutex is safer than passing a stale
boolean to `emitStatus`.

### Acceptance tests

- Inject a client whose frontier request succeeds but `ToMomentums` fails.
- Assert the observed event sequence does not end in `Connected: true`.
- Assert the service has no current client and clears height/chain ID.
- Race the subscription failure against a newer successful `SetNode`; the old
  failure must not disconnect or visually override the new connection.

## PR-04 — Treat unexpected subscription closure as connection degradation

**Priority:** P2

**Primary location:** `app/node_service.go`, `startMomentumLoop`, currently
around lines 170–198

### Problem

The closed-channel fix prevents a CPU spin, but the goroutine now simply exits
when the momentum channel closes. The service retains the client, cached height
and chain ID, and connected status. The UI can consequently show a healthy node
whose height will never advance.

### Required behavior

- Unexpected closure must trigger a reconnect/resubscribe path or mark the
  owning connection generation disconnected/degraded.
- Expected closure caused by `stop`, supersession, or normal teardown must not
  emit a false failure for the replacement connection.
- The loop must still exit without spinning.

### Suggested implementation

On `!ok`, check under `n.mu` whether this subscription still belongs to the
current generation. If it does, either invoke a bounded reconnect path or clear
the current connection and emit a generation-safe disconnected status. Do not
perform blocking connection work while holding `n.mu`.

### Acceptance tests

- Close the momentum channel unexpectedly and assert one teardown/degraded
  transition occurs with no spin.
- Close an old generation after a newer connection is installed and assert the
  new connection remains untouched.
- Exercise repeated closures and verify no leaked goroutines, double-close
  panic, or duplicate reconnect loop.

## PR-05 — Serialize node-mode transitions, not only node connections

**Priority:** P2

**Primary locations:**

- `app/node_service.go`, `SetNodeMode`, currently around lines 268–298
- `frontend/src/views/Settings.vue`, `applyNode` and `confirmStartEmbedded`

### Problem

The new connection generation protects `SetNode`, but `SetNodeMode` performs a
larger unsynchronized sequence:

1. persist `NodeMode`;
2. read all settings and select a URL;
3. stop or start the embedded node;
4. update `n.mode`;
5. call `SetNode`.

Overlapping mode changes can interleave these steps. The final persisted mode,
`n.mode`, embedded-node lifecycle, and connected URL can belong to different
requests. The Settings Apply button is not disabled while an operation is in
flight, and direct Wails calls must be safe regardless.

### Required behavior

- A mode transition must be one ordered operation.
- The winning transition must own the persisted mode, in-memory mode,
  embedded-node state, selected URL, connection generation, and emitted status.
- Superseded transitions must clean up resources they started and must not stop
  resources owned by the winner.
- The UI must not report success for a superseded transition as though it were
  current.

### Suggested implementation

Use a dedicated mode-operation mutex for the whole transition, or introduce a
mode generation whose ownership is checked at every asynchronous boundary.
Capture the selected mode and its URL from one settings mutation/snapshot rather
than writing, unlocking, and rereading independently. Keep embedded start/stop
outside `n.mu`, but associate their results with the mode generation before
installing them.

Disable or supersede the frontend Apply action while it is in flight as defense
in depth.

### Acceptance tests

- Force remote→local and local→embedded calls to overlap at controlled points.
- Assert the last accepted operation consistently owns settings, `n.mode`, URL,
  client, embedded handle, and status.
- Assert a superseded embedded startup cannot install itself or stop the newer
  mode's connection.
- Assert only one success message is associated with the winning UI request.

## PR-06 — Include all accelerator metadata in confirmation

**Priority:** P2

**Primary locations:**

- `app/nom_accelerator.go`, `PrepareCreateProject`, `PrepareAddPhase`, and
  `PrepareUpdatePhase`, currently around lines 301–360
- `app/dto.go`, `CallPreview`
- `frontend/src/components/TxModal.vue`

### Problem

The revised accelerator summaries correctly expose requested ZNN/QSR amounts.
However:

- create-project confirmation omits the description;
- add-phase confirmation omits description and URL;
- update-phase confirmation omits description and URL.

These values are included in the ABI payload and are part of the exact project
or phase record being signed.

### Required behavior

- Create, add-phase, and update-phase confirmation must show project/phase ID as
  applicable, name, full description, full URL, requested ZNN, requested QSR,
  and the actual template fee/amount.
- Values must be derived from the exact held template payload.
- Long descriptions and URLs must wrap without silent truncation.
- Any decode failure must fail closed.

### Suggested implementation

Use the same structured effect mechanism introduced for PR-02 and decode the
accelerator ABI payload after the SDK constructs the template. This avoids
maintaining security-critical prose summaries independently of the encoded
block.

### Acceptance tests

- Table-test create, add-phase, and update-phase payload decoding.
- Assert all metadata and exact base-unit amounts appear in the effect.
- Include Unicode, long descriptions, long valid URLs, and maximum accepted
  amounts.
- Add frontend tests for wrapping and full-value rendering.

## Recommended implementation order

1. Add a shared structured transaction-effect DTO and renderer.
2. Implement and test governance decoding (PR-02).
3. Implement and test accelerator decoding using the same mechanism (PR-06).
4. Serialize account selection and add deterministic concurrency tests (PR-01).
5. Define generation-safe node status/ownership helpers.
6. Fix subscription setup and closure behavior (PR-03 and PR-04).
7. Serialize the complete node-mode transition (PR-05).
8. Run all gates and update this document with commit hashes and checked boxes.

## Completion checklist

Remediated 2026-07-13 in commits `de4f651` (PR-02 + PR-06), `25157ca`
(PR-01), and `f45279c` (PR-03 + PR-04 + PR-05).

- [x] PR-01 fixed with deterministic backend and frontend concurrency tests
      (`25157ca` — selection mutex + persistence-boundary hook test, race-clean;
      frontend latest-intent queue with deferred-promise tests).
- [x] PR-02 fixed for every proposed governance action kind and execution path
      (`de4f651` — generic fail-closed ABI decoder with repack round-trip;
      exhaustive per-kind table test; ExecuteAction decodes the fetched
      action's destination call). NOTE: the decode exposed that
      `liquidity.unlockStakeEntries` carries NO token standard on-chain — the
      form's zts field never reaches the chain; product follow-up required.
- [x] PR-03 fixed without allowing stale failures to override newer connections
      (`f45279c` — status emitted inside the same critical section as the
      generation-checked state change).
- [x] PR-04 fixed without spins, leaks, or stale status (`f45279c` —
      `degradeConnection` on unexpected channel closure; superseded loops exit
      silently; repeated closures are no-ops).
- [x] PR-05 fixed across remote, local, and embedded transitions (`f45279c` —
      operation mutex over the whole transition, single settings snapshot,
      Apply/Start disabled while in flight).
- [x] PR-06 fixed for create/add/update accelerator flows (`de4f651` — same
      decoder; Unicode/long-string/large-amount tests; frontend full-value
      wrap rendering tests).
- [x] `GOWORK=off GOTOOLCHAIN=auto go test ./...` — all packages ok
- [x] `GOWORK=off GOTOOLCHAIN=auto go vet ./...` — clean
- [x] `GOWORK=off GOTOOLCHAIN=auto go test -race ./...` — all packages ok
- [x] `cd frontend && pnpm test` — 294 passed
- [x] `cd frontend && pnpm run typecheck` — clean
- [x] `cd frontend && pnpm run build` — ok
- [x] `bash scripts/govulncheck-gate.sh` — only the five allowlisted findings
- [x] `gosec -conf .gosec.json ./...` — 0 issues
- [x] `cd frontend && pnpm audit --prod` — no known vulnerabilities
- [x] `git diff --check` — clean

## Baseline verification from the re-review

Before this handoff was written, the branch passed:

- Go unit tests, vet, and race detector;
- 290 frontend tests, frontend typecheck, and production build;
- the `govulncheck` allowlist gate;
- `gosec` with zero findings;
- production dependency audit with zero advisories;
- `git diff --check`.

Those passing gates do not exercise the logical interleavings or payload
visibility problems described above. Live-node integration tests were not run
because they require a configured Zenon endpoint.


## Round 3 addendum (2026-07-13)

The re-review of the remediation found 2 P1 and 3 P2 issues, fixed in
`2ae4d94` (P1s) and `de6803c` (P2s):

- [x] P1 — wallet lifecycle serializes with selection: Lock/Unlock acquire the
      selection mutex around their session swap (KDF stays outside); the
      frontend adds a wallet-session token that discards selection responses
      and queued intents from before an unlock/lock.
- [x] P1 — `liquidity.unlockStakeEntries` removed from the propose catalog and
      fail-closed in the builder: a governance action cannot carry the token
      standard the method selects its target with (ExecuteAction always emits
      a zero-amount ZNN block), so the proposal could never perform the
      displayed intent.
- [x] P2 — SelectAccount persist-or-fail: settings read/persist failures fail
      the call with the signer unchanged; persistence commits before the
      in-memory transition.
- [x] P2 — the raw node connector is unexported (`setNode`) and off the
      binding surface; it runs only inside opMu-protected transitions.
- [x] P2 — proposal metadata decodes from the outer Governance.ProposeAction
      envelope of the exact held template (byte round-trip enforced,
      destination/data cross-checked against the built payload).

All gates re-run and green: go test (+ race), vet, 296 frontend tests,
typecheck, build, govulncheck allowlist gate, gosec 0, `pnpm audit --prod`
clean, `git diff --check` clean.


## Round 4 addendum (2026-07-13)

The five production fixes were confirmed correct; two test/acceptance issues
remained, fixed in `f5d1887`:

- [x] P2 — the integration-tag suite (`internal/spike`) compiles against the
      current bindings: ImportKeystore(name), ID-keyed Unlock, two-value
      SelectAccount, SetNodeURL instead of the (now unexported) raw connector,
      ConfirmPublish(preview.HoldID); prepare-time hash/difficulty assertions
      replaced with an on-chain Difficulty>0 check after confirmation.
      `go test -tags integration ./internal/spike -run '^$'` passes.
- [x] P2 — the persist-failure selection test injects a portable write failure
      (data dir redirected to a child of a regular file inside the persist
      hook) instead of Unix directory permissions, so it holds on
      windows-latest too.

Live-node integration tests remain PENDING a configured testnet endpoint with
the `embedded` RPC namespace.
