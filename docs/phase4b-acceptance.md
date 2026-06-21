# Phase 4b — Embedded In-Process Node Acceptance

## Automated verification (2026-06-21) — PASSED
- `go test ./...` — green incl. `internal/embeddednode` (config builder) and `app` (sync helpers + embedded-mode lifecycle via stub starter).
- Frontend `pnpm test` — 20/20 across 12 files (incl. embedded confirm-gating + connecting-to-peers).
- `pnpm run build` — clean.
- `wails build` — compiles + packages the app **with the full go-zenon node linked in**; self-sign tripped on iCloud-added xattrs, signed cleanly after `xattr -cr build/bin/syrius.app && codesign --force --deep -s - …` (environment artifact, not a code issue).

Covered by tests (offline):
- `embeddednode.buildConfig`: DataPath under our dir, empty GenesisFile (→ embedded mainnet genesis), loopback WS, HTTP off, no producer, seeders preserved.
- `computeSync`/`mapSyncState`: percent + ETA; `target==0` → no percent/ETA; `current>=target` → no ETA; state mapping.
- NodeService embedded mode (stub starter): `SetNodeMode("embedded")` + `Connect()` (restart) start the embedded node + persist mode; `SetNodeURL("embedded")` rejected; `DeleteEmbeddedData` refuses while running / removes when stopped; `GetEmbeddedInfo` size.

## Manual acceptance (Phase 4b gate) — PENDING user GUI run
Requires running a real mainnet node in-app (heavy: several GB, hours to fully sync):
1. Launch the app → Settings → Node → select **Embedded** → Apply → confirm the warning → node starts.
2. Sync panel: "connecting to peers…" → peers appear → height climbs; percent + ETA advance.
3. Switch to **Remote** → embedded node stops (sync updates cease); wallet reconnects to remote.
4. With embedded stopped, **Delete embedded data** → size drops to ~0.
5. Quit while embedded is running → process exits cleanly (no orphaned node).
6. Optional opt-in integration test: `go test ./internal/embeddednode/ -tags integration -run TestStartStop -v -timeout 180s` (starts a real mainnet node, confirms SyncInfo answers, stops).

### Result (fill in after the manual run)
- embedded start + peers connect: PASS / FAIL
- height/percent/ETA advance: PASS / FAIL
- clean stop on switch + clean quit: PASS / FAIL
- delete-embedded-data reclaims space: PASS / FAIL
- sync duration observed: ____

## Security recap
- Embedded RPC bound loopback-only, HTTP disabled, no producer key.
- Mainnet chainId (1) ⇒ the Phase-2 send guard still blocks sending unless `AllowMainnetSend` (default false). No bypass.

## Environment note
Repo is in an iCloud-synced folder — `" 2"` collision copies, `node_modules` eviction, and codesign xattr trips recurred throughout Phase 4b (all worked around). Strongly recommend relocating the repo out of the synced folder.
