# Governance kill switch — design

**Date:** 2026-07-19
**Status:** Approved pending user review
**Goal:** Disable the wallet's governance feature end-to-end (UI + backend) until the SDK supports the final governance implementation. All governance code stays intact and tested; re-enabling is a one-line change.

## Background

Governance shipped to `main` (3ae7fae) behind two gates:

- **UI:** the Sidebar tab, NetworkPage panel, and Settings toggle are driven by the single getter `ui.governanceAllowed` = `showGovernance && chainId > 1` (Settings opt-in AND testnet).
- **Backend:** every governance *write* path (`PrepareGovernanceVote`, `PrepareExecuteAction`, `PrepareProposeAction`, plus the `policy` re-check at sign time) routes through `NomService.requireTestnet()` (`app/nom_governance.go`), which hard-blocks mainnet.

The SDK's governance API is being reworked; until the wallet re-pins the updated SDK, the feature must be unreachable — including direct WebView/devtools calls — while leaving the implementation and its test suites in place.

## Design

### Backend (Go, `app/`)

- Add a package-level flag:

  ```go
  // governanceFeatureEnabled gates ALL governance functionality (reads and
  // writes) while the SDK's final governance implementation is pending.
  // Flip to true (and re-pin the updated SDK) to re-enable. A var, not a
  // const, so tests can enable the feature under test.
  var governanceFeatureEnabled = false
  ```

- Add `requireGovernanceEnabled()` returning
  `errors.New("governance is temporarily disabled pending an SDK update")` when the flag is false.
- Check it **first** in `requireTestnet()` — this covers every write path and the sign-time `policy` re-checks with no other call-site changes.
- Add an explicit `requireGovernanceEnabled()` check at the top of the governance **read** methods: `GetActions`, `GetAction`, `GetProposeKinds`.
- Net effect: every governance-bound method errors when disabled, even if invoked directly from devtools.

### Frontend contract

- New read-only bound method on `ConfigService`:

  ```go
  // IsGovernanceFeatureEnabled reports the compile-time governance kill
  // switch. Not persisted — deliberately NOT part of Settings, which
  // round-trips to settings.json.
  func (c *ConfigService) IsGovernanceFeatureEnabled() bool
  ```

  (Refinement over the discussed `GetSettings` field: `GetSettings` returns the persisted `Settings` struct verbatim, so a DTO field would write transient compile-time state to disk. A separate method keeps persisted config and feature flags apart.)
- The generated `frontend/wailsjs/` bindings pick up the new method on the next `wails dev` / `wails build` (or `wails generate module`).
- No changes to the persisted `Settings` struct or `settings.json`. The user's `showGovernance` preference remains stored and untouched, so re-enabling restores each user's prior choice.

### Frontend (Vue, `frontend/src/`)

- `stores/ui.ts`: add state `governanceFeatureEnabled: false`, loaded in `init()` via the new binding (failure ⇒ stays `false`, fail-closed). The getter becomes:

  ```ts
  governanceAllowed(): boolean {
    return this.governanceFeatureEnabled && this.showGovernance && useNodeStore().chainId > 1
  }
  ```

  Sidebar tab and the NetworkPage governance-panel guard already consume this getter, so both vanish with no further changes.
- `views/Settings.vue`: wrap the "Show Governance" toggle block (checkbox + description) in `v-if="ui.governanceFeatureEnabled"` — hidden entirely while disabled; no dead controls.

### Re-enabling later

1. Re-pin the updated `znn-sdk-go` in `go.mod` and adapt governance code to the final SDK API.
2. Flip `governanceFeatureEnabled = true`.

Nothing else changes; all existing gates (Settings opt-in, testnet-only) resume as before.

## Error handling

The disabled error is a plain user-readable message. Since the UI is hidden, users should never see it — it is the devtools / defense-in-depth backstop. No new event types, no new error plumbing.

## Testing

- **Existing suites stay green and intact:** Go governance tests set `governanceFeatureEnabled = true` in their setup (restoring it after, e.g. `t.Cleanup`); vitest suites for the governance panels/store set the ui store's `governanceFeatureEnabled = true` in test setup. The feature code remains fully tested.
- **New disabled-state tests:**
  - Go: with the flag false (default), each bound governance method (`GetActions`, `GetAction`, `GetProposeKinds`, `PrepareGovernanceVote`, `PrepareExecuteAction`, `PrepareProposeAction`) returns the disabled error.
  - ui store: `governanceAllowed` is `false` even with `showGovernance = true` and testnet `chainId`, when `governanceFeatureEnabled` is `false`.
  - Settings view: the Show Governance toggle is absent when the flag is off, present when on.
  - Sidebar: no Governance entry when the flag is off (likely already covered by existing `governanceAllowed`-driven tests; verify).
- Full gates as usual: `GOWORK=off GOTOOLCHAIN=auto go test ./...`, `go vet`, `pnpm run typecheck`, `pnpm test`.

## Out of scope

- Removing or refactoring any governance code.
- SDK re-pin / adapting to the final governance API (happens at re-enable time).
- `docs/` / README updates beyond this spec (the README governance section may later gain a "temporarily disabled" note if desired, but it is not part of this change).
