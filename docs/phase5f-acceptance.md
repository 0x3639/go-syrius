# Phase 5f — Accelerator-Z Acceptance

> Status: **Automated + live read gates PASSED.** The Accelerator read path
> (`AcceleratorApi.GetAll` → `GetProjectById` → `GetPhaseById`) succeeds against the testnet node,
> exercising the exact SDK calls `NomService.GetProjects/GetProject/GetPhase` use. Manual GUI
> acceptance (the on-testnet write flow: donate / vote / create / add+update phase) remains PENDING
> a user run.

## Scope
Full-parity Accelerator-Z: browse projects/phases, donate (ZNN/QSR), Pillar voting
(VoteByName), and project/phase create + update. Backend in `app/nom_accelerator.go` (9 NomService
methods + 4 DTOs in `app/dto.go`); frontend store `accelerator.ts` + route `Accelerator.svelte`.
Built on `znn-sdk-go v0.1.19`: v0.1.18 fixed a backwards Accelerator vote-mapping and exported
`embedded.VoteYes`(0)/`VoteNo`(1)/`VoteAbstain`(2); v0.1.19 fixed `AcceleratorApi.UpdatePhase`,
which packed the wrong ABI method (`Update`, 0 inputs) with 6 args and panicked before returning.

Note on UpdatePhase id semantics: on-chain `UpdatePhase` is keyed by the **project id** (the
contract looks up the project and updates its current voting phase), mirroring `AddPhase` — not a
phase id. The route field is labelled "project id" accordingly.

## Automated verification (2026-06-23) — PASSED
- `GOWORK=off go test ./...` — green across all packages: `app`, `internal/compat`,
  `internal/embeddednode`, `internal/sdksmoke`, `internal/version` (root has no test files).
  `GOWORK=off` is required (a parent `go.work` references a missing sibling module — local-env
  artifact).
- `app` package Phase-5f cases — PASS (alongside the 5a–5e regression tests, still green):
  - `TestProjectDTONilSafe` / `TestProjectDTOMapsFieldsAndPhases` — DTO mapping is nil-safe (nil
    funds→"0", nil votes→zero, nil/empty phases→non-nil empty slice) and maps every project/phase/
    vote field.
  - `TestAcceleratorReadsGuardInputs` — invalid hash rejected before node use; reads need a node;
    `GetVotablePillars` needs an unlocked wallet.
  - `TestPrepareDonateValidatesInput` / `TestDonateTemplateTokenStandards` — token whitelist + amount
    rules; the QSR (non-ZNN) donation template asserts `to=AcceleratorContract` + `zts=QSR` +
    `amount` (donation zts is the chosen token, NOT hardcoded ZNN).
  - `TestVoteConstantsMatchOnChainAuthority` — regression guard: `VoteYes==0 && VoteNo==1 &&
    VoteAbstain==2` (catches any SDK drift of the just-fixed mapping at our layer).
  - `TestPrepareVoteValidatesInput` / `TestVoteTemplate` — id/pillar/vote-range validation before
    node use; VoteByName template = `AcceleratorContract` + ZNN + amount 0.
  - `TestPrepareProjectWritesValidateInput` / `TestProjectWriteTemplateTokenStandards` — shared
    `validateProjectFields` (name ≤30, desc ≤240, URL regex, funds ≤ max) before node use;
    CreateProject carries `constants.ProjectCreationAmount` (1 ZNN), AddPhase/UpdatePhase amount 0,
    all ZNN/AcceleratorContract.
- `GOWORK=off go build ./...` and `GOWORK=off go vet ./app/` — clean.
- `GOWORK=off go vet -tags integration ./internal/spike/` — the opt-in integration test (incl. the
  new `TestReadOnlyAccelerator`) compiles; exit 0.
- Frontend `npx vitest run` — 40/40 across 19 files (incl. `src/routes/Accelerator.test.ts`, 3
  tests: lists projects, hides voting when no pillars owned, shows voting when a pillar is owned).
- `npx svelte-check --threshold error` — `0 errors, 0 warnings` (3 cosmetic hints).
- `git diff --check main..HEAD` — clean (the generated `models.ts` trailing whitespace flagged by
  the final review was stripped; whitespace-only, `git diff -w` empty).

Covered by tests (offline):
- Vote-byte correctness: vote values come only from `embedded.VoteYes/VoteNo/VoteAbstain` (never
  literals); the regression guard pins them to 0/1/2 — the whole reason `znn-sdk-go` was bumped to
  v0.1.18. The frontend `<select>` uses numeric `value={0|1|2}` and the Go guard rejects anything
  out of range.
- Confirm-what-you-sign: every `Prepare*` builds `callExpect` from the SDK template
  (`amount: template.Amount`, deep-copied `Data`); `ConfirmPublish` re-runs `assertMatches` against
  the **built block** before publish, so the confirm modal renders the effect from the block, not
  raw form inputs. Donate's zts tracks the chosen token; the create fee rides `template.Amount`.

## Live read-only smoke (2026-06-23) — PASSED
Run against the testnet node `ws://172.245.236.40:35998` (opt-in integration test
`internal/spike.TestReadOnlyAccelerator`, `-tags integration`).

Observed output (verbatim):
```
=== RUN   TestReadOnlyAccelerator
    readonly_integration_test.go:208: projects: count=0 returned=0
    readonly_integration_test.go:210: no projects on this chain — read path proven, nothing to drill
--- PASS: TestReadOnlyAccelerator (0.21s)
PASS
ok  	github.com/0x3639/go-syrius/internal/spike	0.784s
```

- `AcceleratorApi.GetAll(0, 10)` **succeeded** — `count=0` (this testnet chain currently has no
  Accelerator-Z projects; the call **succeeding** is the verification, and proves the node exposes
  the `embedded` RPC namespace).
- Because `count=0`, the live drill into `GetProjectById` / `GetPhaseById` could not be exercised
  against real project data — that single-entity path is covered by the offline DTO-mapping tests
  and remains to be confirmed live whenever a chain with projects is available (or via the manual
  GUI run on such a chain). Honest limitation, not a failure.

Repro:
```
ZNN_NODE_URL="ws://172.245.236.40:35998" \
  GOWORK=off go test -tags integration ./internal/spike/ -run TestReadOnlyAccelerator -v -count=1
```

## Manual acceptance (Phase 5f gate) — PENDING (user GUI run)
> Node prerequisite (carried from 5b–5e): the connected node must expose the `embedded` RPC
> namespace. A `ledger`-only node returns `embedded.* does not exist` for reads and
> `embedded.* not available` for PoW-requiring actions (donate/vote/create/add+update all require
> PoW). Not a wallet bug.

> The on-testnet write flow below has NOT been run by the user for 5f. Each item is PENDING and no
> tx hashes are recorded.

On a testnet node with the `embedded` namespace enabled, with funded/eligible accounts
(Accelerator route):
1. [PENDING] Open Accelerator-Z → the project list loads (or "No projects"); clicking a project's
   "Phases" expands its phases + per-phase vote tallies.
2. [PENDING] Donate a small QSR amount (e.g. `100000000` base units = 1 QSR) → TxModal shows
   "Donate 1 QSR to Accelerator-Z" (zts = QSR, amount = the donation) → Confirm → block confirms.
3. [PENDING] Vote (requires an address that operates a testnet Pillar): the Vote section appears
   only when `GetVotablePillars` is non-empty; cast Yes/No/Abstain on a live proposal id → TxModal
   shows the vote summary (zts = ZNN, amount 0) → Confirm → vote tally updates. If no testnet Pillar
   is available, record as not-exercised.
4. [PENDING] Create a project (costs 1 ZNN, if testnet ZNN available) → TxModal shows
   "Create project … (1 ZNN fee)" (zts = ZNN, amount = 1 ZNN from the template) → Confirm → the
   project appears in the list. Then Add phase / Update phase on an owned project. If no testnet ZNN
   is available, record as not-exercised.
5. [PENDING] Mainnet guard: with `AllowMainnetSend` false on a mainnet node, all write actions are
   blocked (mainnet-gated).

Confirm-modal check to verify during the manual run (confirm-what-you-sign): the modal must render
the effect from the **built block**, not raw form inputs — Donate = the chosen token (ZNN or QSR);
Vote / AddPhase / UpdatePhase = ZNN, amount 0; CreateProject = ZNN with the 1 ZNN fee. The offline
`TestDonateTemplateTokenStandards` / `TestVoteTemplate` / `TestProjectWriteTemplateTokenStandards`
already lock these against the SDK builders; the vote-byte mapping is locked by
`TestVoteConstantsMatchOnChainAuthority`.

## Security recap
- Reuses the one audited prepare/confirm/publish path; `ConfirmPublish` re-asserts the built block's
  to/zts/amount AND ABI `Data` before publish; mainnet gated by `AllowMainnetSend`; no key material
  in NomService (frontend sends intent only).
- Vote bytes sourced exclusively from the SDK constants (go-zenon authority: vote 0 == "yes"),
  regression-locked — the correctness-critical fix this phase depends on.
- All Accelerator call templates verified against the SDK builders — Donate = chosen token, Vote =
  ZNN/0, Create = ZNN/1-ZNN-fee, AddPhase/UpdatePhase = ZNN/0.
- All write inputs re-validated server-side (field lengths/URL/funds mirror go-zenon; vote range;
  token whitelist) before any node use — never trust frontend validation.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded —
  `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering) but does
  not human-decode arbitrary params; tracked as Phase-5/7 hardening. Pillar-ownership re-validation
  for voting relies on the on-chain check (the contract rejects a non-owner vote); the UI gates by
  `GetVotablePillars`.
