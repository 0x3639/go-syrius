# Governance Kill Switch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Disable the wallet's governance feature end-to-end (UI + backend) behind a single Go flag until the SDK's final governance implementation lands, leaving all governance code intact and tested.

**Architecture:** One package-level `var governanceFeatureEnabled = false` in `app/` gates every governance-bound method (checked first in the existing `requireTestnet()` policy plus explicit checks in the read methods). A new read-only binding `ConfigService.IsGovernanceFeatureEnabled()` surfaces the flag to the frontend, where `ui.governanceAllowed` AND-s it in — hiding the Sidebar tab, the NetworkPage panel, and the Settings toggle through gates that already exist. Spec: `docs/superpowers/specs/2026-07-19-governance-disable-design.md`.

**Tech Stack:** Go (Wails v2 bound services), Vue 3 + TypeScript + Pinia, vitest, go test.

## Global Constraints

- Every Go/wails command MUST be prefixed `GOWORK=off GOTOOLCHAIN=auto` (parent `go.work` references a missing module).
- Frontend commands run in `frontend/` with pnpm 10.17.1.
- The disabled error message is exactly: `governance is temporarily disabled pending an SDK update`.
- The flag is a `var` (not `const`) so tests can flip it; it must NEVER be persisted to `settings.json` or be user-settable.
- Do not remove or refactor any governance code; this change only gates it.
- `wails generate module` churns `frontend/wailsjs/runtime/*` — always revert that churn (`git checkout -- frontend/wailsjs/runtime/`); commit only `frontend/wailsjs/go/app/*` + `models.ts` changes, and check `git diff --check` for trailing whitespace in generated files.
- Commit messages end with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`. If a commit hangs on GPG signing, ask the user to re-warm the agent (`! echo x | gpg --clearsign >/dev/null`).

---

### Task 1: Go kill switch — flag, gate, and tests

**Files:**
- Create: `app/governance_disabled_test.go`
- Modify: `app/nom_governance.go` (flag + error + helper + `requireTestnet` + `GetActions` + `GetAction`)
- Modify: `app/governance_propose.go:600-602` (`GetProposeKinds`)
- Modify: `app/nom_governance_test.go`, `app/governance_propose_test.go` (enable the flag in existing suites)

**Interfaces:**
- Consumes: existing `newNomService(newTestNode(t), newTestWalletService(t), nil)` test constructor; existing `requireTestnet()` policy (already called by `PrepareGovernanceVote`, `PrepareExecuteAction`, `PrepareProposeAction`, and the sign-time `policy:` re-checks).
- Produces: `var governanceFeatureEnabled bool` (default `false`), `var errGovernanceDisabled error`, `func (s *NomService) requireGovernanceEnabled() error`, and test helper `func enableGovernance(t *testing.T)` — Tasks 2+ rely on these exact names.

- [ ] **Step 1: Write the failing test**

Create `app/governance_disabled_test.go`:

```go
package app

import "testing"

// enableGovernance flips the temporary governance kill switch on for one test
// and restores the disabled default afterwards, so the intact governance code
// keeps its coverage while the feature is off. Tests in this package run
// sequentially (no t.Parallel here), so mutating the package var is safe.
func enableGovernance(t *testing.T) {
	t.Helper()
	governanceFeatureEnabled = true
	t.Cleanup(func() { governanceFeatureEnabled = false })
}

// With the kill switch at its shipped default (false), every governance-bound
// method must return errGovernanceDisabled before any validation or node use —
// reads included — so the feature is unreachable even from devtools.
func TestGovernanceDisabled_AllBoundMethodsBlocked(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	if _, err := s.GetActions(0, 20); err != errGovernanceDisabled {
		t.Fatalf("GetActions: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.GetAction(valid); err != errGovernanceDisabled {
		t.Fatalf("GetAction: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.GetProposeKinds(); err != errGovernanceDisabled {
		t.Fatalf("GetProposeKinds: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.PrepareGovernanceVote(valid, "P1", 0); err != errGovernanceDisabled {
		t.Fatalf("PrepareGovernanceVote: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.PrepareExecuteAction(valid); err != errGovernanceDisabled {
		t.Fatalf("PrepareExecuteAction: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.PrepareProposeAction("Act", "d", "https://zenon.org", "spork.create", map[string]string{"name": "S", "description": "d"}); err != errGovernanceDisabled {
		t.Fatalf("PrepareProposeAction: want errGovernanceDisabled, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run TestGovernanceDisabled -v`
Expected: FAIL to compile — `undefined: governanceFeatureEnabled` / `undefined: errGovernanceDisabled`.

- [ ] **Step 3: Implement the flag and gate**

In `app/nom_governance.go`, directly above the existing `errGovernanceMainnet` declaration (line 43), add:

```go
// governanceFeatureEnabled is a TEMPORARY kill switch: it gates ALL governance
// functionality (reads and writes) until the SDK's final governance
// implementation lands. To re-enable: re-pin the updated znn-sdk-go, adapt the
// governance code to its final API, and flip this to true — nothing else
// changes (the Settings opt-in and testnet-only gates resume as before). A
// var, not a const, so the governance test suites can enable the feature
// under test (see enableGovernance in governance_disabled_test.go).
var governanceFeatureEnabled = false

// errGovernanceDisabled is the kill-switch error. Users never see it — the UI
// is hidden while disabled — it is the devtools/defense-in-depth backstop.
var errGovernanceDisabled = errors.New("governance is temporarily disabled pending an SDK update")

// requireGovernanceEnabled blocks every governance entry point while the
// feature is disabled. Checked first in requireTestnet (covering all write
// paths and their sign-time policy re-checks) and explicitly in the reads.
func (s *NomService) requireGovernanceEnabled() error {
	if !governanceFeatureEnabled {
		return errGovernanceDisabled
	}
	return nil
}
```

Change `requireTestnet` (same file) to check it first:

```go
func (s *NomService) requireTestnet() error {
	if err := s.requireGovernanceEnabled(); err != nil {
		return err
	}
	if s.node.currentChainID() == mainnetChainID {
		return errGovernanceMainnet
	}
	return nil
}
```

Add the check as the FIRST statement of `GetActions` and `GetAction` in the same file:

```go
func (s *NomService) GetActions(pageIndex, pageSize uint32) (ActionListDTO, error) {
	if err := s.requireGovernanceEnabled(); err != nil {
		return ActionListDTO{}, err
	}
	if pageSize == 0 || pageSize > 50 {
```

```go
func (s *NomService) GetAction(id string) (ActionDTO, error) {
	if err := s.requireGovernanceEnabled(); err != nil {
		return ActionDTO{}, err
	}
	h, err := parseHash(id)
```

In `app/governance_propose.go`, change `GetProposeKinds` (line 600):

```go
func (s *NomService) GetProposeKinds() ([]ProposeKindDTO, error) {
	if err := s.requireGovernanceEnabled(); err != nil {
		return nil, err
	}
	return proposeKinds(), nil
}
```

- [ ] **Step 4: Run the new test to verify it passes**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run TestGovernanceDisabled -v`
Expected: PASS

- [ ] **Step 5: Re-enable the flag in the existing governance suites**

The existing suites exercise the (now gated) bound methods and will fail with the disabled error. Add `enableGovernance(t)` as the FIRST statement of exactly these tests:

In `app/nom_governance_test.go`: `TestGovernancePrepares_BlockedOnMainnet` (line 50), `TestGetActions_NotConnected` (72), `TestGetAction_BadId` (79), `TestPrepareGovernanceVote_Validation` (86), `TestPrepareExecuteAction_Validation` (103). Example:

```go
func TestGovernancePrepares_BlockedOnMainnet(t *testing.T) {
	enableGovernance(t)
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	...
```

In `app/governance_propose_test.go`: `TestGetProposeKinds_HasSporkNotCustom` (line 12), `TestPrepareProposeAction_Validation` (64). The other tests in these files call unexported helpers (`actionDTO`, `buildProposalPayload`, `proposeKinds`) directly — not gated, leave them untouched.

- [ ] **Step 6: Run the full backend gate**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./... && GOWORK=off GOTOOLCHAIN=auto go vet ./...`
Expected: all packages PASS, vet clean. (`app/tx_service_test.go`, `tx_effect_test.go`, `node_service_test.go`, `config_service_test.go` touch governance only via unexported helpers or synthetic policies — they must pass unchanged.)

- [ ] **Step 7: Commit**

```bash
git add app/nom_governance.go app/governance_propose.go app/governance_disabled_test.go app/nom_governance_test.go app/governance_propose_test.go
git commit -m "feat: gate all governance-bound methods behind temporary kill switch

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: `ConfigService.IsGovernanceFeatureEnabled` binding

**Files:**
- Modify: `app/config_service.go` (add method after `SetShowGovernance`, line ~201)
- Modify: `app/governance_disabled_test.go` (append test)
- Regenerate: `frontend/wailsjs/go/app/ConfigService.d.ts`, `ConfigService.js` (via wails)

**Interfaces:**
- Consumes: `governanceFeatureEnabled` and `enableGovernance(t)` from Task 1.
- Produces: bound method `func (c *ConfigService) IsGovernanceFeatureEnabled() bool`; frontend binding `IsGovernanceFeatureEnabled(): Promise<boolean>` in `frontend/wailsjs/go/app/ConfigService` — Task 3 imports it.

- [ ] **Step 1: Write the failing test**

Append to `app/governance_disabled_test.go`:

```go
// The frontend learns the kill switch via this read-only binding — it is
// deliberately NOT part of Settings, which round-trips to settings.json.
func TestGovernanceDisabled_ConfigReportsFlag(t *testing.T) {
	c := &ConfigService{}
	if c.IsGovernanceFeatureEnabled() {
		t.Fatal("flag must default to disabled")
	}
	enableGovernance(t)
	if !c.IsGovernanceFeatureEnabled() {
		t.Fatal("flag must report enabled when flipped")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run TestGovernanceDisabled_ConfigReportsFlag -v`
Expected: FAIL to compile — `c.IsGovernanceFeatureEnabled undefined`.

- [ ] **Step 3: Implement the method**

In `app/config_service.go`, after `SetShowGovernance` (ends line ~201), add:

```go
// IsGovernanceFeatureEnabled reports the temporary governance kill switch
// (governanceFeatureEnabled in nom_governance.go). Read-only and deliberately
// NOT part of Settings: compile-time state must never persist to — or be
// resurrected from — settings.json.
func (c *ConfigService) IsGovernanceFeatureEnabled() bool {
	return governanceFeatureEnabled
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run TestGovernanceDisabled -v`
Expected: PASS (both tests).

- [ ] **Step 5: Regenerate the wails bindings and revert runtime churn**

```bash
GOWORK=off GOTOOLCHAIN=auto "$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/
git diff --check
```

Expected: `frontend/wailsjs/go/app/ConfigService.d.ts` gains `export function IsGovernanceFeatureEnabled():Promise<boolean>;` and `ConfigService.js` the matching export; `git diff --check` reports nothing (fix trailing whitespace in generated files if it does).

- [ ] **Step 6: Commit**

```bash
git add app/config_service.go app/governance_disabled_test.go frontend/wailsjs/go/app/
git commit -m "feat: expose governance kill switch via read-only ConfigService binding

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: ui store gate + Sidebar/NetworkPage test updates

**Files:**
- Modify: `frontend/src/stores/ui.ts`
- Modify: `frontend/src/stores/ui.test.ts`
- Modify: `frontend/src/components/Sidebar.test.ts:23-31`
- Modify: `frontend/src/views/NetworkPage.test.ts:38,60,68,85,102`

**Interfaces:**
- Consumes: `IsGovernanceFeatureEnabled` binding from Task 2; existing `useNodeStore().chainId`.
- Produces: ui store state `governanceFeatureEnabled: boolean` (default `false`) and the tightened getter `governanceAllowed` — Task 4's `v-if` reads `ui.governanceFeatureEnabled` directly.

All commands in this task run in `frontend/`.

- [ ] **Step 1: Write the failing tests**

In `frontend/src/stores/ui.test.ts`, replace the hoisted mock block (lines 4–8) with:

```ts
const { GetSettings, SetShowGovernance, IsGovernanceFeatureEnabled } = vi.hoisted(() => ({
  GetSettings: vi.fn(),
  SetShowGovernance: vi.fn(),
  IsGovernanceFeatureEnabled: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetShowGovernance, IsGovernanceFeatureEnabled }))
```

Add `import { useNodeStore } from './node'` after the `useUiStore` import, add `IsGovernanceFeatureEnabled.mockReset()` in the `beforeEach`, and append these tests inside the `describe`:

```ts
  // TEMPORARY kill switch: governance is fully disabled pending an SDK update.
  it('governanceAllowed is false while the feature flag is off, even opted-in on testnet', () => {
    const s = useUiStore()
    s.showGovernance = true
    useNodeStore().chainId = 2
    expect(s.governanceAllowed).toBe(false)
  })

  it('governanceAllowed requires flag + opt-in + testnet', () => {
    const s = useUiStore()
    s.governanceFeatureEnabled = true
    s.showGovernance = true
    useNodeStore().chainId = 2
    expect(s.governanceAllowed).toBe(true)
  })

  it('init loads the kill-switch flag from the binding (fail-closed)', async () => {
    GetSettings.mockResolvedValue({})
    IsGovernanceFeatureEnabled.mockResolvedValue(true)
    const s = useUiStore()
    await s.init()
    expect(s.governanceFeatureEnabled).toBe(true)
  })

  it('init keeps the flag false when the binding fails (fail-closed)', async () => {
    GetSettings.mockResolvedValue({})
    IsGovernanceFeatureEnabled.mockRejectedValue(new Error('locked'))
    const s = useUiStore()
    await s.init()
    expect(s.governanceFeatureEnabled).toBe(false)
  })
```

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `pnpm test src/stores/ui.test.ts`
Expected: the two `governanceAllowed` tests FAIL (`governanceAllowed` is `true` without the flag; `governanceFeatureEnabled` is `undefined`); the pre-existing tests still pass.

- [ ] **Step 3: Implement the store changes**

In `frontend/src/stores/ui.ts`:

State — add below `showGovernance: false,` (line 9):

```ts
    // TEMPORARY kill switch mirror (ConfigService.IsGovernanceFeatureEnabled):
    // governance is fully disabled pending an SDK update. Fails CLOSED.
    governanceFeatureEnabled: false,
```

Getter — replace the body of `governanceAllowed` (line 22):

```ts
      return this.governanceFeatureEnabled && this.showGovernance && useNodeStore().chainId > 1
```

`init()` — append a second try/catch after the existing `showGovernance` load (lines 47–51):

```ts
      try {
        this.governanceFeatureEnabled = (await Cfg.IsGovernanceFeatureEnabled()) === true
      } catch {
        /* keep false — fail closed */
      }
```

- [ ] **Step 4: Run the store tests to verify they pass**

Run: `pnpm test src/stores/ui.test.ts`
Expected: PASS (all).

- [ ] **Step 5: Update the consuming component tests**

`frontend/src/components/Sidebar.test.ts` — in `'hides Governance unless opted in on testnet'` (lines 23–31), assert the flag now also gates the tab:

```ts
  it('hides Governance unless the feature flag, opt-in, and testnet all hold', async () => {
    const w = mountSidebar()
    expect(w.text()).not.toContain('Governance')
    const ui = useUiStore(); const node = useNodeStore()
    ui.showGovernance = true; node.chainId = 2
    await w.vm.$nextTick()
    // kill switch off → still hidden even when opted in on testnet
    expect(w.text()).not.toContain('Governance')
    ui.governanceFeatureEnabled = true
    await w.vm.$nextTick()
    expect(w.text()).toContain('Governance')
  })
```

`frontend/src/views/NetworkPage.test.ts` — at lines 38, 60, 68, 85, 102 (every `ui.showGovernance = true`; NOT the `false` case at line 52), add the flag:

```ts
    const ui = useUiStore(); ui.governanceFeatureEnabled = true; ui.showGovernance = true
```

- [ ] **Step 6: Run the component tests to verify they pass**

Run: `pnpm test src/components/Sidebar.test.ts src/views/NetworkPage.test.ts`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/stores/ui.ts frontend/src/stores/ui.test.ts frontend/src/components/Sidebar.test.ts frontend/src/views/NetworkPage.test.ts
git commit -m "feat: gate governanceAllowed behind the kill-switch flag in the ui store

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: Hide the Settings toggle

**Files:**
- Modify: `frontend/src/views/Settings.vue:307` (the "Testnet features" section)
- Modify: `frontend/src/views/Settings.test.ts`

**Interfaces:**
- Consumes: `ui.governanceFeatureEnabled` state from Task 3; `IsGovernanceFeatureEnabled` mock name from Task 2's binding.
- Produces: nothing downstream.

All commands in this task run in `frontend/`.

- [ ] **Step 1: Write the failing tests**

In `frontend/src/views/Settings.test.ts`:

Add the hoisted mock — after line 22 (`const SetShowGovernance = ...`) add, and extend the factory on line 23:

```ts
const IsGovernanceFeatureEnabled = vi.hoisted(() => vi.fn().mockResolvedValue(false))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetChainID, SetAllowMainnetSend, SetShowGovernance, IsGovernanceFeatureEnabled }))
```

(Replace the existing `vi.mock('../../wailsjs/go/app/ConfigService', ...)` line — do not leave two mocks of the same module.)

Update the existing toggle test (line 150) to opt into the flag, and add a disabled-state test after it:

```ts
  it('toggling Show Governance persists via the targeted setter', async () => {
    IsGovernanceFeatureEnabled.mockResolvedValueOnce(true) // kill switch on for this test
    const w = mount(Settings)
    await flush() // onMounted: ui.init() loads showGovernance + the feature flag

    const cb = w.find('input[aria-label="show governance"]')
    expect((cb.element as HTMLInputElement).checked).toBe(false)

    await cb.setValue(true)
    await flush()

    expect(SetShowGovernance).toHaveBeenCalledWith(true)
  })

  it('hides the Show Governance toggle entirely while the kill switch is off', async () => {
    const w = mount(Settings)
    await flush()
    expect(w.find('input[aria-label="show governance"]').exists()).toBe(false)
    expect(w.text()).not.toContain('Show Governance')
  })
```

- [ ] **Step 2: Run tests to verify the new one fails**

Run: `pnpm test src/views/Settings.test.ts`
Expected: `hides the Show Governance toggle...` FAILS (toggle still renders); the rest pass.

- [ ] **Step 3: Implement the template change**

In `frontend/src/views/Settings.vue` line 307, add the `v-if` to the whole "Testnet features" section (it contains only the governance toggle):

```html
    <section v-if="ui.governanceFeatureEnabled" class="rounded-xl border border-border bg-card p-5 space-y-2">
```

No other changes — the checkbox, copy, and `setShowGovernance` wiring stay intact, and the persisted `showGovernance` preference is untouched for when the feature returns.

- [ ] **Step 4: Run tests to verify they pass**

Run: `pnpm test src/views/Settings.test.ts`
Expected: PASS (all).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Settings.vue frontend/src/views/Settings.test.ts
git commit -m "feat: hide the Show Governance settings toggle while the kill switch is off

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: Full verification gates

**Files:** none created/modified (fix-forward only if a gate fails).

**Interfaces:** consumes everything above; produces the green build the branch ships with.

- [ ] **Step 1: Backend gates**

Run: `GOWORK=off GOTOOLCHAIN=auto go build ./... && GOWORK=off GOTOOLCHAIN=auto go vet ./... && GOWORK=off GOTOOLCHAIN=auto go test ./...`
Expected: all PASS, no output from vet.

- [ ] **Step 2: Frontend gates** (in `frontend/`)

Run: `pnpm run typecheck && pnpm test && pnpm run build`
Expected: vue-tsc 0 errors; vitest all green; Vite build succeeds.

- [ ] **Step 3: Commit any stragglers**

Run: `git status --short` — expected clean (excluding pre-existing untracked `.agents/`, `.claude/`). If generated-binding or lockfile drift appears, inspect, stage only intended files, and commit with an explanatory message.
