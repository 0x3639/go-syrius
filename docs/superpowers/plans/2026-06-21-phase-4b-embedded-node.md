# Phase 4b — Embedded In-Process Node Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run a full go-zenon mainnet node in-process as a third node mode (Embedded), with rich sync progress, lifecycle, and data management.

**Architecture:** A new `internal/embeddednode` package wraps go-zenon's `node.NewNode/Start/Stop` (loopback RPC, embedded mainnet genesis). NodeService gains an `"embedded"` mode that starts the node, connects the wallet over loopback (reusing the Phase-4a connect path), and runs a sync poller emitting a new `node:sync` event. The frontend Settings Node section gets an Embedded option with a pre-start warning, a rich sync panel, and delete-data.

**Tech Stack:** Go 1.24+, `github.com/zenon-network/go-zenon/node` (+ chain/genesis), `znn-sdk-go/rpc_client` (`StatsApi`), Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- Mainnet only: empty `GenesisFile` → go-zenon embedded mainnet genesis; built-in `DefaultSeeders`. No genesis shipped.
- Embedded RPC bound to **loopback only** (`WSHost: "127.0.0.1"`, `WSPort: 35998`), `EnableHTTP: false`, `Producer: nil`. Embedded URL is fixed (`ws://127.0.0.1:35998`) and NOT user-editable.
- Sync telemetry from SDK `StatsApi.SyncInfo() (*protocol.SyncInfo{State,CurrentHeight,TargetHeight})` + `NetworkInfo().NumPeers`. Honest fallbacks: `target==0` → no percent/ETA ("connecting to peers…").
- Security: mainnet chainId (1) ⇒ Phase-2 send guard still gates sending (`AllowMainnetSend` default false); no key material; clean `Stop()` on quit; single-instance + data-dir lock.
- `go test ./...` offline (real-node start is `//go:build integration`, opt-in). Frontend `pnpm test` + `pnpm run build` pass.
- ENV HAZARD (iCloud-synced repo): `" 2"` collision copies break builds (`find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} +`); `node_modules` files get evicted (`rm -rf frontend/node_modules && pnpm install`); codesign trips on xattrs (`xattr -cr build/bin`). Commits are GPG-signed.

## File structure

```
internal/embeddednode/embeddednode.go        # NEW: buildConfig + Start/Stop/Handle (go-zenon node lifecycle)
internal/embeddednode/embeddednode_test.go    # NEW: config-builder unit test
internal/embeddednode/embeddednode_integration_test.go  # NEW: //go:build integration start/stop
app/node_sync.go        # NEW: computeSync + mapSyncState pure helpers
app/node_sync_test.go    # NEW: unit tests for the helpers
app/dto.go               # MOD: EmbeddedInfo, SyncStatus, EventNodeSync; ActiveNodeURL embedded; defaultEmbeddedNodeURL
app/node_service.go      # MOD: embedded mode (start/connect/poller/stop), GetEmbeddedInfo, DeleteEmbeddedData, injectable starter
app/node_service_test.go # MOD: offline embedded-mode tests (stub starter)
app/app.go               # MOD: wire embeddednode start/stop into NodeService; OnShutdown stops embedded
frontend/wailsjs/...     # regenerated bindings
frontend/src/lib/stores/node.ts          # MOD: sync sub-store + getEmbeddedInfo/deleteEmbeddedData
frontend/src/lib/components/StatusBar.svelte  # MOD: "Embedded · syncing N%"
frontend/src/routes/Settings.svelte      # MOD: Embedded radio + confirm + sync panel + delete-data
frontend/src/routes/Settings.test.ts     # MOD: embedded UI tests
```

---

## Task 1: `internal/embeddednode` lifecycle

**Files:** Create `internal/embeddednode/embeddednode.go`, `internal/embeddednode/embeddednode_test.go`, `internal/embeddednode/embeddednode_integration_test.go`.

**Interfaces:**
- Consumes: go-zenon `node.DefaultNodeConfig`, `node.NewNode`, `node.RPCConfig`, `(*node.Node).Start/Stop`.
- Produces: `func Start(dataDir string) (*Handle, error)`; `func (*Handle) Stop() error`; `func (*Handle) WSURL() string`; `func (*Handle) DataDir() string`; `const EmbeddedWSPort = 35998`; unexported `buildConfig(dataDir string) node.Config`.

- [ ] **Step 1: Write the failing test**

`internal/embeddednode/embeddednode_test.go`:
```go
package embeddednode

import (
	"path/filepath"
	"testing"
)

func TestBuildConfigLoopbackAndGenesis(t *testing.T) {
	cfg := buildConfig("/tmp/data")
	if cfg.DataPath != filepath.Join("/tmp/data", "embedded") {
		t.Fatalf("DataPath = %q", cfg.DataPath)
	}
	if cfg.GenesisFile != "" {
		t.Fatalf("GenesisFile must be empty to use embedded genesis, got %q", cfg.GenesisFile)
	}
	if cfg.Producer != nil {
		t.Fatalf("Producer must be nil")
	}
	if !cfg.RPC.EnableWS || cfg.RPC.EnableHTTP {
		t.Fatalf("WS must be enabled and HTTP disabled: %+v", cfg.RPC)
	}
	if cfg.RPC.WSHost != "127.0.0.1" || cfg.RPC.WSPort != EmbeddedWSPort {
		t.Fatalf("WS must bind loopback:%d, got %s:%d", EmbeddedWSPort, cfg.RPC.WSHost, cfg.RPC.WSPort)
	}
	if len(cfg.Net.Seeders) == 0 {
		t.Fatalf("expected built-in seeders to be preserved")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/embeddednode/ -run TestBuildConfig -v`
Expected: FAIL — package/func undefined.

- [ ] **Step 3: Implement**

`internal/embeddednode/embeddednode.go`:
```go
// Package embeddednode runs a full go-zenon node in-process (mainnet, loopback
// RPC) so the wallet can use a locally-synced node. It is not Wails-bound.
package embeddednode

import (
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/zenon-network/go-zenon/node"
)

// EmbeddedWSPort is the loopback port the embedded node's WS RPC binds to.
const EmbeddedWSPort = 35998

var (
	mu      sync.Mutex
	current *node.Node // single-instance guard
)

// buildConfig derives the embedded node config from go-zenon defaults, keeping
// the default seeders/peer settings but forcing loopback WS, HTTP off, no
// producer, and an empty GenesisFile (→ embedded mainnet genesis).
func buildConfig(dataDir string) node.Config {
	cfg := node.DefaultNodeConfig // value copy keeps Net defaults (seeders)
	cfg.DataPath = filepath.Join(dataDir, "embedded")
	cfg.WalletPath = filepath.Join(cfg.DataPath, "wallet")
	cfg.GenesisFile = ""
	cfg.Name = "go-syrius-embedded"
	cfg.LogLevel = "warn"
	cfg.Producer = nil
	cfg.RPC = node.RPCConfig{
		EnableWS:   true,
		WSHost:     "127.0.0.1",
		WSPort:     EmbeddedWSPort,
		EnableHTTP: false,
		WSOrigins:  []string{"*"},
	}
	return cfg
}

// Handle owns a running embedded node.
type Handle struct {
	node    *node.Node
	wsURL   string
	dataDir string
}

func (h *Handle) WSURL() string   { return h.wsURL }
func (h *Handle) DataDir() string { return h.dataDir }

// Start brings up the embedded node and returns once its WS RPC accepts a TCP
// connection (or a bounded timeout elapses). Only one embedded node may run.
func Start(dataDir string) (*Handle, error) {
	mu.Lock()
	defer mu.Unlock()
	if current != nil {
		return nil, fmt.Errorf("embedded node already running")
	}
	cfg := buildConfig(dataDir)
	n, err := node.NewNode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("create embedded node: %w", err)
	}
	if err := n.Start(); err != nil {
		return nil, fmt.Errorf("start embedded node: %w", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", EmbeddedWSPort)
	if err := waitForPort(addr, 30*time.Second); err != nil {
		_ = n.Stop()
		return nil, fmt.Errorf("embedded node rpc not ready: %w", err)
	}
	current = n
	return &Handle{node: n, wsURL: fmt.Sprintf("ws://%s", addr), dataDir: cfg.DataPath}, nil
}

// Stop halts the embedded node. Idempotent.
func (h *Handle) Stop() error {
	mu.Lock()
	defer mu.Unlock()
	if h == nil || h.node == nil {
		return nil
	}
	err := h.node.Stop()
	if current == h.node {
		current = nil
	}
	h.node = nil
	return err
}

func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("port %s not open within %s", addr, timeout)
}
```

- [ ] **Step 4: Add the integration test (real node, opt-in)**

`internal/embeddednode/embeddednode_integration_test.go`:
```go
//go:build integration

package embeddednode

import (
	"testing"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
)

// TestStartStop spins up a real mainnet embedded node, confirms its WS RPC
// answers, then stops it. Heavy (downloads peers/genesis); opt-in.
func TestStartStop(t *testing.T) {
	h, err := Start(t.TempDir())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Stop()

	client, err := rpc_client.NewRpcClient(h.WSURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Stop()

	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := client.StatsApi.SyncInfo(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SyncInfo never answered")
		}
		time.Sleep(time.Second)
	}
	if err := h.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
```

- [ ] **Step 5: Run + build**

Run: `go test ./internal/embeddednode/ -run TestBuildConfig -v && go build ./...`
Expected: unit test PASS; build clean (pulls go-zenon `node` + deps). If build fails with `" 2"` collision copies, clean them and retry.

- [ ] **Step 6: Commit**

```bash
git add internal/embeddednode
git commit -m "feat(embeddednode): in-process go-zenon node lifecycle (loopback, embedded genesis)"
```

---

## Task 2: Sync math + DTOs

**Files:** Create `app/node_sync.go`, `app/node_sync_test.go`; Modify `app/dto.go`.

**Interfaces:**
- Consumes: `github.com/zenon-network/go-zenon/protocol` (`SyncState`).
- Produces: `EmbeddedInfo{Running bool; DataDir string; SizeBytes int64}`; `SyncStatus{State string; CurrentHeight,TargetHeight uint64; Percent float64; EtaSeconds int64; Peers int}`; `const EventNodeSync = "node:sync"`; `const defaultEmbeddedNodeURL = "ws://127.0.0.1:35998"`; `(Settings) ActiveNodeURL()` handles `"embedded"`; `heightSample{T time.Time; Height uint64}`; `mapSyncState(protocol.SyncState) string`; `computeSync(samples []heightSample, current, target uint64, peers int, state string) SyncStatus`.

- [ ] **Step 1: Write the failing tests**

`app/node_sync_test.go`:
```go
package app

import (
	"testing"
	"time"

	"github.com/zenon-network/go-zenon/protocol"
)

func TestComputeSyncPercentAndEta(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	samples := []heightSample{
		{T: base, Height: 100},
		{T: base.Add(10 * time.Second), Height: 200}, // 10 blocks/sec
	}
	s := computeSync(samples, 200, 1200, 5, "syncing")
	if s.TargetHeight != 1200 || s.CurrentHeight != 200 {
		t.Fatalf("heights: %+v", s)
	}
	// percent = 200/1200*100 ≈ 16.67
	if s.Percent < 16.6 || s.Percent > 16.7 {
		t.Fatalf("percent = %v", s.Percent)
	}
	// eta = (1200-200)/10 = 100s
	if s.EtaSeconds != 100 {
		t.Fatalf("eta = %d", s.EtaSeconds)
	}
	if s.Peers != 5 || s.State != "syncing" {
		t.Fatalf("misc: %+v", s)
	}
}

func TestComputeSyncNoTargetNoEta(t *testing.T) {
	s := computeSync(nil, 50, 0, 0, "starting")
	if s.Percent != 0 || s.EtaSeconds != 0 {
		t.Fatalf("target==0 must yield no percent/eta: %+v", s)
	}
}

func TestComputeSyncDoneNoEta(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	samples := []heightSample{{T: base, Height: 1000}, {T: base.Add(time.Second), Height: 1200}}
	s := computeSync(samples, 1200, 1200, 8, "synced")
	if s.EtaSeconds != 0 {
		t.Fatalf("current>=target must yield no eta: %+v", s)
	}
	if s.Percent < 99.99 {
		t.Fatalf("percent should be ~100: %v", s.Percent)
	}
}

func TestMapSyncState(t *testing.T) {
	if mapSyncState(protocol.Syncing) != "syncing" {
		t.Fatal("Syncing")
	}
	if mapSyncState(protocol.SyncDone) != "synced" {
		t.Fatal("SyncDone")
	}
	if mapSyncState(protocol.NotEnoughPeers) != "starting" {
		t.Fatal("NotEnoughPeers")
	}
	if mapSyncState(protocol.Unknown) != "starting" {
		t.Fatal("Unknown")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestComputeSync|TestMapSyncState' -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

In `app/dto.go` add (near the other event consts + Settings):
```go
const EventNodeSync = "node:sync"
const defaultEmbeddedNodeURL = "ws://127.0.0.1:35998"

// EmbeddedInfo describes the embedded node's data on disk.
type EmbeddedInfo struct {
	Running   bool   `json:"running"`
	DataDir   string `json:"dataDir"`
	SizeBytes int64  `json:"sizeBytes"`
}

// SyncStatus is the embedded sync snapshot pushed via EventNodeSync.
type SyncStatus struct {
	State         string  `json:"state"`
	CurrentHeight uint64  `json:"currentHeight"`
	TargetHeight  uint64  `json:"targetHeight"`
	Percent       float64 `json:"percent"`
	EtaSeconds    int64   `json:"etaSeconds"`
	Peers         int     `json:"peers"`
}
```
Update `ActiveNodeURL` (in dto.go) to handle embedded:
```go
func (s Settings) ActiveNodeURL() string {
	switch s.NodeMode {
	case "local":
		return s.LocalNodeURL
	case "embedded":
		return defaultEmbeddedNodeURL
	default:
		return s.RemoteNodeURL
	}
}
```

`app/node_sync.go`:
```go
package app

import (
	"time"

	"github.com/zenon-network/go-zenon/protocol"
)

// heightSample is one (time, height) observation for ETA rate calculation.
type heightSample struct {
	T      time.Time
	Height uint64
}

// mapSyncState maps go-zenon's SyncState to a UI string.
func mapSyncState(st protocol.SyncState) string {
	switch st {
	case protocol.Syncing:
		return "syncing"
	case protocol.SyncDone:
		return "synced"
	default: // Unknown, NotEnoughPeers
		return "starting"
	}
}

// computeSync derives percent + ETA from height samples and the node's reported
// current/target heights. With target==0 (peers not reporting yet) there is no
// percent or ETA; ETA is also omitted when the rate is non-positive or already
// at/above target.
func computeSync(samples []heightSample, current, target uint64, peers int, state string) SyncStatus {
	s := SyncStatus{State: state, CurrentHeight: current, TargetHeight: target, Peers: peers}
	if target == 0 {
		return s
	}
	s.Percent = float64(current) / float64(target) * 100
	if s.Percent > 100 {
		s.Percent = 100
	}
	if current >= target || len(samples) < 2 {
		return s
	}
	first, last := samples[0], samples[len(samples)-1]
	dt := last.T.Sub(first.T).Seconds()
	if dt <= 0 || last.Height <= first.Height {
		return s
	}
	rate := float64(last.Height-first.Height) / dt // blocks/sec
	if rate <= 0 {
		return s
	}
	s.EtaSeconds = int64(float64(target-current) / rate)
	return s
}
```

- [ ] **Step 4: Run + build**

Run: `go test ./app/ -run 'TestComputeSync|TestMapSyncState' -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/node_sync.go app/node_sync_test.go app/dto.go
git commit -m "feat(app): embedded sync DTOs + percent/ETA helpers"
```

---

## Task 3: NodeService embedded mode

**Files:** Modify `app/node_service.go`, `app/app.go`; Test `app/node_service_test.go`.

**Interfaces:**
- Consumes: `embeddednode.Start`/`Handle.Stop`/`Handle.WSURL`/`Handle.DataDir`, `computeSync`/`mapSyncState`/`heightSample`, `EmbeddedInfo`, `SyncStatus`, `EventNodeSync`, `StatsApi.SyncInfo`/`NetworkInfo`, `defaultEmbeddedNodeURL`.
- Produces: `"embedded"` accepted by `SetNodeMode`; `SetNodeURL` rejects `"embedded"`; `(*NodeService) GetEmbeddedInfo() (EmbeddedInfo, error)`; `(*NodeService) DeleteEmbeddedData() error`; injectable `embeddedStart func(dataDir string) (embeddedHandle, error)` + `embeddedHandle` interface; `OnShutdown` stops embedded.

- [ ] **Step 1: Write the failing tests**

Add to `app/node_service_test.go`:
```go
func TestSetNodeURLRejectsEmbedded(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("embedded", "ws://127.0.0.1:35998"); err == nil {
		t.Fatal("embedded URL is fixed; SetNodeURL must reject mode embedded")
	}
}

func TestSetNodeModeEmbeddedPersistsAndStarts(t *testing.T) {
	n := newTestNode(t)
	started := false
	// Stub the starter so no real node is spun up; return a handle whose URL is
	// unreachable so the subsequent connect fails — mode must still persist.
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		started = true
		return stubHandle{url: "ws://127.0.0.1:1", dir: dataDir}, nil
	}
	_ = n.SetNodeMode("embedded") // connect will fail (unreachable); ignore
	if !started {
		t.Fatal("embedded starter not invoked")
	}
	cfg, _ := n.GetNodeConfig()
	if cfg.Mode != "embedded" {
		t.Fatalf("mode should persist embedded, got %q", cfg.Mode)
	}
}

func TestDeleteEmbeddedData(t *testing.T) {
	n := newTestNode(t)
	dir, _ := n.config.dataDir()
	emb := filepath.Join(dir, "embedded")
	if err := os.MkdirAll(emb, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(emb, "x"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := n.DeleteEmbeddedData(); err != nil {
		t.Fatalf("DeleteEmbeddedData: %v", err)
	}
	if _, err := os.Stat(emb); !os.IsNotExist(err) {
		t.Fatal("embedded dir should be gone")
	}
	// absent dir is fine
	if err := n.DeleteEmbeddedData(); err != nil {
		t.Fatalf("delete absent: %v", err)
	}
}

func TestGetEmbeddedInfoSize(t *testing.T) {
	n := newTestNode(t)
	dir, _ := n.config.dataDir()
	emb := filepath.Join(dir, "embedded")
	os.MkdirAll(emb, 0o700)
	os.WriteFile(filepath.Join(emb, "x"), make([]byte, 1234), 0o600)
	info, err := n.GetEmbeddedInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.SizeBytes < 1234 {
		t.Fatalf("size = %d", info.SizeBytes)
	}
	if info.Running {
		t.Fatal("not running")
	}
}
```
Add a stub handle near the top of the test file:
```go
type stubHandle struct{ url, dir string }

func (s stubHandle) WSURL() string   { return s.url }
func (s stubHandle) DataDir() string { return s.dir }
func (s stubHandle) Stop() error     { return nil }
```
(Ensure `os`/`path/filepath` are imported in the test file.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestSetNodeURLRejectsEmbedded|TestSetNodeModeEmbedded|TestDeleteEmbeddedData|TestGetEmbeddedInfo' -v`
Expected: FAIL — undefined methods/fields.

- [ ] **Step 3: Implement**

In `app/node_service.go`:
1. Define the handle abstraction + injectable starter; add fields to `NodeService`:
```go
// embeddedHandle abstracts a running embedded node (real or test stub).
type embeddedHandle interface {
	WSURL() string
	DataDir() string
	Stop() error
}
```
Add to the `NodeService` struct (mutex-guarded group): `embedded embeddedHandle`, `embeddedStart func(dataDir string) (embeddedHandle, error)`, `syncStop chan struct{}`.

2. In `newNodeService`, default the starter to the real package (adapting the concrete `*embeddednode.Handle` to the interface):
```go
func newNodeService(c *ConfigService, w *WalletService) *NodeService {
	n := &NodeService{config: c, wallet: w}
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		return embeddednode.Start(dataDir)
	}
	return n
}
```
(Import `github.com/0x3639/go-syrius/internal/embeddednode`. `*embeddednode.Handle` already satisfies `embeddedHandle`.)

3. Accept `"embedded"` in `SetNodeMode`'s validation; reject it in `SetNodeURL`:
```go
// in SetNodeMode validation:
if mode != "remote" && mode != "local" && mode != "embedded" {
	return fmt.Errorf("unknown node mode %q", mode)
}
// in SetNodeURL validation (after the mode check):
if mode == "embedded" {
	return fmt.Errorf("embedded node url is fixed and cannot be changed")
}
```
And keep `SetNodeURL`'s existing remote/local handling (reject any mode not remote/local at the top as before, but now embedded must be rejected with the clearer message — order the embedded check before the generic one).

4. Make `SetNodeMode` start/stop embedded around the connect:
```go
func (n *NodeService) SetNodeMode(mode string) error {
	if mode != "remote" && mode != "local" && mode != "embedded" {
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

	// Tear down any running embedded node when leaving embedded mode.
	if mode != "embedded" {
		n.stopEmbedded()
	}
	n.mu.Lock()
	n.mode = mode
	n.mu.Unlock()

	if mode == "embedded" {
		dir, derr := n.config.dataDir()
		if derr != nil {
			return derr
		}
		h, serr := n.embeddedStart(dir)
		if serr != nil {
			n.emitStatus(false)
			return fmt.Errorf("start embedded node: %w", serr)
		}
		n.mu.Lock()
		n.embedded = h
		n.mu.Unlock()
		if cerr := n.SetNode(h.WSURL()); cerr != nil {
			return cerr
		}
		n.startSyncPoller()
		return nil
	}
	return n.SetNode(s.ActiveNodeURL())
}
```

5. `stopEmbedded` + sync poller:
```go
// stopEmbedded halts the embedded node + sync poller if running.
func (n *NodeService) stopEmbedded() {
	n.mu.Lock()
	if n.syncStop != nil {
		close(n.syncStop)
		n.syncStop = nil
	}
	h := n.embedded
	n.embedded = nil
	n.mu.Unlock()
	if h != nil {
		_ = h.Stop()
	}
}

// startSyncPoller polls StatsApi sync info every 2s and emits node:sync.
func (n *NodeService) startSyncPoller() {
	n.mu.Lock()
	if n.syncStop != nil {
		close(n.syncStop)
	}
	stop := make(chan struct{})
	n.syncStop = stop
	client := n.client
	ctx := n.ctx
	n.mu.Unlock()
	if client == nil || ctx == nil {
		return
	}
	go func() {
		var samples []heightSample
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				info, err := client.StatsApi.SyncInfo()
				if err != nil {
					continue
				}
				peers := 0
				if ni, nerr := client.StatsApi.NetworkInfo(); nerr == nil {
					peers = ni.NumPeers
				}
				samples = append(samples, heightSample{T: now, Height: info.CurrentHeight})
				if len(samples) > 10 {
					samples = samples[len(samples)-10:]
				}
				st := computeSync(samples, info.CurrentHeight, info.TargetHeight, peers, mapSyncState(info.State))
				runtime.EventsEmit(ctx, EventNodeSync, st)
			}
		}
	}()
}
```
(Ensure `time` is imported.)

6. `GetEmbeddedInfo` + `DeleteEmbeddedData`:
```go
// GetEmbeddedInfo reports whether the embedded node is running and its data size.
func (n *NodeService) GetEmbeddedInfo() (EmbeddedInfo, error) {
	dir, err := n.config.dataDir()
	if err != nil {
		return EmbeddedInfo{}, err
	}
	emb := filepath.Join(dir, "embedded")
	n.mu.RLock()
	running := n.embedded != nil
	n.mu.RUnlock()
	return EmbeddedInfo{Running: running, DataDir: emb, SizeBytes: dirSize(emb)}, nil
}

// DeleteEmbeddedData removes the embedded chain DB. Refuses while running.
func (n *NodeService) DeleteEmbeddedData() error {
	n.mu.RLock()
	running := n.embedded != nil
	n.mu.RUnlock()
	if running {
		return errors.New("stop the embedded node first")
	}
	dir, err := n.config.dataDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(dir, "embedded"))
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}
```
(Ensure `os`, `path/filepath`, `errors` imported.)

7. In `app/app.go` `OnShutdown`, stop embedded before disconnect:
```go
func (a *App) OnShutdown(ctx context.Context) {
	a.Node.StopAutoReceive()
	a.Node.stopEmbedded()
	_ = a.Wallet.Lock()
	_ = a.Node.Disconnect()
}
```

- [ ] **Step 4: Run + build**

Run: `go test ./app/ -run 'TestSetNodeURLRejectsEmbedded|TestSetNodeModeEmbedded|TestDeleteEmbeddedData|TestGetEmbeddedInfo' -v && go build ./...`
Expected: PASS. (The embedded-mode test uses the stub starter; no real node.)

- [ ] **Step 5: Commit**

```bash
git add app/node_service.go app/app.go app/node_service_test.go
git commit -m "feat(app): NodeService embedded mode (start/connect/poller/stop + data mgmt)"
```

---

## Task 4: Bindings + node store + StatusBar

**Files:** Modify `frontend/wailsjs/` (generated), `frontend/src/lib/stores/node.ts`, `frontend/src/lib/components/StatusBar.svelte`.

**Interfaces:**
- Consumes: bound `NodeService.GetEmbeddedInfo`/`DeleteEmbeddedData`; `node:sync` event.
- Produces: node store `sync` writable (`SyncStatus`) + actions `getEmbeddedInfo()`/`deleteEmbeddedData()`; StatusBar shows embedded sync.

- [ ] **Step 1: Regenerate bindings**

```bash
"$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/ 2>/dev/null || true
ls frontend/wailsjs/go/app/NodeService.d.ts   # GetEmbeddedInfo/DeleteEmbeddedData present; EmbeddedInfo/SyncStatus in models.ts
```
Revert any `frontend/wailsjs/runtime/*` churn (keep only `frontend/wailsjs/go/...`).

- [ ] **Step 2: Add store sync sub-store + actions**

In `frontend/src/lib/stores/node.ts` add:
```ts
export type SyncStatus = { state: string; currentHeight: number; targetHeight: number; percent: number; etaSeconds: number; peers: number }
export const sync = writable<SyncStatus | null>(null)

export type EmbeddedInfo = { running: boolean; dataDir: string; sizeBytes: number }

export async function getEmbeddedInfo(): Promise<EmbeddedInfo> {
  return (await N.GetEmbeddedInfo()) as EmbeddedInfo
}
export async function deleteEmbeddedData(): Promise<void> {
  await N.DeleteEmbeddedData()
}
```
In `initNodeEvents`, subscribe to `node:sync`:
```ts
  EventsOn('node:sync', (s: SyncStatus) => sync.set(s))
```

- [ ] **Step 3: StatusBar embedded label**

In `frontend/src/lib/components/StatusBar.svelte`, when `$node.mode === 'embedded'` and a sync status exists, show `Embedded · syncing {percent}%`:
```svelte
<script lang="ts">
  import { node, sync } from '../stores/node'
</script>
<!-- existing status spans … then: -->
{#if $node.mode === 'embedded' && $sync}
  <span>Embedded · {$sync.state === 'synced' ? 'synced' : `syncing ${$sync.percent.toFixed(1)}%`}</span>
{/if}
```
(Merge with the existing StatusBar markup/imports; don't duplicate the `node` import.)

- [ ] **Step 4: Build**

Run: `cd frontend && pnpm run build`
Expected: clean (clean `pnpm install` first if node_modules is stale).

- [ ] **Step 5: Commit**

```bash
git add frontend/wailsjs frontend/src/lib/stores/node.ts frontend/src/lib/components/StatusBar.svelte
git commit -m "feat(frontend): embedded bindings + node sync store + StatusBar"
```

---

## Task 5: Settings embedded UI

**Files:** Modify `frontend/src/routes/Settings.svelte`, `frontend/src/routes/Settings.test.ts`.

**Interfaces:**
- Consumes: node store `setMode`/`sync`/`getEmbeddedInfo`/`deleteEmbeddedData`, `node` status store.
- Produces: Embedded radio + pre-start confirm, sync panel, delete-data button.

- [ ] **Step 1: Write the failing test**

Add to `frontend/src/routes/Settings.test.ts` (extend the NodeService mock from Phase 4a + add the new describe):
```ts
// extend the existing NodeService mock with:
//   GetEmbeddedInfo: vi.fn().mockResolvedValue({ running: false, dataDir: '/d/embedded', sizeBytes: 0 }),
//   DeleteEmbeddedData: vi.fn().mockResolvedValue(undefined),

import { sync } from '../lib/stores/node'

describe('Settings embedded', () => {
  it('does not start embedded until the warning is confirmed', async () => {
    render(Settings)
    const emb = await screen.findByLabelText(/embedded/i)
    await fireEvent.click(emb)
    await fireEvent.click(screen.getByRole('button', { name: /apply node/i }))
    // a confirm dialog appears; SetNodeMode not called yet
    expect(N.SetNodeMode).not.toHaveBeenCalledWith('embedded')
    await fireEvent.click(screen.getByRole('button', { name: /start embedded/i }))
    expect(N.SetNodeMode).toHaveBeenCalledWith('embedded')
  })

  it('shows connecting-to-peers when target is 0', async () => {
    sync.set({ state: 'starting', currentHeight: 10, targetHeight: 0, percent: 0, etaSeconds: 0, peers: 0 })
    render(Settings)
    expect(await screen.findByText(/connecting to peers/i)).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Settings.test.ts`
Expected: FAIL — no Embedded radio / confirm / sync panel.

- [ ] **Step 3: Implement**

Extend `Settings.svelte`'s Node section. Script additions:
```svelte
  import { sync, getEmbeddedInfo, deleteEmbeddedData } from '../lib/stores/node'
  let showEmbeddedConfirm = false
  let embeddedSize = 0

  async function refreshEmbedded() {
    try { embeddedSize = (await getEmbeddedInfo()).sizeBytes } catch {}
  }
  // call refreshEmbedded() in the existing onMount after getConfig()

  // applyNode: when the chosen mode is 'embedded' and not yet running, open the confirm
  // instead of calling setMode directly. Adjust applyNode:
  async function applyNode() {
    nodeMsg = ''; nodeErr = ''
    if (nodeMode === 'embedded' && loadedMode !== 'embedded') { showEmbeddedConfirm = true; return }
    try {
      if (remoteDirty) { await setUrl('remote', remoteUrl); remoteDirty = false; loadedRemote = remoteUrl }
      if (localDirty) { await setUrl('local', localUrl); localDirty = false; loadedLocal = localUrl }
      if (modeDirty) { await setMode(nodeMode); modeDirty = false; loadedMode = nodeMode }
      nodeMsg = 'Node settings applied'
    } catch (e: any) { nodeErr = e?.message ?? String(e) }
  }
  async function confirmStartEmbedded() {
    showEmbeddedConfirm = false
    await setMode('embedded'); loadedMode = 'embedded'; modeDirty = false
  }
  async function doDeleteEmbedded() {
    try { await deleteEmbeddedData(); await refreshEmbedded() } catch (e: any) { nodeErr = e?.message ?? String(e) }
  }
  function fmtEta(s: number): string {
    if (s <= 0) return ''
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60)
    return h > 0 ? `${h}h ${m}m` : `${m}m`
  }
```
Markup additions in the Node section (alongside Remote/Local):
```svelte
    <label class="flex items-center gap-2"><input type="radio" bind:group={nodeMode} value="embedded" on:change={() => (modeDirty = true)} /> Embedded</label>
    <p class="text-xs text-muted">Runs a full node in-app at ws://127.0.0.1:35998</p>

    {#if $node.mode === 'embedded' && $sync}
      <div class="rounded bg-bg p-3 space-y-1 text-sm">
        {#if $sync.targetHeight === 0}
          <p class="text-muted">connecting to peers…</p>
        {:else}
          <div class="h-2 w-full rounded bg-surface"><div class="h-2 rounded bg-accent" style="width:{$sync.percent}%"></div></div>
          <p>{$sync.state} · {$sync.currentHeight} / {$sync.targetHeight} ({$sync.percent.toFixed(1)}%){#if $sync.etaSeconds > 0} · ETA {fmtEta($sync.etaSeconds)}{/if}</p>
        {/if}
        <p class="text-muted">{$sync.peers} peers · {(embeddedSize / 1e9).toFixed(2)} GB on disk</p>
      </div>
    {/if}

    {#if $node.mode !== 'embedded'}
      <button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={doDeleteEmbedded}>Delete embedded data ({(embeddedSize / 1e9).toFixed(2)} GB)</button>
    {/if}

    {#if showEmbeddedConfirm}
      <div class="rounded border border-warn/40 bg-bg p-3 space-y-2">
        <p class="text-warn text-sm">Embedded mode runs a full Zenon node in-app: it needs several GB of disk and can take hours to fully sync. Continue?</p>
        <div class="flex gap-2">
          <button class="rounded bg-accent px-3 py-1 text-bg" on:click={confirmStartEmbedded}>Start embedded</button>
          <button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={() => (showEmbeddedConfirm = false)}>Cancel</button>
        </div>
      </div>
    {/if}
```
(Place inside the existing Node `<section>`. Keep the existing Apply/Retry/status from Phase 4a. The `applyNode` shown replaces the Phase-4a version — preserve its remote/local dirty logic exactly as above.)

- [ ] **Step 4: Run + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS; clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Settings.svelte frontend/src/routes/Settings.test.ts
git commit -m "feat(frontend): embedded node UI (warning + rich sync panel + delete data)"
```

---

## Task 6: Verification + acceptance

**Files:** Create `docs/phase4b-acceptance.md`.

- [ ] **Step 1: Full automated verification**

```bash
find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} + 2>/dev/null
go test ./...
cd frontend && pnpm test && pnpm run build && cd ..
xattr -cr build/bin 2>/dev/null; "$(go env GOPATH)/bin/wails" build
```
Expected: backend green, frontend green, app builds.

- [ ] **Step 2: Integration test (opt-in, heavy — real node)**

```bash
go test ./internal/embeddednode/ -tags integration -run TestStartStop -v -timeout 180s
```
Expected: starts a mainnet node, `SyncInfo` answers, stops clean. (Network + disk heavy; may be skipped if offline.)

- [ ] **Step 3: Manual acceptance (Phase 4b gate)**

1. Launch the app → Settings → Node → select **Embedded** → Apply → confirm the warning → node starts.
2. Watch the sync panel: "connecting to peers…" → peers appear → height climbs, percent + ETA advance.
3. Switch to **Remote** → embedded node stops (no more sync updates); wallet reconnects to remote.
4. With embedded stopped, **Delete embedded data** → size drops to ~0.
5. Quit the app while embedded is running → process exits cleanly (no orphaned node).

- [ ] **Step 4: Record the result**

`docs/phase4b-acceptance.md`: automated results + the manual checks (start, peers/height/%/ETA progress, clean stop on switch, delete-data, clean quit), with notes on sync duration observed.

- [ ] **Step 5: Commit**

```bash
git add docs/phase4b-acceptance.md
git commit -m "docs: Phase 4b acceptance record"
```

---

## Self-Review

**Spec coverage:** embeddednode lifecycle + embedded mainnet genesis + loopback RPC (T1); sync DTOs + percent/ETA + state mapping + ActiveNodeURL embedded (T2); NodeService embedded mode start/connect/poller/stop + GetEmbeddedInfo/DeleteEmbeddedData + SetNodeURL-rejects-embedded + OnShutdown (T3); bindings + sync store + StatusBar (T4); Settings embedded UI with warning/confirm + rich sync panel + delete-data + connecting-to-peers fallback (T5); verification + integration + manual acceptance (T6). All spec sections mapped.

**Placeholder scan:** No TBD/TODO. Bindings regen (T4) is environment-run with the revert caution. Real-node start is integration-tagged; offline tests use the injectable `embeddedStart` stub.

**Type consistency:** `embeddednode.Start(dataDir) (*Handle, error)` / `Handle.WSURL()/DataDir()/Stop()` match the `embeddedHandle` interface NodeService consumes. `SyncStatus`/`EmbeddedInfo` Go fields ↔ camelCase TS (`state/currentHeight/targetHeight/percent/etaSeconds/peers`, `running/dataDir/sizeBytes`). `computeSync`/`mapSyncState`/`heightSample` consistent across T2/T3. `EventNodeSync = "node:sync"` matches the store subscription. `ActiveNodeURL` embedded → `defaultEmbeddedNodeURL` consistent.

**Known follow-up (not 4b):** embedded testnet, pillar/producer mode, configurable ports — Phase 5+ / out of scope.
```
