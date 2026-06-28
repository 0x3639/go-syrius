# Governance Module — Design Spec

**Date:** 2026-06-28
**Status:** Approved design, pre-implementation
**Branch:** `governance-module` (off `main`)
**Scope:** Browse + Vote + Execute (Propose deferred — YAGNI)

## 0. Phasing (IMPORTANT)

This spec is delivered in **two phases** so Phase 1 ships against the **current pinned
SDK (`v0.1.20`) with zero go-zenon/SDK changes**:

- **Phase 1 (now) — build & testnet:** Browse (Actions: filter + paging + detail +
  Execute) and a **simple Vote view** (lists open actions, pillar picker, Yes/No/Abstain
  via `VoteByName`). **No per-pillar "needs my vote" state and no top-bar badge** — those
  require a vote-state read the node/SDK don't expose yet.
- **Phase 2 (later) — per-pillar awareness:** add `getPillarVotes` to the go-zenon
  governance RPC + the SDK client (§6), then upgrade the Vote view to the Accelerator-Z
  "needs my vote" experience and add the **top-bar governance ballot badge**.

Everything tagged **[P2]** below is deferred to Phase 2. Unmarked items are Phase 1.

## 1. Goal

Add a **Governance** feature to the wallet so pillar operators can participate in
on-chain governance (the embedded `governance` contract — actions that call other
embedded contracts such as Spork/Bridge). Specifically:

- **Browse** all governance actions (paginated, filterable by status).
- **Vote** on open actions, pillar-gated. *Phase 1:* a flat list of open actions with a
  pillar picker + Yes/No/Abstain. *[P2]:* a consolidated "needs my vote" list mirroring
  the Accelerator-Z Vote view, with a per-pillar "you voted" indicator.
- **Execute** an approved-but-unexecuted action (rare; see §2 note).
- **[P2]** A **top-bar ballot badge** counting actions the operator's pillar still needs
  to vote on, deep-linking to the Vote view.

This mirrors the Accelerator-Z module (`nom_accelerator.go` + `AcceleratorPanel`).
It is frontend-heavy plus a small set of new read/prepare methods on the Go backend.
**Phase 1 needs no SDK/node changes;** the [P2] additions are in §6.

## 2. On-chain semantics (authoritative — verified against the 0x3639/go-zenon fork)

**Action statuses** (`uint8`): `Voting=0, Approved=1, Rejected=2, NoDecision=3`.
Separate `Executed bool`.

**Action types** (assigned by the node at propose time): `Type1=1` (targets the Spork
contract — stricter), `Type2=2` (everything else).

**Multi-round voting.** Each action runs up to **4 rounds** (0–3). Each round has its
own quorum + directional threshold + voting period, from
`GovernanceActionSchedule(type, round)`:

| | Round 0 | 1 | 2 | 3 |
|---|---|---|---|---|
| Type1 ActivePillarThreshold (%) | 66 | 55 | 45 | 40 |
| Type1 DirectionalThreshold (%) | 50 | 55 | 60 | 66 |
| Type1 VotingPeriod (days) | 45 | 21 | 21 | 21 |
| Type2 ActivePillarThreshold (%) | 50 | 40 | 33 | 25 |
| Type2 DirectionalThreshold (%) | 50 | 55 | 60 | 66 |
| Type2 VotingPeriod (days) | 30 | 14 | 14 | 14 |

(`PhaseTimeUnit = 1 day`.) The node's `Action` DTO already exposes the **current
round's** `ActivePillarThreshold`, `DirectionalThreshold`, `VotingPeriod`, `Round`,
`RoundStartTimestamp`, and `Expired` — the frontend reads these off the action rather
than recomputing the schedule.

**Vote check** (`checkActionVoteBreakdown`, per current round) — note it differs from
the accelerator's simple majority + 33% quorum:
- `directionalVotes = Yes + No` (**abstain excluded from directional**, unlike
  accelerator which counts abstain toward quorum).
- **Quorum:** `directionalVotes*100 > numActivePillars * ActivePillarThreshold`.
- **Approved:** quorum met **and** `Yes*100 > directionalVotes * DirectionalThreshold`.
- **Rejected:** quorum met **and** `No*100  > directionalVotes * DirectionalThreshold`.
- Otherwise pending → at period end the round advances (stricter→looser), until round 3;
  no decision by then → `NoDecision`.

**Voting mechanics:** any active pillar; vote keyed by `(CurrentVoteId, pillarName)`,
**changeable**; values `Yes=0, No=1, Abstain=2` (= `definition.Vote*`, re-exported by
the SDK as `embedded.VoteYes/VoteNo/VoteAbstain`). `CurrentVoteId =
ActionVoteId(action.Id, action.Round)` changes each round, so "have I voted" is
**per current round**.

**Execution note (important for the Execute UX).** In the fork's `updateAction`, when a
round is approved the engine sets `Status=Approved`, `Executed=true`, **and emits the
destination contract-send in the same step** — i.e. approval normally **auto-executes**.
`ExecuteAction` (RPC) exists to *trigger* `updateAction` on demand (rather than waiting
for the periodic `UpdateEmbeddedGovernance`). So an `Approved && !Executed` window is
**uncommon**; the Execute button is a best-effort affordance for that edge (and for
nudging a pending action's update). The UI must treat "no executable actions" as the
normal case, not an error.

### Derived sets the UI uses
- **Open action** = `Status == Voting` and not `Expired`. *(Phase 1 Vote list = all open
  actions.)*
- **Executable** = `Status == Approved && !Executed` (rare — see note).
- **Passing (current round)** = `isActionApproved(votes, action, numActivePillars)` using
  the action's own thresholds.
- **[P2] Needs my vote** = an open action where at least one owned pillar has a `nil`
  `GetPillarVotes` slot for the action's `CurrentVoteId`.

## 3. Decisions (from brainstorming)

1. **Scope:** Browse + Vote + Execute. **Propose deferred.** Per-pillar vote awareness +
   badge deferred to **Phase 2** (see §0).
2. **Layout:** `GovernancePanel` with two sub-tabs — **Vote** / **Actions**.
3. **Vote view (Phase 1):** flat list of **open** actions (`Status==Voting && !Expired`)
   with a pillar picker (which owned pillar to vote as, from `GetVotablePillars`) +
   Yes/No/Abstain via `VoteByName`. No "you voted" indicator (can't read per-pillar vote
   state yet). **[P2]:** upgrade to a "needs my vote" list scoped to the *selected* pillar
   (apply the AZ-review lesson: filter by selected pillar, not a global `needsMyVote`;
   reset selection when it leaves the pillar list).
4. **Actions view:** paginated browse (Prev/Next + total count) with a status filter;
   each row expands to detail; **Execute** button appears in the expanded detail of an
   `Approved && !Executed` action.
5. **[P2] Top-bar badge:** a governance ballot badge mirroring the accelerator one
   (`needsVoteCount` → deep-link `?tab=Governance&sub=Vote`).
6. **Confirm-what-you-sign:** both Vote and Execute render the effect from the built
   block, not raw inputs (see §5).

## 4. Architecture

### 4.1 Backend — new methods on `NomService` (`app/nom_governance.go`)

Follow the existing guard pattern (`currentClient()` nil → "not connected";
`activeAddress()` → `errLocked` where an address is needed). Phase 1 SDK surface (all
present in `v0.1.20`): `client.GovernanceApi.GetAllActions / GetActionById / VoteByName /
ExecuteAction`. **[P2]** adds `client.GovernanceApi.GetPillarVotes(name, hashes)` (§6).

- `GetActions(pageIndex, pageSize uint32) (ActionListDTO, error)` — wraps `GetAllActions`;
  returns `{count, list}` (retain count for paging, per the AZ pagination fix). Also the
  source for the Phase 1 Vote list (frontend filters to open actions).
- `GetAction(id string) (ActionDTO, error)` — wraps `GetActionById`.
- `PrepareGovernanceVote(id, pillarName string, vote uint8) (CallPreview, error)` —
  re-validates inputs, builds via `VoteByName`.
- `PrepareExecuteAction(id string) (CallPreview, error)` — builds via `ExecuteAction`.
- Reuses existing `GetVotablePillars()` and `GetActivePillarCount()`.
- **[P2]** `GetVotableActionsForMyPillars() ([]VotableAction, error)` — the aggregating
  read that will power the upgraded Vote view + badge:
  1. Resolve owned pillar names via `PillarApi.GetByOwner(activeAddress)` (none → empty
     list, no error).
  2. Sweep `GetAllActions` (paging) → keep `Status==Voting && !Expired`.
  3. For each owned pillar, call `GetPillarVotes(pillar, [CurrentVoteId…])` once; fill
     per-item `MyVotes` (nil slot → not voted this round).
  4. Map to `[]VotableAction`, computing `NeedsMyVote`.
  - Errors propagate (frontend swallows for the badge, like AZ).

### 4.2 DTOs (`app/dto.go`)

- `ActionDTO` — `id, owner, name, description, url, destination, data (base64), type,
  round, status, executed, expired, creationTimestamp, roundStartTimestamp,
  activePillarThreshold, directionalThreshold, votingPeriod, votes{yes,no,total}`.
- `ActionListDTO{ count int, list []ActionDTO }`.
- **[P2]** `VotableAction` — the `ActionDTO` fields needed by the upgraded Vote view plus
  `myVotes []PillarVoteState` and `needsMyVote bool` (reuse the existing
  `PillarVoteState`).

### 4.3 Frontend

- **Store** `frontend/src/stores/governance.ts` (Pinia) — mirrors `accelerator.ts`:
  state `actions, actionCount, actionPage, votablePillars, numActivePillars`; actions
  `loadActions(page)`, `openAction(id)`, `loadVotablePillars()`. **[P2]** adds
  `votable` state, `needsVoteCount` getter, and `refreshVotable()`.
- **Vote math** `frontend/src/lib/governance.ts` — `ACTION_STATUS` labels
  (`Voting/Approved/Rejected/NoDecision`), `actionTypeLabel` (`Spork`/`Normal`),
  `isActionApproved(votes, action, numPillars)`, `isActionRejected(...)`, an
  `isOpen(action)` helper (`status==Voting && !expired`), and a `quorumProgress` helper
  for display. Pure + unit-tested.
- **Panels** `frontend/src/components/panels/`:
  - `GovernancePanel.vue` — `Tabs` with `Vote` / `Actions`; loads on mount + on
    `wallet.activeIndex` change; accepts `initial-sub` for deep-link.
  - `GovernanceVote.vue` — Phase 1: lists `actions` filtered to open, with a pillar picker
    + Yes/No/Abstain (`PrepareGovernanceVote`). **[P2]:** clone `AcceleratorVote`'s
    per-pillar "needs my vote" filter + "you voted" line.
  - `GovernanceActions.vue` — clone of `AcceleratorProjects` (status filter chips,
    Prev/Next paging + count, expandable detail with destination/decoded-data/thresholds,
    Execute button on executable actions).
- **Home.vue** — add `'Governance'` to `TABS` (8th tab) + `<GovernancePanel>`; extend the
  deep-link mirror to accept `tab=Governance`.
- **[P2] TopBar.vue** — add a governance ballot badge alongside the accelerator one, bound
  to `governance.needsVoteCount`, pushing
  `{ name:'home', query:{ tab:'Governance', sub:'Vote' }}`.

## 5. Confirm-what-you-sign (security)

- **Vote:** confirm modal renders `id` (action name) + selected `pillar` + choice
  label — derived from the built block.
- **Execute:** the sharp edge. A governance action calls **another contract** with
  arbitrary `Data`. The confirm modal renders, from the built block:
  - `Destination` address, labelled with the known embedded contract where recognized
    (Spork/Accelerator/Token/etc. by address).
  - The action `name`/`description` and, where the destination ABI is known, the decoded
    method name; otherwise the raw base64 `Data` is shown verbatim with a "could not
    decode" note. **Never** hide undecodable data.
- All `Prepare*` methods re-validate inputs server-side; never trust frontend validation.

## 6. [P2] Dependencies (SDK + node) — DEFERRED to Phase 2

**Not needed for Phase 1.** These unlock the per-pillar "needs my vote" Vote view + the
top-bar badge. Both additions mirror the existing **accelerator** `getPillarVotes`
exactly; the only substantive change is the contract context (`GovernanceContract` vs
`AcceleratorContract`).

1. **`0x3639/go-zenon` (RPC server), `rpc/api/embedded/governance.go`** — add
   `func (a *GovernanceApi) GetPillarVotes(name string, hashes []types.Hash)
   ([]*definition.PillarVote, error)` using
   `api.GetFrontierContext(a.chain, types.GovernanceContract)` then
   `definition.GetPillarVote(context.Storage(), hash, name)` per hash (nil on
   `ErrDataNonExistent`). Exposes RPC `embedded.governance.getPillarVotes`. New fork
   commit → go-syrius updates its `replace … 0x3639/go-zenon` hash.
2. **`0x3639/znn-sdk-go` (client), `api/embedded/governance.go`** — add
   `func (g *GovernanceApi) GetPillarVotes(name string, hashes []types.Hash)
   ([]*definition.PillarVote, error)` calling
   `g.client.Call(&ans, "embedded.governance.getPillarVotes", name, hashes)`. Tag a new
   SDK version (or re-use the branch) → go-syrius re-pins.

When both land: re-pin the SDK + bump the `replace … 0x3639/go-zenon` fork hash in
go-syrius, then add `GetVotableActionsForMyPillars`, upgrade `GovernanceVote.vue`, and add
the TopBar badge. Phase 1 (Browse + simple Vote + Execute) ships and is testnet-tested
before any of this.

## 7. Testing

- **Backend (Phase 1):** table tests mirroring `nom_accelerator_test.go` for the prepare
  builders — **build each template** so a pack-panic is caught (lesson from the v0.1.19
  UpdatePhase regression). **[P2]:** the votable-action builder + `annotateMyVotes` analogue.
- **Frontend (Phase 1):** vitest for `lib/governance.ts` (vote math across Type1/Type2
  rounds, abstain-excluded-from-directional, `isOpen`), the Vote open-action list + vote
  dispatch, Actions filter/paging, and Execute gating (button only on
  `Approved && !Executed`). **[P2]:** the per-pillar Vote filter (incl. the
  selected-pillar-voted-but-another-hasn't case).
- **Gates:** `pnpm typecheck` + `pnpm test` + `vite build`; `go vet ./... && go test ./...`
  (note the pre-existing local `internal/compat` keystore-roundtrip + `app` keystore
  failures are unrelated to this work).

## 8. Out of scope

- **ProposeAction** (and any "create action" UI).
- Decoding arbitrary destination ABIs beyond known embedded contracts.
- Historical per-round vote breakdowns (only the current round is shown).
