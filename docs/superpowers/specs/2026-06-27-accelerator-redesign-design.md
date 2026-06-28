# Accelerator (AZ) Redesign — Design Spec

**Date:** 2026-06-27
**Status:** Approved design, pre-implementation
**Branch:** new feature branch off `main` (`accelerator-redesign`)

## 1. Goal

Make the Accelerator-Z (AZ) tab simple and logical to navigate, and make the two pillar-operator actions that are hard to find in syrius — **voting on AZs** and **voting on phases** — immediately discoverable. Specifically:

- Post an AZ (create project) and submit/update a phase. *(Both already exist; relocated into a clear "Create" view.)*
- Pillar operators vote on AZs and on phases, from one consolidated "needs my vote" list.
- Browse/filter all AZs: active AZs (in voting), phases awaiting payout, plus the other statuses.
- A **top-bar notification icon with a badge** counting items the operator's pillar still needs to vote on, that jumps straight to the vote list.

This is a frontend-heavy rework plus a small set of new read methods on the Go backend. All existing write paths (`PrepareDonate`/`PrepareVote`/`PrepareCreateProject`/`PrepareAddPhase`/`PrepareUpdatePhase`) are reused unchanged.

## 2. On-chain semantics (authoritative — verified against go-zenon)

Statuses (`uint8`, shared by projects and phases): `Voting=0, Active=1, Paid=2, Closed=3, Completed=4`.

- **Project** walks `Voting → Active → Completed`, or `Voting → Closed` (rejected). Creation costs **1 ZNN** (`ProjectCreationAmount`).
- **Project voting window**: time-based, `AcceleratorProjectVotingPeriod = 14 days` from `CreationTimestamp`. A project flips to `Active` the moment it passes the vote check (checked continuously, ~every 50 min via `UpdateEmbeddedAccelerator`), not at window end. If the 14 days elapse without passing → `Closed`.
- **Phase**: added only by the project `Owner`, only while the project is `Active`, and only if the previous phase is `Paid` (so at most **one open `Voting` phase per project at a time** — always the last in `PhaseIds`). Phase statuses are only `Voting` or `Paid`. **Phase voting has no expiry.**
- **Payment is automatic**: when a phase passes the vote check and the accelerator contract holds enough ZNN+QSR, the update engine emits the payouts to the owner and flips the phase to `Paid` in the same block (sets `AcceptedTimestamp`). There is **no "accepted, awaiting payment" status** — that state is *derived*: phase still `Voting` ∧ passes the vote math ∧ not yet `Paid`.
- **Vote check** (`checkAcceleratorVotes`, identical for projects and phases), both conditions required:
  1. **Majority**: `Yes > No` (strict; ties fail).
  2. **Quorum**: `Total*100 > numActivePillars * 33` (`VoteAcceptanceThreshold = 33`). `Total` counts yes+no+**abstain**; abstain counts toward quorum but not majority.
- **Voting mechanics**: any active pillar; one vote per (pillar, id), **changeable** (re-vote overwrites). Vote values `Yes=0, No=1, Abstain=2`. `GetPillarVotes(name, ids) → []*PillarVote` returns one slot per requested id; **`nil` = that pillar has not voted** on that id; a non-nil slot's `.Vote` is the current choice.

### Derived sets the UI uses
- **Votable AZ** = project `Status==Voting` and `CreationTimestamp + 14d >= now`.
- **Votable phase** = for a project `Status==Active`, its current (last) phase when that phase `Status==Voting`.
- **Needs my vote** = a votable item where at least one of my owned pillars has a `nil` `GetPillarVotes` slot.
- **Passing / awaiting payout** = `isPassing(votes, numActivePillars)` = `yes>no && total*100 > numActivePillars*33`; for a phase still `Voting`, that means accepted-and-payout-pending.

## 3. Decisions (from brainstorming)

1. **Badge** counts items my pillar **hasn't voted on** (`needsMyVote`); it clears as you vote (even abstain).
2. **"Phases for payment" filter** = phases passing the vote but not yet `Paid` (awaiting auto-payout), derived client-side.
3. **Layout** = sub-tabs **Vote / Projects / Create / Donate**.
4. **Multi-pillar**: an item "needs my vote" if *any* owned pillar hasn't voted; the Vote action lets the user pick which pillar.
5. **Voting** keeps using `VoteByName` (existing `PrepareVote(id, pillarName, vote)`).

## 4. Architecture

### 4.1 Backend — new read methods (`app/nom_accelerator.go`)

All follow the existing guard pattern (`currentClient()` nil → "not connected"; `activeAddress()` → `errLocked` where an address is needed). New SDK wrap: `client.AcceleratorApi.GetPillarVotes(name string, hashes []types.Hash) ([]*definition.PillarVote, error)` (present in SDK v0.1.19, not yet wrapped).

- `GetActivePillarCount() (int, error)` — pages `client.PillarApi.GetAll` and returns the active pillar count used for quorum math. (Pages until exhausted; clamps page size.)
- `GetVotableForMyPillars() ([]VotableItem, error)` — the aggregating read powering the Vote view + badge:
  1. Resolve owned pillar names via `client.PillarApi.GetByOwner(activeAddress)` (empty slice if none → returns empty list, no error).
  2. `GetProjects`-equivalent sweep (`AcceleratorApi.GetAll`, paging) to collect: projects with `Status==Voting` and window open; and for `Status==Active` projects, the current (last) phase if `Status==Voting`.
  3. For each owned pillar, call `GetPillarVotes(pillar, allVotableIds)` once; fill per-item `MyVotes`.
  4. Map to `[]VotableItem`, computing `NeedsMyVote`.
  - Returns at most the open votable set (typically small). Errors when locked/disconnected propagate (frontend swallows for the badge).

### 4.2 DTOs (`app/dto.go`)

```go
// PillarVoteState is one owned pillar's vote on a votable item; Vote == -1 means
// the pillar has not voted yet (GetPillarVotes returned a nil slot).
type PillarVoteState struct {
	Pillar string `json:"pillar"`
	Vote   int    `json:"vote"` // -1 not voted, 0 yes, 1 no, 2 abstain
}

// VotableItem is a project or phase currently open for pillar voting, annotated
// with the active address's owned-pillar vote state.
type VotableItem struct {
	Kind           string            `json:"kind"` // "project" | "phase"
	Id             string            `json:"id"`   // votable hash (the vote target)
	ProjectId      string            `json:"projectId"`
	ProjectName    string            `json:"projectName"`
	Name           string            `json:"name"` // project name, or phase name
	ZnnFundsNeeded string            `json:"znnFundsNeeded"`
	QsrFundsNeeded string            `json:"qsrFundsNeeded"`
	Votes          VoteBreakdownDTO  `json:"votes"`
	MyVotes        []PillarVoteState `json:"myVotes"`
	NeedsMyVote    bool              `json:"needsMyVote"`
}
```
`Passing` is **not** stored on the DTO — the frontend computes it from `Votes` + `numActivePillars` via the shared helper (single source of truth; avoids backend/frontend math drift).

### 4.3 Frontend helpers (`frontend/src/lib/accelerator.ts`)

Pure, unit-tested:
- `isPassing(yes: number, no: number, total: number, numPillars: number): boolean` → `yes > no && total * 100 > numPillars * 33`.
- `quorumNeeded(numPillars: number): number` → `Math.ceil(numPillars * 0.33)` (for the progress bar).
- `statusLabel(status: number): string` → `['Voting','Active','Paid','Closed','Completed'][status] ?? 'Unknown'` (with `Closed`→display "Rejected", `Active`→"Accepted" where a project tag is shown — match syrius wording).

### 4.4 Store (`frontend/src/stores/accelerator.ts`)

Extend the existing store (keep `projects`, `selectedProject`, `votablePillars`, `error`, `loadProjects`, `openProject`, `loadVotablePillars`):
- State add: `numActivePillars: number`, `votable: app.VotableItem[]`.
- Getter: `needsVoteCount` = `votable.filter(v => v.needsMyVote).length`.
- Action: `refreshVotable()` → sets `votable = await GetVotableForMyPillars()` and `numActivePillars = await GetActivePillarCount()`; swallows locked/disconnected errors (badge just shows 0). Wired into `Home.refresh()` and the account-switch path.

### 4.5 Components (`frontend/src/components/panels/`)

- **`AcceleratorPanel.vue`** → container with nom-ui `Tabs` (`Vote` / `Projects` / `Create` / `Donate`). Default sub-tab = `Vote` when `ownsPillar`, else `Projects`. On mount: `accelerator.refreshVotable()` + `loadProjects()`. Accepts an optional `initialSub` (driven by the top-bar jump).
- **`AcceleratorVote.vue`** — pillar-gated. Lists `votable.filter(needsMyVote)` (with a toggle to show all votable incl. already-voted). Each row: Project/Phase tag, name, funds, tally + quorum bar (`quorumNeeded`), per-pillar vote state, and **Yes/No/Abstain** buttons → `tx.awaitConfirm(await Nom.PrepareVote(id, pillarName, choice))`. Multi-pillar: a pillar `<select>` (defaults to first owned). If no owned pillar → "Voting is for pillar operators" note. Refresh `accelerator.refreshVotable()` on `tx.status==='done'`.
- **`AcceleratorProjects.vue`** — browse `projects` with filter chips: **All / Voting (active AZs) / Active / Awaiting payout / Completed / Rejected**. "Awaiting payout" filters to projects whose current phase is `Voting` and `isPassing(...)`. Expand a project to show phases (status, tally, awaiting-payout marker). Read-only browse (vote/donate live in their tabs).
- **`AcceleratorCreate.vue`** — Post an AZ (name/url/znn/qsr/description → `PrepareCreateProject`, 1 ZNN) and Submit/Update phase (projectId + fields → `PrepareAddPhase`/`PrepareUpdatePhase`). Reuses existing methods verbatim.
- **`AcceleratorDonate.vue`** — amount + ZNN/QSR → `PrepareDonate`.

### 4.6 Top-bar notification (`frontend/src/components/TopBar.vue`)

- Add a **ballot icon** button to the right-hand icon row (same 9×9 button/SVG/`:disabled="locked"` pattern), shown when `pillar.ownsPillar`. A small badge renders `accelerator.needsVoteCount` when `> 0` (mirrors the `Receive` `:badge` count pattern).
- Click → navigate to Home with the Accelerator Vote view: `router.push({ name: 'home', query: { tab: 'Accelerator', sub: 'Vote' } })`.

### 4.7 Home wiring (`frontend/src/views/Home.vue`)

- Read `route.query.tab` / `route.query.sub` on mount + on change to set the active tab (and pass `initialSub` to `AcceleratorPanel`).
- Add `accelerator.refreshVotable()` to the existing `refresh()` (runs on mount, node events, account switch) so the badge stays current from any screen.

## 5. Funds-safety / invariants (per CLAUDE.md)

- No new write paths; every vote/donate/create/phase action routes through the existing `Prepare*` → `tx.awaitConfirm` → `ConfirmPublish` confirm-what-you-sign pipeline. No state-changing call bypasses `tx.awaitConfirm`.
- New backend methods are **reads only**. Vote values continue to use the on-chain-verified constants (`Yes=0/No=1/Abstain=2`), already guarded in `PrepareVote`.
- The badge/needs-vote logic is advisory UI; the authoritative checks remain on-chain.

## 6. Testing

Backend (`app/nom_accelerator_test.go`):
- `GetVotableForMyPillars`: filters to votable projects (Voting+window) and active-project open phases; annotates `MyVotes` from `GetPillarVotes` (nil → −1); `NeedsMyVote` true when any owned pillar unvoted; empty when no owned pillar. Use DTO-mapping unit tests + guard tests (locked/disconnected) following the existing `TestAcceleratorReadsGuardInputs` pattern.
- `GetActivePillarCount`: guard + counting.
- `GetPillarVotes` wrapper: input validation (hash parsing) + guard.

Frontend:
- `lib/accelerator.test.ts`: `isPassing` (majority, tie-fail, quorum boundary at exactly 33%), `quorumNeeded`, `statusLabel`.
- `AcceleratorVote.test.ts`: renders needs-vote items; Yes/No/Abstain forwards the right `PrepareVote(id, pillar, choice)` to `tx.awaitConfirm`; pillar-gating (no pillar → note); multi-pillar select.
- `AcceleratorProjects.test.ts`: status filter chips; awaiting-payout filter uses `isPassing`.
- `AcceleratorPanel.test.ts`: sub-tab routing; `initialSub` honored.
- `TopBar.test.ts`: ballot icon shows when `ownsPillar`, badge = `needsVoteCount`, click pushes the Accelerator/Vote query.
- `stores/accelerator.test.ts`: `needsVoteCount` getter; `refreshVotable` populates state and swallows errors.

## 7. Out of scope

- Changing the on-chain vote/payment mechanics (payment stays automatic).
- `VoteByProducerAddress` (we keep `VoteByName`).
- Historical vote analytics / per-pillar dashboards beyond "my pillars' vote state."
- Editing a project's metadata after creation (only phase add/update exists on-chain via these methods).
