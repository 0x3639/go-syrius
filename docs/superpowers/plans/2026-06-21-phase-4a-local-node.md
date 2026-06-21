# Phase 4a — Local Node Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Local node mode (connect to a user-run `znnd`) alongside Remote, with mode + per-mode URLs persisted in settings and a Node section in Settings.

**Architecture:** Local and remote share the existing `SetNode(url)` connect/read/status path; a "mode" only selects which persisted URL to connect to and labels the status. Settings migrate from the legacy single `nodeUrl` to `NodeMode` + `RemoteNodeURL` + `LocalNodeURL`.

**Tech Stack:** Go 1.24+, `znn-sdk-go/rpc_client`, Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- Settings model: `NodeMode` (`"remote"`|`"local"`, default `"remote"`), `RemoteNodeURL` (default `wss://my.hc1node.com:35998`), `LocalNodeURL` (default `ws://127.0.0.1:35998`). Active URL chosen by mode.
- Migrate existing `settings.json`: legacy `nodeUrl` → `RemoteNodeURL`; fill defaults; stop writing `nodeUrl`.
- Mode = URL selector; reuse `SetNode` (no new RPC). chainId from `SetNode` already shows mainnet/testnet.
- All mode/URL inputs validated in Go (untrusted WebView); reject unknown modes and non-`ws(s)://` URLs. No secrets involved/logged.
- Local node unreachable → existing disconnected `node:status` + Retry; never crash. Mode persists even if the connect attempt fails.
- `go test ./...` offline; frontend `pnpm test` + `pnpm run build` pass.

## File structure

```
app/dto.go             # MOD: Settings (NodeMode/RemoteNodeURL/LocalNodeURL, deprecate NodeURL) + ActiveNodeURL(); new NodeConfig
app/config_service.go  # MOD: defaultLocalNodeURL const, defaultSettings, migrateSettings in GetSettings
app/node_service.go    # MOD: mode field; SetNodeMode/SetNodeURL/Connect/GetNodeConfig; NodeStatus.Mode reads mode; SetNode stops persisting nodeUrl
app/config_service_test.go  # MOD/NEW: migration + ActiveNodeURL tests
app/node_service_test.go    # NEW: SetNodeMode/SetNodeURL/GetNodeConfig tests (offline)
frontend/wailsjs/...   # regenerated bindings
frontend/src/lib/stores/node.ts   # MOD: setMode/setUrl/getConfig actions
frontend/src/App.svelte           # MOD: onMount connect via Connect() not SetNode(nodeUrl)
frontend/src/routes/Settings.svelte       # MOD: add Node section
frontend/src/routes/Settings.test.ts      # MOD: Node section tests
```

---

## Task 1: Settings model + migration

**Files:** Modify `app/dto.go`, `app/config_service.go`; Test `app/config_service_test.go`.

**Interfaces:**
- Consumes: existing `defaultNodeURL = "wss://my.hc1node.com:35998"`.
- Produces: `Settings` fields `NodeMode`/`RemoteNodeURL`/`LocalNodeURL` (+ deprecated `NodeURL`); `const defaultLocalNodeURL = "ws://127.0.0.1:35998"`; `(Settings) ActiveNodeURL() string`; migration applied in `GetSettings`.

- [ ] **Step 1: Write the failing tests**

Create/extend `app/config_service_test.go`:
```go
package app

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestConfig(t *testing.T) *ConfigService {
	t.Helper()
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	return newConfigService()
}

func TestSettingsMigrationFromLegacyNodeURL(t *testing.T) {
	c := newTestConfig(t)
	d, _ := c.dataDir()
	if err := os.WriteFile(filepath.Join(d, "settings.json"), []byte(`{"nodeUrl":"wss://custom:35998","theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.RemoteNodeURL != "wss://custom:35998" {
		t.Fatalf("legacy nodeUrl should migrate to RemoteNodeURL, got %q", s.RemoteNodeURL)
	}
	if s.LocalNodeURL != defaultLocalNodeURL {
		t.Fatalf("LocalNodeURL default, got %q", s.LocalNodeURL)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode default remote, got %q", s.NodeMode)
	}
	if s.NodeURL != "" {
		t.Fatalf("legacy NodeURL should be cleared, got %q", s.NodeURL)
	}
}

func TestSettingsDefaultsWhenNoFile(t *testing.T) {
	c := newTestConfig(t)
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.NodeMode != "remote" || s.RemoteNodeURL != defaultNodeURL || s.LocalNodeURL != defaultLocalNodeURL {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestActiveNodeURL(t *testing.T) {
	s := Settings{NodeMode: "remote", RemoteNodeURL: "wss://r", LocalNodeURL: "ws://l"}
	if s.ActiveNodeURL() != "wss://r" {
		t.Fatalf("remote active: %q", s.ActiveNodeURL())
	}
	s.NodeMode = "local"
	if s.ActiveNodeURL() != "ws://l" {
		t.Fatalf("local active: %q", s.ActiveNodeURL())
	}
}

func TestMigrationIdempotent(t *testing.T) {
	c := newTestConfig(t)
	s1, _ := c.GetSettings()
	if err := c.SetSettings(s1); err != nil {
		t.Fatal(err)
	}
	s2, _ := c.GetSettings()
	if s2.RemoteNodeURL != s1.RemoteNodeURL || s2.LocalNodeURL != s1.LocalNodeURL || s2.NodeMode != s1.NodeMode {
		t.Fatalf("not idempotent: %+v vs %+v", s1, s2)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestSettings|TestActiveNodeURL|TestMigration' -v`
Expected: FAIL — `defaultLocalNodeURL` / fields undefined.

- [ ] **Step 3: Implement**

In `app/dto.go`, replace the `Settings.NodeURL` line and add fields + helper + DTO. The `Settings` struct becomes:
```go
type Settings struct {
	// Deprecated: read-only for migration from the pre-4a single-URL format.
	NodeURL          string `json:"nodeUrl,omitempty"`
	NodeMode         string `json:"nodeMode"`
	RemoteNodeURL    string `json:"remoteNodeUrl"`
	LocalNodeURL     string `json:"localNodeUrl"`
	Theme            string `json:"theme"`
	LastWallet       string `json:"lastWallet"`
	ActiveAccount    int    `json:"activeAccount"`
	AllowMainnetSend bool   `json:"allowMainnetSend"`
	AutoReceive      bool   `json:"autoReceive"`
	// AccountLabels maps "<wallet>:<index>" to a human label for an account.
	AccountLabels map[string]string `json:"accountLabels"`
}

// ActiveNodeURL returns the URL for the current NodeMode.
func (s Settings) ActiveNodeURL() string {
	if s.NodeMode == "local" {
		return s.LocalNodeURL
	}
	return s.RemoteNodeURL
}

// NodeConfig is the node mode + per-mode URLs for the settings UI.
type NodeConfig struct {
	Mode      string `json:"mode"`
	RemoteURL string `json:"remoteUrl"`
	LocalURL  string `json:"localUrl"`
}
```
Keep `const defaultNodeURL = "wss://my.hc1node.com:35998"` and add next to it:
```go
const defaultLocalNodeURL = "ws://127.0.0.1:35998"
```
In `app/config_service.go`, replace `defaultSettings` and add migration:
```go
func defaultSettings() Settings {
	return Settings{
		NodeMode:      "remote",
		RemoteNodeURL: defaultNodeURL,
		LocalNodeURL:  defaultLocalNodeURL,
		Theme:         "dark",
		ActiveAccount: 0,
	}
}

// migrateSettings fills new node fields and migrates the deprecated single
// nodeUrl. Idempotent and safe on default settings.
func migrateSettings(s *Settings) {
	if s.RemoteNodeURL == "" {
		if s.NodeURL != "" {
			s.RemoteNodeURL = s.NodeURL
		} else {
			s.RemoteNodeURL = defaultNodeURL
		}
	}
	if s.LocalNodeURL == "" {
		s.LocalNodeURL = defaultLocalNodeURL
	}
	if s.NodeMode == "" {
		s.NodeMode = "remote"
	}
	if s.Theme == "" {
		s.Theme = "dark"
	}
	s.NodeURL = "" // stop persisting the deprecated field
}
```
Change `GetSettings` to unmarshal onto a zero `Settings` then migrate (so legacy `nodeUrl` isn't shadowed by a pre-filled default):
```go
func (c *ConfigService) GetSettings() (Settings, error) {
	d, err := c.dataDir()
	if err != nil {
		return Settings{}, err
	}
	raw, err := os.ReadFile(filepath.Join(d, "settings.json"))
	if os.IsNotExist(err) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, err
	}
	migrateSettings(&s)
	return s, nil
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run 'TestSettings|TestActiveNodeURL|TestMigration' -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/dto.go app/config_service.go app/config_service_test.go
git commit -m "feat(app): node mode + per-mode URL settings with legacy migration"
```

---

## Task 2: NodeService mode API

**Files:** Modify `app/node_service.go`; Test `app/node_service_test.go`.

**Interfaces:**
- Consumes: `Settings.ActiveNodeURL()`, `NodeConfig`, existing `SetNode(url) error`, `*ConfigService.GetSettings/SetSettings`.
- Produces: `(*NodeService)` `SetNodeMode(mode string) error`, `SetNodeURL(mode, url string) error`, `Connect() error`, `GetNodeConfig() (NodeConfig, error)`; new unexported `mode string` field; `NodeStatus().Mode` reflects it.

- [ ] **Step 1: Write the failing tests**

Create `app/node_service_test.go`:
```go
package app

import "testing"

func newTestNode(t *testing.T) *NodeService {
	t.Helper()
	return newNodeService(newTestConfig(t), nil)
}

func TestSetNodeModeRejectsUnknown(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeMode("bogus"); err == nil {
		t.Fatal("expected unknown mode to error")
	}
}

func TestSetNodeModePersistsEvenIfUnreachable(t *testing.T) {
	n := newTestNode(t)
	// No local node is running; the connect attempt fails, but the chosen mode
	// must still be persisted (user intent), and reflected by GetNodeConfig.
	_ = n.SetNodeMode("local") // connect error expected and ignored here
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "local" {
		t.Fatalf("mode should persist as local, got %q", cfg.Mode)
	}
	if n.NodeStatus().Mode != "local" {
		t.Fatalf("NodeStatus().Mode should be local, got %q", n.NodeStatus().Mode)
	}
}

func TestSetNodeURLValidatesAndPersists(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("bogus", "ws://x"); err == nil {
		t.Fatal("expected unknown mode to error")
	}
	if err := n.SetNodeURL("local", "http://x"); err == nil {
		t.Fatal("expected non-ws scheme to error")
	}
	// Setting the non-active mode's URL persists without a reconnect (no error).
	if err := n.SetNodeURL("local", "ws://127.0.0.1:9"); err != nil {
		t.Fatalf("SetNodeURL(local): %v", err)
	}
	cfg, _ := n.GetNodeConfig()
	if cfg.LocalURL != "ws://127.0.0.1:9" {
		t.Fatalf("LocalURL not persisted: %q", cfg.LocalURL)
	}
}

func TestGetNodeConfigDefaults(t *testing.T) {
	n := newTestNode(t)
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "remote" || cfg.RemoteURL != defaultNodeURL || cfg.LocalURL != defaultLocalNodeURL {
		t.Fatalf("unexpected node config: %+v", cfg)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestSetNodeMode|TestSetNodeURL|TestGetNodeConfig' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement**

In `app/node_service.go`: add `mode string` to the `NodeService` struct (in the mutex-protected group). Ensure `"strings"` and `"fmt"` are imported. Change `NodeStatus()` to report the mode (default `"remote"` when empty):
```go
func (n *NodeService) NodeStatus() NodeStatus {
	n.mu.RLock()
	connected := n.client != nil
	height := n.height
	mode := n.mode
	n.mu.RUnlock()
	if mode == "" {
		mode = "remote"
	}
	return NodeStatus{Mode: mode, Connected: connected, Syncing: false, Height: height, Peers: 0}
}
```
In `SetNode`, **remove** the block that persists the deprecated URL:
```go
	if s, err := n.config.GetSettings(); err == nil {
		s.NodeURL = url
		_ = n.config.SetSettings(s)
	}
```
(persistence now belongs to `SetNodeMode`/`SetNodeURL`). If `emitStatus` builds a `NodeStatus`, make it read `n.mode` the same way (or have it call the corrected status builder) so emitted events carry the right mode.

Add the mode API:
```go
// SetNodeMode persists the node mode and connects to that mode's URL. The mode
// is persisted before connecting, so an unreachable node leaves the chosen mode
// in effect (the UI shows disconnected + Retry).
func (n *NodeService) SetNodeMode(mode string) error {
	if mode != "remote" && mode != "local" {
		return fmt.Errorf("unknown node mode %q", mode)
	}
	s, err := n.config.GetSettings()
	if err != nil {
		return err
	}
	s.NodeMode = mode
	if err := n.config.SetSettings(s); err != nil {
		return err
	}
	n.mu.Lock()
	n.mode = mode
	n.mu.Unlock()
	return n.SetNode(s.ActiveNodeURL())
}

// SetNodeURL persists a mode's URL (validated) and reconnects if it is active.
func (n *NodeService) SetNodeURL(mode, url string) error {
	if mode != "remote" && mode != "local" {
		return fmt.Errorf("unknown node mode %q", mode)
	}
	if !strings.HasPrefix(url, "ws://") && !strings.HasPrefix(url, "wss://") {
		return fmt.Errorf("node url must start with ws:// or wss://")
	}
	s, err := n.config.GetSettings()
	if err != nil {
		return err
	}
	if mode == "local" {
		s.LocalNodeURL = url
	} else {
		s.RemoteNodeURL = url
	}
	if err := n.config.SetSettings(s); err != nil {
		return err
	}
	if mode == s.NodeMode {
		return n.SetNode(url)
	}
	return nil
}

// Connect connects to the active mode's URL using persisted settings.
func (n *NodeService) Connect() error {
	s, err := n.config.GetSettings()
	if err != nil {
		return err
	}
	n.mu.Lock()
	n.mode = s.NodeMode
	n.mu.Unlock()
	return n.SetNode(s.ActiveNodeURL())
}

// GetNodeConfig returns the node mode and per-mode URLs for the settings UI.
func (n *NodeService) GetNodeConfig() (NodeConfig, error) {
	s, err := n.config.GetSettings()
	if err != nil {
		return NodeConfig{}, err
	}
	return NodeConfig{Mode: s.NodeMode, RemoteURL: s.RemoteNodeURL, LocalURL: s.LocalNodeURL}, nil
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run 'TestSetNodeMode|TestSetNodeURL|TestGetNodeConfig' -v && go build ./...`
Expected: PASS (the unreachable-connect tests pass because mode is persisted before the failing connect).

- [ ] **Step 5: Commit**

```bash
git add app/node_service.go app/node_service_test.go
git commit -m "feat(app): NodeService mode API (SetNodeMode/SetNodeURL/Connect/GetNodeConfig)"
```

---

## Task 3: Bindings + node store + startup connect

**Files:** Modify `frontend/wailsjs/` (generated), `frontend/src/lib/stores/node.ts`, `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: bound `NodeService.SetNodeMode`/`SetNodeURL`/`Connect`/`GetNodeConfig`.
- Produces: node store actions `setMode(mode)`/`setUrl(mode,url)`/`getConfig()`; App connects via `Connect()`.

- [ ] **Step 1: Regenerate bindings**

```bash
"$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/ 2>/dev/null || true
ls frontend/wailsjs/go/app/NodeService.d.ts  # should list SetNodeMode/SetNodeURL/Connect/GetNodeConfig
```
Revert any `frontend/wailsjs/runtime/*` churn (CLI-version skew); keep only `frontend/wailsjs/go/...` additions.

- [ ] **Step 2: Add store actions + fix startup connect**

In `frontend/src/lib/stores/node.ts`, add (importing the binding namespace):
```ts
import * as N from '../../../wailsjs/go/app/NodeService'

export type NodeConfig = { mode: string; remoteUrl: string; localUrl: string }

export async function getConfig(): Promise<NodeConfig> {
  return (await N.GetNodeConfig()) as NodeConfig
}
export async function setMode(mode: string): Promise<void> {
  try { await N.SetNodeMode(mode) } catch { /* status event reflects disconnected */ }
}
export async function setUrl(mode: string, url: string): Promise<void> {
  await N.SetNodeURL(mode, url)
}
```
(`setMode` swallows the connect error because an unreachable node is surfaced via the `node:status` event + Retry, not a thrown rejection; `setUrl` surfaces validation errors to the caller.)

In `frontend/src/App.svelte`, replace the legacy connect line:
```svelte
      if (s.nodeUrl) await N.SetNode(s.nodeUrl)
```
with:
```svelte
      await N.Connect()
```
(If `N` is the `NodeService` binding import already present, reuse it; the `s.nodeUrl` read can be dropped.)

- [ ] **Step 3: Build to verify**

Run: `cd frontend && pnpm run build`
Expected: clean build (TS resolves the new bindings/actions).

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs frontend/src/lib/stores/node.ts frontend/src/App.svelte
git commit -m "feat(frontend): node mode bindings + store actions + startup Connect()"
```

---

## Task 4: Settings Node section (UI)

**Files:** Modify `frontend/src/routes/Settings.svelte`; Test `frontend/src/routes/Settings.test.ts`.

**Interfaces:**
- Consumes: node store `getConfig`/`setMode`/`setUrl`, `node` status store.
- Produces: a Node section (mode radios, per-mode URL inputs, Apply, status, Retry).

- [ ] **Step 1: Write the failing test**

Add to `frontend/src/routes/Settings.test.ts` (keep existing tests; extend the mock):
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/NodeService', () => ({
  GetNodeConfig: vi.fn().mockResolvedValue({ mode: 'remote', remoteUrl: 'wss://r:35998', localUrl: 'ws://127.0.0.1:35998' }),
  SetNodeMode: vi.fn().mockResolvedValue(undefined),
  SetNodeURL: vi.fn().mockResolvedValue(undefined),
}))

import Settings from './Settings.svelte'
import * as N from '../../wailsjs/go/app/NodeService'

describe('Settings node section', () => {
  it('switching to Local calls SetNodeMode', async () => {
    render(Settings)
    const localRadio = await screen.findByLabelText(/local/i)
    await fireEvent.click(localRadio)
    await fireEvent.click(screen.getByRole('button', { name: /apply node/i }))
    expect(N.SetNodeMode).toHaveBeenCalledWith('local')
  })
})
```
(If the existing Settings.test.ts already mocks WalletService/runtime, keep those mocks; just add the NodeService mock and this describe block.)

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Settings.test.ts`
Expected: FAIL — no Local radio / "Apply node" button yet.

- [ ] **Step 3: Implement**

Add a Node section to `frontend/src/routes/Settings.svelte`. In the script, load config on mount and track edits:
```svelte
  import { onMount } from 'svelte'
  import { node } from '../lib/stores/node'
  import { getConfig, setMode, setUrl } from '../lib/stores/node'

  let nodeMode = 'remote'
  let remoteUrl = ''
  let localUrl = ''
  let nodeMsg = ''
  let nodeErr = ''
  let loadedMode = 'remote'
  let loadedRemote = ''
  let loadedLocal = ''

  onMount(async () => {
    const c = await getConfig()
    nodeMode = loadedMode = c.mode
    remoteUrl = loadedRemote = c.remoteUrl
    localUrl = loadedLocal = c.localUrl
  })

  async function applyNode() {
    nodeMsg = ''; nodeErr = ''
    try {
      if (remoteUrl !== loadedRemote) { await setUrl('remote', remoteUrl); loadedRemote = remoteUrl }
      if (localUrl !== loadedLocal) { await setUrl('local', localUrl); loadedLocal = localUrl }
      if (nodeMode !== loadedMode) { await setMode(nodeMode); loadedMode = nodeMode }
      else if (nodeMode === 'remote' ? remoteUrl !== loadedRemote : localUrl !== loadedLocal) { await setMode(nodeMode) }
      nodeMsg = 'Node settings applied'
    } catch (e: any) { nodeErr = e?.message ?? String(e) }
  }
  async function retryNode() { await setMode(nodeMode) }
```
And in the markup, a Node section:
```svelte
  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Node</h2>
    <label class="flex items-center gap-2"><input type="radio" bind:group={nodeMode} value="remote" /> Remote</label>
    <input class="w-full rounded bg-bg px-3 py-2 font-mono text-sm" bind:value={remoteUrl} aria-label="remote node url" />
    <label class="flex items-center gap-2"><input type="radio" bind:group={nodeMode} value="local" /> Local</label>
    <input class="w-full rounded bg-bg px-3 py-2 font-mono text-sm" bind:value={localUrl} aria-label="local node url" />
    <div class="flex items-center gap-3">
      <button class="rounded bg-accent px-3 py-1 text-bg" on:click={applyNode} aria-label="Apply node">Apply node</button>
      {#if !$node.connected}<button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={retryNode}>Retry</button>{/if}
    </div>
    <p class="text-xs text-muted">{$node.connected ? `Connected (${$node.mode}) · height ${$node.height}` : `Disconnected (${$node.mode})`}</p>
    {#if nodeMsg}<p class="text-success text-sm">{nodeMsg}</p>{/if}
    {#if nodeErr}<p class="text-error text-sm" role="alert">{nodeErr}</p>{/if}
  </section>
```
(Place it alongside the existing Change-password / Reveal-mnemonic sections. Reuse the existing `onMount` if Settings already has one — merge the config load into it.)

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Settings.svelte frontend/src/routes/Settings.test.ts
git commit -m "feat(frontend): Node section in Settings (mode + per-mode URLs + retry)"
```

---

## Task 5: Verification + acceptance

**Files:** Create `docs/phase4a-acceptance.md`.

- [ ] **Step 1: Full automated verification**

```bash
go test ./...
cd frontend && pnpm test && pnpm run build && cd ..
"$(go env GOPATH)/bin/wails" build
```
Expected: backend green, frontend green, app builds.

- [ ] **Step 2: Manual acceptance (Phase 4a gate)**

1. Launch the app (Remote by default) → connects, StatusBar shows height + chainId.
2. Open Settings → Node; switch to **Local** with a running `znnd` at `ws://127.0.0.1:35998` → Apply → connects; status shows height + chainId of the local node.
3. Stop `znnd` (or use Local with no node) → Apply/Retry → clean **Disconnected (local)** state, no crash.
4. Switch back to **Remote** → reconnects.
5. Edit a URL, Apply, restart the app → the edited URLs and selected mode persist (settings migration + persistence).

- [ ] **Step 3: Record the result**

`docs/phase4a-acceptance.md`: automated results + the manual checks (mode switch, local connect height/chainId, disconnected+retry, persistence across restart), with the `znnd` version used.

- [ ] **Step 4: Commit**

```bash
git add docs/phase4a-acceptance.md
git commit -m "docs: Phase 4a acceptance record"
```

---

## Self-Review

**Spec coverage:** Settings model + migration + `ActiveNodeURL` (T1); `SetNodeMode`/`SetNodeURL`/`Connect`/`GetNodeConfig` + `NodeStatus.Mode` + `SetNode` stops persisting `nodeUrl` (T2); bindings + store actions + startup `Connect()` (T3); Settings Node section UI (T4); verification + manual acceptance incl. persistence-across-restart (T5). All spec sections mapped.

**Placeholder scan:** No TBD/TODO. Bindings regen (T3) is environment-run with the exact command + revert caution. The `applyNode` logic handles the "active URL edited but mode unchanged → reconnect" case explicitly.

**Type consistency:** Go `SetNodeMode(mode)`, `SetNodeURL(mode,url)`, `Connect()`, `GetNodeConfig() NodeConfig{Mode,RemoteURL,LocalURL}` match the TS store wrappers (`setMode`/`setUrl`/`getConfig`) and the `NodeConfig{mode,remoteUrl,localUrl}` camelCase shape. `Settings.NodeMode/RemoteNodeURL/LocalNodeURL` + `ActiveNodeURL()` consistent across T1/T2. `NodeStatus.Mode` (existing field) now driven by `n.mode`.

**Known follow-up (not 4a):** embedded in-process node is Phase 4b. Auto-detecting/launching `znnd` is out of scope.
