# Governance Module — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Governance NoM tab to the wallet — browse governance actions, vote on open actions (pillar-gated), and execute approved actions — built entirely against the current pinned SDK with no go-zenon/SDK changes.

**Architecture:** Mirror the existing Accelerator-Z module. New `NomService` methods in `app/nom_governance.go` wrap the SDK `GovernanceApi`; new DTOs in `app/dto.go`. Frontend adds a `governance` Pinia store, a pure vote-math `lib/governance.ts`, and a `GovernancePanel` (Vote / Actions sub-tabs) registered as the 8th NoM tab.

**Tech Stack:** Go 1.25.11 + Wails v2, `znn-sdk-go` `GovernanceApi`; Vue 3 + TS + Pinia + nom-ui; vitest + @vue/test-utils.

## Global Constraints

- All local Go/Wails commands run with `GOWORK=off GOTOOLCHAIN=auto` prefixed (parent `go.work` hazard). Frontend uses pnpm 10.17.1 in `frontend/`.
- SDK pinned at `github.com/0x3639/znn-sdk-go v0.1.20-0.20260628104800-69a09c93f3c3`; go-zenon via `replace … github.com/0x3639/go-zenon v0.0.0-20260615011802-81c247408859`. **Do not change these pins in Phase 1.**
- Binding invariant: the frontend sends intent only; every state-changing Go method re-validates inputs and never trusts frontend validation. Confirm-what-you-sign: previews derive from the built block.
- Vote values: `Yes=0, No=1, Abstain=2` (`embedded.VoteYes/VoteNo/VoteAbstain`).
- Action statuses: `Voting=0, Approved=1, Rejected=2, NoDecision=3`; types `Spork=1, Normal=2`.
- Per-round vote math (abstain excluded from directional): `directional = yes+no`; quorum `directional*100 > numActivePillars*activePillarThreshold`; approved adds `yes*100 > directional*directionalThreshold`; rejected adds `no*100 > directional*directionalThreshold`.
- Wails bindings under `frontend/wailsjs/` are git-tracked and regenerated with `GOWORK=off wails generate module`.
- Everything tagged `[P2]` in the spec (`docs/superpowers/specs/2026-06-28-governance-module-design.md`) is OUT OF SCOPE here: no `GetPillarVotes`, no `GetVotableActionsForMyPillars`, no per-pillar "needs my vote", no top-bar badge.
- Pre-existing local failures unrelated to this work: `internal/compat` keystore-roundtrip and one `app` keystore test (`incorrect password`). Do not try to fix them; scope `go test` to `./app` or specific runs to avoid noise.

---

### Task 1: Backend DTOs + read methods (GetActions / GetAction)

**Files:**
- Create: `app/nom_governance.go`
- Modify: `app/dto.go` (append `ActionDTO`, `ActionListDTO`)
- Test: `app/nom_governance_test.go`

**Interfaces:**
- Consumes: existing `s.node.currentClient() *embedded.Client` (nil when disconnected); `voteBreakdownDTO(*embedded.VoteBreakdown) VoteBreakdownDTO`, `bigStr`, `parseHash(string) (types.Hash, error)` from `nom_accelerator.go`; `types.GovernanceContract`.
- Produces: `actionDTO(*embedded.Action) ActionDTO`; `func (s *NomService) GetActions(pageIndex, pageSize uint32) (ActionListDTO, error)`; `func (s *NomService) GetAction(id string) (ActionDTO, error)`; DTO types `ActionDTO`, `ActionListDTO`.

- [ ] **Step 1: Add the DTOs to `app/dto.go`**

Append after `ProjectListDTO`:

```go
// ActionDTO is one governance action with its current-round vote tally and the
// per-round thresholds the node computed for it. Reuses VoteBreakdownDTO.
type ActionDTO struct {
	Id                    string           `json:"id"`
	Owner                 string           `json:"owner"`
	Name                  string           `json:"name"`
	Description           string           `json:"description"`
	Url                   string           `json:"url"`
	Destination           string           `json:"destination"`
	Data                  string           `json:"data"` // base64 ABI call data
	Type                  int              `json:"type"` // 1 Spork, 2 Normal
	Round                 int              `json:"round"`
	Status                int              `json:"status"` // 0 Voting,1 Approved,2 Rejected,3 NoDecision
	Executed              bool             `json:"executed"`
	Expired               bool             `json:"expired"`
	CreationTimestamp     int64            `json:"creationTimestamp"`
	RoundStartTimestamp   int64            `json:"roundStartTimestamp"`
	ActivePillarThreshold uint32           `json:"activePillarThreshold"`
	DirectionalThreshold  uint32           `json:"directionalThreshold"`
	VotingPeriod          int64            `json:"votingPeriod"`
	Votes                 VoteBreakdownDTO `json:"votes"`
}

// ActionListDTO is one page of governance actions.
type ActionListDTO struct {
	Count int         `json:"count"`
	List  []ActionDTO `json:"list"`
}
```

- [ ] **Step 2: Write the failing test `app/nom_governance_test.go`**

```go
package app

import (
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestActionDTO_MapsFieldsAndVotes(t *testing.T) {
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	// The SDK client Action has FLAT fields (it is NOT the go-zenon server
	// Action that embeds *definition.ActionVariable).
	a := &embedded.Action{
		Id:                    id,
		Owner:                 types.GovernanceContract,
		Name:                  "Act",
		Description:           "desc",
		Url:                   "https://zenon.org",
		Destination:           types.SporkContract,
		Data:                  "AAEC",
		Type:                  1,
		Round:                 0,
		Status:                0,
		Executed:              false,
		Expired:               false,
		ActivePillarThreshold: 66,
		DirectionalThreshold:  50,
		VotingPeriod:          3888000,
		Votes:                 &embedded.VoteBreakdown{Total: 3, Yes: 2, No: 1},
	}
	d := actionDTO(a)
	if d.Id != id.String() || d.Destination != types.SporkContract.String() {
		t.Fatalf("hash/addr mapping wrong: %+v", d)
	}
	if d.Type != 1 || d.ActivePillarThreshold != 66 || d.DirectionalThreshold != 50 {
		t.Fatalf("type/threshold mapping wrong: %+v", d)
	}
	if d.Votes.Yes != 2 || d.Votes.No != 1 || d.Votes.Total != 3 {
		t.Fatalf("votes mapping wrong: %+v", d.Votes)
	}
}

func TestActionDTO_NilSafe(t *testing.T) {
	d := actionDTO(nil)
	if d.Id != "" || d.Votes.Total != 0 {
		t.Fatalf("nil action must map to zero DTO: %+v", d)
	}
}

func TestGetActions_NotConnected(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.GetActions(0, 20); err == nil || err.Error() != "not connected" {
		t.Fatalf("want not connected; got %v", err)
	}
}

func TestGetAction_BadId(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.GetAction("not-a-hash"); err == nil {
		t.Fatal("bad id must error")
	}
}
```

Note: `newTestNode(t)` and `newTestWalletService(t)` are the existing test helpers used throughout `nom_accelerator_test.go` (a disconnected node → `currentClient()` returns nil → "not connected"). The third `newNomService` arg (the `TxService`) is `nil` here because these tests never reach `prepareCall`.

- [ ] **Step 3: Run the test — verify it fails**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestActionDTO|TestGetAction' -v`
Expected: FAIL — `undefined: actionDTO`, `s.GetActions`, `s.GetAction`.

- [ ] **Step 4: Create `app/nom_governance.go`**

```go
package app

import (
	"errors"
	"fmt"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
)

// actionDTO maps an SDK governance Action (flat client struct: the on-chain
// fields plus the node's computed current-round fields) to the wire DTO.
// Nil-safe. Reuses voteBreakdownDTO from nom_accelerator.go.
func actionDTO(a *embedded.Action) ActionDTO {
	if a == nil {
		return ActionDTO{}
	}
	return ActionDTO{
		Id:                    a.Id.String(),
		Owner:                 a.Owner.String(),
		Name:                  a.Name,
		Description:           a.Description,
		Url:                   a.Url,
		Destination:           a.Destination.String(),
		Data:                  a.Data,
		Type:                  int(a.Type),
		Round:                 int(a.Round),
		Status:                int(a.Status),
		Executed:              a.Executed,
		Expired:               a.Expired,
		CreationTimestamp:     a.CreationTimestamp,
		RoundStartTimestamp:   a.RoundStartTimestamp,
		ActivePillarThreshold: a.ActivePillarThreshold,
		DirectionalThreshold:  a.DirectionalThreshold,
		VotingPeriod:          a.VotingPeriod,
		Votes:                 voteBreakdownDTO(a.Votes),
	}
}

// GetActions returns one page of governance actions (node ordering). pageSize is
// clamped to [1,50]. This is also the source the frontend Vote view filters to
// open actions in Phase 1.
func (s *NomService) GetActions(pageIndex, pageSize uint32) (ActionListDTO, error) {
	if pageSize == 0 || pageSize > 50 {
		pageSize = 50
	}
	client := s.node.currentClient()
	if client == nil {
		return ActionListDTO{}, errors.New("not connected")
	}
	list, err := client.GovernanceApi.GetAllActions(pageIndex, pageSize)
	if err != nil {
		return ActionListDTO{}, err
	}
	out := ActionListDTO{Count: list.Count, List: make([]ActionDTO, 0, len(list.List))}
	for _, a := range list.List {
		out.List = append(out.List, actionDTO(a))
	}
	return out, nil
}

// GetAction returns a single governance action by id.
func (s *NomService) GetAction(id string) (ActionDTO, error) {
	h, err := parseHash(id)
	if err != nil {
		return ActionDTO{}, fmt.Errorf("invalid action id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return ActionDTO{}, errors.New("not connected")
	}
	a, err := client.GovernanceApi.GetActionById(h)
	if err != nil {
		return ActionDTO{}, err
	}
	return actionDTO(a), nil
}
```

Note: Task 1 imports only `errors`, `fmt`, and the `embedded` alias — `types`/`strings` are NOT used yet (`go build` errors on unused imports), so they are added in Task 2.

- [ ] **Step 5: Run the tests — verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestActionDTO|TestGetAction|TestGetActions' -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Vet + build**

Run: `GOWORK=off GOTOOLCHAIN=auto go vet ./app/... && GOWORK=off GOTOOLCHAIN=auto go build ./...`
Expected: no errors (ignore the gopsutil/IOKit cgo deprecation warning).

- [ ] **Step 7: Commit**

```bash
git add app/nom_governance.go app/dto.go app/nom_governance_test.go
git commit -S -m "feat(governance): backend ActionDTO + GetActions/GetAction reads"
```

---

### Task 2: Backend prepare builders (Vote + Execute)

**Files:**
- Modify: `app/nom_governance.go`
- Test: `app/nom_governance_test.go`

**Interfaces:**
- Consumes: `s.tx.prepareCall(template *nom.AccountBlock, expect callExpect, summary string) (CallPreview, error)`; `callExpect{to,zts,amount,data}`; `embedded.VoteYes/VoteNo/VoteAbstain`; `client.GovernanceApi.VoteByName(id types.Hash, pillarName string, vote uint8) *nom.AccountBlock`; `client.GovernanceApi.ExecuteAction(id types.Hash) *nom.AccountBlock`; `client.GovernanceApi.GetActionById`.
- Produces: `func (s *NomService) PrepareGovernanceVote(id, pillarName string, vote uint8) (CallPreview, error)`; `func (s *NomService) PrepareExecuteAction(id string) (CallPreview, error)`.

- [ ] **Step 1: Write the failing tests (append to `app/nom_governance_test.go`)**

```go
func TestPrepareGovernanceVote_Validation(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareGovernanceVote("bad-id", "P1", 0); err == nil {
		t.Fatal("bad id must error")
	}
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.PrepareGovernanceVote(valid, "  ", 0); err == nil {
		t.Fatal("empty pillar must error")
	}
	if _, err := s.PrepareGovernanceVote(valid, "P1", 9); err == nil {
		t.Fatal("invalid vote value must error")
	}
	if _, err := s.PrepareGovernanceVote(valid, "P1", 0); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid vote should hit not-connected; got %v", err)
	}
}

func TestPrepareExecuteAction_Validation(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareExecuteAction("bad-id"); err == nil {
		t.Fatal("bad id must error")
	}
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.PrepareExecuteAction(valid); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid execute should hit not-connected; got %v", err)
	}
}

// Build the SDK templates directly to catch a pack-panic at construction
// (the lesson from the v0.1.19 UpdatePhase regression).
func TestGovernanceWriteTemplates_NoPanic(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	h := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	vote := api.VoteByName(h, "P1", embedded.VoteYes)
	if vote.ToAddress != types.GovernanceContract || vote.TokenStandard != types.ZnnTokenStandard || vote.Amount.Sign() != 0 {
		t.Fatalf("vote template wrong: %+v", vote)
	}
	exec := api.ExecuteAction(h)
	if exec.ToAddress != types.GovernanceContract || exec.Amount.Sign() != 0 {
		t.Fatalf("execute template wrong: %+v", exec)
	}
}
```

- [ ] **Step 2: Run the tests — verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestPrepareGovernance|TestPrepareExecute|TestGovernanceWriteTemplates' -v`
Expected: FAIL — `s.PrepareGovernanceVote`, `s.PrepareExecuteAction` undefined.

- [ ] **Step 3: Add the builders to `app/nom_governance.go`**

Extend Task 1's import block to add `"strings"` and `"github.com/zenon-network/go-zenon/common/types"` (the `embedded` alias is already there from Task 1). The block becomes: `"errors"`, `"fmt"`, `"strings"`, `embedded "github.com/0x3639/znn-sdk-go/api/embedded"`, `"github.com/zenon-network/go-zenon/common/types"`. Then add:

```go
// PrepareGovernanceVote builds a VoteByName template for one of the active
// address's Pillars. vote MUST be embedded.VoteYes/VoteNo/VoteAbstain. Field
// validation runs before any node use; Pillar ownership is enforced on-chain.
func (s *NomService) PrepareGovernanceVote(id, pillarName string, vote uint8) (CallPreview, error) {
	h, err := parseHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid action id: %w", err)
	}
	name := strings.TrimSpace(pillarName)
	if name == "" {
		return CallPreview{}, errors.New("pillar name is required")
	}
	if vote != embedded.VoteYes && vote != embedded.VoteNo && vote != embedded.VoteAbstain {
		return CallPreview{}, errors.New("vote must be yes, no, or abstain")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.GovernanceApi.VoteByName(h, name, vote)
	label := map[uint8]string{embedded.VoteYes: "yes", embedded.VoteNo: "no", embedded.VoteAbstain: "abstain"}[vote]
	return s.tx.prepareCall(template,
		callExpect{to: types.GovernanceContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Vote %s on governance action %s as %s", label, id, name))
}

// PrepareExecuteAction builds an ExecuteAction template. It first fetches the
// action so the confirm summary names the action and the contract it will call
// (confirm-what-you-sign: the on-chain ToAddress is the governance contract, so
// the real effect — the destination call — is surfaced in the summary).
func (s *NomService) PrepareExecuteAction(id string) (CallPreview, error) {
	h, err := parseHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid action id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	a, err := client.GovernanceApi.GetActionById(h)
	if err != nil {
		return CallPreview{}, err
	}
	d := actionDTO(a)
	template := client.GovernanceApi.ExecuteAction(h)
	return s.tx.prepareCall(template,
		callExpect{to: types.GovernanceContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Execute governance action %q (calls %s)", d.Name, d.Destination))
}
```

- [ ] **Step 4: Run the tests — verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestPrepareGovernance|TestPrepareExecute|TestGovernanceWriteTemplates' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Full app test + vet + build**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run Governance && GOWORK=off GOTOOLCHAIN=auto go vet ./app/... && GOWORK=off GOTOOLCHAIN=auto go build ./...`
Expected: PASS / no errors.

- [ ] **Step 6: Commit**

```bash
git add app/nom_governance.go app/nom_governance_test.go
git commit -S -m "feat(governance): PrepareGovernanceVote + PrepareExecuteAction builders"
```

---

### Task 3: Regenerate Wails bindings

**Files:**
- Modify (generated): `frontend/wailsjs/go/app/NomService.d.ts`, `frontend/wailsjs/go/app/NomService.js`, `frontend/wailsjs/go/models.ts`

**Interfaces:**
- Produces (TS): `GetActions(arg1:number,arg2:number):Promise<app.ActionListDTO>`, `GetAction(arg1:string):Promise<app.ActionDTO>`, `PrepareGovernanceVote(arg1:string,arg2:string,arg3:number):Promise<app.CallPreview>`, `PrepareExecuteAction(arg1:string):Promise<app.CallPreview>`; `app.ActionDTO`, `app.ActionListDTO` in `models.ts`.

- [ ] **Step 1: Generate bindings**

Run (from repo root; `wails` is NOT on PATH): `GOWORK=off GOTOOLCHAIN=auto ~/go/bin/wails generate module`
Expected: regenerates `frontend/wailsjs/...`. **Keep only the wanted churn:** `NomService.d.ts`, `NomService.js`, and `models.ts`. If the wails runtime files change (`frontend/wailsjs/runtime/*`), revert them: `git checkout HEAD -- frontend/wailsjs/runtime`. If `wails generate module` fails in this environment, fall back to hand-editing: add the four function signatures to `NomService.d.ts` (mirror `GetProjects`/`PrepareVote`) and the matching `export function …{ return window['go']['app']['NomService']['<Name>'](...args); }` wrappers to `NomService.js`, and add `ActionDTO`/`ActionListDTO` classes to `models.ts` (mirror `ProjectDTO`/`ProjectListDTO`).

- [ ] **Step 2: Verify the new symbols exist**

Run: `grep -n "GetActions\|PrepareGovernanceVote\|PrepareExecuteAction\|GetAction\b" frontend/wailsjs/go/app/NomService.d.ts && grep -n "class ActionDTO\|class ActionListDTO" frontend/wailsjs/go/models.ts`
Expected: all four functions + both classes present.

- [ ] **Step 3: Frontend typecheck still compiles**

Run: `cd frontend && pnpm run typecheck`
Expected: PASS (no usages yet; just confirms the generated TS is well-formed).

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs/
git commit -S -m "chore(governance): regenerate Wails bindings for governance methods"
```

---

### Task 4: Frontend vote-math lib (`lib/governance.ts`)

**Files:**
- Create: `frontend/src/lib/governance.ts`
- Test: `frontend/src/lib/governance.test.ts`

**Interfaces:**
- Produces: `ACTION_STATUS`, `actionStatusLabel(n:number):string`, `actionTypeLabel(t:number):string`, `isOpen(a:{status:number,expired:boolean}):boolean`, `isActionApproved(v:Votes, a:Thresholds, numPillars:number):boolean`, `isActionRejected(...):boolean`, where `Votes = {yes:number,no:number,total:number}` and `Thresholds = {activePillarThreshold:number,directionalThreshold:number}`.

- [ ] **Step 1: Write the failing test `frontend/src/lib/governance.test.ts`**

```ts
import { describe, it, expect } from 'vitest'
import {
  actionStatusLabel,
  actionTypeLabel,
  isOpen,
  isActionApproved,
  isActionRejected,
} from './governance'

describe('governance vote math', () => {
  it('labels statuses and types', () => {
    expect(actionStatusLabel(0)).toBe('Voting')
    expect(actionStatusLabel(3)).toBe('NoDecision')
    expect(actionStatusLabel(9)).toBe('#9')
    expect(actionTypeLabel(1)).toBe('Spork')
    expect(actionTypeLabel(2)).toBe('Normal')
  })
  it('isOpen = Voting && !expired', () => {
    expect(isOpen({ status: 0, expired: false })).toBe(true)
    expect(isOpen({ status: 0, expired: true })).toBe(false)
    expect(isOpen({ status: 1, expired: false })).toBe(false)
  })
  it('approval needs quorum on yes+no AND directional yes-share (abstain excluded)', () => {
    const thr = { activePillarThreshold: 50, directionalThreshold: 50 }
    // 100 pillars, round-0 Type2: quorum needs (yes+no)*100 > 5000 → >50 directional votes
    // 40 yes / 10 no / 30 abstain → directional 50 → 5000 !> 5000 → no quorum
    expect(isActionApproved({ yes: 40, no: 10, total: 80 }, thr, 100)).toBe(false)
    // 60 yes / 10 no → directional 70 → 7000 > 5000 quorum; yes 6000 > 70*50=3500 → approved
    expect(isActionApproved({ yes: 60, no: 10, total: 70 }, thr, 100)).toBe(true)
    // abstain does not help quorum: 40 yes / 5 no / 100 abstain → directional 45 → no quorum
    expect(isActionApproved({ yes: 40, no: 5, total: 145 }, thr, 100)).toBe(false)
  })
  it('rejection mirrors approval on the no-share', () => {
    const thr = { activePillarThreshold: 50, directionalThreshold: 50 }
    expect(isActionRejected({ yes: 10, no: 60, total: 70 }, thr, 100)).toBe(true)
    expect(isActionRejected({ yes: 60, no: 10, total: 70 }, thr, 100)).toBe(false)
  })
})
```

- [ ] **Step 2: Run it — verify it fails**

Run: `cd frontend && pnpm exec vitest run src/lib/governance.test.ts`
Expected: FAIL — cannot resolve `./governance`.

- [ ] **Step 3: Create `frontend/src/lib/governance.ts`**

```ts
// Governance action vote math — mirrors go-zenon checkActionVoteBreakdown.
// Per the CURRENT round only; abstain is EXCLUDED from the directional total
// (unlike the accelerator, which counts abstain toward quorum). Thresholds are
// read off the action (the node computes the current round's values).
export const ACTION_STATUS = ['Voting', 'Approved', 'Rejected', 'NoDecision'] as const

export function actionStatusLabel(n: number): string {
  return ACTION_STATUS[n] ?? `#${n}`
}

export function actionTypeLabel(t: number): string {
  return t === 1 ? 'Spork' : t === 2 ? 'Normal' : `#${t}`
}

export function isOpen(a: { status: number; expired: boolean }): boolean {
  return a.status === 0 && !a.expired
}

type Votes = { yes: number; no: number; total: number }
type Thresholds = { activePillarThreshold: number; directionalThreshold: number }

function quorumMet(v: Votes, a: Thresholds, numPillars: number): boolean {
  const directional = v.yes + v.no
  return directional * 100 > numPillars * a.activePillarThreshold
}

export function isActionApproved(v: Votes, a: Thresholds, numPillars: number): boolean {
  const directional = v.yes + v.no
  return quorumMet(v, a, numPillars) && v.yes * 100 > directional * a.directionalThreshold
}

export function isActionRejected(v: Votes, a: Thresholds, numPillars: number): boolean {
  const directional = v.yes + v.no
  return quorumMet(v, a, numPillars) && v.no * 100 > directional * a.directionalThreshold
}
```

- [ ] **Step 4: Run it — verify it passes**

Run: `cd frontend && pnpm exec vitest run src/lib/governance.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/governance.ts frontend/src/lib/governance.test.ts
git commit -S -m "feat(governance): vote-math lib (lib/governance)"
```

---

### Task 5: Frontend store (`stores/governance.ts`)

**Files:**
- Create: `frontend/src/stores/governance.ts`
- Test: `frontend/src/stores/governance.test.ts`

**Interfaces:**
- Consumes: `Nom.GetActions`, `Nom.GetAction`, `Nom.GetVotablePillars`, `Nom.GetActivePillarCount` from `../../wailsjs/go/app/NomService`; `app.ActionDTO`.
- Produces: `useGovernanceStore` with state `{ actions: app.ActionDTO[], actionCount: number, actionPage: number, selectedAction: app.ActionDTO|null, votablePillars: string[], numActivePillars: number, error: string }` and actions `loadActions(page=0)`, `openAction(id)`, `loadVotablePillars()`, `loadActivePillarCount()`.

- [ ] **Step 1: Write the failing test `frontend/src/stores/governance.test.ts`**

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetActions, GetVotablePillars, GetActivePillarCount } = vi.hoisted(() => ({
  GetActions: vi.fn(),
  GetVotablePillars: vi.fn(),
  GetActivePillarCount: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetActions,
  GetAction: vi.fn(),
  GetVotablePillars,
  GetActivePillarCount,
}))

import { useGovernanceStore } from './governance'

describe('governance store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loadActions sets actions + count + page', async () => {
    GetActions.mockResolvedValue({ count: 42, list: [{ id: 'a1' }, { id: 'a2' }] })
    const s = useGovernanceStore()
    await s.loadActions(1)
    expect(GetActions).toHaveBeenCalledWith(1, 20)
    expect(s.actions).toEqual([{ id: 'a1' }, { id: 'a2' }])
    expect(s.actionCount).toBe(42)
    expect(s.actionPage).toBe(1)
  })

  it('loadActions surfaces error', async () => {
    GetActions.mockRejectedValue(new Error('boom'))
    const s = useGovernanceStore()
    await s.loadActions()
    expect(s.error).toBe('boom')
  })

  it('loadVotablePillars swallows errors to empty', async () => {
    GetVotablePillars.mockRejectedValue(new Error('locked'))
    const s = useGovernanceStore()
    await s.loadVotablePillars()
    expect(s.votablePillars).toEqual([])
  })

  it('loadActivePillarCount swallows errors to 0', async () => {
    GetActivePillarCount.mockRejectedValue(new Error('locked'))
    const s = useGovernanceStore()
    await s.loadActivePillarCount()
    expect(s.numActivePillars).toBe(0)
  })
})
```

- [ ] **Step 2: Run it — verify it fails**

Run: `cd frontend && pnpm exec vitest run src/stores/governance.test.ts`
Expected: FAIL — cannot resolve `./governance`.

- [ ] **Step 3: Create `frontend/src/stores/governance.ts`**

```ts
import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

const PAGE_SIZE = 20

export const useGovernanceStore = defineStore('governance', {
  state: () => ({
    actions: [] as app.ActionDTO[],
    actionCount: 0,
    actionPage: 0,
    selectedAction: null as app.ActionDTO | null,
    votablePillars: [] as string[],
    numActivePillars: 0,
    error: '',
  }),
  actions: {
    async loadActions(page = 0) {
      this.error = ''
      try {
        const list = await Nom.GetActions(page, PAGE_SIZE)
        this.actions = list.list ?? []
        this.actionCount = list.count ?? 0
        this.actionPage = page
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      }
    },
    async openAction(id: string) {
      this.error = ''
      try {
        this.selectedAction = await Nom.GetAction(id)
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      }
    },
    async loadVotablePillars() {
      try {
        this.votablePillars = await Nom.GetVotablePillars()
      } catch {
        this.votablePillars = [] // locked / not connected ⇒ no voting
      }
    },
    async loadActivePillarCount() {
      try {
        this.numActivePillars = await Nom.GetActivePillarCount()
      } catch {
        this.numActivePillars = 0
      }
    },
  },
})
```

- [ ] **Step 4: Run it — verify it passes**

Run: `cd frontend && pnpm exec vitest run src/stores/governance.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/governance.ts frontend/src/stores/governance.test.ts
git commit -S -m "feat(governance): Pinia store (actions/pillars/active-count)"
```

---

### Task 6: `GovernanceActions.vue` (browse + filter + paging + detail + execute)

**Files:**
- Create: `frontend/src/components/panels/GovernanceActions.vue`
- Test: `frontend/src/components/panels/GovernanceActions.test.ts`

**Interfaces:**
- Consumes: `useGovernanceStore` (`actions`, `actionCount`, `actionPage`, `numActivePillars`, `loadActions`); `useTxStore().awaitConfirm`; `Nom.PrepareExecuteAction`; `lib/governance` (`actionStatusLabel`, `actionTypeLabel`, `isActionApproved`); `formatAmount` is not needed (actions carry no funds). nom-ui `Button`.
- Produces: the Actions sub-view component (default export).

- [ ] **Step 1: Write the failing test `frontend/src/components/panels/GovernanceActions.test.ts`**

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
const { PrepareExecuteAction } = vi.hoisted(() => ({ PrepareExecuteAction: vi.fn(() => Promise.resolve({ summary: 'x' })) }))
vi.mock('../../../wailsjs/go/app/NomService', () => ({ PrepareExecuteAction, GetActions: vi.fn() }))

import GovernanceActions from './GovernanceActions.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'

function setup() {
  setActivePinia(createPinia())
  const gov = useGovernanceStore()
  gov.numActivePillars = 100
  gov.actions = [
    { id: '0xv', name: 'OpenAct', destination: 'z1dest', data: 'AAEC', type: 2, round: 0, status: 0,
      executed: false, expired: false, activePillarThreshold: 50, directionalThreshold: 50,
      votes: { yes: 60, no: 10, total: 70 } },
    { id: '0xa', name: 'ApprovedAct', destination: 'z1dest', data: '', type: 2, round: 0, status: 1,
      executed: false, expired: false, activePillarThreshold: 50, directionalThreshold: 50,
      votes: { yes: 0, no: 0, total: 0 } },
    { id: '0xe', name: 'ExecutedAct', destination: 'z1dest', data: '', type: 2, round: 0, status: 1,
      executed: true, expired: false, activePillarThreshold: 50, directionalThreshold: 50,
      votes: { yes: 0, no: 0, total: 0 } },
  ] as never
  return { gov }
}

describe('GovernanceActions', () => {
  it('shows all actions by default', () => {
    setup()
    const w = mount(GovernanceActions)
    expect(w.text()).toContain('OpenAct')
    expect(w.text()).toContain('ApprovedAct')
  })

  it('Approved filter shows only status=1 actions', async () => {
    setup()
    const w = mount(GovernanceActions)
    await w.find('button[aria-label="filter Approved"]').trigger('click')
    expect(w.text()).toContain('ApprovedAct')
    expect(w.text()).not.toContain('OpenAct')
  })

  it('Execute button shows only for Approved && !executed and dispatches', async () => {
    const { gov } = setup()
    const awaitConfirm = vi.spyOn(useTxStore(), 'awaitConfirm').mockImplementation(() => {})
    const w = mount(GovernanceActions)
    // expand ApprovedAct
    await w.find('button[aria-label="details 0xa"]').trigger('click')
    const execBtn = w.find('button[aria-label="execute 0xa"]')
    expect(execBtn.exists()).toBe(true)
    await execBtn.trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareExecuteAction).toHaveBeenCalledWith('0xa')
    expect(awaitConfirm).toHaveBeenCalled()
    // ExecutedAct must NOT offer execute
    await w.find('button[aria-label="details 0xe"]').trigger('click')
    expect(w.find('button[aria-label="execute 0xe"]').exists()).toBe(false)
  })
})
```

- [ ] **Step 2: Run it — verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernanceActions.test.ts`
Expected: FAIL — cannot resolve `./GovernanceActions.vue`.

- [ ] **Step 3: Create `frontend/src/components/panels/GovernanceActions.vue`**

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'
import { actionStatusLabel, actionTypeLabel, isActionApproved } from '../../lib/governance'
import type { app } from '../../../wailsjs/go/models'

const gov = useGovernanceStore()
const tx = useTxStore()
const { actions, actionCount, actionPage, numActivePillars } = storeToRefs(gov)
const error = ref('')

const PAGE_SIZE = 20
const pageCount = computed(() => Math.max(1, Math.ceil(actionCount.value / PAGE_SIZE)))
const hasPrev = computed(() => actionPage.value > 0)
const hasNext = computed(() => actionPage.value + 1 < pageCount.value)

const FILTERS = ['All', 'Voting', 'Approved', 'Rejected', 'NoDecision'] as const
type Filter = (typeof FILTERS)[number]
const filter = ref<Filter>('All')
const expanded = ref<string | null>(null)

const filtered = computed(() =>
  (actions.value ?? []).filter((a) => {
    switch (filter.value) {
      case 'Voting': return a.status === 0
      case 'Approved': return a.status === 1
      case 'Rejected': return a.status === 2
      case 'NoDecision': return a.status === 3
      default: return true
    }
  }),
)

function executable(a: app.ActionDTO): boolean {
  return a.status === 1 && !a.executed
}
function passing(a: app.ActionDTO): boolean {
  return a.status === 0 && isActionApproved(a.votes, a, numActivePillars.value)
}
function toggle(id: string) {
  expanded.value = expanded.value === id ? null : id
}
function goPage(page: number) {
  expanded.value = null
  gov.loadActions(page)
}
async function execute(id: string) {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareExecuteAction(id))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="space-y-3 p-4">
    <div class="flex flex-wrap gap-1">
      <button
        v-for="f in FILTERS"
        :key="f"
        type="button"
        class="rounded-full border px-3 py-1 text-xs transition-colors"
        :class="filter === f ? 'border-primary bg-primary/15 text-primary' : 'border-border text-muted-foreground hover:text-foreground'"
        :aria-label="`filter ${f}`"
        :aria-pressed="filter === f"
        @click="filter = f"
      >{{ f }}</button>
    </div>

    <p v-if="filtered.length === 0" class="text-sm text-muted-foreground">No matching actions.</p>

    <div
      v-for="a in filtered"
      :key="a.id"
      class="space-y-1 rounded-lg border border-border bg-card p-3 text-sm"
    >
      <div class="flex items-center justify-between gap-2">
        <span class="font-medium text-foreground">{{ a.name }}</span>
        <span class="text-xs text-muted-foreground">
          <span class="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase">{{ actionTypeLabel(a.type) }}</span>
          {{ actionStatusLabel(a.status) }}
          <span v-if="passing(a)" class="text-primary"> · passing</span>
        </span>
      </div>
      <p class="text-xs text-muted-foreground">
        {{ a.votes.yes }} yes · {{ a.votes.no }} no · {{ a.votes.total }} votes (round {{ a.round + 1 }})
      </p>
      <Button variant="outline" class="px-2 py-1 text-xs" :aria-label="`details ${a.id}`" @click="toggle(a.id)">
        {{ expanded === a.id ? 'Hide' : 'Details' }}
      </Button>
      <template v-if="expanded === a.id">
        <p class="ml-1 mt-1 break-all text-xs text-muted-foreground">{{ a.description }}</p>
        <p class="ml-1 text-xs text-muted-foreground">Calls: {{ a.destination }}</p>
        <p class="ml-1 break-all text-xs text-muted-foreground">Data (base64): {{ a.data || '—' }}</p>
        <p class="ml-1 text-xs text-muted-foreground">
          Thresholds: {{ a.activePillarThreshold }}% quorum · {{ a.directionalThreshold }}% directional
        </p>
        <Button
          v-if="executable(a)"
          class="mt-1 px-2 py-1 text-xs"
          :aria-label="`execute ${a.id}`"
          @click="execute(a.id)"
        >Execute</Button>
      </template>
    </div>

    <div v-if="pageCount > 1" class="flex items-center justify-between gap-2 pt-1">
      <Button variant="outline" class="px-2 py-1 text-xs" :disabled="!hasPrev" aria-label="previous page" @click="goPage(actionPage - 1)">Prev</Button>
      <span class="text-xs text-muted-foreground">Page {{ actionPage + 1 }} of {{ pageCount }} · {{ actionCount }} actions</span>
      <Button variant="outline" class="px-2 py-1 text-xs" :disabled="!hasNext" aria-label="next page" @click="goPage(actionPage + 1)">Next</Button>
    </div>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
```

- [ ] **Step 4: Run it — verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernanceActions.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/panels/GovernanceActions.vue frontend/src/components/panels/GovernanceActions.test.ts
git commit -S -m "feat(governance): Actions sub-view (browse/filter/paging/execute)"
```

---

### Task 7: `GovernanceVote.vue` (open actions + pillar picker + vote)

**Files:**
- Create: `frontend/src/components/panels/GovernanceVote.vue`
- Test: `frontend/src/components/panels/GovernanceVote.test.ts`

**Interfaces:**
- Consumes: `useGovernanceStore` (`actions`, `votablePillars`, `numActivePillars`); `useTxStore().awaitConfirm`; `Nom.PrepareGovernanceVote`; `lib/governance` (`isOpen`, `isActionApproved`). nom-ui `Button`.
- Produces: the Vote sub-view component (default export). Phase 1: NO per-pillar "needs my vote" state — lists all open actions.

- [ ] **Step 1: Write the failing test `frontend/src/components/panels/GovernanceVote.test.ts`**

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
const { PrepareGovernanceVote } = vi.hoisted(() => ({ PrepareGovernanceVote: vi.fn(() => Promise.resolve({ summary: 'v' })) }))
vi.mock('../../../wailsjs/go/app/NomService', () => ({ PrepareGovernanceVote }))

import GovernanceVote from './GovernanceVote.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'

function setup(opts: { pillars?: string[] } = {}) {
  setActivePinia(createPinia())
  const gov = useGovernanceStore()
  gov.numActivePillars = 100
  gov.votablePillars = opts.pillars ?? ['P1']
  gov.actions = [
    { id: '0xopen', name: 'OpenAct', type: 2, round: 0, status: 0, executed: false, expired: false,
      activePillarThreshold: 50, directionalThreshold: 50, votes: { yes: 1, no: 0, total: 1 } },
    { id: '0xclosed', name: 'ClosedAct', type: 2, round: 0, status: 1, executed: true, expired: false,
      activePillarThreshold: 50, directionalThreshold: 50, votes: { yes: 0, no: 0, total: 0 } },
    { id: '0xexpired', name: 'ExpiredAct', type: 2, round: 0, status: 0, executed: false, expired: true,
      activePillarThreshold: 50, directionalThreshold: 50, votes: { yes: 0, no: 0, total: 0 } },
  ] as never
  return { gov }
}

describe('GovernanceVote', () => {
  it('shows a pillar-operator note when no pillar is owned', () => {
    setup({ pillars: [] })
    const w = mount(GovernanceVote)
    expect(w.text().toLowerCase()).toContain('pillar operators')
  })

  it('lists only open actions (Voting && !expired)', () => {
    setup()
    const w = mount(GovernanceVote)
    expect(w.text()).toContain('OpenAct')
    expect(w.text()).not.toContain('ClosedAct')
    expect(w.text()).not.toContain('ExpiredAct')
  })

  it('forwards a Yes vote with (id, selectedPillar, 0)', async () => {
    const awaitConfirm = vi.spyOn(useTxStore(), 'awaitConfirm').mockImplementation(() => {})
    setup({ pillars: ['P1', 'P2'] })
    const w = mount(GovernanceVote)
    await w.find('select[aria-label="vote pillar"]').setValue('P2')
    await w.find('button[aria-label="vote yes 0xopen"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareGovernanceVote).toHaveBeenCalledWith('0xopen', 'P2', 0)
    expect(awaitConfirm).toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run it — verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernanceVote.test.ts`
Expected: FAIL — cannot resolve `./GovernanceVote.vue`.

- [ ] **Step 3: Create `frontend/src/components/panels/GovernanceVote.vue`**

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'
import { isOpen, isActionApproved } from '../../lib/governance'
import type { app } from '../../../wailsjs/go/models'

const gov = useGovernanceStore()
const tx = useTxStore()
const { actions, votablePillars, numActivePillars } = storeToRefs(gov)
const error = ref('')

const ownsPillar = computed(() => votablePillars.value.length > 0)
const selectedPillar = ref('')
watch(
  votablePillars,
  (list) => {
    if (!list.includes(selectedPillar.value)) selectedPillar.value = list[0] ?? ''
  },
  { immediate: true },
)

// Phase 1: list ALL open actions (no per-pillar "needs my vote" yet — that
// arrives in Phase 2 with the governance getPillarVotes read).
const openActions = computed(() => (actions.value ?? []).filter(isOpen))

function passing(a: app.ActionDTO): boolean {
  return isActionApproved(a.votes, a, numActivePillars.value)
}

async function vote(id: string, choice: number) {
  error.value = ''
  if (!selectedPillar.value) {
    error.value = 'Select a pillar to vote as.'
    return
  }
  try {
    tx.awaitConfirm(await Nom.PrepareGovernanceVote(id, selectedPillar.value, choice))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="space-y-3 p-4">
    <p v-if="!ownsPillar" class="text-sm text-muted-foreground">
      Voting on governance actions is for pillar operators. Register or run a pillar to vote.
    </p>
    <template v-else>
      <label class="flex items-center gap-2 text-sm text-muted-foreground">
        Vote as
        <select
          v-model="selectedPillar"
          aria-label="vote pillar"
          class="rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary"
        >
          <option v-for="n in votablePillars" :key="n" :value="n">{{ n }}</option>
        </select>
      </label>

      <p v-if="openActions.length === 0" class="text-sm text-muted-foreground">
        No governance actions are open for voting right now.
      </p>

      <div
        v-for="a in openActions"
        :key="a.id"
        class="space-y-2 rounded-lg border border-border bg-card p-3"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-sm font-medium text-foreground">{{ a.name }}</span>
          <span class="text-xs text-muted-foreground">round {{ a.round + 1 }}</span>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ a.votes.yes }} yes · {{ a.votes.no }} no · {{ a.votes.total }} votes
          ({{ a.activePillarThreshold }}% quorum / {{ a.directionalThreshold }}% directional)
          <span v-if="passing(a)" class="text-primary"> · passing</span>
        </p>
        <div class="flex flex-wrap gap-2">
          <Button :aria-label="`vote yes ${a.id}`" @click="vote(a.id, 0)">Yes</Button>
          <Button variant="outline" :aria-label="`vote no ${a.id}`" @click="vote(a.id, 1)">No</Button>
          <Button variant="outline" :aria-label="`vote abstain ${a.id}`" @click="vote(a.id, 2)">Abstain</Button>
        </div>
      </div>
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
```

- [ ] **Step 4: Run it — verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernanceVote.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/panels/GovernanceVote.vue frontend/src/components/panels/GovernanceVote.test.ts
git commit -S -m "feat(governance): Vote sub-view (open actions + pillar picker)"
```

---

### Task 8: `GovernancePanel.vue` + Home.vue tab registration (integration)

**Files:**
- Create: `frontend/src/components/panels/GovernancePanel.vue`
- Test: `frontend/src/components/panels/GovernancePanel.test.ts`
- Modify: `frontend/src/views/Home.vue`

**Interfaces:**
- Consumes: `useGovernanceStore` (`loadActions`, `loadVotablePillars`, `loadActivePillarCount`); `useWalletStore().activeIndex`; nom-ui `Tabs/TabsList/TabsTrigger/TabsContent`; `GovernanceVote.vue`, `GovernanceActions.vue`.
- Produces: `GovernancePanel` (accepts `initial-sub?: string`); `'Governance'` entry in Home's `TABS`.

- [ ] **Step 1: Write the failing test `frontend/src/components/panels/GovernancePanel.test.ts`**

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Tabs: { props: ['modelValue'], template: '<div><slot /></div>' },
  TabsList: { template: '<div><slot /></div>' },
  TabsTrigger: { props: ['value'], template: '<button :aria-label="`sub ${value}`"><slot /></button>' },
  TabsContent: { props: ['value'], template: '<div><slot /></div>' },
  Button: { props: ['variant', 'disabled'], template: '<button><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  GetActions: vi.fn(() => Promise.resolve({ count: 0, list: [] })),
  GetVotablePillars: vi.fn(() => Promise.resolve([])),
  GetActivePillarCount: vi.fn(() => Promise.resolve(0)),
  PrepareGovernanceVote: vi.fn(),
  PrepareExecuteAction: vi.fn(),
}))

import GovernancePanel from './GovernancePanel.vue'
import { useGovernanceStore } from '../../stores/governance'

describe('GovernancePanel', () => {
  it('loads governance data on mount and renders both sub-tabs', async () => {
    setActivePinia(createPinia())
    const gov = useGovernanceStore()
    const loadActions = vi.spyOn(gov, 'loadActions')
    const w = mount(GovernancePanel)
    await new Promise((r) => setTimeout(r))
    expect(loadActions).toHaveBeenCalled()
    expect(w.find('button[aria-label="sub Vote"]').exists()).toBe(true)
    expect(w.find('button[aria-label="sub Actions"]').exists()).toBe(true)
  })
})
```

- [ ] **Step 2: Run it — verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernancePanel.test.ts`
Expected: FAIL — cannot resolve `./GovernancePanel.vue`.

- [ ] **Step 3: Create `frontend/src/components/panels/GovernancePanel.vue`**

```vue
<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useGovernanceStore } from '../../stores/governance'
import { useWalletStore } from '../../stores/wallet'
import GovernanceVote from './GovernanceVote.vue'
import GovernanceActions from './GovernanceActions.vue'

const props = defineProps<{ initialSub?: string }>()
const gov = useGovernanceStore()
const wallet = useWalletStore()

const sub = ref(props.initialSub || 'Actions')
watch(
  () => props.initialSub,
  (v) => {
    if (v) sub.value = v
  },
)

function load() {
  gov.loadActions()
  gov.loadVotablePillars()
  gov.loadActivePillarCount()
}
onMounted(load)
watch(() => wallet.activeIndex, load)
</script>

<template>
  <div class="p-4">
    <Tabs v-model="sub">
      <TabsList class="w-full justify-start">
        <TabsTrigger value="Vote">Vote</TabsTrigger>
        <TabsTrigger value="Actions">Actions</TabsTrigger>
      </TabsList>
      <TabsContent value="Vote"><GovernanceVote /></TabsContent>
      <TabsContent value="Actions"><GovernanceActions /></TabsContent>
    </Tabs>
  </div>
</template>
```

- [ ] **Step 4: Run it — verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernancePanel.test.ts`
Expected: PASS (1 test).

- [ ] **Step 5: Register the tab in `frontend/src/views/Home.vue`**

Add the import alongside the other panel imports (near line 25):

```ts
import GovernancePanel from '../components/panels/GovernancePanel.vue'
```

Add `'Governance'` to the `TABS` array (line ~43):

```ts
const TABS = ['Tokens', 'Rewards', 'Plasma', 'Pillar', 'Staking', 'Sentinels', 'Accelerator', 'Governance']
```

Add the `TabsContent` block after the Accelerator one (line ~136):

```vue
        <TabsContent value="Governance"><GovernancePanel /></TabsContent>
```

- [ ] **Step 6: Typecheck + full frontend suite + build**

Run: `cd frontend && pnpm run typecheck && pnpm test && pnpm run build`
Expected: typecheck clean; all tests pass (existing + new governance tests); build succeeds.

- [ ] **Step 7: Backend gates**

Run: `GOWORK=off GOTOOLCHAIN=auto go vet ./... && GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run Governance`
Expected: vet clean; governance tests pass.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/panels/GovernancePanel.vue frontend/src/components/panels/GovernancePanel.test.ts frontend/src/views/Home.vue
git commit -S -m "feat(governance): GovernancePanel + register Governance NoM tab"
```

---

## Self-Review notes

- **Spec coverage:** Browse (Task 6) ✓; simple Vote (Task 7) ✓; Execute in detail (Task 6) ✓; backend reads+prepares (Tasks 1–2) ✓; vote math incl. abstain-excluded-from-directional (Task 4) ✓; tab registration (Task 8) ✓; bindings (Task 3) ✓. All `[P2]` items intentionally excluded.
- **Confirm-what-you-sign:** Execute summary names the destination contract (Task 2) since the on-chain `ToAddress` is the governance contract; the existing `tx` confirm modal renders the built-block summary.
- **Type consistency:** `ActionDTO` fields (Go `app/dto.go`) ↔ generated `app.ActionDTO` (TS) ↔ store/components; `isActionApproved(votes, action, numPillars)` signature identical across Tasks 4/6/7; vote values `0/1/2` consistent end-to-end.
- **Out of scope (deferred to Phase 2):** `GetPillarVotes`, `GetVotableActionsForMyPillars`, `VotableAction` DTO, per-pillar Vote filter, `needsVoteCount`, TopBar badge.
