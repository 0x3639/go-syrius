# Node-Mode Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the Local node mode, make embedded-node teardown bounded (a wedged `Stop()` can no longer freeze all transitions), emit mode/status at transition start, and surface a `stalled` sync state.

**Architecture:** Backend-first: settings/DTO/service drop `local` and migrate persisted configs; `SetNodeMode` flips mode + emits before teardown; `stopEmbedded` waits ≤10s then abandons the handle and marks the service wedged; the sync poller gains a pure `stallTracker`. The frontend then drops the Local UI, renders `stalled`, and bindings are regenerated. Spec: `docs/superpowers/specs/2026-08-03-node-mode-hardening-design.md` (§2 records the live incident that drove this).

**Tech Stack:** Go (Wails services), Vue 3 + Pinia + Vitest, generated Wails bindings.

## Global Constraints

- Branch: `node-mode-hardening` (spec committed at `7b5e15a`).
- Go commands need `GOWORK=off GOTOOLCHAIN=auto` (parent go.work hazard — CLAUDE.md). Frontend commands run from `frontend/` with pnpm.
- Locked values (spec §4/§6): `embeddedStopTimeout` default `10 * time.Second`; wedged error text `embedded node did not shut down cleanly — restart go-syrius before using Embedded again`; `syncStallAfter = 3 * time.Minute`; stalled UI text `sync stalled — restart go-syrius if this persists`; Remote hint addition `Running your own znnd? Point Remote at ws://127.0.0.1:35998.`
- The crypto-critical path (keystore, derivation, signing, PoW) must be untouched: no diffs outside `app/dto.go`, `app/config_service.go`, `app/node_service.go`, `app/node_sync.go`, their tests, `frontend/src/views/Settings.vue`, `frontend/src/views/Settings.test.ts`, `frontend/src/stores/node.ts`, `frontend/src/stores/node.test.ts`, `frontend/wailsjs/**` (regenerated), `plan.md`, `CLAUDE.md`.
- Wedged-stop timeout is a package-level `var` (not const) so tests can shorten it; restore in test cleanup.
- GPG commit rules: if a commit fails on gpg, STOP and report BLOCKED (never `--no-gpg-sign`). Every commit message ends with the trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.

---

### Task 1: Backend — remove Local mode + settings migration

**Files:**
- Modify: `app/dto.go` (Settings struct :31-57, `ActiveNodeURL` :65-75, `NodeConfig` :77-82, `defaultLocalNodeURL` :137)
- Modify: `app/config_service.go` (`defaultSettings` :60-69, `migrateSettings` :71-93)
- Modify: `app/node_service.go` (`SetNodeMode` accept list :355, `SetNodeURL` :444-483, `GetNodeConfig` :625-633)
- Test: `app/config_service_test.go`, `app/node_service_test.go`

**Interfaces:**
- Consumes: existing `Settings`, `migrateSettings`, `newTestNode(t)` test helper.
- Produces (later tasks rely on): `Settings` without `LocalNodeURL`; `NodeConfig{Mode, RemoteURL}` (no `LocalURL`); `SetNodeMode`/`SetNodeURL` accepting only `"remote"`/`"embedded"`.

- [ ] **Step 1: Write the failing tests**

In `app/config_service_test.go` add (follow the file's existing test style for constructing a ConfigService or call `migrateSettings` directly — it is a pure function on `*Settings`):

```go
func TestMigrateSettingsLocalModeBecomesRemote(t *testing.T) {
	s := Settings{NodeMode: "local", RemoteNodeURL: "wss://example.org:35998"}
	migrateSettings(&s)
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode = %q, want remote", s.NodeMode)
	}
	if s.RemoteNodeURL != "wss://example.org:35998" {
		t.Fatalf("RemoteNodeURL changed: %q", s.RemoteNodeURL)
	}
}

func TestSettingsJSONDropsLegacyLocalKey(t *testing.T) {
	// Old settings.json with a localNodeUrl key must still parse (unknown keys
	// are ignored) and must not resurface on the next marshal.
	raw := []byte(`{"nodeMode":"local","remoteNodeUrl":"wss://r","localNodeUrl":"ws://127.0.0.1:35998"}`)
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal legacy settings: %v", err)
	}
	migrateSettings(&s)
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "localNodeUrl") {
		t.Fatalf("legacy localNodeUrl key persisted: %s", out)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode = %q, want remote", s.NodeMode)
	}
}
```

In `app/node_service_test.go` add (mirroring `TestSetNodeModeRejectsUnknown` at :130):

```go
func TestSetNodeModeRejectsLocal(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeMode("local"); err == nil {
		t.Fatal("SetNodeMode(local) succeeded, want error")
	}
}

func TestSetNodeURLRejectsLocal(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("local", "ws://127.0.0.1:35998"); err == nil {
		t.Fatal("SetNodeURL(local) succeeded, want error")
	}
}
```

Add the imports the config test needs (`encoding/json`, `strings`) if absent.

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auTO go test ./app -run 'TestMigrateSettingsLocal|TestSettingsJSONDropsLegacy|TestSetNodeModeRejectsLocal|TestSetNodeURLRejectsLocal' -v` (note: `GOTOOLCHAIN=auto` all-lowercase — fix the casing when running).
Expected: the two rejection tests FAIL (local currently accepted); migration tests FAIL (`NodeMode` stays `local`, key persists).

- [ ] **Step 3: Implement**

`app/dto.go`:
- Delete the `LocalNodeURL string \`json:"localNodeUrl"\`` field from `Settings`.
- `ActiveNodeURL`: delete the `case "local"` branch.
- `NodeConfig`: delete the `LocalURL` field.
- Delete the `defaultLocalNodeURL` const (:137).

`app/config_service.go`:
- `defaultSettings`: delete the `LocalNodeURL: defaultLocalNodeURL,` line.
- `migrateSettings`: delete the `if s.LocalNodeURL == "" {…}` block; change the mode-normalize line to:

```go
	if s.NodeMode != "remote" && s.NodeMode != "embedded" {
		s.NodeMode = "remote" // includes migrating the removed "local" mode
	}
```

`app/node_service.go`:
- `SetNodeMode` guard becomes `if mode != "remote" && mode != "embedded" {`.
- `SetNodeURL`: delete the `if mode != "remote" && mode != "local"` branch's local acceptance — guard becomes `if mode != "remote" {` (embedded already rejected above it); delete the `if mode == "local" { s.LocalNodeURL = url } else { … }` branch — always `s.RemoteNodeURL = url`.
- `GetNodeConfig`: drop the `LocalURL` assignment.
- Grep `rg -n '"local"|LocalNodeURL|LocalURL|defaultLocalNodeURL' app/` → no hits outside tests.

- [ ] **Step 4: Run the focused tests, then the full Go suite**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app -run 'TestMigrateSettingsLocal|TestSettingsJSONDropsLegacy|TestSetNodeModeRejectsLocal|TestSetNodeURLRejectsLocal' -v` → PASS.
Then `GOWORK=off GOTOOLCHAIN=auto go test ./... && GOWORK=off GOTOOLCHAIN=auto go vet ./...` — fix any existing test that referenced local mode (e.g. persistence tests asserting `LocalNodeURL`) by updating it to the two-mode world, keeping its original intent.
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add app/
git commit -m "feat(node): remove Local node mode; migrate persisted local configs to remote

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: Backend — honest transition status + bounded teardown + wedged guard

**Files:**
- Modify: `app/node_service.go` (`SetNodeMode` :354-387, `stopEmbedded` :486-499, `NodeService` struct :40-73)
- Test: `app/node_service_test.go`

**Interfaces:**
- Consumes: Task 1's two-mode `SetNodeMode`; existing `embeddedHandle` interface (`WSURL() string; DataDir() string; Stop() error`, node_service.go:25-30); `newTestNode(t)`.
- Produces: `var embeddedStopTimeout = 10 * time.Second`; `n.embeddedWedged bool` (guarded by `n.mu`); wedged error text (Global Constraints); transition emits mode change before teardown.
- **Reach note:** `App.OnShutdown` (app/app.go:42) calls this same `stopEmbedded()` — bounding it also fixes the reported quit-hang (force-quit) with a wedged node. No app.go change needed; reviewers should verify the call site benefits.

- [ ] **Step 1: Write the failing test**

Add to `app/node_service_test.go`:

```go
// wedgedHandle simulates the 2026-08-03 incident: an embedded node whose
// Stop() never returns (spec §2/§4).
type wedgedHandle struct {
	stopCalled chan struct{}
	release    chan struct{}
}

func (w *wedgedHandle) WSURL() string   { return "ws://127.0.0.1:35998" }
func (w *wedgedHandle) DataDir() string { return "" }
func (w *wedgedHandle) Stop() error {
	close(w.stopCalled)
	<-w.release
	return nil
}

func TestSetNodeModeBoundedTeardownOnWedgedStop(t *testing.T) {
	old := embeddedStopTimeout
	embeddedStopTimeout = 100 * time.Millisecond
	t.Cleanup(func() { embeddedStopTimeout = old })

	n := newTestNode(t)
	w := &wedgedHandle{stopCalled: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(w.release) }) // let the background waiter exit
	n.mu.Lock()
	n.embedded = w
	n.mode = "embedded"
	n.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- n.SetNodeMode("remote") }()

	// Stop must have been attempted…
	select {
	case <-w.stopCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop was never called")
	}
	// …and the transition must complete despite Stop never returning.
	// (The remote dial fails fast against the unreachable test URL — an error
	// return is fine; a hang is the bug.)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SetNodeMode hung on wedged embedded Stop")
	}

	// Honest status: the mode flipped even though Stop is still blocked.
	if got := n.NodeStatus().Mode; got != "remote" {
		t.Fatalf("mode = %q, want remote", got)
	}
	// The wedged service refuses to restart embedded in-process.
	if err := n.SetNodeMode("embedded"); err == nil ||
		!strings.Contains(err.Error(), "restart go-syrius") {
		t.Fatalf("SetNodeMode(embedded) after wedge: err = %v, want restart-required error", err)
	}
}
```

(Add `"strings"`/`"time"` imports if absent; `newTestNode`'s config must point remote at an unreachable URL — check the helper at :125 and reuse whatever it already does for `TestSetNodeModePersistsEvenIfUnreachable`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app -run TestSetNodeModeBoundedTeardownOnWedgedStop -v`
Expected: FAIL — `embeddedStopTimeout` undefined; after adding only the var it fails with "SetNodeMode hung on wedged embedded Stop".

- [ ] **Step 3: Implement**

`app/node_service.go` — struct additions (next to `embedded embeddedHandle`):

```go
	// embeddedWedged latches when an embedded stop timed out: the abandoned
	// node still owns the WS port, so starting another in-process is doomed —
	// require an app restart instead (spec §4).
	embeddedWedged bool
```

Package-level, near the other timeouts/consts:

```go
// embeddedStopTimeout bounds how long a mode transition waits for the embedded
// node to stop. A wedged Stop() (2026-08-03 incident) otherwise blocks opMu —
// and with it every future transition — forever. Var, not const: tests shorten it.
var embeddedStopTimeout = 10 * time.Second
```

`stopEmbedded` becomes bounded (same call sites, same signature):

```go
// stopEmbedded halts the embedded node + sync poller if running, waiting at
// most embeddedStopTimeout for the node to stop. On timeout the handle is
// abandoned (the background goroutine keeps waiting; if the stop ever finishes
// the embeddednode package's single-instance guard clears itself) and the
// service latches embeddedWedged — see SetNodeMode's embedded guard.
func (n *NodeService) stopEmbedded() {
	n.mu.Lock()
	if n.syncStop != nil {
		close(n.syncStop)
		n.syncStop = nil
	}
	h := n.embedded
	n.embedded = nil
	n.mu.Unlock()
	if h == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = h.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(embeddedStopTimeout):
		log.Printf("WARN: embedded node did not stop within %s; abandoning handle (restart recommended before using Embedded again)", embeddedStopTimeout)
		n.mu.Lock()
		n.embeddedWedged = true
		n.mu.Unlock()
	}
	n.appNapEnd()
}
```

(Use the file's existing logging approach — if it uses nothing today, `log.Printf` with the `log` import is acceptable; no sensitive data is involved.)

`SetNodeMode` — reorder and guard (replacing :375-386):

```go
	// Refuse embedded while a previous embedded node is wedged: the abandoned
	// node still owns the WS port, so an in-process restart cannot succeed.
	if mode == "embedded" {
		n.mu.Lock()
		wedged := n.embeddedWedged
		n.mu.Unlock()
		if wedged {
			return errors.New("embedded node did not shut down cleanly — restart go-syrius before using Embedded again")
		}
	}

	// Honest transition status (spec §5): flip the mode and tell the UI the
	// switch has STARTED before any teardown or dialing. The frontend clears
	// stale embedded sync state on this mode change; the footer shows
	// "Disconnected (<new mode>)" until the connect lands.
	n.mu.Lock()
	n.mode = mode
	n.emitStatusLocked(false)
	n.mu.Unlock()

	// Tear down any running embedded node when leaving embedded mode (bounded).
	if mode != "embedded" {
		n.stopEmbedded()
	}

	if mode == "embedded" {
		return n.startEmbedded()
	}
	return n.setNode(target)
```

Note: `emitStatusLocked(false)` reports `connected && n.client != nil` — the old client is still installed at that instant, but `connected=false` is passed explicitly, so the emit is honest. The old client is torn down moments later by `stopEmbedded`/`setNode`'s `disconnectLocked` exactly as today.

- [ ] **Step 4: Run the test, then the full suite**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app -run TestSetNodeModeBoundedTeardownOnWedgedStop -v` → PASS (in ~1s, not 10 — the shortened var proves the bound is honored).
Then `GOWORK=off GOTOOLCHAIN=auto go test ./... && GOWORK=off GOTOOLCHAIN=auto go vet ./...` → green (existing transition tests must still pass; the earlier mode flip is compatible with `TestSetNodeModePersistsEvenIfUnreachable`).

- [ ] **Step 5: Commit**

```bash
git add app/node_service.go app/node_service_test.go
git commit -m "fix(node): bounded embedded teardown + wedged guard; emit mode at transition start

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: Backend — sync stall detection

**Files:**
- Modify: `app/node_sync.go` (add `stallTracker`), `app/node_service.go` (`startSyncPoller` :521-562)
- Test: `app/node_sync_test.go`

**Interfaces:**
- Consumes: existing `computeSync(samples, current, target, peers, state)`, poller loop shape.
- Produces: `const syncStallAfter = 3 * time.Minute`; `stallTracker` with `observe(now time.Time, height uint64, state string) bool` and `observeError(now time.Time) bool`; emitted `SyncStatus.State == "stalled"`.

- [ ] **Step 1: Write the failing tests**

Add to `app/node_sync_test.go`:

```go
func TestStallTrackerFlagsFrozenHeight(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	if st.observe(t0, 100, "syncing") {
		t.Fatal("first sample must establish baseline, not stall")
	}
	if st.observe(t0.Add(time.Minute), 100, "syncing") {
		t.Fatal("stalled before syncStallAfter elapsed")
	}
	if !st.observe(t0.Add(syncStallAfter), 100, "syncing") {
		t.Fatal("frozen height past syncStallAfter must report stalled")
	}
	// Any advance resets the clock.
	if st.observe(t0.Add(syncStallAfter+time.Second), 101, "syncing") {
		t.Fatal("advancing height must clear the stall")
	}
	if st.observe(t0.Add(syncStallAfter+2*time.Second), 101, "syncing") {
		t.Fatal("stall must not re-trigger immediately after an advance")
	}
}

func TestStallTrackerNeverFlagsSynced(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	st.observe(t0, 100, "synced")
	if st.observe(t0.Add(2*syncStallAfter), 100, "synced") {
		t.Fatal("a synced node with a quiet chain is not stalled")
	}
}

func TestStallTrackerErrorsConvergeToStalled(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	if st.observeError(t0) {
		t.Fatal("errors before any baseline must not report stalled")
	}
	st.observe(t0, 100, "syncing")
	if st.observeError(t0.Add(time.Minute)) {
		t.Fatal("stalled too early on errors")
	}
	if !st.observeError(t0.Add(syncStallAfter)) {
		t.Fatal("persistent errors past syncStallAfter must report stalled")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app -run TestStallTracker -v`
Expected: FAIL — `stallTracker`/`syncStallAfter` undefined.

- [ ] **Step 3: Implement**

`app/node_sync.go`:

```go
// syncStallAfter is how long an unsynced node's height may stand still before
// the poller reports "stalled" (spec §6 — the 2026-08-03 wedged-p2p incident
// showed "starting · 100.0%" forever while nothing moved).
const syncStallAfter = 3 * time.Minute

// stallTracker decides when sync progress should be reported as stalled.
// Pure state machine; the poller feeds it wall-clock samples.
type stallTracker struct {
	baseline    bool
	lastHeight  uint64
	lastAdvance time.Time
}

// observe folds a successful sample. Returns true when the node is unsynced
// and its height has not advanced for syncStallAfter.
func (st *stallTracker) observe(now time.Time, height uint64, state string) bool {
	if !st.baseline || height > st.lastHeight {
		st.baseline = true
		st.lastHeight = height
		st.lastAdvance = now
		return false
	}
	return state != "synced" && now.Sub(st.lastAdvance) >= syncStallAfter
}

// observeError reports whether persistent RPC errors should surface as
// stalled: only once a baseline exists (before that, the connection path —
// not the poller — owns the "not working" story) and the window has elapsed.
func (st *stallTracker) observeError(now time.Time) bool {
	return st.baseline && now.Sub(st.lastAdvance) >= syncStallAfter
}
```

`app/node_service.go` `startSyncPoller` goroutine — integrate (replacing the loop body's sample handling):

```go
	go func() {
		var samples []heightSample
		var tracker stallTracker
		var lastTarget uint64
		var lastPeers int
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				info, err := client.StatsApi.SyncInfo()
				if err != nil {
					// A dead node stops answering; after the stall window,
					// say so instead of freezing the last good frame.
					if tracker.observeError(now) {
						runtime.EventsEmit(ctx, EventNodeSync, SyncStatus{
							State:         "stalled",
							CurrentHeight: tracker.lastHeight,
							TargetHeight:  lastTarget,
							Peers:         lastPeers,
						})
					}
					continue
				}
				peers := 0
				if ni, nerr := client.StatsApi.NetworkInfo(); nerr == nil {
					peers = ni.NumPeers
				}
				lastTarget = info.TargetHeight
				lastPeers = peers
				samples = append(samples, heightSample{T: now, Height: info.CurrentHeight})
				if len(samples) > 10 {
					samples = samples[len(samples)-10:]
				}
				state := mapSyncState(info.State)
				if tracker.observe(now, info.CurrentHeight, state) {
					state = "stalled"
				}
				st := computeSync(samples, info.CurrentHeight, info.TargetHeight, peers, state)
				runtime.EventsEmit(ctx, EventNodeSync, st)
				n.noteSyncHeight(info.CurrentHeight)
			}
		}
	}()
```

- [ ] **Step 4: Run tests, then full suite**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app -run TestStallTracker -v` → PASS.
Then `GOWORK=off GOTOOLCHAIN=auto go test ./... && GOWORK=off GOTOOLCHAIN=auto go vet ./... && GOWORK=off GOTOOLCHAIN=auto go build ./...` → green.

- [ ] **Step 5: Commit**

```bash
git add app/node_sync.go app/node_sync_test.go app/node_service.go
git commit -m "feat(node): report stalled sync when height freezes or the node stops answering

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: Frontend — two-mode Settings + stalled rendering

**Files:**
- Modify: `frontend/src/views/Settings.vue` (script: localUrl/localDirty/loadedLocal state and applyNode branches; template: Node section :261-298)
- Modify: `frontend/src/stores/node.ts` (`NodeConfig` type :7)
- Test: `frontend/src/views/Settings.test.ts`, `frontend/src/stores/node.test.ts`

**Interfaces:**
- Consumes: Task 1's backend `NodeConfig{mode, remoteUrl}` (bindings regenerated in Task 5 — until then the extra TS field is merely unused, nothing breaks).
- Produces: Settings UI with exactly two radios; stalled alert rendering.

- [ ] **Step 1: Update/add the failing tests**

`frontend/src/views/Settings.test.ts`:
- Update the hoisted `GetNodeConfig` mock to `{ mode: 'remote', remoteUrl: 'wss://old' }` (drop `localUrl`).
- Rewrite the existing local-mode Apply test (the one selecting `input[type="radio"][value="local"]` around :80) to target embedded instead — it should now click `input[type="radio"][value="embedded"]`, confirm the embedded warning appears, click `Start embedded`, and assert `SetNodeMode` was called with `'embedded'` (mirror the existing embedded-confirm test if one exists; if one already covers this, repurpose the local test to the two-radio assertion below and delete the duplicate).
- Add:

```ts
it('offers exactly two node modes: remote and embedded', async () => {
  const w = mount(Settings)
  await flush()
  const radios = w.findAll('input[type="radio"]')
  expect(radios.map((r) => (r.element as HTMLInputElement).value).sort()).toEqual(['embedded', 'remote'])
  expect(w.find('input[aria-label="ws endpoint url"]').exists()).toBe(false) // local URL field gone
  expect(w.text()).toContain('Running your own znnd? Point Remote at ws://127.0.0.1:35998.')
})

it('renders a stalled sync state as an alert', async () => {
  const node = useNodeStore()
  node.mode = 'embedded'
  node.sync = { state: 'stalled', currentHeight: 100, targetHeight: 200, percent: 50, etaSeconds: 0, peers: 2 }
  const w = mount(Settings)
  await flush()
  const alert = w.find('[role="alert"].text-destructive')
  expect(alert.exists()).toBe(true)
  expect(alert.text()).toContain('sync stalled — restart go-syrius if this persists')
})
```

(Adapt store access/mount helpers to the file's existing patterns — it already mounts Settings with pinia + mocked bindings; `flush` mirrors its existing async helper.)

`frontend/src/stores/node.test.ts`: grep for `localUrl` and update any fixture to the two-field `NodeConfig`.

- [ ] **Step 2: Run to verify the new tests fail**

Run (in `frontend/`): `pnpm test`
Expected: the two new tests FAIL (three radios today; no stalled alert); the rewritten embedded test fails if the flow assertion doesn't match yet.

- [ ] **Step 3: Implement**

`frontend/src/stores/node.ts`: `export type NodeConfig = { mode: string; remoteUrl: string }`.

`frontend/src/views/Settings.vue` script:
- Delete `localUrl`, `loadedLocal`, `localDirty`.
- `onMounted`: delete the `loadedLocal`/`localUrl` assignments.
- `applyNode`: delete `localEdited`; the reconnect condition simplifies to:

```ts
    const remoteEdited = remoteDirty.value && remoteUrl.value !== loadedRemote
    if (remoteEdited) { await node.setUrl('remote', remoteUrl.value); loadedRemote = remoteUrl.value }
    if (nodeMode.value !== loadedMode) { await node.setMode(nodeMode.value); loadedMode = nodeMode.value }
    else if (nodeMode.value === 'remote' && remoteEdited) { await node.setMode(nodeMode.value) }
```

Template — Node section:
- Delete the Local radio label and its `<input … v-model="localUrl" …aria-label="ws endpoint url" />` line.
- After the Remote URL input add: `<p class="text-xs text-muted-foreground">Running your own znnd? Point Remote at ws://127.0.0.1:35998.</p>`
- In the sync panel, replace the state line (`<p>{{ node.sync.state }} · …</p>`) with:

```html
          <p v-if="node.sync.state === 'stalled'" class="text-destructive" role="alert">
            sync stalled — restart go-syrius if this persists · {{ node.sync.currentHeight }} / {{ node.sync.targetHeight }}
          </p>
          <p v-else>{{ node.sync.state }} · {{ node.sync.currentHeight }} / {{ node.sync.targetHeight }} ({{ node.sync.percent.toFixed(1) }}%)<template v-if="node.sync.etaSeconds > 0"> · ETA {{ fmtEta(node.sync.etaSeconds) }}</template></p>
```

(The progress bar and peers/disk line stay exactly as they are — spec §6 keeps them visible.)

- [ ] **Step 4: Run frontend suite + typecheck**

Run (in `frontend/`): `pnpm test && pnpm run typecheck`
Expected: all green (fix any other test referencing the local radio/field).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Settings.vue frontend/src/views/Settings.test.ts frontend/src/stores/node.ts frontend/src/stores/node.test.ts
git commit -m "feat(settings): two node modes (remote/embedded) + stalled sync alert

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: Bindings regen, docs, full gates

**Files:**
- Modify: `frontend/wailsjs/go/models.ts` (+ any `frontend/wailsjs/go/app/*` the generator touches)
- Modify: `plan.md`, `CLAUDE.md`

**Interfaces:**
- Consumes: Tasks 1–4 complete.
- Produces: bindings matching the two-mode `NodeConfig`/`Settings`; docs describing two node modes.

- [ ] **Step 1: Regenerate bindings**

Run: `GOWORK=off GOTOOLCHAIN=auto wails generate module` (use the repository-standard full wails binary path if PATH needs it — see CLAUDE.md).
Inspect `git diff frontend/wailsjs/`: keep the `localNodeUrl`/`localUrl` removals in `models.ts` and any legitimately regenerated method files; revert unrelated churn (`git checkout -- <file>`) exactly as prior phases did (CLI-version skew produces reordering noise — discard it).

- [ ] **Step 2: Update docs**

- `plan.md`: `rg -n "local" plan.md` — update the node-modes description (§4 "Three node modes" and any enumerations): three modes → two (Remote, Embedded), with one added line: "Local mode removed 2026-08-03 (`docs/superpowers/specs/2026-08-03-node-mode-hardening-design.md`) — self-hosted `znnd` users point Remote at `ws://127.0.0.1:35998`."
- `CLAUDE.md`: update the two mentions — "all three node modes (remote/local/embedded)" → "both node modes (remote/embedded)", and the "Three node modes" section: retitle "Two node modes", delete the Local list item, and append the same one-line removal note.

- [ ] **Step 3: Full gates**

```bash
GOWORK=off GOTOOLCHAIN=auto go test ./...
GOWORK=off GOTOOLCHAIN=auto go vet ./...
GOWORK=off GOTOOLCHAIN=auto go build ./...
cd frontend && pnpm test && pnpm run typecheck && pnpm run build
```

Expected: all green.

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs plan.md CLAUDE.md
git commit -m "chore(node): regenerate bindings for two-mode NodeConfig; update docs

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Self-review notes

- Spec coverage: §3 → Tasks 1/4/5; §4 → Task 2; §5 → Task 2 (emit before teardown; frontend needs no change — verified against `applyStatus`); §6 → Tasks 3/4; §7 test list mapped 1:1; §8 follow-ups intentionally have no tasks; §9 gates → Task 5 Step 3.
- Manual acceptance (not automatable): after merge, a real Embedded→Remote switch on the user's machine shows Disconnected(remote)→Connected(remote), and a wedged node (if it recurs) yields the restart error — noted for the user's next live session; the wedged-handle unit test simulates the incident deterministically.
- Type consistency: `embeddedStopTimeout` (var), `stallTracker.observe/observeError`, `syncStallAfter`, two-field `NodeConfig` used consistently across tasks.
- Task 2 Step 2's Run line contains a deliberate note fixing its own casing typo; the command to actually run is with `GOTOOLCHAIN=auto`.
