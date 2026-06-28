# Accelerator (AZ) Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Accelerator-Z tab simple to navigate — a consolidated pillar "needs my vote" list (AZs + phases), filterable project browse (incl. awaiting-payout), clear create/donate, and a top-bar vote-notification badge.

**Architecture:** Frontend rework of the single `AcceleratorPanel` into a sub-tabbed container (Vote / Projects / Create / Donate), backed by two new read-only `NomService` methods (`GetActivePillarCount`, `GetVotableForMyPillars`) that wrap the SDK's `GetPillarVotes`. All existing write paths (`PrepareDonate`/`PrepareVote`/`PrepareCreateProject`/`PrepareAddPhase`/`PrepareUpdatePhase`) are reused unchanged. A top-bar ballot icon shows a badge counting items the operator's pillar hasn't voted on.

**Tech Stack:** Go + Wails v2 + znn-sdk-go v0.1.19 `AcceleratorApi`; Vue 3 + TS + Pinia + nom-ui; Vitest + @vue/test-utils.

## Global Constraints

- All `go`/`wails` commands run with `GOWORK=off GOTOOLCHAIN=auto`.
- **Commit convention (inherited):** gpg pinentry hangs non-interactively → implementers STAGE ONLY (`git add`, NO commit); the controller commits with `--no-gpg-sign`. Never stage `go.mod`/`go.sum` churn, the untracked `animation/`, or `.superpowers/`.
- **Bindings:** HAND-EDIT `frontend/wailsjs/**` per Task 3 (avoids go.mod toolchain churn). `frontend/wailsjs/**` is marked generated in `.gitattributes`.
- **NoM-confirm invariant:** every state-changing action routes `Nom.PrepareX(...)` → `tx.awaitConfirm(preview)` → global `NomConfirm` → `ConfirmPublish`. No write bypasses `tx.awaitConfirm`. **No new write methods** — reads only.
- **On-chain statuses** (`int`): `Voting=0, Active=1, Paid=2, Closed=3, Completed=4`.
- **Vote values:** `Yes=0, No=1, Abstain=2` (use `embedded.VoteYes/VoteNo/VoteAbstain` in Go; literals 0/1/2 in the frontend).
- **Votable AZ** = project `status==0` AND `creationTimestamp + AcceleratorProjectVotingPeriod (14 days) >= now`. **Votable phase** = `status==1` project's last phase when that phase `status==0`.
- **Vote-passing math (single source — `lib/accelerator.ts`):** `isPassing = yes > no && total*100 > numPillars*33`. `quorumNeeded = ceil(numPillars*0.33)`. `VoteAcceptanceThreshold = 33`.
- **Badge** counts items where `needsMyVote` (any owned pillar not yet voted).
- **Backend test command:** `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run '<Name>' -v`. **Frontend:** `cd frontend && pnpm exec vitest run <path>`; typecheck `cd frontend && pnpm run typecheck`.
- **Commit trailer (every commit):**
  ```
  Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
  ```

## File Structure

- `app/dto.go` — add `PillarVoteState`, `VotableItem`.
- `app/nom_accelerator.go` — add status consts, `buildVotableItems`, `annotateMyVotes`, `GetActivePillarCount`, `GetVotableForMyPillars`.
- `app/nom_accelerator_test.go` — new tests.
- `frontend/wailsjs/go/app/NomService.{d.ts,js}`, `frontend/wailsjs/go/models.ts` — new bindings + models.
- `frontend/src/lib/accelerator.ts` (+ `.test.ts`) — vote-math helpers.
- `frontend/src/stores/accelerator.ts` (+ `.test.ts`) — votable state + `needsVoteCount` + `refreshVotable`.
- `frontend/src/components/panels/AcceleratorVote.vue`, `AcceleratorProjects.vue`, `AcceleratorCreate.vue`, `AcceleratorDonate.vue` (+ `.test.ts` each) — the four sub-views.
- `frontend/src/components/panels/AcceleratorPanel.vue` (+ `.test.ts`) — container with sub-tabs.
- `frontend/src/components/TopBar.vue` (+ existing `.test.ts`) — ballot icon + badge.
- `frontend/src/views/Home.vue` — query→tab/sub + `refreshVotable` in `refresh()`.

---

### Task 1: Backend — votable DTOs + `buildVotableItems` pure helper

**Files:**
- Modify: `app/dto.go` (after `ProjectListDTO`)
- Modify: `app/nom_accelerator.go` (status consts + helper)
- Test: `app/nom_accelerator_test.go`

**Interfaces:**
- Produces: `PillarVoteState{Pillar string; Vote int}`, `VotableItem{Kind,Id,ProjectId,ProjectName,Name,ZnnFundsNeeded,QsrFundsNeeded string; Votes VoteBreakdownDTO; MyVotes []PillarVoteState; NeedsMyVote bool}`; `buildVotableItems(projects []*embedded.Project, nowUnix int64) []VotableItem`; status consts `statusVoting=0`, `statusActive=1`.

- [ ] **Step 1: Write the failing test**

Add to `app/nom_accelerator_test.go`:

```go
func TestBuildVotableItems(t *testing.T) {
	now := int64(1_000_000)
	openProj := &embedded.Project{
		Id:             types.HexToHashPanic("01" + strings32),
		Name:           "OpenAZ",
		Status:         0, // Voting
		CreationTimestamp: now - 10,
		ZnnFundsNeeded: big.NewInt(100), QsrFundsNeeded: big.NewInt(200),
		Votes:          &embedded.VoteBreakdown{Total: 1, Yes: 1, No: 0},
	}
	expiredProj := &embedded.Project{
		Id:     types.HexToHashPanic("02" + strings32),
		Name:   "ExpiredAZ", Status: 0,
		CreationTimestamp: now - int64(constants.AcceleratorProjectVotingPeriod) - 1,
		Votes:  &embedded.VoteBreakdown{},
	}
	activeWithOpenPhase := &embedded.Project{
		Id:     types.HexToHashPanic("03" + strings32),
		Name:   "ActiveAZ", Status: 1, // Active
		Phases: []*embedded.Phase{{
			Phase: &embedded.PhaseInfo{
				Id:     types.HexToHashPanic("04" + strings32),
				Name:   "PhaseOne", Status: 0, // Voting
				ZnnFundsNeeded: big.NewInt(5), QsrFundsNeeded: big.NewInt(6),
			},
			Votes: &embedded.VoteBreakdown{Total: 2, Yes: 1, No: 1},
		}},
	}
	activePaidPhase := &embedded.Project{
		Id: types.HexToHashPanic("05" + strings32), Name: "DoneAZ", Status: 1,
		Phases: []*embedded.Phase{{Phase: &embedded.PhaseInfo{Name: "Paid", Status: 2}, Votes: &embedded.VoteBreakdown{}}},
	}

	items := buildVotableItems([]*embedded.Project{openProj, expiredProj, activeWithOpenPhase, activePaidPhase}, now)
	if len(items) != 2 {
		t.Fatalf("expected 2 votable items (open project + open phase), got %d: %+v", len(items), items)
	}
	if items[0].Kind != "project" || items[0].Name != "OpenAZ" || items[0].Votes.Yes != 1 {
		t.Fatalf("project item wrong: %+v", items[0])
	}
	if items[1].Kind != "phase" || items[1].Name != "PhaseOne" || items[1].ProjectName != "ActiveAZ" {
		t.Fatalf("phase item wrong: %+v", items[1])
	}
	if items[1].ZnnFundsNeeded != "5" || items[1].Votes.Total != 2 {
		t.Fatalf("phase funds/votes wrong: %+v", items[1])
	}
	// Default annotation: no pillars yet → not flagged needs-vote.
	if items[0].NeedsMyVote || items[0].MyVotes != nil {
		t.Fatalf("expected unannotated item: %+v", items[0])
	}
}
```

Add this helper near the top of the test file (after the imports), used by the test above:

```go
// strings32 is 31 zero bytes (hex) so a 1-byte prefix forms a 32-byte hash.
var strings32 = "00000000000000000000000000000000000000000000000000000000000000"[:62]
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestBuildVotableItems' -v`
Expected: FAIL — `undefined: buildVotableItems` (and `VotableItem`).

- [ ] **Step 3: Add the DTOs**

In `app/dto.go`, after the `ProjectListDTO` struct:

```go
// PillarVoteState is one owned pillar's vote on a votable item; Vote == -1 means
// the pillar has not voted yet.
type PillarVoteState struct {
	Pillar string `json:"pillar"`
	Vote   int    `json:"vote"` // -1 not voted, 0 yes, 1 no, 2 abstain
}

// VotableItem is a project or phase currently open for pillar voting, annotated
// with the active address's owned-pillar vote state.
type VotableItem struct {
	Kind           string            `json:"kind"` // "project" | "phase"
	Id             string            `json:"id"`
	ProjectId      string            `json:"projectId"`
	ProjectName    string            `json:"projectName"`
	Name           string            `json:"name"`
	ZnnFundsNeeded string            `json:"znnFundsNeeded"`
	QsrFundsNeeded string            `json:"qsrFundsNeeded"`
	Votes          VoteBreakdownDTO  `json:"votes"`
	MyVotes        []PillarVoteState `json:"myVotes"`
	NeedsMyVote    bool              `json:"needsMyVote"`
}
```

- [ ] **Step 4: Add the status consts + `buildVotableItems`**

In `app/nom_accelerator.go`, add after the imports:

```go
// Accelerator-Z project/phase status values (mirrors go-zenon definition).
const (
	statusVoting    = 0
	statusActive    = 1
	statusPaid      = 2
	statusClosed    = 3
	statusCompleted = 4
)

// buildVotableItems returns the currently votable projects (Voting + 14-day
// window open) and the open current phase of each Active project, mapped to
// VotableItems WITHOUT per-pillar annotation. nowUnix is the reference time for
// the voting-window check. Pure: no node I/O.
func buildVotableItems(projects []*embedded.Project, nowUnix int64) []VotableItem {
	out := make([]VotableItem, 0)
	for _, p := range projects {
		if p == nil {
			continue
		}
		switch int(p.Status) {
		case statusVoting:
			if p.CreationTimestamp+int64(constants.AcceleratorProjectVotingPeriod) >= nowUnix {
				out = append(out, VotableItem{
					Kind:           "project",
					Id:             p.Id.String(),
					ProjectId:      p.Id.String(),
					ProjectName:    p.Name,
					Name:           p.Name,
					ZnnFundsNeeded: bigStr(p.ZnnFundsNeeded),
					QsrFundsNeeded: bigStr(p.QsrFundsNeeded),
					Votes:          voteBreakdownDTO(p.Votes),
				})
			}
		case statusActive:
			if len(p.Phases) == 0 {
				continue
			}
			cur := p.Phases[len(p.Phases)-1]
			if cur == nil || cur.Phase == nil || int(cur.Phase.Status) != statusVoting {
				continue
			}
			out = append(out, VotableItem{
				Kind:           "phase",
				Id:             cur.Phase.Id.String(),
				ProjectId:      p.Id.String(),
				ProjectName:    p.Name,
				Name:           cur.Phase.Name,
				ZnnFundsNeeded: bigStr(cur.Phase.ZnnFundsNeeded),
				QsrFundsNeeded: bigStr(cur.Phase.QsrFundsNeeded),
				Votes:          voteBreakdownDTO(cur.Votes),
			})
		}
	}
	return out
}
```

> Note: `constants.AcceleratorProjectVotingPeriod` is the 14-day window (`14 * PhaseTimeUnit`, in seconds). If the symbol name differs in the pinned go-zenon, grep `vm/constants` for `AcceleratorProject.*Period` and use the exact name; it is a seconds value.

- [ ] **Step 5: Run test to verify it passes**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestBuildVotableItems' -v`
Expected: PASS. Also `GOWORK=off GOTOOLCHAIN=auto go build ./app/` → clean.

- [ ] **Step 6: Stage (do not commit)**

```bash
git add app/dto.go app/nom_accelerator.go app/nom_accelerator_test.go
```
Then STOP — controller commits.

---

### Task 2: Backend — `GetActivePillarCount`, `GetVotableForMyPillars`, `annotateMyVotes`

**Files:**
- Modify: `app/nom_accelerator.go` (add `time` + `definition` imports; helper + 2 methods)
- Test: `app/nom_accelerator_test.go`

**Interfaces:**
- Consumes: `buildVotableItems`, `VotableItem`, `PillarVoteState` (Task 1).
- Produces: `annotateMyVotes(items []VotableItem, pillarName string, votes []*definition.PillarVote)`; `(*NomService) GetActivePillarCount() (int, error)`; `(*NomService) GetVotableForMyPillars() ([]VotableItem, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/nom_accelerator_test.go`:

```go
func TestAnnotateMyVotes(t *testing.T) {
	idA := types.HexToHashPanic("0a" + strings32)
	idB := types.HexToHashPanic("0b" + strings32)
	items := []VotableItem{{Id: idA.String()}, {Id: idB.String()}}
	// Pillar voted "no" (1) on A only; B is nil/absent → not voted.
	votes := []*definition.PillarVote{{Id: idA, Name: "MyPillar", Vote: 1}, nil}
	annotateMyVotes(items, "MyPillar", votes)
	if len(items[0].MyVotes) != 1 || items[0].MyVotes[0].Pillar != "MyPillar" || items[0].MyVotes[0].Vote != 1 {
		t.Fatalf("A should be voted no by MyPillar: %+v", items[0])
	}
	if items[0].NeedsMyVote {
		t.Fatalf("A is voted, must not need vote: %+v", items[0])
	}
	if items[1].MyVotes[0].Vote != -1 || !items[1].NeedsMyVote {
		t.Fatalf("B unvoted must be -1 + needsMyVote: %+v", items[1])
	}
}

func TestAcceleratorVoteReadsGuard(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// Not connected (test node has no client) → both reads error, no panic.
	if _, err := s.GetActivePillarCount(); err == nil {
		t.Fatal("GetActivePillarCount must error when not connected")
	}
	if _, err := s.GetVotableForMyPillars(); err == nil {
		t.Fatal("GetVotableForMyPillars must error when not connected/locked")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestAnnotateMyVotes|TestAcceleratorVoteReadsGuard' -v`
Expected: FAIL — `undefined: annotateMyVotes` / `GetActivePillarCount` / `GetVotableForMyPillars`.

- [ ] **Step 3: Add imports**

In `app/nom_accelerator.go`, extend the import block with `"time"` and the go-zenon definition package:

```go
	"time"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
	definition "github.com/zenon-network/go-zenon/vm/embedded/definition"
	constants "github.com/zenon-network/go-zenon/vm/constants"
```
(Keep existing imports; add `time` in the stdlib group and `definition` alongside the other go-zenon imports.)

- [ ] **Step 4: Add the helper + methods**

Append to `app/nom_accelerator.go`:

```go
// annotateMyVotes records, for one pillar, its vote on each item (or -1 = not
// voted), and flags items the pillar still needs to vote on. votes is the result
// of GetPillarVotes(pillarName, ids); entries may be nil (no vote) and are keyed
// by their Id so ordering is irrelevant.
func annotateMyVotes(items []VotableItem, pillarName string, votes []*definition.PillarVote) {
	voted := make(map[string]int, len(votes))
	for _, v := range votes {
		if v != nil {
			voted[v.Id.String()] = int(v.Vote)
		}
	}
	for i := range items {
		vote := -1
		if vv, ok := voted[items[i].Id]; ok {
			vote = vv
		}
		items[i].MyVotes = append(items[i].MyVotes, PillarVoteState{Pillar: pillarName, Vote: vote})
		if vote == -1 {
			items[i].NeedsMyVote = true
		}
	}
}

// GetActivePillarCount returns the number of pillars (for AZ quorum math). Uses
// the AcceleratorApi/PillarApi page count.
func (s *NomService) GetActivePillarCount() (int, error) {
	client := s.node.currentClient()
	if client == nil {
		return 0, errors.New("not connected")
	}
	list, err := client.PillarApi.GetAll(0, 1)
	if err != nil {
		return 0, err
	}
	return list.Count, nil
}

// GetVotableForMyPillars returns the items currently open for voting (projects in
// their voting window + Active projects' open phases), annotated with the vote
// state of each pillar the active address owns. Drives the Vote view + the
// top-bar badge. Read-only.
func (s *NomService) GetVotableForMyPillars() ([]VotableItem, error) {
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}

	// Collect all projects (page through GetAll).
	all := make([]*embedded.Project, 0)
	var pageIndex uint32 = 0
	const pageSize uint32 = 50
	for {
		list, err := client.AcceleratorApi.GetAll(pageIndex, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, list.List...)
		if len(all) >= list.Count || len(list.List) == 0 {
			break
		}
		pageIndex++
	}

	items := buildVotableItems(all, time.Now().Unix())
	if len(items) == 0 {
		return items, nil
	}

	// Annotate with each owned pillar's vote across all votable ids.
	pillars, err := client.PillarApi.GetByOwner(addr)
	if err != nil {
		return nil, err
	}
	if len(pillars) == 0 {
		return items, nil // no pillar → nothing actionable; NeedsMyVote stays false
	}
	ids := make([]types.Hash, 0, len(items))
	for _, it := range items {
		if h, err := types.HexToHash(it.Id); err == nil {
			ids = append(ids, h)
		}
	}
	for _, p := range pillars {
		votes, err := client.AcceleratorApi.GetPillarVotes(p.Name, ids)
		if err != nil {
			return nil, err
		}
		annotateMyVotes(items, p.Name, votes)
	}
	return items, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestAnnotateMyVotes|TestAcceleratorVoteReadsGuard|TestBuildVotableItems' -v`
Expected: PASS. Then `GOWORK=off GOTOOLCHAIN=auto go test ./app/` (full package — the 2 local `secrets/` keystore tests may fail, that's pre-existing/environmental) and `GOWORK=off GOTOOLCHAIN=auto go vet ./app/` → no vet errors. Confirm no NEW failures beyond the known `TestImportListUnlockLock` / `TestSigningKeyPairMatchesActiveAddress`.

- [ ] **Step 6: Stage**

```bash
git add app/nom_accelerator.go app/nom_accelerator_test.go
```
STOP — controller commits.

---

### Task 3: Wails bindings for the new read methods + models

**Files:**
- Modify: `frontend/wailsjs/go/app/NomService.d.ts`
- Modify: `frontend/wailsjs/go/app/NomService.js`
- Modify: `frontend/wailsjs/go/models.ts`

**Interfaces:**
- Produces: TS `GetActivePillarCount(): Promise<number>`, `GetVotableForMyPillars(): Promise<Array<app.VotableItem>>`; models `app.VotableItem`, `app.PillarVoteState`.

> Match the generated format exactly (read neighboring entries first). Placement within the file is not load-bearing.

- [ ] **Step 1: Add `.d.ts` declarations**

In `frontend/wailsjs/go/app/NomService.d.ts` add:

```ts
export function GetActivePillarCount():Promise<number>;

export function GetVotableForMyPillars():Promise<Array<app.VotableItem>>;
```

- [ ] **Step 2: Add `.js` bindings**

In `frontend/wailsjs/go/app/NomService.js` add:

```js
export function GetActivePillarCount() {
  return window['go']['app']['NomService']['GetActivePillarCount']();
}

export function GetVotableForMyPillars() {
  return window['go']['app']['NomService']['GetVotableForMyPillars']();
}
```

- [ ] **Step 3: Add the model classes**

In `frontend/wailsjs/go/models.ts`, inside `export namespace app {`, add (near `VoteBreakdownDTO`):

```ts
	export class PillarVoteState {
	    pillar: string;
	    vote: number;

	    static createFrom(source: any = {}) {
	        return new PillarVoteState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pillar = source["pillar"];
	        this.vote = source["vote"];
	    }
	}
	export class VotableItem {
	    kind: string;
	    id: string;
	    projectId: string;
	    projectName: string;
	    name: string;
	    znnFundsNeeded: string;
	    qsrFundsNeeded: string;
	    votes: VoteBreakdownDTO;
	    myVotes: PillarVoteState[];
	    needsMyVote: boolean;

	    static createFrom(source: any = {}) {
	        return new VotableItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.projectName = source["projectName"];
	        this.name = source["name"];
	        this.znnFundsNeeded = source["znnFundsNeeded"];
	        this.qsrFundsNeeded = source["qsrFundsNeeded"];
	        this.votes = this.convertValues(source["votes"], VoteBreakdownDTO);
	        this.myVotes = this.convertValues(source["myVotes"], PillarVoteState);
	        this.needsMyVote = source["needsMyVote"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
```

- [ ] **Step 4: Verify typecheck**

Run: `cd frontend && pnpm run typecheck`
Expected: PASS.

- [ ] **Step 5: Stage**

```bash
git add frontend/wailsjs/go/app/NomService.d.ts frontend/wailsjs/go/app/NomService.js frontend/wailsjs/go/models.ts
```
STOP — controller commits.

---

### Task 4: Frontend — `lib/accelerator.ts` vote-math helpers

**Files:**
- Create: `frontend/src/lib/accelerator.ts`
- Test: `frontend/src/lib/accelerator.test.ts`

**Interfaces:**
- Produces: `isPassing(yes,no,total,numPillars): boolean`, `quorumNeeded(numPillars): number`, `statusLabel(n): string`, `AZ_STATUS: readonly string[]`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/accelerator.test.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { isPassing, quorumNeeded, statusLabel } from './accelerator'

describe('accelerator vote math', () => {
  it('isPassing requires strict majority AND >33% turnout', () => {
    // 4 yes, 1 no, 5 total, 10 pillars → 5*100=500 > 10*33=330 ✓, 4>1 ✓
    expect(isPassing(4, 1, 5, 10)).toBe(true)
    // tie fails majority
    expect(isPassing(2, 2, 5, 10)).toBe(false)
    // below quorum: 3 total of 10 pillars → 300 <= 330
    expect(isPassing(3, 0, 3, 10)).toBe(false)
    // exactly 33% fails (strict >): 33 total, 100 pillars → 3300 <= 3300
    expect(isPassing(33, 0, 33, 100)).toBe(false)
    // just over: 34 total, 100 pillars → 3400 > 3300
    expect(isPassing(34, 0, 34, 100)).toBe(true)
  })
  it('quorumNeeded is ceil(33%)', () => {
    expect(quorumNeeded(100)).toBe(33)
    expect(quorumNeeded(10)).toBe(4) // ceil(3.3)
    expect(quorumNeeded(0)).toBe(0)
  })
  it('statusLabel maps known statuses', () => {
    expect(statusLabel(0)).toBe('Voting')
    expect(statusLabel(1)).toBe('Active')
    expect(statusLabel(4)).toBe('Completed')
    expect(statusLabel(9)).toBe('#9')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/lib/accelerator.test.ts`
Expected: FAIL — cannot resolve `./accelerator`.

- [ ] **Step 3: Implement**

Create `frontend/src/lib/accelerator.ts`:

```ts
// Accelerator-Z vote math — the single source of truth shared by the Vote view,
// the Projects "awaiting payout" filter, and the quorum bar. Mirrors go-zenon's
// checkAcceleratorVotes: strict majority (yes>no) AND turnout above 33% of the
// active pillar count.
export const AZ_STATUS = ['Voting', 'Active', 'Paid', 'Closed', 'Completed'] as const

export function statusLabel(n: number): string {
  return AZ_STATUS[n] ?? `#${n}`
}

export function quorumNeeded(numPillars: number): number {
  return Math.ceil(numPillars * 0.33)
}

export function isPassing(yes: number, no: number, total: number, numPillars: number): boolean {
  return yes > no && total * 100 > numPillars * 33
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/lib/accelerator.test.ts`
Expected: PASS.

- [ ] **Step 5: Stage**

```bash
git add frontend/src/lib/accelerator.ts frontend/src/lib/accelerator.test.ts
```
STOP — controller commits.

---

### Task 5: Frontend — extend the accelerator store

**Files:**
- Modify: `frontend/src/stores/accelerator.ts`
- Test: `frontend/src/stores/accelerator.test.ts` (new)

**Interfaces:**
- Consumes: bindings `GetVotableForMyPillars`, `GetActivePillarCount` (Task 3).
- Produces: store state `votable: app.VotableItem[]`, `numActivePillars: number`; getter `needsVoteCount`; action `refreshVotable()`. Keeps existing `projects`/`selectedProject`/`votablePillars`/`error` + `loadProjects`/`openProject`/`loadVotablePillars`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/stores/accelerator.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAcceleratorStore } from './accelerator'

const GetVotableForMyPillars = vi.hoisted(() => vi.fn())
const GetActivePillarCount = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetVotableForMyPillars, GetActivePillarCount,
  GetProjects: vi.fn(), GetProject: vi.fn(), GetVotablePillars: vi.fn(),
}))

beforeEach(() => setActivePinia(createPinia()))

describe('accelerator store votable', () => {
  it('refreshVotable populates state and needsVoteCount counts needs-vote items', async () => {
    GetVotableForMyPillars.mockResolvedValue([
      { id: '0xa', needsMyVote: true },
      { id: '0xb', needsMyVote: false },
      { id: '0xc', needsMyVote: true },
    ])
    GetActivePillarCount.mockResolvedValue(42)
    const s = useAcceleratorStore()
    await s.refreshVotable()
    expect(s.votable.length).toBe(3)
    expect(s.numActivePillars).toBe(42)
    expect(s.needsVoteCount).toBe(2)
  })

  it('refreshVotable swallows errors (locked/disconnected → empty)', async () => {
    GetVotableForMyPillars.mockRejectedValue(new Error('locked'))
    GetActivePillarCount.mockResolvedValue(0)
    const s = useAcceleratorStore()
    await s.refreshVotable()
    expect(s.votable).toEqual([])
    expect(s.needsVoteCount).toBe(0)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/stores/accelerator.test.ts`
Expected: FAIL — `refreshVotable` / `needsVoteCount` undefined.

- [ ] **Step 3: Extend the store**

Replace `frontend/src/stores/accelerator.ts` with:

```ts
import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const useAcceleratorStore = defineStore('accelerator', {
  state: () => ({
    projects: [] as app.ProjectDTO[],
    selectedProject: null as app.ProjectDTO | null,
    votablePillars: [] as string[],
    votable: [] as app.VotableItem[],
    numActivePillars: 0,
    error: '',
  }),
  getters: {
    needsVoteCount(state): number {
      return state.votable.filter((v) => v.needsMyVote).length
    },
  },
  actions: {
    async loadProjects(page = 0) {
      this.error = ''
      try {
        const list = await Nom.GetProjects(page, 20)
        this.projects = list.list ?? []
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      }
    },
    async openProject(id: string) {
      this.error = ''
      try {
        this.selectedProject = await Nom.GetProject(id)
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
    // Votable items for the active address's pillars + active pillar count, for
    // the Vote view and the top-bar badge. Swallows errors (badge shows 0).
    async refreshVotable() {
      try {
        this.votable = await Nom.GetVotableForMyPillars()
        this.numActivePillars = await Nom.GetActivePillarCount()
      } catch {
        this.votable = []
      }
    },
  },
})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/stores/accelerator.test.ts`
Expected: PASS. Then `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 5: Stage**

```bash
git add frontend/src/stores/accelerator.ts frontend/src/stores/accelerator.test.ts
```
STOP — controller commits.

---

### Task 6: Frontend — `AcceleratorVote.vue`

**Files:**
- Create: `frontend/src/components/panels/AcceleratorVote.vue`
- Test: `frontend/src/components/panels/AcceleratorVote.test.ts`

**Interfaces:**
- Consumes: accelerator store (`votable`, `votablePillars`, `numActivePillars`, `refreshVotable`); pillar store `ownsPillar`; `tx.awaitConfirm`; `Nom.PrepareVote`; `lib/accelerator` (`isPassing`, `quorumNeeded`); `lib/format` (`formatAmount`).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/panels/AcceleratorVote.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareVote: vi.fn(() => Promise.resolve({ kind: 'vote' })),
}))

import AcceleratorVote from './AcceleratorVote.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'

function setup(opts: { ownsPillar?: boolean } = {}) {
  setActivePinia(createPinia())
  const acc = useAcceleratorStore()
  const pillar = usePillarStore()
  const tx = useTxStore()
  vi.spyOn(acc, 'refreshVotable').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  acc.votablePillars = ['MyPillar']
  acc.numActivePillars = 10
  acc.votable = [
    { kind: 'project', id: '0xp1', projectId: '0xp1', projectName: 'AZ-One', name: 'AZ-One',
      znnFundsNeeded: '100000000', qsrFundsNeeded: '0',
      votes: { yes: 1, no: 0, total: 1 }, myVotes: [{ pillar: 'MyPillar', vote: -1 }], needsMyVote: true },
    { kind: 'phase', id: '0xph', projectId: '0xp2', projectName: 'AZ-Two', name: 'Phase-1',
      znnFundsNeeded: '0', qsrFundsNeeded: '0',
      votes: { yes: 0, no: 0, total: 0 }, myVotes: [{ pillar: 'MyPillar', vote: 0 }], needsMyVote: false },
  ] as never
  pillar.myPillar = (opts.ownsPillar === false ? null : { name: 'MyPillar' }) as never
  return { acc, tx, awaitConfirm }
}

describe('AcceleratorVote', () => {
  it('shows a pillar-operator note when no pillar is owned', () => {
    setup({ ownsPillar: false })
    const w = mount(AcceleratorVote)
    expect(w.text().toLowerCase()).toContain('pillar operators')
  })

  it('lists only items the selected pillar has not voted on (default)', () => {
    setup()
    const w = mount(AcceleratorVote)
    expect(w.text()).toContain('AZ-One')   // not voted → shown
    expect(w.text()).not.toContain('Phase-1') // already voted → hidden by default
  })

  it('forwards a Yes vote with (id, pillar, 0)', async () => {
    const { awaitConfirm } = setup()
    const w = mount(AcceleratorVote)
    await w.find('button[aria-label="vote yes 0xp1"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareVote).toHaveBeenCalledWith('0xp1', 'MyPillar', 0)
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'vote' })
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorVote.test.ts`
Expected: FAIL — cannot resolve `./AcceleratorVote.vue`.

- [ ] **Step 3: Create the component**

Create `frontend/src/components/panels/AcceleratorVote.vue`:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import { isPassing, quorumNeeded } from '../../lib/accelerator'
import type { app } from '../../../wailsjs/go/models'

const acc = useAcceleratorStore()
const pillar = usePillarStore()
const tx = useTxStore()
const { votable, votablePillars, numActivePillars } = storeToRefs(acc)
const { ownsPillar } = storeToRefs(pillar)
const error = ref('')

const selectedPillar = ref('')
const showAll = ref(false)
watch(
  votablePillars,
  (list) => {
    if (list.length && !selectedPillar.value) selectedPillar.value = list[0]
  },
  { immediate: true },
)

function myVote(item: app.VotableItem): number {
  const e = item.myVotes?.find((m) => m.pillar === selectedPillar.value)
  return e ? e.vote : -1
}
const VOTE_LABELS = ['yes', 'no', 'abstain']
const items = computed(() =>
  showAll.value ? votable.value : votable.value.filter((it) => myVote(it) === -1),
)

async function vote(id: string, choice: number) {
  error.value = ''
  if (!selectedPillar.value) {
    error.value = 'Select a pillar to vote as.'
    return
  }
  try {
    tx.awaitConfirm(await Nom.PrepareVote(id, selectedPillar.value, choice))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

watch(
  () => tx.status,
  (s) => {
    if (s === 'done') acc.refreshVotable()
  },
)
</script>

<template>
  <div class="space-y-3 p-4">
    <p v-if="!ownsPillar" class="text-sm text-muted-foreground">
      Voting on Accelerator-Z proposals is for pillar operators. Register or run a pillar to vote.
    </p>
    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-2">
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
        <label class="flex items-center gap-2 text-xs text-muted-foreground">
          <input v-model="showAll" type="checkbox" aria-label="show all votable" />
          Show items I've already voted on
        </label>
      </div>

      <p v-if="items.length === 0" class="text-sm text-muted-foreground">
        Nothing awaiting your vote right now.
      </p>

      <div
        v-for="it in items"
        :key="it.id"
        class="space-y-2 rounded-lg border border-border bg-card p-3"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">{{ it.kind }}</span>
          <span class="text-sm font-medium text-foreground">{{ it.name }}</span>
          <span v-if="it.kind === 'phase'" class="text-xs text-muted-foreground">· {{ it.projectName }}</span>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ formatAmount(it.znnFundsNeeded, 8) }} ZNN / {{ formatAmount(it.qsrFundsNeeded, 8) }} QSR
        </p>
        <p class="text-xs text-muted-foreground">
          {{ it.votes.yes }} yes · {{ it.votes.no }} no · {{ it.votes.total }} votes
          (quorum {{ quorumNeeded(numActivePillars) }})
          <span
            v-if="isPassing(it.votes.yes, it.votes.no, it.votes.total, numActivePillars)"
            class="text-primary"
          > · passing</span>
        </p>
        <p v-if="myVote(it) !== -1" class="text-xs text-primary">
          You voted: {{ VOTE_LABELS[myVote(it)] }} (you can change it)
        </p>
        <div class="flex flex-wrap gap-2">
          <Button :aria-label="`vote yes ${it.id}`" @click="vote(it.id, 0)">Yes</Button>
          <Button variant="outline" :aria-label="`vote no ${it.id}`" @click="vote(it.id, 1)">No</Button>
          <Button variant="outline" :aria-label="`vote abstain ${it.id}`" @click="vote(it.id, 2)">Abstain</Button>
        </div>
      </div>
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorVote.test.ts`
Expected: PASS. Then `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 5: Stage**

```bash
git add frontend/src/components/panels/AcceleratorVote.vue frontend/src/components/panels/AcceleratorVote.test.ts
```
STOP — controller commits.

---

### Task 7: Frontend — `AcceleratorProjects.vue`

**Files:**
- Create: `frontend/src/components/panels/AcceleratorProjects.vue`
- Test: `frontend/src/components/panels/AcceleratorProjects.test.ts`

**Interfaces:**
- Consumes: accelerator store (`projects`, `numActivePillars`); `lib/accelerator` (`statusLabel`, `isPassing`); `lib/format` (`formatAmount`); nom-ui `Button`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/panels/AcceleratorProjects.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
}))

import AcceleratorProjects from './AcceleratorProjects.vue'
import { useAcceleratorStore } from '../../stores/accelerator'

function setup() {
  setActivePinia(createPinia())
  const acc = useAcceleratorStore()
  acc.numActivePillars = 10
  acc.projects = [
    { id: '0xv', name: 'VotingAZ', status: 0, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 0, no: 0, total: 0 }, phases: [] },
    { id: '0xa', name: 'ActiveAZ', status: 1, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 0, no: 0, total: 0 },
      phases: [{ id: '0xph', name: 'Ph', status: 0, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 8, no: 0, total: 8 } }] },
    { id: '0xc', name: 'DoneAZ', status: 4, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 0, no: 0, total: 0 }, phases: [] },
  ] as never
  return { acc }
}

describe('AcceleratorProjects filters', () => {
  it('shows all by default', () => {
    setup()
    const w = mount(AcceleratorProjects)
    expect(w.text()).toContain('VotingAZ')
    expect(w.text()).toContain('ActiveAZ')
    expect(w.text()).toContain('DoneAZ')
  })

  it('"Active AZs" filter shows only Voting projects', async () => {
    setup()
    const w = mount(AcceleratorProjects)
    await w.find('button[aria-label="filter Voting"]').trigger('click')
    expect(w.text()).toContain('VotingAZ')
    expect(w.text()).not.toContain('DoneAZ')
  })

  it('"Awaiting payout" shows the active project whose phase passes the vote', async () => {
    setup()
    const w = mount(AcceleratorProjects)
    await w.find('button[aria-label="filter Awaiting payout"]').trigger('click')
    // ActiveAZ's phase: 8 yes of 8 total, 10 pillars → 800>330 ✓, passing
    expect(w.text()).toContain('ActiveAZ')
    expect(w.text()).not.toContain('VotingAZ')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorProjects.test.ts`
Expected: FAIL — cannot resolve `./AcceleratorProjects.vue`.

- [ ] **Step 3: Create the component**

Create `frontend/src/components/panels/AcceleratorProjects.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useAcceleratorStore } from '../../stores/accelerator'
import { formatAmount } from '../../lib/format'
import { statusLabel, isPassing } from '../../lib/accelerator'
import type { app } from '../../../wailsjs/go/models'

const acc = useAcceleratorStore()
const { projects, numActivePillars } = storeToRefs(acc)

const FILTERS = ['All', 'Voting', 'Active', 'Awaiting payout', 'Completed', 'Rejected'] as const
type Filter = (typeof FILTERS)[number]
const filter = ref<Filter>('All')
const expanded = ref<string | null>(null)

function currentPhase(p: app.ProjectDTO): app.PhaseDTO | null {
  return p.phases && p.phases.length ? p.phases[p.phases.length - 1] : null
}
function phasePassing(ph: app.PhaseDTO): boolean {
  return ph.status === 0 && isPassing(ph.votes.yes, ph.votes.no, ph.votes.total, numActivePillars.value)
}
function awaitingPayout(p: app.ProjectDTO): boolean {
  const ph = currentPhase(p)
  return !!ph && phasePassing(ph)
}
const filtered = computed(() =>
  (projects.value ?? []).filter((p) => {
    switch (filter.value) {
      case 'Voting': return p.status === 0
      case 'Active': return p.status === 1
      case 'Completed': return p.status === 4
      case 'Rejected': return p.status === 3
      case 'Awaiting payout': return awaitingPayout(p)
      default: return true
    }
  }),
)
function toggle(id: string) {
  expanded.value = expanded.value === id ? null : id
}
function label(f: Filter): string {
  return f === 'Voting' ? 'Active AZs' : f
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
      >{{ label(f) }}</button>
    </div>

    <p v-if="filtered.length === 0" class="text-sm text-muted-foreground">No matching projects.</p>

    <div
      v-for="p in filtered"
      :key="p.id"
      class="space-y-1 rounded-lg border border-border bg-card p-3 text-sm"
    >
      <div class="flex items-center justify-between gap-2">
        <span class="font-medium text-foreground">{{ p.name }}</span>
        <span class="text-xs text-muted-foreground"
          >{{ statusLabel(p.status) }}<span v-if="awaitingPayout(p)" class="text-primary"> · awaiting payout</span></span
        >
      </div>
      <p class="text-xs text-muted-foreground">
        {{ formatAmount(p.znnFundsNeeded, 8) }} ZNN / {{ formatAmount(p.qsrFundsNeeded, 8) }} QSR ·
        {{ p.votes.yes }}/{{ p.votes.no }}/{{ p.votes.total }}
      </p>
      <Button variant="outline" class="px-2 py-1 text-xs" :aria-label="`phases ${p.name}`" @click="toggle(p.id)">
        {{ expanded === p.id ? 'Hide phases' : 'Phases' }}
      </Button>
      <template v-if="expanded === p.id">
        <div v-for="ph in p.phases" :key="ph.id" class="ml-3 mt-1 text-xs text-muted-foreground">
          {{ ph.name }} · {{ statusLabel(ph.status) }} · {{ ph.votes.yes }}/{{ ph.votes.no }}/{{ ph.votes.total }}
          <span v-if="phasePassing(ph)" class="text-primary"> · awaiting payout</span>
        </div>
        <p v-if="!p.phases || p.phases.length === 0" class="ml-3 mt-1 text-xs text-muted-foreground">No phases.</p>
      </template>
    </div>
  </div>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorProjects.test.ts`
Expected: PASS. Then `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 5: Stage**

```bash
git add frontend/src/components/panels/AcceleratorProjects.vue frontend/src/components/panels/AcceleratorProjects.test.ts
```
STOP — controller commits.

---

### Task 8: Frontend — extract `AcceleratorDonate.vue` + `AcceleratorCreate.vue`

**Files:**
- Create: `frontend/src/components/panels/AcceleratorDonate.vue` (+ `.test.ts`)
- Create: `frontend/src/components/panels/AcceleratorCreate.vue` (+ `.test.ts`)

**Interfaces:**
- Consumes: accelerator store (`loadProjects`), `tx.awaitConfirm`, `Nom.PrepareDonate`/`PrepareCreateProject`/`PrepareAddPhase`/`PrepareUpdatePhase`, nom-ui `Input`/`Button`, `Field`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/panels/AcceleratorDonate.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDonate: vi.fn(() => Promise.resolve({ kind: 'donate' })),
}))

import AcceleratorDonate from './AcceleratorDonate.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useTxStore } from '../../stores/tx'

describe('AcceleratorDonate', () => {
  it('forwards the donate call', async () => {
    setActivePinia(createPinia())
    const tx = useTxStore()
    const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
    const w = mount(AcceleratorDonate)
    await w.find('input[aria-label="donate amount"]').setValue('100000000')
    await w.find('button[aria-label="donate"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareDonate).toHaveBeenCalledWith('100000000', 'QSR')
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'donate' })
  })
})
```

Create `frontend/src/components/panels/AcceleratorCreate.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareCreateProject: vi.fn(() => Promise.resolve({ kind: 'create' })),
  PrepareAddPhase: vi.fn(() => Promise.resolve({ kind: 'addPhase' })),
  PrepareUpdatePhase: vi.fn(() => Promise.resolve({ kind: 'updatePhase' })),
}))

import AcceleratorCreate from './AcceleratorCreate.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useTxStore } from '../../stores/tx'

describe('AcceleratorCreate', () => {
  it('forwards create + add-phase calls with the form fields', async () => {
    setActivePinia(createPinia())
    const tx = useTxStore()
    vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
    const w = mount(AcceleratorCreate)
    await w.find('input[aria-label="create name"]').setValue('Proj')
    await w.find('input[aria-label="create url"]').setValue('https://x.io')
    await w.find('input[aria-label="create znn"]').setValue('100')
    await w.find('input[aria-label="create qsr"]').setValue('200')
    await w.find('input[aria-label="create description"]').setValue('desc')
    await w.find('button[aria-label="create project"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareCreateProject).toHaveBeenCalledWith('Proj', 'desc', 'https://x.io', '100', '200')

    await w.find('input[aria-label="project id"]').setValue('0xabc')
    await w.find('input[aria-label="phase name"]').setValue('Ph1')
    await w.find('input[aria-label="phase url"]').setValue('https://y.io')
    await w.find('input[aria-label="phase znn"]').setValue('10')
    await w.find('input[aria-label="phase qsr"]').setValue('20')
    await w.find('input[aria-label="phase description"]').setValue('pdesc')
    await w.find('button[aria-label="add phase"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareAddPhase).toHaveBeenCalledWith('0xabc', 'Ph1', 'pdesc', 'https://y.io', '10', '20')
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorDonate.test.ts src/components/panels/AcceleratorCreate.test.ts`
Expected: FAIL — cannot resolve the two `.vue` files.

- [ ] **Step 3: Create `AcceleratorDonate.vue`**

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useTxStore } from '../../stores/tx'
import Field from '../Field.vue'

const acc = useAcceleratorStore()
const tx = useTxStore()
const amount = ref('')
const token = ref('QSR')
const error = ref('')

async function donate() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareDonate(amount.value, token.value))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') acc.loadProjects()
  },
)
</script>

<template>
  <div class="space-y-3 p-4">
    <p class="text-xs text-muted-foreground">Donate ZNN or QSR to the Accelerator-Z funding pool.</p>
    <div class="flex items-end gap-2">
      <div class="flex-1">
        <Field label="Amount (base units)">
          <Input v-model="amount" placeholder="amount (base units)" aria-label="donate amount" />
        </Field>
      </div>
      <select
        v-model="token"
        class="rounded border border-border bg-muted px-3 py-2 text-foreground outline-none focus:ring-2 focus:ring-primary"
        aria-label="donate token"
      >
        <option value="ZNN">ZNN</option>
        <option value="QSR">QSR</option>
      </select>
      <Button aria-label="donate" @click="donate">Donate</Button>
    </div>
    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
```

- [ ] **Step 4: Create `AcceleratorCreate.vue`**

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useTxStore } from '../../stores/tx'
import Field from '../Field.vue'

const acc = useAcceleratorStore()
const tx = useTxStore()
const error = ref('')

// create project
const cName = ref('')
const cDesc = ref('')
const cUrl = ref('')
const cZnn = ref('')
const cQsr = ref('')
// add/update phase — both keyed by the PROJECT id on-chain
const phProjectId = ref('')
const phName = ref('')
const phDesc = ref('')
const phUrl = ref('')
const phZnn = ref('')
const phQsr = ref('')

function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e)
}
async function createProject() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCreateProject(cName.value, cDesc.value, cUrl.value, cZnn.value, cQsr.value))
  } catch (e) {
    fail(e)
  }
}
async function addPhase() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareAddPhase(phProjectId.value, phName.value, phDesc.value, phUrl.value, phZnn.value, phQsr.value))
  } catch (e) {
    fail(e)
  }
}
async function updatePhase() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareUpdatePhase(phProjectId.value, phName.value, phDesc.value, phUrl.value, phZnn.value, phQsr.value))
  } catch (e) {
    fail(e)
  }
}
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') acc.loadProjects()
  },
)
</script>

<template>
  <div class="space-y-5 p-4">
    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h3 class="text-sm font-medium text-foreground">Post an AZ (create project · 1 ZNN fee)</h3>
      <div class="grid grid-cols-2 gap-2">
        <Field label="Name"><Input v-model="cName" placeholder="name" aria-label="create name" /></Field>
        <Field label="URL"><Input v-model="cUrl" placeholder="url" aria-label="create url" /></Field>
        <Field label="ZNN needed (base units)"><Input v-model="cZnn" placeholder="ZNN needed" aria-label="create znn" /></Field>
        <Field label="QSR needed (base units)"><Input v-model="cQsr" placeholder="QSR needed" aria-label="create qsr" /></Field>
      </div>
      <Field label="Description"><Input v-model="cDesc" placeholder="description" aria-label="create description" /></Field>
      <Button aria-label="create project" @click="createProject">Create project</Button>
    </section>

    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h3 class="text-sm font-medium text-foreground">Submit / update a phase</h3>
      <p class="text-xs text-muted-foreground">
        Both use the project id; Update edits the project's current (voting) phase. Only the project
        owner can add or update phases (enforced on-chain).
      </p>
      <Field label="Project id"><Input v-model="phProjectId" placeholder="project id (0x…)" aria-label="project id" /></Field>
      <div class="grid grid-cols-2 gap-2">
        <Field label="Name"><Input v-model="phName" placeholder="name" aria-label="phase name" /></Field>
        <Field label="URL"><Input v-model="phUrl" placeholder="url" aria-label="phase url" /></Field>
        <Field label="ZNN needed (base units)"><Input v-model="phZnn" placeholder="ZNN needed" aria-label="phase znn" /></Field>
        <Field label="QSR needed (base units)"><Input v-model="phQsr" placeholder="QSR needed" aria-label="phase qsr" /></Field>
      </div>
      <Field label="Description"><Input v-model="phDesc" placeholder="description" aria-label="phase description" /></Field>
      <div class="flex gap-2">
        <Button variant="outline" aria-label="add phase" @click="addPhase">Add phase</Button>
        <Button variant="outline" aria-label="update phase" @click="updatePhase">Update phase</Button>
      </div>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorDonate.test.ts src/components/panels/AcceleratorCreate.test.ts`
Expected: PASS. Then `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 6: Stage**

```bash
git add frontend/src/components/panels/AcceleratorDonate.vue frontend/src/components/panels/AcceleratorDonate.test.ts frontend/src/components/panels/AcceleratorCreate.vue frontend/src/components/panels/AcceleratorCreate.test.ts
```
STOP — controller commits.

---

### Task 9: Frontend — `AcceleratorPanel.vue` container (replace, with sub-tabs)

**Files:**
- Modify (replace): `frontend/src/components/panels/AcceleratorPanel.vue`
- Modify (replace): `frontend/src/components/panels/AcceleratorPanel.test.ts` (was the old monolithic panel test)

**Interfaces:**
- Consumes: `AcceleratorVote`/`AcceleratorProjects`/`AcceleratorCreate`/`AcceleratorDonate`; accelerator store (`refreshVotable`/`loadProjects`/`loadVotablePillars`); pillar store `ownsPillar`; wallet store `activeIndex`.
- Produces: prop `initialSub?: string` (driven by the top-bar jump / Home query).

- [ ] **Step 1: Write the failing container test**

Replace `frontend/src/components/panels/AcceleratorPanel.test.ts` with:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Tabs: { props: ['modelValue'], template: '<div><slot /></div>' },
  TabsList: { template: '<div><slot /></div>' },
  TabsTrigger: { props: ['value'], template: '<button><slot /></button>' },
  TabsContent: { props: ['value'], template: '<div><slot /></div>' },
}))
vi.mock('./AcceleratorVote.vue', () => ({ default: { name: 'AcceleratorVote', template: '<div data-test="vote" />' } }))
vi.mock('./AcceleratorProjects.vue', () => ({ default: { name: 'AcceleratorProjects', template: '<div data-test="projects" />' } }))
vi.mock('./AcceleratorCreate.vue', () => ({ default: { name: 'AcceleratorCreate', template: '<div data-test="create" />' } }))
vi.mock('./AcceleratorDonate.vue', () => ({ default: { name: 'AcceleratorDonate', template: '<div data-test="donate" />' } }))

import AcceleratorPanel from './AcceleratorPanel.vue'
import { useAcceleratorStore } from '../../stores/accelerator'

function setup() {
  setActivePinia(createPinia())
  const acc = useAcceleratorStore()
  vi.spyOn(acc, 'refreshVotable').mockResolvedValue()
  vi.spyOn(acc, 'loadProjects').mockResolvedValue()
  vi.spyOn(acc, 'loadVotablePillars').mockResolvedValue()
  return { acc }
}

describe('AcceleratorPanel container', () => {
  it('renders all four sub-views (Tabs stub shows all content)', () => {
    setup()
    const w = mount(AcceleratorPanel)
    expect(w.find('[data-test="vote"]').exists()).toBe(true)
    expect(w.find('[data-test="projects"]').exists()).toBe(true)
    expect(w.find('[data-test="create"]').exists()).toBe(true)
    expect(w.find('[data-test="donate"]').exists()).toBe(true)
  })

  it('refreshes votable + projects on mount', () => {
    const { acc } = setup()
    mount(AcceleratorPanel)
    expect(acc.refreshVotable).toHaveBeenCalled()
    expect(acc.loadProjects).toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorPanel.test.ts`
Expected: FAIL — the current panel renders inline markup, not the mocked sub-view components (`[data-test="vote"]` absent).

- [ ] **Step 3: Replace the panel with the container**

Replace the entire contents of `frontend/src/components/panels/AcceleratorPanel.vue` with:

```vue
<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useAcceleratorStore } from '../../stores/accelerator'
import { usePillarStore } from '../../stores/pillar'
import { useWalletStore } from '../../stores/wallet'
import AcceleratorVote from './AcceleratorVote.vue'
import AcceleratorProjects from './AcceleratorProjects.vue'
import AcceleratorCreate from './AcceleratorCreate.vue'
import AcceleratorDonate from './AcceleratorDonate.vue'

const props = defineProps<{ initialSub?: string }>()
const acc = useAcceleratorStore()
const pillar = usePillarStore()
const wallet = useWalletStore()
const { ownsPillar } = storeToRefs(pillar)

const sub = ref(props.initialSub || (ownsPillar.value ? 'Vote' : 'Projects'))
watch(
  () => props.initialSub,
  (v) => {
    if (v) sub.value = v
  },
)

function load() {
  acc.refreshVotable()
  acc.loadProjects()
  acc.loadVotablePillars()
}
onMounted(load)
watch(() => wallet.activeIndex, load)
</script>

<template>
  <div class="p-4">
    <Tabs v-model="sub">
      <TabsList class="w-full justify-start">
        <TabsTrigger value="Vote">Vote</TabsTrigger>
        <TabsTrigger value="Projects">Projects</TabsTrigger>
        <TabsTrigger value="Create">Create</TabsTrigger>
        <TabsTrigger value="Donate">Donate</TabsTrigger>
      </TabsList>
      <TabsContent value="Vote"><AcceleratorVote /></TabsContent>
      <TabsContent value="Projects"><AcceleratorProjects /></TabsContent>
      <TabsContent value="Create"><AcceleratorCreate /></TabsContent>
      <TabsContent value="Donate"><AcceleratorDonate /></TabsContent>
    </Tabs>
  </div>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/AcceleratorPanel.test.ts`
Expected: PASS. Then run the whole panels dir to confirm no fallout: `cd frontend && pnpm exec vitest run src/components/panels` → all PASS. `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 5: Stage**

```bash
git add frontend/src/components/panels/AcceleratorPanel.vue frontend/src/components/panels/AcceleratorPanel.test.ts
```
STOP — controller commits.

---

### Task 10: Frontend — top-bar ballot badge + Home query wiring

**Files:**
- Modify: `frontend/src/components/TopBar.vue` (+ `TopBar.test.ts`)
- Modify: `frontend/src/views/Home.vue`

**Interfaces:**
- Consumes: pillar store `ownsPillar`; accelerator store `needsVoteCount`, `refreshVotable`; vue-router `useRoute`/`useRouter`.

- [ ] **Step 1: Write the failing TopBar test**

Append to `frontend/src/components/TopBar.test.ts` (read its existing mocks/setup first; it mounts `TopBar` with a router). Add a test that the ballot icon shows with a badge when a pillar is owned and there are items to vote on:

```ts
// (add near the other tests in TopBar.test.ts)
import { useAcceleratorStore } from '../stores/accelerator'

it('shows the accelerator vote badge when a pillar is owned with pending votes', async () => {
  const { wrapper } = mountTopBar() // use the file's existing mount helper
  const pillar = usePillarStore()
  const acc = useAcceleratorStore()
  pillar.myPillar = { name: 'P' } as never
  acc.votable = [{ needsMyVote: true }, { needsMyVote: true }] as never
  await wrapper.vm.$nextTick()
  const btn = wrapper.find('button[aria-label="Accelerator votes"]')
  expect(btn.exists()).toBe(true)
  expect(btn.text()).toContain('2')
})
```
> If `TopBar.test.ts` lacks a reusable mount helper, mirror its existing `mount(TopBar, { global: { plugins: [router, pinia] } })` setup inline. The pillar/accelerator stores resolve via the test's active pinia.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/TopBar.test.ts`
Expected: FAIL — no `button[aria-label="Accelerator votes"]`.

- [ ] **Step 3: Add the ballot button to `TopBar.vue`**

In `frontend/src/components/TopBar.vue` script, add imports + stores:

```ts
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
```
and after the existing store setup:

```ts
const pillar = usePillarStore()
const accelerator = useAcceleratorStore()
function gotoVotes() {
  router.push({ name: 'home', query: { tab: 'Accelerator', sub: 'Vote' } })
}
```

In the icon row (template), add this button just before the `<span class="mx-1 h-5 w-px bg-border"></span>` divider:

```html
<button
  v-if="!locked && pillar.ownsPillar"
  type="button"
  :title="accelerator.needsVoteCount > 0 ? `${accelerator.needsVoteCount} AZ item(s) to vote on` : 'Accelerator votes'"
  aria-label="Accelerator votes"
  class="relative grid h-9 w-9 place-items-center rounded-lg text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
  @click="gotoVotes"
>
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 12h6M9 16h6M9 8h6"/><rect width="16" height="20" x="4" y="2" rx="2"/></svg>
  <span
    v-if="accelerator.needsVoteCount > 0"
    class="absolute -right-0.5 -top-0.5 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold text-primary-foreground"
    >{{ accelerator.needsVoteCount }}</span
  >
</button>
```

- [ ] **Step 4: Wire `Home.vue`**

In `frontend/src/views/Home.vue`:
- Add to imports: `import { useRoute } from 'vue-router'` and `import { useAcceleratorStore } from '../stores/accelerator'`.
- Add stores: `const route = useRoute()` and `const accelerator = useAcceleratorStore()`.
- Add `accelerator.refreshVotable(),` to the `Promise.all([...])` in `refresh()`.
- Add an `initialSub` ref and a query-applier, and pass it to the panel:

```ts
const initialSub = ref('')
function applyQuery() {
  const t = route.query.tab
  if (typeof t === 'string' && TABS.includes(t)) active.value = t
  const sub = route.query.sub
  initialSub.value = typeof sub === 'string' ? sub : ''
}
```
Call `applyQuery()` inside `onMounted` (before/after `refresh()`), and `watch(() => route.query, applyQuery)`.

Change the panel usage to:
```html
<TabsContent value="Accelerator"><AcceleratorPanel :initial-sub="initialSub" /></TabsContent>
```

- [ ] **Step 5: Run tests + typecheck**

Run: `cd frontend && pnpm exec vitest run src/components/TopBar.test.ts && pnpm run typecheck`
Expected: PASS / clean.

- [ ] **Step 6: Stage**

```bash
git add frontend/src/components/TopBar.vue frontend/src/components/TopBar.test.ts frontend/src/views/Home.vue
```
STOP — controller commits.

---

### Task 11: Integration — full suites, typecheck, build

**Files:** none (verification only; commit only if a glue fix is needed).

- [ ] **Step 1: Full frontend suite + typecheck + build**

Run: `cd frontend && pnpm test && pnpm run typecheck && pnpm run build`
Expected: all suites PASS (incl. the existing Sentinel/Pillar/Plasma tests — confirms no regression from the TopBar/Home/store changes), typecheck clean, Vite build succeeds.

- [ ] **Step 2: Full backend test + vet + build**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./... ; GOWORK=off GOTOOLCHAIN=auto go vet ./... ; GOWORK=off GOTOOLCHAIN=auto go build ./...`
Expected: accelerator + all other package tests PASS; `vet`/`build` clean. The only acceptable failures are the pre-existing local-`secrets/` keystore tests (`TestImportListUnlockLock`, `TestSigningKeyPairMatchesActiveAddress`, `internal/compat TestSyriusKeystoreRoundTrip`) — they `t.Skip` in CI. Confirm no NEW failures.

- [ ] **Step 3 (manual, optional): live smoke test**

`GOWORK=off ~/go/bin/wails dev`; on a pillar-owning account, confirm: the top-bar ballot badge shows a count; clicking it lands on Accelerator → Vote; the Vote list shows items not yet voted; casting a vote opens the confirm dialog; Projects filters (Active AZs / Awaiting payout) behave; Create posts an AZ. (Acceptance, not a required automated gate.)

- [ ] **Step 4: Stage any glue fix (only if Steps 1–2 required one)**

```bash
git add -A -- ':!animation' ':!.superpowers'
```
STOP — controller commits.

---

## Self-Review

**Spec coverage:**
- Post an AZ → Task 8 (`AcceleratorCreate`, `PrepareCreateProject`). ✓
- Submit a phase → Task 8 (`PrepareAddPhase`/`PrepareUpdatePhase`). ✓
- Pillar vote on AZ + phases → Task 6 (`AcceleratorVote`, `PrepareVote`), backed by Task 2 (`GetVotableForMyPillars`). ✓
- Filter active AZs + active phases for payment → Task 7 (`AcceleratorProjects` filters incl. "Awaiting payout" via `isPassing`). ✓
- Easy "needs my vote" view → Task 6. ✓
- Top-bar notification icon + badge for action → Task 10 (ballot icon + `needsVoteCount` badge + jump). ✓
- Sub-tabs Vote/Projects/Create/Donate → Task 9 container. ✓
- Badge counts unvoted items → Task 1/2 (`NeedsMyVote`), Task 5 (`needsVoteCount`). ✓
- Awaiting-payout = passing+unpaid (derived) → Task 4 (`isPassing`) used in Task 7. ✓
- Central refresh (badge current from any screen) → Task 10 (`refreshVotable` in `Home.refresh`) + Task 9 (account switch). ✓
- Reads-only backend / NoM-confirm reuse → Tasks 1–2 reads; vote via existing `PrepareVote`. ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete content. Two clearly-flagged verify-in-context notes: the exact `constants.AcceleratorProjectVotingPeriod` symbol (Task 1) and reusing TopBar.test's existing mount helper (Task 10) — both with concrete fallbacks.

**Type consistency:** `VotableItem`/`PillarVoteState` fields match across Go DTO (Task 1), bindings/models (Task 3), and frontend usage (Tasks 5–7, 10): `kind/id/projectId/projectName/name/znnFundsNeeded/qsrFundsNeeded/votes/myVotes/needsMyVote` and `pillar/vote`. `GetVotableForMyPillars(): VotableItem[]` and `GetActivePillarCount(): number` consistent (Task 2/3/5). `isPassing(yes,no,total,numPillars)` / `quorumNeeded(numPillars)` / `statusLabel(n)` signatures consistent (Task 4 → 6/7). `PrepareVote(id, pillarName, vote)` matches the existing backend signature (Task 6). Store members (`votable`, `numActivePillars`, `needsVoteCount`, `refreshVotable`) consistent (Task 5 → 6/9/10).
