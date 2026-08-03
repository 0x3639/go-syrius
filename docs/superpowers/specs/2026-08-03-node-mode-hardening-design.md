# Node-Mode Hardening: Remove Local, Bounded Teardown, Honest Status, Stall Detection

**Date:** 2026-08-03
**Status:** Proposed
**Scope:** `app/node_service.go`, `app/config*.go` (settings migration), `internal/embeddednode`, `frontend/src/views/Settings.vue`, `frontend/src/stores/node.ts` (+tests, regenerated bindings), `plan.md` + CLAUDE.md one-line updates (three node modes → two)
**Audience:** Implementer and reviewer
**Related:** plan.md §4 (three node modes — this spec supersedes it to two), `docs/superpowers/specs/…phase-4…` if present

## 1. Summary

Four changes to node-mode handling, driven by a live incident diagnosed on
2026-08-03:

1. **Remove Local mode.** The wallet offers Remote and Embedded only.
2. **Bounded embedded teardown.** A wedged `node.Stop()` can no longer hang a
   mode switch (and with it every later transition) forever.
3. **Honest transition status.** The UI reflects a mode switch the moment it
   starts, not only after it completes.
4. **Sync stall detection.** A syncing embedded node whose height stops
   advancing is reported as `stalled`, not left frozen at "starting · 100.0%".

## 2. Incident record (evidence for the design)

Observed live (2026-08-03, app PID 24157):

- The embedded node's p2p layer died silently: the process held **zero**
  network sockets besides the loopback WS RPC — no listener on 35995, no UDP
  discovery socket, no peer TCP connections — while `stats.networkInfo` kept
  reporting a stale `numPeers: 2` and sync height stopped advancing.
- `SetNodeMode("remote")` then blocked forever inside
  `stopEmbedded → node.Stop()` (wedged p2p/sync goroutines). Evidence: the
  embedded RPC port stayed in LISTEN, the app never dialed the remote URL, and
  the status line kept saying "Connected (embedded)" because `n.mode` is only
  assigned *after* `stopEmbedded` returns.
- Because `SetNodeMode` holds `opMu` for the whole transition, every later
  Apply/Retry queued behind the hung one. Only an app restart recovers.
- The frontend's existing stale-sync defenses (clear-on-mode-change, straggler
  drop) were **not** at fault; the store faithfully displayed a backend that
  never transitioned.

Root cause of the p2p death itself is in the go-zenon fork (pinned commit
`81c2474`, still devp2p) — out of scope here (§8).

## 3. Remove Local mode

- **Settings UI** (`frontend/src/views/Settings.vue`): the Node section offers
  two radios — Remote (URL field) and Embedded. The Local radio, its URL
  field, and the `localUrl`/`localDirty` state are removed. The Embedded hint
  keeps its current text; the Remote hint gains: "Running your own znnd? Point
  Remote at ws://127.0.0.1:35998."
- **Backend** (`app/node_service.go`): `SetNodeMode` and `SetNodeURL` reject
  `"local"` with the existing `unknown node mode %q` error (they simply drop
  it from their accept lists). The `NodeConfig` DTO loses `localUrl`.
- **Settings migration** (where `Settings` is loaded): a persisted
  `NodeMode: "local"` loads as `"remote"`; the `LocalNodeURL` field is removed
  from the struct (unknown JSON keys are ignored on read, and the next write
  drops it). The migration happens on load so a user upgrading mid-`local`
  simply comes up in Remote mode pointing at their configured remote URL.
- **No capability loss:** local `znnd` users use Remote mode with a
  `ws://127.0.0.1:…` URL. `SetNodeURL("remote", …)` already accepts `ws://`.
- **Bindings** regenerated (`wails generate module`); only relevant
  `frontend/wailsjs` diffs are kept.

## 4. Bounded embedded teardown

`stopEmbedded` becomes bounded:

- It runs `h.Stop()` in a goroutine and waits up to
  `embeddedStopTimeout = 10 * time.Second`.
- **On completion in time:** behavior identical to today.
- **On timeout:** log a warning (no sensitive data — there is none here),
  abandon the handle (drop the service's reference; the background goroutine
  keeps waiting and, if the stop ever finishes, the embeddednode package's
  single-instance guard clears itself), end the App Nap assertion, and
  continue the transition. The service records `embeddedWedged = true`.
- A later `SetNodeMode("embedded")` while `embeddedWedged` is set returns the
  error: `embedded node did not shut down cleanly — restart go-syrius before
  using Embedded again` (the wedged process still owns the WS port, so an
  in-process restart cannot work; honesty beats a doomed retry).
- The sync poller is stopped (channel close) before the bounded wait, exactly
  as today.
- `opMu` is therefore held at most ~10s by a wedged teardown; the transition
  proceeds to connect the requested mode either way.

## 5. Honest transition status

Reorder `SetNodeMode` so the UI learns about the switch immediately:

1. Validate + persist mode (unchanged).
2. Set `n.mode = mode` and emit `node:status` with `connected: false` **before**
   any teardown or dialing.
3. Tear down (bounded) / start embedded / dial, as today; success and failure
   emits are unchanged.

Frontend effect (no store change needed): `applyStatus` sees the
embedded→remote mode flip at step 2 and clears `sync`/`syncing`; the Settings
sync panel (gated on `mode === 'embedded'`) disappears; the footer shows
"Disconnected (remote)" during the dial, then "Connected (remote)".
Switching *to* embedded likewise shows "Disconnected (embedded)" during
startup, which is accurate.

## 6. Sync stall detection

In `startSyncPoller`'s loop:

- Track `lastAdvance time.Time` (initialized at poller start) and the last
  seen height; any height increase resets `lastAdvance`.
- `syncStallAfter = 3 * time.Minute`. When `state != "synced"` and
  `time.Since(lastAdvance) >= syncStallAfter`, the emitted `SyncStatus.State`
  becomes `"stalled"` (overriding the mapped state). Consecutive `SyncInfo`
  RPC errors do not reset the timer, so a dead node also converges to
  `stalled` (once at least one successful sample established a baseline; when
  even the first samples error, the poller keeps emitting nothing, and the
  transition/status path — not the poller — reports disconnection).
- **UI** (`Settings.vue`): when `node.sync.state === 'stalled'`, the panel
  shows `sync stalled — restart go-syrius if this persists` in destructive
  styling in place of the normal state line; the progress bar and counts stay
  visible. The peers line renders normally (a stalled node's peer count is
  exactly the suspect data the user should see).
- `stalled` is a UI state only; it does not trigger automatic remediation
  (YAGNI — an in-process embedded restart is impossible while wedged, §4).

## 7. Testing requirements

**Go (`app/…_test.go`):**
- `SetNodeMode("local")` and `SetNodeURL("local", …)` return the unknown-mode
  error.
- Settings migration: a settings JSON with `"nodeMode": "local"` (and a
  `localNodeUrl` key) loads as mode `remote`; saving drops the legacy key.
- Bounded stop: with a stop hook that blocks forever (inject via the existing
  `embeddedStart`-style seam or a small `stopFn` seam on the handle),
  `SetNodeMode("remote")` returns within the timeout envelope, and a
  subsequent `SetNodeMode("embedded")` returns the wedged-restart error.
- Status ordering: transition emits a `connected:false` status with the NEW
  mode before the connect attempt (capture via the existing event-recording
  test seam if present, else assert on state).
- Stall computation: unit-test the stall decision (last-advance tracking → 
  `stalled` override) as a pure helper.

**Frontend:**
- `Settings.vue` renders exactly two node radios (remote, embedded); no
  `localUrl` field; the znnd hint is present.
- Stalled rendering: `node.sync.state === 'stalled'` shows the stalled message
  with `role="alert"`/destructive class.
- Node store: existing tests still pass (mode-flip clearing already covered).

## 8. Out of scope / follow-ups (recorded, not implemented)

- **go-zenon fork p2p death** (the actual root cause of the wedge): reproduce
  under `wails dev` with terminal-attached stderr and send SIGQUIT to capture
  the goroutine dump identifying the deadlock; fix lands in the fork and gets
  re-pinned. Owner: user (fork maintainer).
- **Leaked closed FDs:** the wedged process held ~47 CLOSED TCP FDs on the
  35998 listener — likely reconnect churn never releasing file descriptors;
  investigate separately.
- Auto-restart / self-healing of a wedged embedded node (impossible
  in-process while the old node owns the port).

## 9. Acceptance criteria

- Local mode is gone from UI, DTO, and backend accept lists; a `local`
  settings file migrates to remote cleanly.
- With a hung embedded stop (simulated), Apply→Remote completes: UI shows
  Disconnected(remote) → Connected(remote) within seconds; a follow-up switch
  to Embedded yields the restart-required error; nothing deadlocks.
- During any mode switch the footer/panel never claim the previous mode's
  connected state.
- A height-frozen unsynced embedded node shows `stalled` within ~3 minutes.
- `GOWORK=off GOTOOLCHAIN=auto go test ./...`, `go vet`, frontend
  `pnpm test`/`typecheck`/`build` all green; keystore/derivation untouched.
