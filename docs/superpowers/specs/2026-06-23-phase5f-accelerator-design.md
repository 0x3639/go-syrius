# Phase 5f — Accelerator-Z design

**Date:** 2026-06-23
**Branch:** `phase-5f-accelerator`
**Scope:** Full parity (browse + donate + voting + project/phase creation)

## Context

Phase 5f is the final NoM feature sub-phase (plan.md §3, Phase 5). It brings
parity with syrius's Accelerator-Z screen: the on-chain crowdfunding/governance
system where projects request ZNN/QSR funding, split work into phases, the
community donates, and Pillars vote on funding/approval.

The backend already exists in `znn-sdk-go`'s `AcceleratorApi`; this phase is the
thin binding layer + frontend rebuild, following the exact pattern established by
the prior NoM sub-phases (Plasma 5a … Tokens 5e): `NomService` read methods
returning DTOs, `Prepare*` methods returning a `CallPreview` through the shared
confirm-what-you-sign path (`tx.prepareCall`), a per-feature Svelte store, one
route, and a manual testnet acceptance record.

A prerequisite SDK bug was fixed first and shipped as `znn-sdk-go v0.1.18`: the
`AcceleratorApi.VoteByName` doc comment had the vote mapping backwards. The
on-chain authority (go-zenon `vm/embedded/definition`) is **`VoteYes=0`,
`VoteNo=1`, `VoteAbstain=2`**, and the contract tallies a vote of `0` as "yes".
v0.1.18 corrects the docs and **exports `embedded.VoteYes/VoteNo/VoteAbstain`**
(aliased to the go-zenon constants) so this phase references them instead of
hardcoding vote bytes. The `go.mod` bump to v0.1.18 is the first commit on this
branch.

## SDK surface used (`AcceleratorApi`)

- Reads: `GetAll(pageIndex, pageSize) -> *ProjectList`, `GetProjectById(id) ->
  *Project`, `GetPhaseById(id) -> *Phase`, `GetVoteBreakdown(id) ->
  *VoteBreakdown`, `GetPillarVotes(name, hashes) -> []*PillarVote`.
- Writes (return unsigned `*nom.AccountBlock` templates):
  - `Donate(amount, tokenStandard)` — ZNN or QSR.
  - `VoteByName(id, pillarName, vote)` — Pillar-operator only.
  - `CreateProject(name, description, url, znnFundsNeeded, qsrFundsNeeded)` —
    costs a fixed **1 ZNN** creation fee (`ProjectCreationAmount`).
  - `AddPhase(id, name, description, url, znnFundsNeeded, qsrFundsNeeded)`.
  - `UpdatePhase(id, name, description, url, znnFundsNeeded, qsrFundsNeeded)`.
- Pillar ownership (for voting eligibility): `PillarApi.GetByOwner(address) ->
  []*PillarInfo`.

On-chain validation rules to mirror (go-zenon `vm/constants` +
`vm/embedded/implementation/accelerator.go`):
- Name: 1–30 chars (`ProjectNameLengthMax = 30`; empty rejected).
- Description: 1–240 chars (`ProjectDescriptionLengthMax = 240`; empty rejected).
- URL: non-empty and must match the contract regex
  `^([Hh][Tt][Tt][Pp][Ss]?://)?[a-zA-Z0-9]{2,60}\.[a-zA-Z]{1,6}([-a-zA-Z0-9()@:%_+.~#?&/=]{0,100})$`.
- ZnnFundsNeeded ≤ 5000 ZNN (`ProjectZnnMaximumFunds`); QsrFundsNeeded ≤ 50000
  QSR (`ProjectQsrMaximumFunds`).
- Phase funds: each phase's funds ≤ the project's; the sum of a project's phase
  funds ≤ the project's funds (enforced on-chain at add/update; we surface clear
  errors but the chain is the final arbiter).
- Project creation fee: exactly 1 ZNN (`ProjectCreationAmount`).

## Backend — `NomService` additions

DTOs added to `app/dto.go`; methods added to `app/nom_service.go`.

### Reads (return plain DTOs, like `GetMyTokens`/`GetTokenByZts`)

- `GetProjects(pageIndex, pageSize uint32) (ProjectListDTO, error)` — wraps
  `GetAll`. Paged project summaries: id, name, owner, status, ZNN/QSR requested,
  vote breakdown (Yes/No/Total), phase count, creation timestamp.
- `GetProject(id string) (ProjectDTO, error)` — `GetProjectById`, plus its phase
  summaries (resolved via `GetPhaseById` over `PhaseIds`) and vote breakdown.
- `GetPhase(id string) (PhaseDTO, error)` — `GetPhaseById` (embeds vote
  breakdown).
- `GetVotablePillars() ([]string, error)` — `PillarApi.GetByOwner(activeAddr)`
  mapped to names; drives voting eligibility and the pillar picker.

### Writes (via `tx.prepareCall` -> `CallPreview`, confirm-what-you-sign)

- `PrepareDonate(amount, token string) (CallPreview, error)` — `token ∈
  {"ZNN","QSR"}`, amount parses to > 0 at 8 decimals. -> `Donate`.
- `PrepareVote(id, pillarName string, vote uint8) (CallPreview, error)` —
  validates `vote ∈ {VoteYes, VoteNo, VoteAbstain}` using the SDK constants;
  re-validates `pillarName` is owned by the active address via `GetByOwner`. ->
  `VoteByName`.
- `PrepareCreateProject(name, description, url, znnNeeded, qsrNeeded string)
  (CallPreview, error)` — field validation mirroring on-chain rules (name ≤ 30,
  description ≤ 240, url length, funds parse); surfaces the 1 ZNN fee. ->
  `CreateProject`.
- `PrepareAddPhase(projectId, name, description, url, znnNeeded, qsrNeeded
  string) (CallPreview, error)` — `projectId` must be a project owned by the
  active address (re-validated). -> `AddPhase`.
- `PrepareUpdatePhase(phaseId, name, description, url, znnNeeded, qsrNeeded
  string) (CallPreview, error)` -> `UpdatePhase`.

### Correctness invariants

- Vote values come from `embedded.VoteYes/VoteNo/VoteAbstain` (go-zenon
  authority), never raw ints in app code.
- Every `Prepare*` re-validates inputs server-side (amounts, field lengths,
  pillar/project ownership) — frontend validation is never trusted.
- All writes go through the existing `CallPreview` -> confirm modal -> submit
  path; the confirm renders the effect derived from the built block.

## Frontend (`frontend/src/`)

- **Nav:** add `'accelerator'` to the `View` union in `lib/stores/nav.ts`; add a
  nav button in `App.svelte` after Tokens.
- **Store — `lib/stores/accelerator.ts`** (mirrors `token.ts`): `projects` +
  paging cursor, `selectedProject`/`phases`, `votablePillars`, `loading`/`error`.
  Actions delegate to bindings (`loadProjects`, `openProject`,
  `loadVotablePillars`, the `prepare*` calls); refresh the affected read after a
  successful submit.
- **Route — `routes/Accelerator.svelte`** (one route, multiple sections, like
  `Tokens.svelte`):
  - **Browse** — paged project list (name, status, ZNN/QSR requested, Yes/No/Total
    votes). Click a project to expand its phases + per-phase vote breakdown and
    acceptance-threshold status.
  - **Donate** — amount + token toggle (ZNN/QSR) -> `PrepareDonate` -> `TxModal`.
  - **Vote** — rendered only when `votablePillars` is non-empty. Target
    (project/phase id from browse selection), pillar (if operator owns > 1), and
    Yes/No/Abstain -> `PrepareVote` -> confirm. The three choices map to the SDK
    vote constants surfaced from the binding, not raw ints.
  - **Create / Manage** — "Create project" form (shows the 1 ZNN fee); and for
    projects owned by the active address, "Add phase" / "Update phase" forms.
    Gated behind a disclosure to keep the default view clean.
- All writes reuse the existing `CallPreview` -> `TxModal` confirm-what-you-sign
  path; no new signing UI.

## Testing & acceptance

- **`app/nom_service_test.go`** (mirror token validation tests): bad donate
  amount/token; vote value out of range; empty/non-owned pillar; project/phase
  field-length validation; and an explicit assertion that a "Yes" vote emits byte
  `0` — guards the just-fixed SDK mapping against regression at our layer.
- **`frontend/src/routes/Accelerator.test.ts`** (mirror `Tokens.test.ts`):
  sections render; Vote hidden when no votable pillars; donate token toggle;
  confirm path invoked.
- **`docs/phase5f-acceptance.md`** — manual testnet record: browse live projects,
  donate a small QSR amount, and (if a testnet pillar is available) one live vote.

## Branch & commit plan

Branch `phase-5f-accelerator` off `main`. Commit order:
1. `go.mod`/`go.sum` bump to SDK v0.1.18 (done).
2. Backend: DTOs + `NomService` reads + `Prepare*` writes (+ unit tests).
3. Frontend: store + route + nav (+ component test).
4. Acceptance doc.

Merge per the SDD workflow once the manual testnet acceptance passes.
