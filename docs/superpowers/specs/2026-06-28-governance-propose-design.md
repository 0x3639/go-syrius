# Governance Propose Tab — Design Spec

**Date:** 2026-06-28
**Status:** Approved design, pre-implementation
**Branch:** `governance-module` (continues the Phase-1 work; Propose is the follow-on)
**Builds on:** `docs/superpowers/specs/2026-06-28-governance-module-design.md` (Phase 1: Browse + Vote + Execute)

## 1. Goal

Add a **Propose** sub-tab to the Governance panel so a pillar operator can submit a new
on-chain governance **action** (which other pillars then vote on, and which executes on
approval). Wraps the SDK `GovernanceApi.ProposeAction(name, description, url, destination,
data) *nom.AccountBlock` — **cost: exactly 1 ZNN, non-refundable**
(`constants.ProjectCreationAmount`).

A governance action is a *privileged call to another embedded contract*: `destination` is
the target contract and `data` is base64-encoded ABI call bytes. Users should not hand-encode
that, so the form builds `destination`+`data` from typed inputs via the SDK `Payload…`
helpers, with a `Custom` escape hatch for raw input.

## 2. Scope — action kinds (full coverage)

The SDK exposes **26** `Payload…` helpers plus a raw path. The form supports all of them,
grouped:

- **Spork (2):** `PayloadSporkCreate(name, description)`, `PayloadSporkActivate(id hash)`.
- **Bridge (15):** AddNetwork, RemoveNetwork, SetTokenPair, RemoveTokenPair, Halt(signature),
  Unhalt, Emergency, ChangeAdministrator(addr), ChangeTssECDSAPubKey(pubKey, signature,
  newSignature), SetAllowKeygen(bool), SetOrchestratorInfo(windowSize, keyGenThreshold,
  confirmationsToFinality, estimatedMomentumTime), SetMetadata(metadata),
  SetNetworkMetadata(networkClass, chainId, metadata), RevokeUnwrapRequest(txHash, logIndex),
  NominateGuardians(guardians []addr).
- **Liquidity (9):** Fund(znnReward, qsrReward), BurnZnn(amount), SetTokenTuple(tokenStandards
  []string, znnPercentages []uint32, qsrPercentages []uint32, minAmounts []bigint),
  SetIsHalted(bool), UnlockStakeEntries(zts), SetAdditionalReward(znnReward, qsrAmount),
  ChangeAdministrator(addr), NominateGuardians(guardians []addr), Emergency.
- **Custom (advanced):** raw `destination` (z1…) + base64 `data`.

> **Open scope note (decide at review):** Liquidity was discovered after the design Q (the
> options only named Spork+Bridge+Custom). It is included here for true full coverage; cut it
> by deleting its catalog entries + builder cases if undesired — nothing else depends on it.

**Testability caveat (honest):** Only **Spork Create** and **Custom** are realistically
exercisable on a normal testnet. Bridge/Liquidity ops require bridge-admin / TSS / liquidity-
admin state and will typically be **rejected on execution** — but the *proposal* still posts
and is votable, so the propose→vote flow is testable with any kind; only the eventual
contract call may fail. Spork Create (name + description only) is the recommended test path.

## 3. Architecture — schema-driven (one form, one dispatcher)

Avoids 26 components and 26 bound methods. A single catalog is the source of truth for both
field rendering and the prepare call; only the per-kind payload *construction* is bespoke Go.

### 3.1 Backend (`app/governance_propose.go`, methods on `NomService`)

- **`GetProposeKinds() ([]ProposeKindDTO, error)`** — returns the catalog the form renders
  from (single source of truth; no duplicated TS schema → no drift). Pure/static (no node I/O
  beyond the existing connection guard is even needed; it can return the static catalog
  regardless of connection). Shape:
  ```go
  type ProposeFieldDTO struct {
      Key         string `json:"key"`
      Label       string `json:"label"`
      Type        string `json:"type"` // "text" | "number" | "bool" | "address" | "hash" | "amount" | "base64" | "list"
      Placeholder string `json:"placeholder"`
      Required    bool   `json:"required"`
  }
  type ProposeKindDTO struct {
      Kind   string            `json:"kind"`   // stable id, e.g. "spork.create", "bridge.setTokenPair", "custom"
      Label  string            `json:"label"`  // human label
      Group  string            `json:"group"`  // "Spork" | "Bridge" | "Liquidity" | "Custom"
      Fields []ProposeFieldDTO `json:"fields"`
  }
  ```
- **`PrepareProposeAction(name, description, url, kind string, params map[string]string) (CallPreview, error)`**
  — the one write-prepare method:
  1. Validate the governance metadata: `name`/`description`/`url` non-empty (and `url` against
     the same URL rule the accelerator uses, reused if applicable). Then `client :=
     currentClient()`; nil → "not connected" (the `Payload…` helpers are methods on
     `client.GovernanceApi`, so a client is required before building the payload).
  2. `payload, err := buildProposalPayload(client, kind, params)` — dispatches on `kind` to a
     small per-kind builder that parses+validates `params` (typed: uint32, `*big.Int`,
     `types.Address`, `types.Hash`, bool, comma-separated lists) and calls the matching SDK
     `Payload…` helper. `custom` parses `destination` (HexToAddress) + `data` (validated as
     standard base64). Unknown kind → error.
  3. `template := client.GovernanceApi.ProposeAction(name, description, url,
     payload.Destination, payload.Data)`.
  4. `prepareCall(template, callExpect{to: types.GovernanceContract, zts:
     types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil),
     template.Data...)}, summary)` — confirm-what-you-sign. `summary` = `Propose "<name>" (1
     ZNN) → <kind label> calls <destination>`.
- **`buildProposalPayload(client, kind, params)`** lives in the same file as a `map[string]func`
  or `switch`; each case is a few lines (parse → SDK helper). A shared `proposeKinds` catalog
  (the same data `GetProposeKinds` returns) documents the fields so the two stay aligned in one
  file.

All parsing/validation is server-side; the form is never trusted.

### 3.2 Frontend

- **Store (`stores/governance.ts`):** add `proposeKinds: ProposeKindDTO[]` + `loadProposeKinds()`
  (calls `Nom.GetProposeKinds()`, swallows to `[]` on error like the other loaders).
  `GovernancePanel` calls it in `load()`.
- **`GovernancePropose.vue`** (3rd sub-tab): a **kind dropdown** (grouped by `group`) + the
  governance metadata fields (name, description, url) + the selected kind's dynamic fields
  rendered from its `ProposeFieldDTO[]` (input type by `field.type`; `bool`→checkbox,
  `list`→comma-separated text, etc.). Collects values into `params: Record<string,string>`,
  calls `tx.awaitConfirm(await Nom.PrepareProposeAction(name, description, url, kind, params))`.
  Gated like the Vote view: only meaningful for pillar operators? — **No**: anyone with 1 ZNN
  can propose, so NO pillar gate; show the form to any unlocked, connected wallet. Surface
  errors via a local `error` ref + `tx.status/tx.error`.
- **`GovernancePanel.vue`:** add the `Propose` sub-tab trigger + `<TabsContent>`.

### 3.3 Bindings
Regenerate Wails bindings (`~/go/bin/wails generate module`) for `GetProposeKinds` +
`PrepareProposeAction` and the new DTOs; keep only NomService + models.ts churn (revert runtime).

## 4. Confirm-what-you-sign / security

- Preview derives from the **built block** (destination + data come from the SDK helper / the
  template, never echoed raw form text into the effect). The confirm summary names the kind and
  destination; the 1 ZNN fee is read from `template.Amount`, never hardcoded.
- `callExpect` asserts `to: GovernanceContract`, `zts: ZNN`, `amount: template.Amount`, and a
  **copy** of `template.Data`.
- Every field re-validated/parsed server-side in `buildProposalPayload`; malformed params →
  error before any node use.
- `Custom` `data` must validate as standard base64 (reject otherwise) and `destination` as a
  valid address.

## 5. Implementation order (vertical slice first)

To get an early testable milestone, build in this order (the plan will mirror it):
1. **Framework + Spork + Custom** — `ProposeKindDTO`/`ProposeFieldDTO`, `GetProposeKinds`
   returning Spork(Create/Activate)+Custom, `PrepareProposeAction` + `buildProposalPayload` for
   those 3 kinds, bindings, store `loadProposeKinds`, `GovernancePropose.vue`, panel tab. **This
   slice is end-to-end testable (Spork Create → propose → vote).**
2. **Bridge kinds** — add the 15 Bridge catalog entries + builder cases (+ tests). Mechanical.
3. **Liquidity kinds** — add the 9 Liquidity catalog entries + builder cases (+ tests).
   Mechanical; cut here if scope is trimmed at review.

## 6. Testing

- **Backend:** table tests for `buildProposalPayload` per kind — valid params → a payload with
  the expected destination (and non-panicking template build, the v0.1.19 lesson); bad/missing
  params → error. Validation tests for `PrepareProposeAction` (empty name/url, unknown kind,
  not-connected). A test asserting `GetProposeKinds` returns the expected groups/kinds.
- **Frontend:** vitest for the dynamic form (renders the selected kind's fields from the
  catalog; switching kind swaps fields; bool/list field types), the Custom path, and that
  submit dispatches `PrepareProposeAction(name, desc, url, kind, params)` with the collected
  params. Panel test updated for the 3rd tab.
- **Gates:** `pnpm typecheck` + `pnpm test` + `vite build`; `go vet ./...` + `go test ./app/
  -run Governance`.

## 7. Out of scope

- Editing/cancelling a proposed action (no such on-chain op for governance actions).
- Per-kind "advanced validation" beyond what the contract itself enforces (the node is the
  authority; we do basic type/format validation only).
- Decoding/displaying an existing action's `data` back into typed fields (the Actions tab
  already shows raw destination + base64 data).
