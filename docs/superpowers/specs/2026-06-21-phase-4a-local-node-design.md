# Phase 4a — Local Node Mode Design

**Date:** 2026-06-21
**Status:** Approved
**Scope:** Phase 4a of the syrius-wails roadmap — the first half of "node modes." Add a **local** node mode (connect to a user-run `znnd`) alongside the existing **remote** mode, with mode + per-mode URLs in settings and a Node section in Settings. The **embedded** in-process node is deferred to Phase 4b (its own spec).

## Goal

Let the user choose Remote or Local node, each with its own persisted URL, switch between them, and see live connection status — reusing the existing connect/read/status path (local is just a different URL + label).

## Locked decisions (brainstorming 2026-06-21)

- **Scope split:** local mode now (4a); embedded in-process node later (4b).
- **Settings model:** separate `NodeMode` + `RemoteNodeURL` + `LocalNodeURL` (switching preserves both URLs); active URL chosen by mode.
- **UI:** a Node section in the existing `Settings.svelte` (not a new dashboard panel); the dashboard `StatusBar` keeps showing mode/connected/height.
- **Local default URL:** `ws://127.0.0.1:35998` (znnd default).

## Context

Phase 1 built NodeService with `SetNode(url) error` (connects an `rpc_client.RpcClient`, sets height + chainId from the frontier momentum, starts the momentum subscription, emits `node:status`), `Disconnect()`, `NodeStatus() NodeStatus`, and the read methods. `NodeStatus` already has a `Mode string` field (currently hardcoded `"remote"`). `Settings` currently has a single `NodeURL string` (default `wss://my.hc1node.com:35998` via `defaultNodeURL`); the loader default is `Settings{NodeURL: defaultNodeURL, Theme: "dark", ActiveAccount: 0}`. App startup calls `SetNode(settings.NodeURL)`.

This phase changes the URL model and adds mode-switching; the connect machinery itself is unchanged.

## Architecture

Local and remote share the **same** connect/read/status path. A "mode" only selects which persisted URL to hand to `SetNode`, and labels the status. No new RPC code.

```
SetNodeMode("local") ─▶ resolve LocalNodeURL ─▶ SetNode(url)  [existing connect + node:status]
SetNodeMode("remote")─▶ resolve RemoteNodeURL ─▶ SetNode(url)
SetNodeURL("local", "ws://…") ─▶ persist; if active mode, reconnect
```

The chainId already surfaced by `SetNode` tells the user whether their local `znnd` is mainnet or testnet — no separate network handling needed in 4a.

## Components

### ConfigService / Settings (`app/dto.go`, `app/config_service.go`)

- Replace the single `NodeURL` with:
  - `NodeMode string` (`json:"nodeMode"`) — `"remote"` | `"local"`, default `"remote"`.
  - `RemoteNodeURL string` (`json:"remoteNodeUrl"`) — default `wss://my.hc1node.com:35998` (the existing `defaultNodeURL`).
  - `LocalNodeURL string` (`json:"localNodeUrl"`) — default `ws://127.0.0.1:35998` (new const `defaultLocalNodeURL`).
- Keep a deprecated `NodeURL string` (`json:"nodeUrl,omitempty"`) field **for read-only migration** of existing `settings.json`.
- **Migration in the settings loader** (after unmarshal, before returning): if `RemoteNodeURL == ""` → set it to `NodeURL` if non-empty, else `defaultNodeURL`; if `LocalNodeURL == ""` → `defaultLocalNodeURL`; if `NodeMode == ""` → `"remote"`; then clear `NodeURL` (stop writing it). This is idempotent and safe on a fresh (default) settings too.
- Add a helper `(s Settings) ActiveNodeURL() string` → returns `LocalNodeURL` when `NodeMode == "local"`, else `RemoteNodeURL`.

### NodeService (`app/node_service.go`)

- `SetNodeMode(mode string) error` — reject any mode other than `"remote"`/`"local"`; load settings, set `NodeMode`, persist; resolve `settings.ActiveNodeURL()`; call `SetNode(url)`. Set the in-memory mode used by `NodeStatus().Mode`.
- `SetNodeURL(mode, url string) error` — reject bad mode; basic URL sanity (non-empty, `ws://`/`wss://` scheme); load settings, set `RemoteNodeURL`/`LocalNodeURL` for that mode, persist; if `mode == current NodeMode`, call `SetNode(url)` to reconnect.
- `NodeStatus().Mode` reflects the persisted `NodeMode` (read from settings/in-memory, not hardcoded). `SetNode` keeps emitting `node:status`.
- A `GetNodeConfig() (NodeConfig, error)` read method for the UI: `{Mode, RemoteURL, LocalURL string}`.

### App wiring (`app/app.go`)

- OnStartup connects using `settings.ActiveNodeURL()` and the saved `NodeMode` (instead of the old single `NodeURL`).

### DTOs

- `NodeConfig{ Mode, RemoteURL, LocalURL string }` (camelCase `mode`/`remoteUrl`/`localUrl`).
- `NodeStatus` unchanged (already has `Mode`).

## Frontend

- **Node section in `Settings.svelte`:** radio **Remote / Local**; an editable URL input for each mode (seeded from `GetNodeConfig`); an **Apply** button that calls `SetNodeURL(mode, url)` for an edited URL and `SetNodeMode(mode)` when the selected mode changes; live status line (mode, connected, height, peers, chainId) from the `node` store / `node:status`; a **Retry** button (re-invokes `SetNodeMode(currentMode)`) shown when disconnected.
- **Stores:** extend the `node` store with `setMode(mode)` / `setUrl(mode, url)` / `getConfig()` actions wrapping the bindings; it already subscribes to `node:status`.
- The dashboard `StatusBar` is unchanged (already shows mode/connected/height).

## Error handling

- Invalid mode → "unknown node mode" error from the backend (never trust the WebView).
- Empty/malformed URL → rejected with a clear message; the previous connection is left as-is (don't disconnect on a bad edit).
- Local node unreachable → `SetNode` fails to fetch the frontier momentum → existing disconnected `node:status` (connected=false); the UI shows disconnected + Retry. No crash, no panic.
- Switching modes while a wallet is unlocked is fine (node and wallet are independent); balances/history refresh off the new connection via the existing flow.

## Testing

- **Backend (Go, offline):**
  - Settings migration: legacy `{"nodeUrl":"wss://x"}` loads with `RemoteNodeURL=="wss://x"`, `LocalNodeURL==defaultLocalNodeURL`, `NodeMode=="remote"`, and `NodeURL` cleared; fresh/default settings fill all three defaults; migration is idempotent on re-save+reload.
  - `ActiveNodeURL()` returns the right URL per mode.
  - `SetNodeMode`: rejects `"bogus"`; `"local"` persists mode and selects `LocalNodeURL`; `NodeStatus().Mode` reflects it. (Use a NodeService with a temp data dir; assert persisted settings + selected URL without requiring a live node — e.g. by asserting the URL passed to connect or the persisted `NodeMode`.)
  - `SetNodeURL`: rejects bad mode and empty/`http://` URL; updates the correct field; persists.
- **Frontend (Vitest, mocked bindings):** Node section renders both URLs from `GetNodeConfig`; selecting Local calls `SetNodeMode('local')`; editing a URL + Apply calls `SetNodeURL`; status line reflects connected vs disconnected; Retry calls `setMode(currentMode)`.
- **Acceptance (manual):** with a real `znnd` running locally, switch to Local → connects, shows height + chainId; stop `znnd` / no local node → clean disconnected state + Retry; switch back to Remote → reconnects. Confirm both URLs persist across an app restart.

## Security

- No key material involved. All mode/URL inputs validated in Go (untrusted WebView). No secrets logged. The chain-id guard from Phase 2 still governs sending, so pointing Local at a mainnet `znnd` does not bypass `AllowMainnetSend`.

## Exit criteria (Phase 4a → 4b)

- Remote/Local mode switch with per-mode persisted URLs; local connects to a running `znnd` and shows height/chainId; clean disconnected state + Retry when local is down; both URLs survive restart.
- Settings migration from the legacy single `nodeUrl` works.
- `go test ./...` (offline) and frontend unit tests pass.

## Out of scope (deferred)

- **Embedded in-process node (Phase 4b)** — `node.NewNode/Start/Stop` lifecycle, genesis/seeder config, initial sync UX, disk warnings.
- NoM contract features (Phase 5); Ledger (Phase 6); packaging/signing + nom-ui-style design pass (Phase 7).
- Auto-detecting a local node; bundling/launching `znnd`; multi-node failover.
