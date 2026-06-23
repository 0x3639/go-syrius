# Phase 5f — Accelerator-Z Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full-parity Accelerator-Z support (browse projects/phases, donate, Pillar voting, project/phase create+update) to go-syrius, following the established NoM pattern.

**Architecture:** Backend reads return plain DTOs; every state-changing action is a `Prepare*` method that builds an SDK template and routes it through the shared confirm-what-you-sign path (`TxService.prepareCall` → `CallPreview` → `TxModal` → `ConfirmPublish`). New backend code lives in `app/nom_accelerator.go` (keeping `app/nom_service.go` from growing further); DTOs join the others in `app/dto.go`. Frontend mirrors `Tokens.svelte`: one route with sections, one feature store.

**Tech Stack:** Go 1.22+, `znn-sdk-go` `AcceleratorApi`/`PillarApi`, go-zenon `common/types` + `vm/constants` + `vm/embedded/definition`; Svelte + TS + Tailwind, Vitest, `@testing-library/svelte`.

## Global Constraints

- SDK pinned at `github.com/0x3639/znn-sdk-go v0.1.18` (already bumped; first commit on this branch). Vote bytes MUST come from `embedded.VoteYes` (0) / `embedded.VoteNo` (1) / `embedded.VoteAbstain` (2) — never literals.
- Branch: `phase-5f-accelerator`.
- Build/test with `GOWORK=off` (e.g. `GOWORK=off go test ./app/...`). Frontend tests: `cd frontend && npx vitest run <file>`.
- Every `Prepare*` re-validates inputs server-side; field validation runs BEFORE any node use, so valid input on a disconnected node returns the error string `"not connected"` (tests rely on this).
- `callExpect` passed to `prepareCall` MUST match the SDK template's `ToAddress`/`TokenStandard`/`Amount`/`Data` or `assertMatches` rejects publication. Use `template.Amount` and `append([]byte(nil), template.Data...)`.
- On-chain validation rules to mirror (go-zenon): name 1–30 chars, description 1–240 chars, URL non-empty + matches the contract regex, ZNN funds ≤ `constants.ProjectZnnMaximumFunds` (5000 ZNN), QSR funds ≤ `constants.ProjectQsrMaximumFunds` (50000 QSR), project creation fee = `constants.ProjectCreationAmount` (1 ZNN, carried by the template).
- All embedded targets use `types.AcceleratorContract`.
- Status enum (uint8): `0=Voting, 1=Active, 2=Paid, 3=Closed`.

---

## File Structure

- `app/dto.go` — add `VoteBreakdownDTO`, `PhaseDTO`, `ProjectDTO`, `ProjectListDTO`.
- `app/nom_accelerator.go` (new) — mapping helpers, read methods, `Prepare*` writes, shared validation.
- `app/nom_accelerator_test.go` (new) — unit tests for mapping, validation, template token-standards, vote-constant guard.
- `frontend/src/lib/stores/accelerator.ts` (new) — feature store.
- `frontend/src/routes/Accelerator.svelte` (new) — the route.
- `frontend/src/routes/Accelerator.test.ts` (new) — component tests.
- `frontend/src/lib/stores/nav.ts` — add `'accelerator'` to `View`.
- `frontend/src/App.svelte` — import + route the new view.
- `frontend/src/routes/Dashboard.svelte` — add the nav button.
- `docs/phase5f-acceptance.md` (new) — manual testnet record.

Wails regenerates `frontend/wailsjs/go/app/NomService.*` and `models.ts` from the Go methods during `wails dev`/`wails build` (or `wails generate module`); the frontend imports the new methods/DTOs from there.

---

## Task 1: Accelerator DTOs + mapping helpers

**Files:**
- Modify: `app/dto.go` (append after `TokenInfo`, ~line 239)
- Create: `app/nom_accelerator.go`
- Test: `app/nom_accelerator_test.go`

**Interfaces:**
- Produces: DTO types `VoteBreakdownDTO`, `PhaseDTO`, `ProjectDTO`, `ProjectListDTO`; helpers `voteBreakdownDTO(*embedded.VoteBreakdown) VoteBreakdownDTO`, `phaseDTO(*embedded.Phase) PhaseDTO`, `projectDTO(*embedded.Project) ProjectDTO`.

- [ ] **Step 1: Write the failing test** — `app/nom_accelerator_test.go`

```go
package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestProjectDTONilSafe(t *testing.T) {
	// A project with nil funds/votes/phases must map without panicking.
	p := &embedded.Project{Name: "Proj", Status: 1}
	dto := projectDTO(p)
	if dto.Name != "Proj" || dto.Status != 1 {
		t.Fatalf("unexpected dto: %+v", dto)
	}
	if dto.ZnnFundsNeeded != "0" || dto.QsrFundsNeeded != "0" {
		t.Fatalf("nil funds must map to \"0\": %+v", dto)
	}
	if dto.Votes.Total != 0 || dto.Phases == nil {
		t.Fatalf("nil votes/phases must map to zero/empty, got %+v", dto)
	}
}

func TestProjectDTOMapsFieldsAndPhases(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	p := &embedded.Project{
		Id:             types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		Owner:          owner,
		Name:           "Proj",
		ZnnFundsNeeded: big.NewInt(150),
		QsrFundsNeeded: big.NewInt(250),
		Status:         2,
		Votes:          &embedded.VoteBreakdown{Total: 5, Yes: 3, No: 2},
		Phases: []*embedded.Phase{{
			Phase: &embedded.PhaseInfo{Name: "P1", ZnnFundsNeeded: big.NewInt(10), QsrFundsNeeded: big.NewInt(20), Status: 1},
			Votes: &embedded.VoteBreakdown{Total: 4, Yes: 4, No: 0},
		}},
	}
	dto := projectDTO(p)
	if dto.Owner != owner.String() || dto.ZnnFundsNeeded != "150" || dto.Votes.Yes != 3 {
		t.Fatalf("project fields not mapped: %+v", dto)
	}
	if len(dto.Phases) != 1 || dto.Phases[0].Name != "P1" || dto.Phases[0].Votes.Yes != 4 {
		t.Fatalf("phase fields not mapped: %+v", dto.Phases)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./app/ -run TestProjectDTO -v`
Expected: FAIL — `undefined: projectDTO` (and the DTO types).

- [ ] **Step 3: Add the DTOs** — append to `app/dto.go`

```go
// VoteBreakdownDTO is the Yes/No/Total Pillar-vote tally for a project or phase.
type VoteBreakdownDTO struct {
	Total uint32 `json:"total"`
	Yes   uint32 `json:"yes"`
	No    uint32 `json:"no"`
}

// PhaseDTO is one Accelerator-Z phase with its vote tally.
type PhaseDTO struct {
	Id                string           `json:"id"`
	ProjectId         string           `json:"projectId"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	Url               string           `json:"url"`
	ZnnFundsNeeded    string           `json:"znnFundsNeeded"`
	QsrFundsNeeded    string           `json:"qsrFundsNeeded"`
	CreationTimestamp int64            `json:"creationTimestamp"`
	AcceptedTimestamp int64            `json:"acceptedTimestamp"`
	Status            int              `json:"status"`
	Votes             VoteBreakdownDTO `json:"votes"`
}

// ProjectDTO is one Accelerator-Z project with its phases and vote tally.
type ProjectDTO struct {
	Id                  string           `json:"id"`
	Owner               string           `json:"owner"`
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	Url                 string           `json:"url"`
	ZnnFundsNeeded      string           `json:"znnFundsNeeded"`
	QsrFundsNeeded      string           `json:"qsrFundsNeeded"`
	CreationTimestamp   int64            `json:"creationTimestamp"`
	LastUpdateTimestamp int64            `json:"lastUpdateTimestamp"`
	Status              int              `json:"status"`
	Votes               VoteBreakdownDTO `json:"votes"`
	Phases              []PhaseDTO       `json:"phases"`
}

// ProjectListDTO is one page of Accelerator-Z projects.
type ProjectListDTO struct {
	Count int          `json:"count"`
	List  []ProjectDTO `json:"list"`
}
```

- [ ] **Step 4: Add the mapping helpers** — create `app/nom_accelerator.go`

```go
package app

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
	constants "github.com/zenon-network/go-zenon/vm/constants"
)

// bigStr renders a possibly-nil *big.Int as a base-10 string ("0" when nil).
func bigStr(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

func voteBreakdownDTO(v *embedded.VoteBreakdown) VoteBreakdownDTO {
	if v == nil {
		return VoteBreakdownDTO{}
	}
	return VoteBreakdownDTO{Total: v.Total, Yes: v.Yes, No: v.No}
}

func phaseDTO(p *embedded.Phase) PhaseDTO {
	if p == nil || p.Phase == nil {
		return PhaseDTO{}
	}
	pi := p.Phase
	return PhaseDTO{
		Id:                pi.Id.String(),
		ProjectId:         pi.ProjectID.String(),
		Name:              pi.Name,
		Description:       pi.Description,
		Url:               pi.Url,
		ZnnFundsNeeded:    bigStr(pi.ZnnFundsNeeded),
		QsrFundsNeeded:    bigStr(pi.QsrFundsNeeded),
		CreationTimestamp: pi.CreationTimestamp,
		AcceptedTimestamp: pi.AcceptedTimestamp,
		Status:            int(pi.Status),
		Votes:             voteBreakdownDTO(p.Votes),
	}
}

func projectDTO(p *embedded.Project) ProjectDTO {
	if p == nil {
		return ProjectDTO{Phases: []PhaseDTO{}}
	}
	phases := make([]PhaseDTO, 0, len(p.Phases))
	for _, ph := range p.Phases {
		phases = append(phases, phaseDTO(ph))
	}
	return ProjectDTO{
		Id:                  p.Id.String(),
		Owner:               p.Owner.String(),
		Name:                p.Name,
		Description:         p.Description,
		Url:                 p.Url,
		ZnnFundsNeeded:      bigStr(p.ZnnFundsNeeded),
		QsrFundsNeeded:      bigStr(p.QsrFundsNeeded),
		CreationTimestamp:   p.CreationTimestamp,
		LastUpdateTimestamp: p.LastUpdateTimestamp,
		Status:              int(p.Status),
		Votes:               voteBreakdownDTO(p.Votes),
		Phases:              phases,
	}
}

// acceleratorURLRe mirrors the project/phase URL rule enforced on-chain in
// go-zenon vm/embedded/implementation/accelerator.go.
var acceleratorURLRe = regexp.MustCompile(`^([Hh][Tt][Tt][Pp][Ss]?://)?[a-zA-Z0-9]{2,60}\.[a-zA-Z]{1,6}([-a-zA-Z0-9()@:%_+.~#?&/=]{0,100})$`)

// validateProjectFields mirrors the on-chain create/add/update validation and
// parses the funds amounts. It performs no node I/O.
func validateProjectFields(name, description, url, znnNeeded, qsrNeeded string) (*big.Int, *big.Int, error) {
	if l := len(name); l == 0 || l > constants.ProjectNameLengthMax {
		return nil, nil, fmt.Errorf("name must be 1-%d characters", constants.ProjectNameLengthMax)
	}
	if l := len(description); l == 0 || l > constants.ProjectDescriptionLengthMax {
		return nil, nil, fmt.Errorf("description must be 1-%d characters", constants.ProjectDescriptionLengthMax)
	}
	if len(url) == 0 || !acceleratorURLRe.MatchString(url) {
		return nil, nil, errors.New("invalid URL")
	}
	znn, ok := new(big.Int).SetString(strings.TrimSpace(znnNeeded), 10)
	if !ok || znn.Sign() < 0 {
		return nil, nil, errors.New("invalid ZNN funds amount")
	}
	qsr, ok := new(big.Int).SetString(strings.TrimSpace(qsrNeeded), 10)
	if !ok || qsr.Sign() < 0 {
		return nil, nil, errors.New("invalid QSR funds amount")
	}
	if znn.Cmp(constants.ProjectZnnMaximumFunds) > 0 {
		return nil, nil, errors.New("ZNN funds exceed the maximum")
	}
	if qsr.Cmp(constants.ProjectQsrMaximumFunds) > 0 {
		return nil, nil, errors.New("QSR funds exceed the maximum")
	}
	return znn, qsr, nil
}

// parseHash trims and parses a 0x… hash id (project or phase).
func parseHash(id string) (types.Hash, error) {
	return types.HexToHash(strings.TrimSpace(id))
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `GOWORK=off go test ./app/ -run TestProjectDTO -v`
Expected: PASS (both tests).

- [ ] **Step 6: Commit**

```bash
git add app/dto.go app/nom_accelerator.go app/nom_accelerator_test.go
git commit -m "feat(app): Accelerator-Z DTOs + mapping helpers + field validation"
```

---

## Task 2: Accelerator read methods

**Files:**
- Modify: `app/nom_accelerator.go`
- Test: `app/nom_accelerator_test.go`

**Interfaces:**
- Consumes: `projectDTO`, `phaseDTO`, `parseHash` (Task 1); `NomService` fields `node`, `wallet` (`app/nom_service.go:45`); `errLocked` (existing package var); `newTestNode`/`newTestWalletService` (existing test helpers).
- Produces: `(*NomService).GetProjects(pageIndex, pageSize uint32) (ProjectListDTO, error)`, `GetProject(id string) (ProjectDTO, error)`, `GetPhase(id string) (PhaseDTO, error)`, `GetVotablePillars() ([]string, error)`.

- [ ] **Step 1: Write the failing test** — append to `app/nom_accelerator_test.go`

```go
func TestAcceleratorReadsGuardInputs(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)

	// Invalid hash is rejected before any node use.
	if _, err := s.GetProject("not-a-hash"); err == nil {
		t.Fatal("GetProject: invalid hash must error")
	}
	if _, err := s.GetPhase("not-a-hash"); err == nil {
		t.Fatal("GetPhase: invalid hash must error")
	}
	// Valid hash with a disconnected node surfaces "not connected".
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.GetProject(valid); err == nil || err.Error() != "not connected" {
		t.Fatalf("GetProject: want not-connected, got %v", err)
	}
	// Browse list also needs a node.
	if _, err := s.GetProjects(0, 20); err == nil || err.Error() != "not connected" {
		t.Fatalf("GetProjects: want not-connected, got %v", err)
	}
	// Voting eligibility needs an unlocked wallet (locked in this test).
	if _, err := s.GetVotablePillars(); err == nil {
		t.Fatal("GetVotablePillars: locked wallet must error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./app/ -run TestAcceleratorReadsGuardInputs -v`
Expected: FAIL — `undefined: (*NomService).GetProject`.

- [ ] **Step 3: Add the read methods** — append to `app/nom_accelerator.go`

```go
// GetProjects returns one page of Accelerator-Z projects (as the node orders
// them). pageSize is clamped to [1,50].
func (s *NomService) GetProjects(pageIndex, pageSize uint32) (ProjectListDTO, error) {
	if pageSize == 0 || pageSize > 50 {
		pageSize = 50
	}
	client := s.node.currentClient()
	if client == nil {
		return ProjectListDTO{}, errors.New("not connected")
	}
	list, err := client.AcceleratorApi.GetAll(pageIndex, pageSize)
	if err != nil {
		return ProjectListDTO{}, err
	}
	out := ProjectListDTO{Count: list.Count, List: make([]ProjectDTO, 0, len(list.List))}
	for _, p := range list.List {
		out.List = append(out.List, projectDTO(p))
	}
	return out, nil
}

// GetProject returns a single project (with embedded phases + vote tally).
func (s *NomService) GetProject(id string) (ProjectDTO, error) {
	h, err := parseHash(id)
	if err != nil {
		return ProjectDTO{}, fmt.Errorf("invalid project id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return ProjectDTO{}, errors.New("not connected")
	}
	p, err := client.AcceleratorApi.GetProjectById(h)
	if err != nil {
		return ProjectDTO{}, err
	}
	return projectDTO(p), nil
}

// GetPhase returns a single phase (with its vote tally).
func (s *NomService) GetPhase(id string) (PhaseDTO, error) {
	h, err := parseHash(id)
	if err != nil {
		return PhaseDTO{}, fmt.Errorf("invalid phase id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return PhaseDTO{}, errors.New("not connected")
	}
	ph, err := client.AcceleratorApi.GetPhaseById(h)
	if err != nil {
		return PhaseDTO{}, err
	}
	return phaseDTO(ph), nil
}

// GetVotablePillars returns the names of Pillars the active address owns, used
// to gate and drive the voting UI. Empty slice ⇒ the address cannot vote.
func (s *NomService) GetVotablePillars() ([]string, error) {
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	pillars, err := client.PillarApi.GetByOwner(addr)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pillars))
	for _, p := range pillars {
		names = append(names, p.Name)
	}
	return names, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOWORK=off go test ./app/ -run TestAcceleratorReadsGuardInputs -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_accelerator.go app/nom_accelerator_test.go
git commit -m "feat(app): Accelerator-Z reads (projects/project/phase/votable-pillars)"
```

---

## Task 3: PrepareDonate

**Files:**
- Modify: `app/nom_accelerator.go`
- Test: `app/nom_accelerator_test.go`

**Interfaces:**
- Consumes: `NomService.tx.prepareCall` (`app/tx_service.go:226`), `callExpect` (`app/tx_service.go:35`), `CallPreview` (`app/dto.go:141`), `formatBaseAmount` (`app/nom_service.go:21`).
- Produces: `(*NomService).PrepareDonate(amount, token string) (CallPreview, error)`.

- [ ] **Step 1: Write the failing test** — append to `app/nom_accelerator_test.go`

```go
func TestPrepareDonateValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	bad := []struct{ amount, token string }{
		{"0", "ZNN"},        // non-positive
		{"-5", "ZNN"},       // negative
		{"abc", "ZNN"},      // unparseable
		{"100", "DOGE"},     // unknown token
		{"100", ""},         // empty token
	}
	for _, c := range bad {
		if _, err := s.PrepareDonate(c.amount, c.token); err == nil {
			t.Fatalf("donate(%q,%q): expected validation error", c.amount, c.token)
		}
	}
	// Valid input passes validation and only then hits the disconnected node.
	if _, err := s.PrepareDonate("100", "QSR"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid donate should hit not-connected; got %v", err)
	}
}

func TestDonateTemplateTokenStandards(t *testing.T) {
	api := embedded.NewAcceleratorApi(nil) // builder needs no client
	amt := big.NewInt(123)
	d := api.Donate(amt, types.QsrTokenStandard)
	if d.ToAddress != types.AcceleratorContract || d.TokenStandard != types.QsrTokenStandard || d.Amount.Cmp(amt) != 0 {
		t.Fatalf("donate template wrong: %+v", d)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareDonate|TestDonateTemplate' -v`
Expected: FAIL — `undefined: (*NomService).PrepareDonate`.

- [ ] **Step 3: Add PrepareDonate** — append to `app/nom_accelerator.go`

```go
// PrepareDonate builds a Donate template (ZNN or QSR) for the Accelerator and
// routes it through confirm-what-you-sign. Inputs are validated first.
func (s *NomService) PrepareDonate(amount, token string) (CallPreview, error) {
	var ts types.ZenonTokenStandard
	var symbol string
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "ZNN":
		ts, symbol = types.ZnnTokenStandard, "ZNN"
	case "QSR":
		ts, symbol = types.QsrTokenStandard, "QSR"
	default:
		return CallPreview{}, errors.New("donation token must be ZNN or QSR")
	}
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("donation amount must be greater than 0")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.Donate(amt, ts)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: ts, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Donate %s %s to Accelerator-Z", formatBaseAmount(amt.String(), 8), symbol))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareDonate|TestDonateTemplate' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_accelerator.go app/nom_accelerator_test.go
git commit -m "feat(app): Accelerator-Z PrepareDonate (ZNN/QSR) via confirm path"
```

---

## Task 4: PrepareVote

**Files:**
- Modify: `app/nom_accelerator.go`
- Test: `app/nom_accelerator_test.go`

**Interfaces:**
- Consumes: `parseHash` (Task 1), `prepareCall`/`callExpect`, `embedded.VoteYes/VoteNo/VoteAbstain` (SDK v0.1.18).
- Produces: `(*NomService).PrepareVote(id, pillarName string, vote uint8) (CallPreview, error)`.

- [ ] **Step 1: Write the failing test** — append to `app/nom_accelerator_test.go`

```go
func TestVoteConstantsMatchOnChainAuthority(t *testing.T) {
	// Regression guard for the v0.1.18 SDK fix: a "yes" vote MUST serialize as 0.
	if embedded.VoteYes != 0 || embedded.VoteNo != 1 || embedded.VoteAbstain != 2 {
		t.Fatalf("vote constants drifted: yes=%d no=%d abstain=%d", embedded.VoteYes, embedded.VoteNo, embedded.VoteAbstain)
	}
}

func TestPrepareVoteValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	// bad hash
	if _, err := s.PrepareVote("nope", "MyPillar", embedded.VoteYes); err == nil {
		t.Fatal("vote: bad id must error")
	}
	// empty pillar name
	if _, err := s.PrepareVote(valid, "  ", embedded.VoteYes); err == nil {
		t.Fatal("vote: empty pillar must error")
	}
	// out-of-range vote value
	if _, err := s.PrepareVote(valid, "MyPillar", 7); err == nil {
		t.Fatal("vote: out-of-range vote must error")
	}
	// valid input → disconnected node
	if _, err := s.PrepareVote(valid, "MyPillar", embedded.VoteNo); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid vote should hit not-connected; got %v", err)
	}
}

func TestVoteTemplate(t *testing.T) {
	api := embedded.NewAcceleratorApi(nil)
	h := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	v := api.VoteByName(h, "MyPillar", embedded.VoteYes)
	if v.ToAddress != types.AcceleratorContract || v.TokenStandard != types.ZnnTokenStandard || v.Amount.Sign() != 0 {
		t.Fatalf("vote template wrong: %+v", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareVote|TestVoteTemplate|TestVoteConstants' -v`
Expected: FAIL — `undefined: (*NomService).PrepareVote`.

- [ ] **Step 3: Add PrepareVote** — append to `app/nom_accelerator.go`

```go
// PrepareVote builds a VoteByName template for one of the active address's
// Pillars. vote MUST be embedded.VoteYes/VoteNo/VoteAbstain. Field validation
// runs before any node use; Pillar ownership is enforced on-chain.
func (s *NomService) PrepareVote(id, pillarName string, vote uint8) (CallPreview, error) {
	h, err := parseHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid proposal id: %w", err)
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
	template := client.AcceleratorApi.VoteByName(h, name, vote)
	label := map[uint8]string{embedded.VoteYes: "yes", embedded.VoteNo: "no", embedded.VoteAbstain: "abstain"}[vote]
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Vote %s on %s as %s", label, id, name))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareVote|TestVoteTemplate|TestVoteConstants' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_accelerator.go app/nom_accelerator_test.go
git commit -m "feat(app): Accelerator-Z PrepareVote (VoteByName) + vote-constant guard"
```

---

## Task 5: PrepareCreateProject / PrepareAddPhase / PrepareUpdatePhase

**Files:**
- Modify: `app/nom_accelerator.go`
- Test: `app/nom_accelerator_test.go`

**Interfaces:**
- Consumes: `validateProjectFields`, `parseHash` (Task 1); `prepareCall`/`callExpect`; `constants.ProjectCreationAmount`.
- Produces: `PrepareCreateProject(name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error)`, `PrepareAddPhase(projectId, name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error)`, `PrepareUpdatePhase(phaseId, name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error)`.

- [ ] **Step 1: Write the failing test** — append to `app/nom_accelerator_test.go`

```go
func TestPrepareProjectWritesValidateInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	longName := strings.Repeat("x", 31)
	goodURL := "https://zenon.org"

	// CreateProject field validation (no id involved).
	bad := []struct{ name, desc, url, znn, qsr string }{
		{"", "desc", goodURL, "1", "1"},        // empty name
		{longName, "desc", goodURL, "1", "1"},  // name too long
		{"Proj", "", goodURL, "1", "1"},        // empty description
		{"Proj", "desc", "not a url", "1", "1"}, // bad url
		{"Proj", "desc", goodURL, "x", "1"},    // bad znn
		{"Proj", "desc", goodURL, "1", "x"},    // bad qsr
	}
	for i, c := range bad {
		if _, err := s.PrepareCreateProject(c.name, c.desc, c.url, c.znn, c.qsr); err == nil {
			t.Fatalf("create case %d: expected validation error", i)
		}
	}
	// Valid create → not connected.
	if _, err := s.PrepareCreateProject("Proj", "A real description", goodURL, "100", "1000"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid create should hit not-connected; got %v", err)
	}

	// AddPhase / UpdatePhase additionally validate the id.
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.PrepareAddPhase("bad-id", "Proj", "desc", goodURL, "1", "1"); err == nil {
		t.Fatal("addphase: bad id must error")
	}
	if _, err := s.PrepareAddPhase(valid, "", "desc", goodURL, "1", "1"); err == nil {
		t.Fatal("addphase: empty name must error")
	}
	if _, err := s.PrepareAddPhase(valid, "Phase", "desc", goodURL, "1", "1"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid addphase should hit not-connected; got %v", err)
	}
	if _, err := s.PrepareUpdatePhase("bad-id", "Phase", "desc", goodURL, "1", "1"); err == nil {
		t.Fatal("updatephase: bad id must error")
	}
	if _, err := s.PrepareUpdatePhase(valid, "Phase", "desc", goodURL, "1", "1"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid updatephase should hit not-connected; got %v", err)
	}
}

func TestProjectWriteTemplateTokenStandards(t *testing.T) {
	api := embedded.NewAcceleratorApi(nil)
	h := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	create := api.CreateProject("Proj", "desc", "https://zenon.org", big.NewInt(100), big.NewInt(1000))
	if create.ToAddress != types.AcceleratorContract || create.TokenStandard != types.ZnnTokenStandard {
		t.Fatalf("create template wrong: %+v", create)
	}
	if create.Amount.Cmp(constants.ProjectCreationAmount) != 0 {
		t.Fatalf("create fee=%v want %v", create.Amount, constants.ProjectCreationAmount)
	}
	add := api.AddPhase(h, "Phase", "desc", "https://zenon.org", big.NewInt(1), big.NewInt(1))
	if add.ToAddress != types.AcceleratorContract || add.Amount.Sign() != 0 {
		t.Fatalf("addphase template wrong: %+v", add)
	}
}
```

(Ensure the test file imports `"strings"` and `constants "github.com/zenon-network/go-zenon/vm/constants"`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareProjectWrites|TestProjectWriteTemplate' -v`
Expected: FAIL — `undefined: (*NomService).PrepareCreateProject`.

- [ ] **Step 3: Add the three write methods** — append to `app/nom_accelerator.go`

```go
// PrepareCreateProject builds a CreateProject template. The 1 ZNN fee is read
// from the template, never hardcoded. Fields are validated before node use.
func (s *NomService) PrepareCreateProject(name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error) {
	znn, qsr, err := validateProjectFields(name, description, url, znnNeeded, qsrNeeded)
	if err != nil {
		return CallPreview{}, err
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.CreateProject(name, description, url, znn, qsr)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Create project %q (1 ZNN fee)", name))
}

// PrepareAddPhase builds an AddPhase template for an existing project. Project
// ownership is enforced on-chain.
func (s *NomService) PrepareAddPhase(projectId, name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error) {
	h, err := parseHash(projectId)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid project id: %w", err)
	}
	znn, qsr, err := validateProjectFields(name, description, url, znnNeeded, qsrNeeded)
	if err != nil {
		return CallPreview{}, err
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.AddPhase(h, name, description, url, znn, qsr)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Add phase %q to project %s", name, projectId))
}

// PrepareUpdatePhase builds an UpdatePhase template for an existing phase.
func (s *NomService) PrepareUpdatePhase(phaseId, name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error) {
	h, err := parseHash(phaseId)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid phase id: %w", err)
	}
	znn, qsr, err := validateProjectFields(name, description, url, znnNeeded, qsrNeeded)
	if err != nil {
		return CallPreview{}, err
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.UpdatePhase(h, name, description, url, znn, qsr)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Update phase %s", phaseId))
}
```

- [ ] **Step 4: Run the full app suite**

Run: `GOWORK=off go test ./app/... -v 2>&1 | tail -20`
Expected: PASS (new tests + existing unaffected).

- [ ] **Step 5: Commit**

```bash
git add app/nom_accelerator.go app/nom_accelerator_test.go
git commit -m "feat(app): Accelerator-Z project/phase create + update (full parity)"
```

---

## Task 6: Frontend store

**Files:**
- Create: `frontend/src/lib/stores/accelerator.ts`

**Interfaces:**
- Consumes: generated bindings `wailsjs/go/app/NomService` (regenerated once the backend methods exist — run `wails generate module` or `wails dev` first), `app.ProjectDTO`/`app.PhaseDTO` from `wailsjs/go/models`.
- Produces: stores `projects`, `selectedProject`, `votablePillars`, `accLoading`, `accError`; functions `loadProjects(page?)`, `openProject(id)`, `loadVotablePillars()`.

- [ ] **Step 1: Regenerate bindings** (backend methods must exist)

Run: `cd /Users/dfriestedt/Github/go-syrius && GOWORK=off wails generate module`
Expected: updates `frontend/wailsjs/go/app/NomService.{js,d.ts}` and `models.ts` with the new methods/DTOs (e.g. `GetProjects`, `ProjectDTO`). If `wails generate module` is unavailable, `GOWORK=off wails dev` regenerates them on start.

- [ ] **Step 2: Write the store** — create `frontend/src/lib/stores/accelerator.ts`

```ts
import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const projects = writable<app.ProjectDTO[]>([])
export const selectedProject = writable<app.ProjectDTO | null>(null)
export const votablePillars = writable<string[]>([])
export const accError = writable('')

export async function loadProjects(page = 0): Promise<void> {
  accError.set('')
  try {
    const list = await Nom.GetProjects(page, 20)
    projects.set(list.list ?? [])
  } catch (e: any) {
    accError.set(e?.message ?? String(e))
  }
}

export async function openProject(id: string): Promise<void> {
  accError.set('')
  try {
    selectedProject.set(await Nom.GetProject(id))
  } catch (e: any) {
    accError.set(e?.message ?? String(e))
  }
}

export async function loadVotablePillars(): Promise<void> {
  try {
    votablePillars.set(await Nom.GetVotablePillars())
  } catch {
    votablePillars.set([]) // locked / not connected ⇒ no voting
  }
}
```

- [ ] **Step 3: Type-check**

Run: `cd frontend && npx svelte-check --threshold error --tsconfig ./tsconfig.json 2>&1 | tail -15`
Expected: no errors referencing `accelerator.ts`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/stores/accelerator.ts frontend/wailsjs
git commit -m "feat(frontend): Accelerator-Z store + regenerated bindings"
```

---

## Task 7: Frontend route + nav wiring

**Files:**
- Create: `frontend/src/routes/Accelerator.svelte`
- Create: `frontend/src/routes/Accelerator.test.ts`
- Modify: `frontend/src/lib/stores/nav.ts` (add `'accelerator'` to `View`)
- Modify: `frontend/src/App.svelte` (import + route)
- Modify: `frontend/src/routes/Dashboard.svelte` (nav button)

**Interfaces:**
- Consumes: `accelerator` store (Task 6); `tx`/`awaitConfirm` (`lib/stores/tx.ts`); `view` (`lib/stores/nav.ts`); `formatAmount` (`lib/format.ts`); `TxModal`/`TxResult` components; `Nom.PrepareDonate/PrepareVote/PrepareCreateProject/PrepareAddPhase/PrepareUpdatePhase`.

- [ ] **Step 1: Add the view type** — `frontend/src/lib/stores/nav.ts`

```ts
export type View = 'dashboard' | 'send' | 'create' | 'import' | 'unlock' | 'settings' | 'plasma' | 'stake' | 'pillars' | 'sentinels' | 'tokens' | 'accelerator'
```

- [ ] **Step 2: Write the failing component test** — `frontend/src/routes/Accelerator.test.ts`

```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'

const mocks = vi.hoisted(() => ({
  GetProjects: vi.fn(), GetProject: vi.fn(), GetPhase: vi.fn(), GetVotablePillars: vi.fn(),
  PrepareDonate: vi.fn(), PrepareVote: vi.fn(),
  PrepareCreateProject: vi.fn(), PrepareAddPhase: vi.fn(), PrepareUpdatePhase: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NomService', () => mocks)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Accelerator from './Accelerator.svelte'

const PROJ = {
  id: '0xabc', owner: 'z1qme', name: 'My Project', description: 'd', url: 'https://x.org',
  znnFundsNeeded: '100', qsrFundsNeeded: '1000', creationTimestamp: 0, lastUpdateTimestamp: 0,
  status: 0, votes: { total: 3, yes: 2, no: 1 }, phases: [],
}

describe('Accelerator', () => {
  it('lists projects from GetProjects', async () => {
    mocks.GetProjects.mockResolvedValue({ count: 1, list: [PROJ] })
    mocks.GetVotablePillars.mockResolvedValue([])
    render(Accelerator)
    expect(await screen.findByText(/My Project/)).toBeTruthy()
  })

  it('hides the voting section when the address owns no pillars', async () => {
    mocks.GetProjects.mockResolvedValue({ count: 0, list: [] })
    mocks.GetVotablePillars.mockResolvedValue([])
    render(Accelerator)
    await screen.findByText(/Accelerator-Z/)
    expect(screen.queryByLabelText('vote target id')).toBeNull()
  })

  it('shows the voting section when the address owns a pillar', async () => {
    mocks.GetProjects.mockResolvedValue({ count: 0, list: [] })
    mocks.GetVotablePillars.mockResolvedValue(['MyPillar'])
    render(Accelerator)
    expect(await screen.findByLabelText('vote target id')).toBeTruthy()
  })
})
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `cd frontend && npx vitest run src/routes/Accelerator.test.ts`
Expected: FAIL — cannot resolve `./Accelerator.svelte`.

- [ ] **Step 4: Write the route** — `frontend/src/routes/Accelerator.svelte`

```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { projects, selectedProject, votablePillars, accError, loadProjects, openProject, loadVotablePillars } from '../lib/stores/accelerator'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let error = ''
  // donate
  let donateAmount = '', donateToken = 'QSR'
  // vote
  let voteId = '', votePillar = '', voteChoice = 0 // 0=yes,1=no,2=abstain (embedded.Vote*)
  // create project
  let cName = '', cDesc = '', cUrl = '', cZnn = '', cQsr = ''
  // add/update phase
  let phProjectOrPhaseId = '', phName = '', phDesc = '', phUrl = '', phZnn = '', phQsr = ''

  const STATUS = ['Voting', 'Active', 'Paid', 'Closed']
  function statusLabel(n: number) { return STATUS[n] ?? `#${n}` }

  onMount(() => { loadProjects(); loadVotablePillars() })
  $: if ($tx.status === 'done') { loadProjects(); loadVotablePillars() }
  $: if ($votablePillars.length > 0 && votePillar === '') votePillar = $votablePillars[0]

  function fail(e: any) { error = e?.message ?? String(e) }

  async function donate() {
    error = ''
    try { awaitConfirm((await Nom.PrepareDonate(donateAmount, donateToken)) as any) } catch (e) { fail(e) }
  }
  async function vote() {
    error = ''
    try { awaitConfirm((await Nom.PrepareVote(voteId, votePillar, voteChoice)) as any) } catch (e) { fail(e) }
  }
  async function createProject() {
    error = ''
    try { awaitConfirm((await Nom.PrepareCreateProject(cName, cDesc, cUrl, cZnn, cQsr)) as any) } catch (e) { fail(e) }
  }
  async function addPhase() {
    error = ''
    try { awaitConfirm((await Nom.PrepareAddPhase(phProjectOrPhaseId, phName, phDesc, phUrl, phZnn, phQsr)) as any) } catch (e) { fail(e) }
  }
  async function updatePhase() {
    error = ''
    try { awaitConfirm((await Nom.PrepareUpdatePhase(phProjectOrPhaseId, phName, phDesc, phUrl, phZnn, phQsr)) as any) } catch (e) { fail(e) }
  }
</script>

<div class="mx-auto mt-8 w-[44rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Accelerator-Z</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Projects</h2>
    {#each $projects as p}
      <div class="border-b border-muted/20 py-2 text-sm space-y-1">
        <p class="font-mono">{p.name} · {statusLabel(p.status)} · {formatAmount(p.znnFundsNeeded, 8)} ZNN / {formatAmount(p.qsrFundsNeeded, 8)} QSR</p>
        <p class="text-xs text-muted">votes: {p.votes.yes} yes / {p.votes.no} no / {p.votes.total} total</p>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => openProject(p.id)} aria-label={`open ${p.name}`}>Phases</button>
        {#if $selectedProject && $selectedProject.id === p.id}
          {#each $selectedProject.phases as ph}
            <div class="ml-4 mt-1 text-xs text-muted">
              {ph.name} · {statusLabel(ph.status)} · {ph.votes.yes}/{ph.votes.no}/{ph.votes.total} · <span class="font-mono">{ph.id}</span>
            </div>
          {:else}
            <p class="ml-4 mt-1 text-xs text-muted">No phases.</p>
          {/each}
        {/if}
      </div>
    {/each}
    {#if $projects.length === 0}<p class="text-xs text-muted">No projects.</p>{/if}
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Donate</h2>
    <div class="flex gap-2 items-center">
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="amount (base units)" bind:value={donateAmount} aria-label="donate amount" />
      <select class="rounded bg-bg px-2 py-1 text-sm" bind:value={donateToken} aria-label="donate token">
        <option value="ZNN">ZNN</option>
        <option value="QSR">QSR</option>
      </select>
      <button class="rounded bg-accent px-3 py-1 text-bg text-sm" on:click={donate}>Donate</button>
    </div>
  </section>

  {#if $votablePillars.length > 0}
  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Vote (Pillar operator)</h2>
    <div class="flex flex-wrap gap-2 items-center">
      <input class="rounded bg-bg px-2 py-1 text-sm w-80" placeholder="project or phase id (0x…)" bind:value={voteId} aria-label="vote target id" />
      <select class="rounded bg-bg px-2 py-1 text-sm" bind:value={votePillar} aria-label="vote pillar">
        {#each $votablePillars as name}<option value={name}>{name}</option>{/each}
      </select>
      <select class="rounded bg-bg px-2 py-1 text-sm" bind:value={voteChoice} aria-label="vote choice">
        <option value={0}>Yes</option>
        <option value={1}>No</option>
        <option value={2}>Abstain</option>
      </select>
      <button class="rounded bg-accent px-3 py-1 text-bg text-sm" on:click={vote}>Vote</button>
    </div>
  </section>
  {/if}

  <details class="rounded bg-surface p-4">
    <summary class="text-sm text-muted cursor-pointer">Create / manage</summary>
    <div class="mt-3 space-y-4">
      <div class="space-y-2">
        <h3 class="text-xs text-muted">Create project (1 ZNN fee)</h3>
        <div class="grid grid-cols-2 gap-2">
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="name" bind:value={cName} aria-label="create name" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="url" bind:value={cUrl} aria-label="create url" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="ZNN needed (base units)" bind:value={cZnn} aria-label="create znn" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="QSR needed (base units)" bind:value={cQsr} aria-label="create qsr" />
        </div>
        <input class="w-full rounded bg-bg px-2 py-1 text-sm" placeholder="description" bind:value={cDesc} aria-label="create description" />
        <button class="rounded bg-accent px-3 py-1 text-bg text-sm" on:click={createProject}>Create project</button>
      </div>
      <div class="space-y-2">
        <h3 class="text-xs text-muted">Add / update phase</h3>
        <input class="w-full rounded bg-bg px-2 py-1 text-sm" placeholder="project id (add) or phase id (update)" bind:value={phProjectOrPhaseId} aria-label="phase target id" />
        <div class="grid grid-cols-2 gap-2">
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="name" bind:value={phName} aria-label="phase name" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="url" bind:value={phUrl} aria-label="phase url" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="ZNN needed (base units)" bind:value={phZnn} aria-label="phase znn" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="QSR needed (base units)" bind:value={phQsr} aria-label="phase qsr" />
        </div>
        <input class="w-full rounded bg-bg px-2 py-1 text-sm" placeholder="description" bind:value={phDesc} aria-label="phase description" />
        <div class="flex gap-2">
          <button class="rounded border border-muted/40 px-3 py-1 text-sm" on:click={addPhase}>Add phase</button>
          <button class="rounded border border-muted/40 px-3 py-1 text-sm" on:click={updatePhase}>Update phase</button>
        </div>
      </div>
    </div>
  </details>

  {#if error || $accError}<p class="text-error text-sm" role="alert">{error || $accError}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
```

- [ ] **Step 5: Route the view** — `frontend/src/App.svelte`

Add the import next to the other route imports (after `Tokens` at line 16):

```svelte
  import Accelerator from './routes/Accelerator.svelte'
```

Add the route branch next to the `tokens` branch (after line 41):

```svelte
{:else if $view === 'accelerator'}
  <Accelerator />
```

- [ ] **Step 6: Add the nav button** — `frontend/src/routes/Dashboard.svelte`

Next to the Tokens button (line 53), add:

```svelte
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('accelerator')}>Accelerator</button>
```

- [ ] **Step 7: Run the component test to verify it passes**

Run: `cd frontend && npx vitest run src/routes/Accelerator.test.ts`
Expected: PASS (all three cases).

- [ ] **Step 8: Type-check the frontend**

Run: `cd frontend && npx svelte-check --threshold error --tsconfig ./tsconfig.json 2>&1 | tail -15`
Expected: 0 errors.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/routes/Accelerator.svelte frontend/src/routes/Accelerator.test.ts frontend/src/lib/stores/nav.ts frontend/src/App.svelte frontend/src/routes/Dashboard.svelte
git commit -m "feat(frontend): Accelerator-Z route (browse/donate/vote/manage) + nav"
```

---

## Task 8: Manual testnet acceptance + record

**Files:**
- Create: `docs/phase5f-acceptance.md`

This task is manual (mirrors `docs/phase5e-acceptance.md`). Use the testnet node `ws://172.245.236.40:35998` and an unlocked test wallet.

- [ ] **Step 1: Build and run** — `GOWORK=off wails dev`, connect to the testnet node, unlock the wallet.

- [ ] **Step 2: Browse** — open Accelerator-Z; confirm the project list loads and a project's "Phases" button expands its phases with vote tallies.

- [ ] **Step 3: Donate** — donate a small QSR amount (e.g. `100000000` base units = 1 QSR); confirm the TxModal renders "Donate 1 QSR to Accelerator-Z" and the block confirms on-chain.

- [ ] **Step 4: Vote (if a testnet pillar is available)** — with an address owning a testnet Pillar, cast one vote on a live proposal id; confirm the modal summary and on-chain result. If no pillar is available, note it as not-exercised.

- [ ] **Step 5: Create (optional, costs 1 ZNN)** — if testnet ZNN is available, create a throwaway project and verify it appears in the list. Otherwise note as not-exercised.

- [ ] **Step 6: Record** — write `docs/phase5f-acceptance.md` capturing what was exercised, tx hashes, and any deviations. Commit:

```bash
git add docs/phase5f-acceptance.md
git commit -m "docs: Phase 5f Accelerator-Z acceptance record"
```

---

## Finalization

After all tasks pass, finish the branch per the SDD workflow (superpowers:finishing-a-development-branch): squash-free merge of `phase-5f-accelerator` into `main` with a merge commit summarizing the phase, then update the Phase 5 memory and mark Accelerator-Z done in `plan.md` §3.

## Verification (end-to-end)

- Backend: `GOWORK=off go test ./app/... ` — all green, including the new `nom_accelerator_test.go`.
- Backend build: `GOWORK=off go build ./...` — clean.
- Frontend unit: `cd frontend && npx vitest run src/routes/Accelerator.test.ts` — green.
- Frontend types: `cd frontend && npx svelte-check --threshold error` — 0 errors.
- Manual: the Task 8 testnet checklist (browse + donate at minimum).
