# Phase 4a — Local Node Mode Acceptance

## Automated verification (2026-06-21) — PASSED
- `go test ./...` — green (app + internal/*), incl. settings-migration and NodeService mode-API tests.
- Frontend `pnpm test` — 18/18 across 12 files (incl. Settings Node-section test).
- `pnpm run build` — clean.
- `wails build` — compiles, packages, self-signs → `build/bin/syrius.app`.
  - Note: codesign initially failed on iCloud-added extended attributes ("resource fork … not allowed"); fixed by `xattr -cr build/bin`. Environment artifact (repo lives in an iCloud-synced folder), not a code issue.

Covered by Go unit tests (offline):
- Settings migration: legacy `nodeUrl` → `RemoteNodeURL`; defaults fill `LocalNodeURL`/`NodeMode`; idempotent; `nodeUrl` cleared.
- `ActiveNodeURL()` selects per mode.
- `SetNodeMode` rejects unknown modes, persists the mode before connecting (so an unreachable node still leaves the mode in effect), reflected in `NodeStatus().Mode` + `GetNodeConfig`.
- `SetNodeURL` rejects bad mode + non-`ws(s)://` URL, persists the correct field.

## Manual acceptance (Phase 4a gate) — PENDING user GUI run
Requires a running `znnd` (cannot be done headless):
1. Launch `build/bin/syrius.app` (Remote by default) → connects; StatusBar shows height + chainId.
2. Settings → Node → switch to **Local** with `znnd` at `ws://127.0.0.1:35998` → Apply → connects; status shows local height + chainId.
3. Stop `znnd` (or Local with no node) → Apply/Retry → clean **Disconnected (local)** state, no crash.
4. Switch back to **Remote** → reconnects.
5. Edit a URL, Apply, restart the app → edited URLs + selected mode persist.

### Result (fill in after the manual run)
- znnd version tested: ____
- Local connect height/chainId: ____
- disconnected+retry clean: PASS / FAIL
- mode switch + persistence across restart: PASS / FAIL

## Environment notes (recurring)
- The repo is in an iCloud-synced folder. Two recurring hazards observed during Phase 4a:
  - `" 2"`-suffixed file-sync conflict copies regenerate and break `go build`/frontend build (clean with `find ... -name '* 2.*' -delete`).
  - `node_modules` files get evicted/corrupted (e.g. missing `vitest/cli-wrapper.js`) → `rm -rf frontend/node_modules && pnpm install`.
  - `codesign` fails on iCloud xattrs → `xattr -cr build/bin`.
  - Strongly recommend moving the repo out of the synced folder (or excluding it from iCloud) for Phase 4b onward.
