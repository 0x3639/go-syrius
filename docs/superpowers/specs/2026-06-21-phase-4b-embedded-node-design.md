# Phase 4b — Embedded In-Process Node Design

**Date:** 2026-06-21
**Status:** Approved
**Scope:** Phase 4b of the syrius-wails roadmap — the second half of "node modes." Run a full go-zenon node **in-process** as a third node mode (Embedded), alongside Remote and Local (Phase 4a). This is the feature where Wails materially beats the Flutter original. Builds on Phase 4a's mode infrastructure.

## Goal

Let the user run a real Zenon mainnet node inside the app: start it from Settings (with a disk/time warning), watch rich sync progress (%, ETA, peers, height), connect the wallet to it, stop it cleanly on switch/quit, and delete its data to reclaim space.

## Locked decisions (brainstorming 2026-06-21)

- **Network:** mainnet only — use go-zenon's **embedded genesis** (no genesis file shipped). Testnet stays available via Remote/Local.
- **Sync UX:** rich — state + current/target height + percent + ETA + peers, from `stats.SyncInfo` / `NetworkInfo`.
- **Controls:** start/stop (on mode switch / quit) + a pre-start disk/time warning + a "Delete embedded data" action; show on-disk size.
- **No SDK / no go-zenon forks:** import go-zenon's `node` package directly; consume sync via the SDK's `StatsApi`.

## Context (verified against go-zenon @ v0.0.8-alphanet / SDK @ v0.1.16)

- `node.NewNode(*node.Config) (*node.Node, error)`, `(*Node).Start() error`, `(*Node).Stop() error`. `Start` brings up the chain, p2p server (built-in `p2p.DefaultSeeders`), and RPC.
- `node.Config.makeGenesisConfig()`: when `GenesisFile == ""` it falls back to `genesis.MakeEmbeddedGenesisConfig()` (the embedded **mainnet** genesis present in `chain/genesis/embedded_genesis_string.go`) and logs "using embedded genesis". (It `os.Exit(1)`s only if there is **no** embedded genesis — not the case here.)
- Default ports: p2p listen 35995, HTTP 35997, WS 35998; defaults bind `0.0.0.0` with HTTP+WS enabled — we override RPC to loopback + HTTP off.
- Sync source: SDK `client.StatsApi.SyncInfo() (*protocol.SyncInfo{State, CurrentHeight, TargetHeight}, error)` and `NetworkInfo()` (peer count, `NumPeers`). Phase 4a's `NodeStatus{Mode,Connected,Syncing,Height,Peers}` already exists.

Phase 4a established `NodeMode` (`remote`/`local`) + `RemoteNodeURL`/`LocalNodeURL` + `SetNodeMode`/`SetNodeURL`/`Connect`/`GetNodeConfig`, a Node section in Settings, and `SetNode` as a pure connect that emits disconnect on failure.

## Architecture

```
SetNodeMode("embedded") ─▶ embeddednode.Start(dataDir) ─▶ (WS up on 127.0.0.1:35998)
                          ─▶ SetNode("ws://127.0.0.1:35998")   [reuse Phase-1 connect]
                          ─▶ start sync poller ─▶ emits node:sync {state,current,target,percent,eta,peers}
switch away / quit ─▶ embeddednode.Stop()
```

The embedded node is just another connection target once running; the wallet talks to it over loopback RPC exactly like a remote/local node. The new work is lifecycle + sync telemetry + data management + UI.

## Components

### `internal/embeddednode` (new package, not Wails-bound)

Wraps the go-zenon node lifecycle, isolated from `app/`.

- `Start(dataDir string) (*Handle, error)` — builds:
  ```
  node.Config{
    DataPath:    filepath.Join(dataDir, "embedded"),
    GenesisFile: "",                 // → embedded mainnet genesis
    Name:        "go-syrius-embedded",
    LogLevel:    "warn",
    Producer:    nil,
    RPC:  RPCConfig{ EnableWS: true, WSHost: "127.0.0.1", WSPort: 35998, EnableHTTP: false, WSOrigins: []string{"*"} },
    Net:  NetConfig{ /* go-zenon defaults: ListenPort 35995, DefaultSeeders, peer limits */ },
  }
  ```
  `node.NewNode(cfg)` → run `node.Start()` in a goroutine; poll the WS until it accepts a connection (bounded timeout) or the goroutine reports an error; return a `Handle` or an error (incl. data-dir lock / port in use).
- `Stop() error` — `node.Stop()`; idempotent; safe if never started.
- `Handle` — `WSURL() string` (`ws://127.0.0.1:35998`), `DataDir() string`.

Guard: a process-global single-instance guard (only one embedded node at a time).

### NodeService additions (`app/node_service.go`)

- `NodeMode` accepts `"embedded"`. Validation in `SetNodeMode`/`SetNodeURL` updated to allow it. `ActiveNodeURL()` returns the fixed embedded loopback URL for embedded mode (embedded URL is **not** user-editable; `SetNodeURL` rejects mode `"embedded"`).
- `SetNodeMode("embedded")` — persist mode; `embeddednode.Start(dataDir)`; on success `SetNode(handle.WSURL())`; start the sync poller. On `Start` failure, emit a disconnected status with a clear error (mode stays persisted; the UI shows the error + Retry).
- Stopping: switching to another mode, `Disconnect`, or `OnShutdown` calls a `stopEmbeddedLocked()` that `embeddednode.Stop()`s and ends the poller.
- **Sync poller** (goroutine, ~2s): `StatsApi.SyncInfo()` + `NetworkInfo()`; compute percent/ETA via a pure helper; emit `node:sync`. Ends when embedded stops.
- `GetEmbeddedInfo() (EmbeddedInfo, error)` — `{Running bool, DataDir string, SizeBytes int64}` (dir-size walk; 0 if absent).
- `DeleteEmbeddedData() error` — refuse if embedded is the active/running mode; else `os.RemoveAll(<dataDir>/embedded)`.

### Sync math (pure helper, unit-tested)

`computeSync(samples []HeightSample, info SyncInfo, peers int) SyncStatus` where a `HeightSample{t, height}` rolling window yields blocks/sec; `percent = current/target` (0 when `target==0`); `eta = (target-current)/rate` (omitted when `target==0`, `rate<=0`, or `current>=target`). State maps go-zenon `SyncState` → `"starting"|"syncing"|"synced"`.

### DTOs / events

- `EmbeddedInfo{ Running bool; DataDir string; SizeBytes int64 }` (camelCase).
- `SyncStatus{ State string; CurrentHeight, TargetHeight uint64; Percent float64; EtaSeconds int64; Peers int }` (camelCase) — payload of the new `node:sync` event (`EventNodeSync = "node:sync"`).
- `NodeConfig` (Phase 4a) unchanged; `Mode` may now be `"embedded"`.

### App wiring

`OnShutdown` already stops auto-receive + disconnects; add `stopEmbedded` to the shutdown path so the node halts cleanly on quit.

## Frontend

In the Settings **Node** section (Phase 4a):
- Add an **Embedded** radio; show its fixed URL read-only (`ws://127.0.0.1:35998`).
- **Pre-start confirm:** choosing Embedded + Apply opens a confirm dialog ("Runs a full Zenon node in-app: needs several GB of disk and can take hours to fully sync. Continue?") before `setMode('embedded')`. Switching *away* needs no confirm.
- **Rich sync panel** (shown while embedded active), driven by `node:sync`: state label, `current / target` height, a **progress bar (percent)**, **ETA** (human-formatted), **peers**, and **data size** (from `getEmbeddedInfo`). When `targetHeight == 0`, show "connecting to peers…" (no bogus %/ETA).
- **Delete embedded data** button — enabled only when embedded is not the active mode; confirms → `deleteEmbeddedData()` → refresh size.
- Stores: `node` store gains a `sync` writable (from `node:sync`) + actions `getEmbeddedInfo()` / `deleteEmbeddedData()`. `StatusBar` shows `Embedded · syncing N%` when embedded.

## Error handling

- `Start` failure (port in use, data-dir locked, disk error) → disconnected status + clear error + Retry; mode stays persisted.
- Delete while running → rejected with "stop the embedded node first".
- `targetHeight == 0` / no peers yet → "connecting to peers…", never a fake ETA.
- WS not up within the start timeout → `Start` returns an error (treated as above).

## Testing

- **Backend (Go, offline):**
  - `embeddednode` config builder: DataPath under our dir, `GenesisFile==""`, WS loopback + HTTP off, no producer. (Build the config without starting a node.)
  - `computeSync` pure helper: percent/ETA correct; `target==0` → percent 0, no ETA; `current>=target` → synced, no ETA; rate from samples.
  - `DeleteEmbeddedData` refuses while running; removes the dir when stopped; absent dir → size 0, no error.
  - `SetNodeMode("embedded")` selects the fixed loopback URL and marks mode; `SetNodeURL("embedded", …)` rejected.
- **Integration (`//go:build integration`, opt-in, heavy):** `embeddednode.Start(tmp)` → WS accepts + `StatsApi.SyncInfo()` returns within a timeout → `Stop()` clean. (Briefly runs a real mainnet node.)
- **Frontend (Vitest, mocked bindings):** Embedded radio + confirm gating (no `setMode` without confirm); sync panel renders state/%/ETA/peers from a mocked `node:sync` and shows "connecting to peers…" at `target==0`; delete-data disabled while embedded active.
- **Acceptance (manual gate):** switch to Embedded → node starts, peers connect, height climbs, %/ETA advance; switch to Remote → node stops; Delete embedded data reclaims space. (Full sync is hours; the gate is "visible progress + clean stop/delete," not full sync.)

## Security

- RPC bound to **loopback only**, HTTP disabled, no producer key — the embedded node is a sync/read node for the wallet, not a pillar and not network-exposed.
- chainId is mainnet (1) ⇒ the Phase-2 chain-id send guard still blocks sending unless `AllowMainnetSend` (default false). No bypass: `disconnectLocked` already resets chainID on stop.
- No key material involved; nothing sensitive logged; clean `Stop()` on quit; data-dir lock prevents a second instance.

## Exit criteria (Phase 4b → Phase 5)

- Embedded mode starts a real mainnet node in-process, connects the wallet over loopback, shows rich sync progress (%, ETA, peers, height) that advances, and stops cleanly on switch/quit.
- Delete-embedded-data reclaims space (only when stopped).
- `go test ./...` (offline) + frontend unit tests pass; the integration start/stop test passes when run opt-in.

## Out of scope (deferred)

- Embedded **testnet** node (would need a testnet genesis + seeders).
- Running embedded as a **pillar/producer**; inbound-peer port forwarding/UPnP; configurable embedded ports.
- NoM contract features (Phase 5); Ledger (Phase 6); packaging/signing + nom-ui design pass (Phase 7).
