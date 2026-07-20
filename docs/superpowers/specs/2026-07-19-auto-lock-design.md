# Wallet auto-lock on inactivity — design

**Date:** 2026-07-19
**Status:** Approved pending user review
**Goal:** Lock the wallet automatically after a configurable period of user inactivity. Default 5 minutes; configurable in Settings via presets (1 / 5 / 15 / 30 minutes / Never).

## Background

Locking exists and is solid: `WalletService.Lock()` (app/wallet_service.go:371) zeroes the keystore, bumps the session generation (`gen`) — which invalidates held/prepared blocks via `onSessionChange` — and emits `wallet:locked`. The router guard (frontend/src/router/index.ts:39-43) redirects to `/unlock` whenever `wallet.locked` is true.

**Gap:** lock has only ever been frontend-initiated (TopBar → `wallet.lock()`), so nothing in the frontend listens to the `wallet:locked` event. A backend-initiated lock needs that listener.

## Architecture decision

**Backend-owned deadline, frontend activity pings.** The keystore lives in Go; the deadline that protects it must too. A watchdog in `WalletService` locks when `now − lastActivity > timeout`. The frontend reports genuine user input via a throttled `NoteActivity()` binding. This fails **closed**: a hung or crashed WebView stops pinging and the wallet locks anyway. (Rejected: a frontend-owned JS idle timer — fails open, and puts a security deadline on the untrusted side of the binding boundary.)

## Design

### Config

- `Settings` (app/dto.go) gains `AutoLockMinutes int` with json tag `autoLockMinutes`. Semantics: `0` = Never; valid values `{0, 1, 5, 15, 30}`.
- Migration: existing `settings.json` files lack the field. Since Go unmarshals a missing int as 0 — which must mean "Never" only when *chosen* — the field is a `*int` in the persisted struct OR migration uses a sentinel: **decision: pointer-free approach** — `migrateSettings` cannot distinguish "absent" from "explicit 0" on a plain int, so the field is persisted as `AutoLockMinutes *int`; `migrateSettings` sets it to `ptr(5)` when nil. The DTO surface to the frontend stays a plain number (nil never survives migration).
- Setter lives on **WalletService** (not ConfigService), because it must also update the live watchdog: `SetAutoLockMinutes(m int) error` — validates against the preset set, persists via `w.config.updateSettings`, and updates the cached live timeout atomically. `GetSettings` still exposes the value for the Settings UI to display.

### Backend (app/wallet_service.go)

- New state on `WalletService`: `lastActivity` timestamp and `autoLockMinutes` cache (both guarded — own small mutex or atomics; must not take `w.mu` from the ticker except when actually locking).
- `NoteActivity()` — new bound method: updates `lastActivity`; no-op while locked. Never errors.
- Watchdog goroutine: started on successful `Unlock` (after the keystore is installed), stopped by `Lock()` (including manual lock). Ticks every 15 seconds; on each tick, if `autoLockMinutes > 0` and `time.Since(lastActivity) > timeout`, it calls `w.Lock()`. Worst-case overshoot is timeout + one tick (~15s) — acceptable. `autoLockMinutes == 0` (Never): the ticker keeps running but never fires a lock (simpler than start/stop churn on setting changes; the tick is trivially cheap).
- `Unlock` initializes `lastActivity = now` and loads `autoLockMinutes` from settings.
- Timeout changes apply immediately: the ticker reads the cached value each tick.
- Idempotency: `Lock()` is already safe to call when locked; a race between manual lock and the watchdog is harmless.

### Frontend

- **`wallet:locked` listener** (the missing piece): registered once in the wallet store (same pattern as `node.initEvents`), wired from AppShell. On event: if already locked, no-op; else set `locked = true`, perform the same local session teardown the manual `lock()` action does (whatever cleanup it already runs besides calling `W.Lock`), and `router.push({ name: 'unlock' })` — the router guard alone only evaluates on navigation, so the push is explicit.
- **Activity capture** in AppShell (mounted only while unlocked): `window` listeners for `pointerdown`, `keydown`, `wheel` — passive, capture phase — throttled so `NoteActivity()` fires at most once per 15 seconds. Removed on unmount. No mousemove (too chatty; pointerdown/keydown/wheel cover genuine interaction).
- **Settings UI**: an "Auto-lock" row in the Security section — `<select>` with options 1 / 5 / 15 / 30 minutes / Never, bound to the persisted value from `GetSettings`, persisting via the new `SetAutoLockMinutes` binding (targeted-setter pattern, like Show Governance / auto-receive).

### Edge behavior (deliberate)

- An untouched confirm dialog **does** auto-lock — an unattended machine with a prepared transaction is the worst case. The existing session-generation bump discards the held block; the tx store's existing lock handling shows the flow as reset, not a phantom "awaiting".
- A long PoW after a recent click is not interrupted: the click reset the timer; PoW duration (seconds) is far below any preset.
- Lock while the user is on any authenticated route lands them on `/unlock`; unlocking returns them to `/dashboard` (existing behavior — no "return to previous route" scope creep).

### Events / contract additions

- New bound methods: `WalletService.NoteActivity()`, `WalletService.SetAutoLockMinutes(int)`.
- `Settings` DTO: `autoLockMinutes` field.
- No new events — `wallet:locked` already exists.

## Error handling

- `SetAutoLockMinutes` rejects values outside `{0, 1, 5, 15, 30}` with a plain error; the Settings UI only offers valid options, so users never see it.
- Persistence failure in the setter surfaces to the Settings UI like other setters; the live timeout still updates (in-memory behavior wins for the session).
- `NoteActivity` never errors; the frontend fire-and-forgets it.

## Testing

- **Go:** watchdog locks after expiry (inject a short timeout + short tick for the test — e.g. unexported fields settable in-package); `NoteActivity` defers expiry; `0` never locks; `SetAutoLockMinutes` validates and persists; migration defaults absent → 5; explicit 0 survives round-trip; manual `Lock` stops the watchdog (no second lock later).
- **Frontend:** `wallet:locked` event flips `locked` and routes to `/unlock` (idempotent when already locked); AppShell registers/unregisters activity listeners and throttles `NoteActivity` calls; Settings dropdown renders current value and persists changes.
- Full gates as usual: `GOWORK=off GOTOOLCHAIN=auto go test ./...`, `go vet`, `pnpm run typecheck`, `pnpm test`.

## Out of scope

- OS-level triggers (screen lock / sleep detection) — could be a later enhancement.
- "Return to previous route after re-unlock."
- Countdown warnings/toasts before locking.
- Changing what `Lock()` itself does — the feature only adds a new trigger for the existing mechanism.
