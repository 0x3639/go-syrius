# Governance Propose Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Propose sub-tab to the Governance panel so a pillar operator can submit a new on-chain governance action (Spork / Bridge / Liquidity / Custom) that other pillars then vote on.

**Architecture:** Schema-driven. A single backend catalog (`GetProposeKinds`) is the source of truth the frontend renders fields from; a single dispatcher (`PrepareProposeAction(name, description, url, kind, params)`) parses+validates the per-kind params, builds `destination`+`data` via the SDK `Payload…` helpers, wraps with `ProposeAction`, and runs the existing confirm-what-you-sign `prepareCall` seam. One dynamic Vue form renders any kind's fields.

**Tech Stack:** Go 1.25.11 + Wails v2, `znn-sdk-go` `GovernanceApi` (pinned `v0.1.20-0.20260628114538-fb41ea138645`); Vue 3 + TS + Pinia + nom-ui; vitest.

## Global Constraints

- All local Go/Wails commands prefixed `GOWORK=off GOTOOLCHAIN=auto`. `wails` is NOT on PATH → `~/go/bin/wails`. Frontend commands run in `frontend/` (pnpm 10.17.1).
- Do NOT change the SDK / go-zenon pins.
- Binding invariant: frontend sends intent only; every state-changing Go method re-validates ALL inputs server-side before any node use; never trust the form.
- Confirm-what-you-sign: preview derives from the BUILT block. `PrepareProposeAction`'s `callExpect` asserts `to: types.GovernanceContract`, `zts: types.ZnnTokenStandard`, `amount: template.Amount` (the 1 ZNN fee, read from the template — never hardcoded), and a COPY of `template.Data` (`append([]byte(nil), template.Data...)`).
- ProposeAction cost is exactly 1 ZNN, non-refundable.
- Wails bindings (`frontend/wailsjs/`) are git-tracked; regenerate with `GOWORK=off GOTOOLCHAIN=auto ~/go/bin/wails generate module`, keep ONLY `NomService.{d.ts,js}` + `models.ts` (revert `frontend/wailsjs/runtime` churn via `git checkout HEAD -- frontend/wailsjs/runtime`).
- nom-ui `Button` test stubs MUST declare `emits: ['click']` (Vue default `inheritAttrs` otherwise double-fires `@click`); do NOT use `inheritAttrs:false` (strips aria-label fallthrough).
- Commits are SIGNED and made by the CONTROLLER; implementers STAGE ONLY (`git add …`), never `git commit`.
- Pre-existing UNRELATED local failures: `internal/compat` keystore + one `app` keystore test. Scope `go test` to `./app/ -run <pattern>`.
- No pillar gate on Propose (anyone with 1 ZNN can propose); the form shows for any unlocked, connected wallet.

### Parsing toolkit (defined in Task 1, used by all kinds)

`map[string]string` params, parsed/validated server-side:
- `reqParam(p,key) (string,error)` — trimmed, non-empty.
- `optParam(p,key) string` — trimmed (may be "").
- `parseU32Param(p,key) (uint32,error)`, `parseU64Param(p,key) (uint64,error)`, `parseBoolParam(p,key) (bool,error)`, `parseBigIntParam(p,key) (*big.Int,error)` (sign ≥ 0), `parseAddrParam(p,key) (types.Address,error)`, `parseHashParam(p,key) (types.Hash,error)`, `parseZtsParam(p,key) (types.ZenonTokenStandard,error)`.
- List params are comma-separated: `parseAddrList`, `parseU32List`, `parseBigIntList`, `parseStrList(p,key) ([]string,error)`.

### Field `type` vocabulary (catalog → form renderer)

`text | number | bool | address | hash | amount | base64 | list` — the form (Task 3) implements a renderer for EACH so Bridge/Liquidity kinds (Tasks 5–6) need no frontend change.

---

### Task 1: Backend framework — DTOs, parsing toolkit, GetProposeKinds (Spork+Custom), PrepareProposeAction

**Files:**
- Create: `app/governance_propose.go`
- Modify: `app/dto.go` (append `ProposeFieldDTO`, `ProposeKindDTO`)
- Test: `app/governance_propose_test.go`

**Interfaces:**
- Consumes: `s.node.currentClient()`; `s.tx.prepareCall`; `callExpect`; `parseHash` (from nom_accelerator.go); `client.GovernanceApi.ProposeAction(name, description, url string, destination types.Address, data string) *nom.AccountBlock`; `client.GovernanceApi.PayloadSporkCreate(name, description string) ProposalPayload`; `client.GovernanceApi.PayloadSporkActivate(id types.Hash) ProposalPayload`; `embedded.ProposalPayload{Destination types.Address; Data string}`.
- Produces: DTOs `ProposeFieldDTO`, `ProposeKindDTO`; `func (s *NomService) GetProposeKinds() ([]ProposeKindDTO, error)`; `func (s *NomService) PrepareProposeAction(name, description, url, kind string, params map[string]string) (CallPreview, error)`; `func buildProposalPayload(client *rpc_client.RpcClient, kind string, params map[string]string) (embedded.ProposalPayload, error)`; the `proposeKinds()` catalog; the parsing toolkit helpers listed above.

- [ ] **Step 1: Add DTOs to `app/dto.go`** (append after `ActionListDTO`)

```go
// ProposeFieldDTO is one input field the Propose form renders for an action kind.
type ProposeFieldDTO struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // text|number|bool|address|hash|amount|base64|list
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
}

// ProposeKindDTO is one proposable governance action kind + its input schema.
type ProposeKindDTO struct {
	Kind   string            `json:"kind"`  // stable id, e.g. "spork.create"
	Label  string            `json:"label"`
	Group  string            `json:"group"` // Spork|Bridge|Liquidity|Custom
	Fields []ProposeFieldDTO `json:"fields"`
}
```

- [ ] **Step 2: Write the failing tests `app/governance_propose_test.go`**

```go
package app

import (
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestGetProposeKinds_HasSporkAndCustom(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	kinds, err := s.GetProposeKinds()
	if err != nil {
		t.Fatalf("GetProposeKinds err: %v", err)
	}
	byId := map[string]ProposeKindDTO{}
	for _, k := range kinds {
		byId[k.Kind] = k
	}
	for _, want := range []string{"spork.create", "spork.activate", "custom"} {
		if _, ok := byId[want]; !ok {
			t.Fatalf("missing kind %q", want)
		}
	}
	if byId["spork.create"].Group != "Spork" || len(byId["spork.create"].Fields) != 2 {
		t.Fatalf("spork.create schema wrong: %+v", byId["spork.create"])
	}
	if byId["custom"].Group != "Custom" {
		t.Fatalf("custom group wrong: %+v", byId["custom"])
	}
}

func TestBuildProposalPayload_SporkCreate(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	// build directly via the SDK helper to confirm our dispatcher mirrors it
	want := api.PayloadSporkCreate("MySpork", "desc")
	got, err := buildProposalPayloadWith(api, "spork.create", map[string]string{"name": "MySpork", "description": "desc"})
	if err != nil {
		t.Fatalf("build err: %v", err)
	}
	if got.Destination != want.Destination || got.Data != want.Data {
		t.Fatalf("spork.create payload mismatch: got %+v want %+v", got, want)
	}
}

func TestBuildProposalPayload_Custom(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	dest := types.SporkContract.String()
	got, err := buildProposalPayloadWith(api, "custom", map[string]string{"destination": dest, "data": "AAEC"})
	if err != nil {
		t.Fatalf("custom err: %v", err)
	}
	if got.Destination != types.SporkContract || got.Data != "AAEC" {
		t.Fatalf("custom payload wrong: %+v", got)
	}
	if _, err := buildProposalPayloadWith(api, "custom", map[string]string{"destination": dest, "data": "not base64!!"}); err == nil {
		t.Fatal("invalid base64 data must error")
	}
	if _, err := buildProposalPayloadWith(api, "custom", map[string]string{"destination": "nope", "data": "AAEC"}); err == nil {
		t.Fatal("invalid destination must error")
	}
}

func TestBuildProposalPayload_UnknownKind(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	if _, err := buildProposalPayloadWith(api, "bogus.kind", map[string]string{}); err == nil {
		t.Fatal("unknown kind must error")
	}
}

func TestPrepareProposeAction_Validation(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	good := map[string]string{"name": "S", "description": "d"}
	if _, err := s.PrepareProposeAction("", "d", "https://zenon.org", "spork.create", good); err == nil {
		t.Fatal("empty action name must error")
	}
	if _, err := s.PrepareProposeAction("Act", "d", "bad-url", "spork.create", good); err == nil {
		t.Fatal("bad url must error")
	}
	if _, err := s.PrepareProposeAction("Act", "d", "https://zenon.org", "spork.create", good); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid propose should hit not-connected; got %v", err)
	}
}
```

Note: the tests call `buildProposalPayloadWith(api, kind, params)` — a thin seam that takes a `*embedded.GovernanceApi` directly so payload construction is unit-testable without a live `rpc_client`. `buildProposalPayload(client, …)` (the production entry) calls `buildProposalPayloadWith(client.GovernanceApi, …)`.

- [ ] **Step 3: Run the tests — verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestGetProposeKinds|TestBuildProposalPayload|TestPrepareProposeAction' -v`
Expected: FAIL — undefined `GetProposeKinds`, `buildProposalPayloadWith`, `PrepareProposeAction`.

- [ ] **Step 4: Create `app/governance_propose.go`**

```go
package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	rpc_client "github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/common/types"
)

// ---- parsing toolkit (shared by all kinds) ----

func reqParam(p map[string]string, key string) (string, error) {
	v := strings.TrimSpace(p[key])
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}
func optParam(p map[string]string, key string) string { return strings.TrimSpace(p[key]) }

func parseU32Param(p map[string]string, key string) (uint32, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be a non-negative whole number", key)
	}
	return uint32(n), nil
}
func parseU64Param(p map[string]string, key string) (uint64, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a non-negative whole number", key)
	}
	return n, nil
}
func parseBoolParam(p map[string]string, key string) (bool, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return false, err
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}
func parseBigIntParam(p map[string]string, key string) (*big.Int, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return nil, err
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.Sign() < 0 {
		return nil, fmt.Errorf("%s must be a non-negative integer amount", key)
	}
	return n, nil
}
func parseAddrParam(p map[string]string, key string) (types.Address, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return types.Address{}, err
	}
	a, err := types.ParseAddress(s)
	if err != nil {
		return types.Address{}, fmt.Errorf("%s is not a valid address", key)
	}
	return a, nil
}
func parseHashParam(p map[string]string, key string) (types.Hash, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return types.Hash{}, err
	}
	h, err := types.HexToHash(s)
	if err != nil {
		return types.Hash{}, fmt.Errorf("%s is not a valid hash", key)
	}
	return h, nil
}
func parseZtsParam(p map[string]string, key string) (types.ZenonTokenStandard, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return types.ZenonTokenStandard{}, err
	}
	z, err := types.ParseZTS(s)
	if err != nil {
		return types.ZenonTokenStandard{}, fmt.Errorf("%s is not a valid token standard", key)
	}
	return z, nil
}
func splitList(p map[string]string, key string) ([]string, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, x := range parts {
		if t := strings.TrimSpace(x); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s must have at least one value", key)
	}
	return out, nil
}
func parseStrList(p map[string]string, key string) ([]string, error) { return splitList(p, key) }
func parseAddrList(p map[string]string, key string) ([]types.Address, error) {
	items, err := splitList(p, key)
	if err != nil {
		return nil, err
	}
	out := make([]types.Address, 0, len(items))
	for _, s := range items {
		a, err := types.ParseAddress(s)
		if err != nil {
			return nil, fmt.Errorf("%s contains an invalid address: %s", key, s)
		}
		out = append(out, a)
	}
	return out, nil
}
func parseU32List(p map[string]string, key string) ([]uint32, error) {
	items, err := splitList(p, key)
	if err != nil {
		return nil, err
	}
	out := make([]uint32, 0, len(items))
	for _, s := range items {
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%s contains an invalid number: %s", key, s)
		}
		out = append(out, uint32(n))
	}
	return out, nil
}
func parseBigIntList(p map[string]string, key string) ([]*big.Int, error) {
	items, err := splitList(p, key)
	if err != nil {
		return nil, err
	}
	out := make([]*big.Int, 0, len(items))
	for _, s := range items {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok || n.Sign() < 0 {
			return nil, fmt.Errorf("%s contains an invalid amount: %s", key, s)
		}
		out = append(out, n)
	}
	return out, nil
}

// ---- catalog (single source of truth for the form) ----

func proposeKinds() []ProposeKindDTO {
	return []ProposeKindDTO{
		{Kind: "spork.create", Label: "Spork — Create", Group: "Spork", Fields: []ProposeFieldDTO{
			{Key: "name", Label: "Spork name", Type: "text", Placeholder: "my-spork", Required: true},
			{Key: "description", Label: "Spork description", Type: "text", Placeholder: "What this spork gates", Required: true},
		}},
		{Kind: "spork.activate", Label: "Spork — Activate", Group: "Spork", Fields: []ProposeFieldDTO{
			{Key: "id", Label: "Spork id (hash)", Type: "hash", Placeholder: "0x…", Required: true},
		}},
		{Kind: "custom", Label: "Custom (advanced)", Group: "Custom", Fields: []ProposeFieldDTO{
			{Key: "destination", Label: "Destination contract", Type: "address", Placeholder: "z1…", Required: true},
			{Key: "data", Label: "Call data (base64)", Type: "base64", Placeholder: "base64-encoded ABI call bytes", Required: true},
		}},
	}
}

// ---- dispatcher ----

func buildProposalPayloadWith(api *embedded.GovernanceApi, kind string, p map[string]string) (embedded.ProposalPayload, error) {
	switch kind {
	case "spork.create":
		name, err := reqParam(p, "name")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		desc, err := reqParam(p, "description")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadSporkCreate(name, desc), nil
	case "spork.activate":
		id, err := parseHashParam(p, "id")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadSporkActivate(id), nil
	case "custom":
		dest, err := parseAddrParam(p, "destination")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		data, err := reqParam(p, "data")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		if _, err := base64.StdEncoding.DecodeString(data); err != nil {
			return embedded.ProposalPayload{}, errors.New("data must be valid standard base64")
		}
		return embedded.ProposalPayload{Destination: dest, Data: data}, nil
	}
	return embedded.ProposalPayload{}, fmt.Errorf("unknown action kind %q", kind)
}

func buildProposalPayload(client *rpc_client.RpcClient, kind string, p map[string]string) (embedded.ProposalPayload, error) {
	return buildProposalPayloadWith(client.GovernanceApi, kind, p)
}

// ---- bound methods ----

// GetProposeKinds returns the static catalog of proposable action kinds + their
// input schema. No node I/O; safe before connection (the form renders from it).
func (s *NomService) GetProposeKinds() ([]ProposeKindDTO, error) {
	return proposeKinds(), nil
}

// PrepareProposeAction validates the metadata + per-kind params server-side,
// builds destination+data via the SDK Payload helper, and wraps ProposeAction
// (1 ZNN fee, read from the template). Confirm-what-you-sign via prepareCall.
func (s *NomService) PrepareProposeAction(name, description, url, kind string, params map[string]string) (CallPreview, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	url = strings.TrimSpace(url)
	if name == "" {
		return CallPreview{}, errors.New("action name is required")
	}
	if description == "" {
		return CallPreview{}, errors.New("action description is required")
	}
	if url == "" || !acceleratorURLRe.MatchString(url) {
		return CallPreview{}, errors.New("invalid URL")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	payload, err := buildProposalPayload(client, kind, params)
	if err != nil {
		return CallPreview{}, err
	}
	template := client.GovernanceApi.ProposeAction(name, description, url, payload.Destination, payload.Data)
	label := kind
	for _, k := range proposeKinds() {
		if k.Kind == kind {
			label = k.Label
			break
		}
	}
	return s.tx.prepareCall(template,
		callExpect{to: types.GovernanceContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Propose %q (1 ZNN) — %s calls %s", name, label, payload.Destination.String()))
}
```

Note on imports: `acceleratorURLRe` is defined in `app/nom_accelerator.go` (reused). If `rpc_client` is already imported under a different alias elsewhere in the package, match that alias; the canonical path is `github.com/0x3639/znn-sdk-go/rpc_client`.

- [ ] **Step 5: Run the tests — verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestGetProposeKinds|TestBuildProposalPayload|TestPrepareProposeAction' -v`
Expected: PASS (6 tests).

- [ ] **Step 6: Vet + build**

Run: `GOWORK=off GOTOOLCHAIN=auto go vet ./app/... && GOWORK=off GOTOOLCHAIN=auto go build ./...`
Expected: no errors (ignore the gopsutil/IOKit cgo deprecation warning).

- [ ] **Step 7: Stage (controller commits)**

```bash
git add app/governance_propose.go app/dto.go app/governance_propose_test.go
```
Do NOT commit.

---

### Task 2: Regenerate Wails bindings

**Files:**
- Modify (generated): `frontend/wailsjs/go/app/NomService.{d.ts,js}`, `frontend/wailsjs/go/models.ts`

**Interfaces:**
- Produces (TS): `GetProposeKinds():Promise<Array<app.ProposeKindDTO>>`, `PrepareProposeAction(arg1:string,arg2:string,arg3:string,arg4:string,arg5:{[key:string]:string}):Promise<app.CallPreview>`; `app.ProposeKindDTO`, `app.ProposeFieldDTO` in models.ts.

- [ ] **Step 1: Generate**

Run (repo root): `GOWORK=off GOTOOLCHAIN=auto ~/go/bin/wails generate module`
Then revert unrelated runtime churn if any: `git checkout HEAD -- frontend/wailsjs/runtime`.

- [ ] **Step 2: Verify symbols**

Run: `grep -n "GetProposeKinds\|PrepareProposeAction" frontend/wailsjs/go/app/NomService.d.ts && grep -n "class ProposeKindDTO\|class ProposeFieldDTO" frontend/wailsjs/go/models.ts`
Expected: both functions + both classes present.

- [ ] **Step 3: Typecheck**

Run: `cd frontend && pnpm run typecheck`
Expected: PASS.

- [ ] **Step 4: Stage** (controller commits)

```bash
git add frontend/wailsjs/
```
Confirm via `git status --short frontend/wailsjs/` that ONLY `NomService.d.ts`, `NomService.js`, `models.ts` changed; report the list. Do NOT commit.

---

### Task 3: Frontend — store loader + `GovernancePropose.vue` dynamic form

**Files:**
- Modify: `frontend/src/stores/governance.ts`
- Create: `frontend/src/components/panels/GovernancePropose.vue`
- Test: `frontend/src/components/panels/GovernancePropose.test.ts`

**Interfaces:**
- Consumes: `Nom.GetProposeKinds`, `Nom.PrepareProposeAction`; `useGovernanceStore` (adds `proposeKinds`, `loadProposeKinds`); `useTxStore().awaitConfirm`; `app.ProposeKindDTO`/`app.ProposeFieldDTO`.
- Produces: store state `proposeKinds: app.ProposeKindDTO[]` + action `loadProposeKinds()`; the `GovernancePropose.vue` component.

- [ ] **Step 1: Add store state+action to `frontend/src/stores/governance.ts`**

In `state`, add: `proposeKinds: [] as app.ProposeKindDTO[],`
In `actions`, add:

```ts
    async loadProposeKinds() {
      try {
        this.proposeKinds = await Nom.GetProposeKinds()
      } catch {
        this.proposeKinds = [] // not connected / error ⇒ no form options
      }
    },
```

- [ ] **Step 2: Write the failing test `frontend/src/components/panels/GovernancePropose.test.ts`**

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], emits: ['click'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
const { PrepareProposeAction } = vi.hoisted(() => ({ PrepareProposeAction: vi.fn(() => Promise.resolve({ summary: 'p' })) }))
vi.mock('../../../wailsjs/go/app/NomService', () => ({ PrepareProposeAction, GetProposeKinds: vi.fn() }))

import GovernancePropose from './GovernancePropose.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'

function setup() {
  setActivePinia(createPinia())
  const gov = useGovernanceStore()
  gov.proposeKinds = [
    { kind: 'spork.create', label: 'Spork — Create', group: 'Spork', fields: [
      { key: 'name', label: 'Spork name', type: 'text', placeholder: '', required: true },
      { key: 'description', label: 'Spork description', type: 'text', placeholder: '', required: true },
    ] },
    { kind: 'custom', label: 'Custom (advanced)', group: 'Custom', fields: [
      { key: 'destination', label: 'Destination', type: 'address', placeholder: '', required: true },
      { key: 'data', label: 'Data', type: 'base64', placeholder: '', required: true },
    ] },
  ] as never
  return { gov }
}

describe('GovernancePropose', () => {
  it('renders the selected kind\'s fields and swaps them when kind changes', async () => {
    setup()
    const w = mount(GovernancePropose)
    // default kind = first (spork.create) → its 2 fields present
    expect(w.find('input[aria-label="field name"]').exists()).toBe(true)
    expect(w.find('input[aria-label="field description"]').exists()).toBe(true)
    expect(w.find('input[aria-label="field destination"]').exists()).toBe(false)
    // switch to custom → its fields appear, spork fields gone
    await w.find('select[aria-label="propose kind"]').setValue('custom')
    expect(w.find('input[aria-label="field destination"]').exists()).toBe(true)
    expect(w.find('input[aria-label="field name"]').exists()).toBe(false)
  })

  it('submits PrepareProposeAction with (name, description, url, kind, params)', async () => {
    const awaitConfirm = vi.spyOn(useTxStore(), 'awaitConfirm').mockImplementation(() => {})
    setup()
    const w = mount(GovernancePropose)
    await w.find('input[aria-label="action name"]').setValue('Act')
    await w.find('input[aria-label="action description"]').setValue('about')
    await w.find('input[aria-label="action url"]').setValue('https://zenon.org')
    await w.find('input[aria-label="field name"]').setValue('MySpork')
    await w.find('input[aria-label="field description"]').setValue('sdesc')
    await w.find('button[aria-label="submit proposal"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareProposeAction).toHaveBeenCalledWith('Act', 'about', 'https://zenon.org', 'spork.create', { name: 'MySpork', description: 'sdesc' })
    expect(awaitConfirm).toHaveBeenCalled()
  })
})
```

- [ ] **Step 3: Run it — verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernancePropose.test.ts`
Expected: FAIL — cannot resolve `./GovernancePropose.vue`.

- [ ] **Step 4: Create `frontend/src/components/panels/GovernancePropose.vue`**

```vue
<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'
import type { app } from '../../../wailsjs/go/models'

const gov = useGovernanceStore()
const tx = useTxStore()
const { proposeKinds } = storeToRefs(gov)
const error = ref('')

// governance metadata
const name = ref('')
const description = ref('')
const url = ref('')

// selected kind + its per-field values
const selectedKind = ref('')
const params = reactive<Record<string, string>>({})

watch(
  proposeKinds,
  (list) => {
    if (list.length && !list.some((k) => k.kind === selectedKind.value)) {
      selectedKind.value = list[0].kind
    }
  },
  { immediate: true },
)

const currentKind = computed<app.ProposeKindDTO | undefined>(() =>
  (proposeKinds.value ?? []).find((k) => k.kind === selectedKind.value),
)

// reset params when the kind changes so stale fields don't leak across kinds
watch(selectedKind, () => {
  for (const key of Object.keys(params)) delete params[key]
  for (const f of currentKind.value?.fields ?? []) params[f.key] = f.type === 'bool' ? 'false' : ''
})

function inputType(t: string): string {
  return t === 'number' ? 'number' : 'text'
}

async function submit() {
  error.value = ''
  if (!currentKind.value) {
    error.value = 'Select an action kind.'
    return
  }
  const payload: Record<string, string> = {}
  for (const f of currentKind.value.fields) payload[f.key] = params[f.key] ?? ''
  try {
    tx.awaitConfirm(await Nom.PrepareProposeAction(name.value, description.value, url.value, selectedKind.value, payload))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="space-y-3 p-4">
    <p class="text-xs text-muted-foreground">Proposing an action costs 1 ZNN (non-refundable).</p>

    <label class="block text-sm">
      <span class="text-muted-foreground">Action name</span>
      <input v-model="name" aria-label="action name" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary" />
    </label>
    <label class="block text-sm">
      <span class="text-muted-foreground">Description</span>
      <input v-model="description" aria-label="action description" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary" />
    </label>
    <label class="block text-sm">
      <span class="text-muted-foreground">URL</span>
      <input v-model="url" aria-label="action url" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary" />
    </label>

    <label class="block text-sm">
      <span class="text-muted-foreground">Action kind</span>
      <select v-model="selectedKind" aria-label="propose kind" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary">
        <option v-for="k in proposeKinds" :key="k.kind" :value="k.kind">{{ k.group }} · {{ k.label }}</option>
      </select>
    </label>

    <template v-for="f in currentKind?.fields ?? []" :key="f.key">
      <label class="block text-sm">
        <span class="text-muted-foreground">{{ f.label }}<span v-if="f.required" class="text-destructive"> *</span></span>
        <label v-if="f.type === 'bool'" class="mt-1 flex items-center gap-2">
          <input
            type="checkbox"
            :aria-label="`field ${f.key}`"
            :checked="params[f.key] === 'true'"
            @change="params[f.key] = ($event.target as HTMLInputElement).checked ? 'true' : 'false'"
          />
          <span class="text-xs text-muted-foreground">{{ f.placeholder || 'enabled' }}</span>
        </label>
        <input
          v-else
          v-model="params[f.key]"
          :type="inputType(f.type)"
          :aria-label="`field ${f.key}`"
          :placeholder="f.placeholder"
          class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary"
        />
        <span v-if="f.type === 'list'" class="text-[10px] text-muted-foreground">comma-separated</span>
      </label>
    </template>

    <Button aria-label="submit proposal" @click="submit">Propose (1 ZNN)</Button>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
```

- [ ] **Step 5: Run it — verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernancePropose.test.ts`
Expected: PASS (2 tests).

- [ ] **Step 6: Typecheck**

Run: `cd frontend && pnpm run typecheck`
Expected: clean.

- [ ] **Step 7: Stage** (controller commits)

```bash
git add frontend/src/stores/governance.ts frontend/src/components/panels/GovernancePropose.vue frontend/src/components/panels/GovernancePropose.test.ts
```
Do NOT commit.

---

### Task 4: Wire the Propose sub-tab into `GovernancePanel` (vertical slice complete)

**Files:**
- Modify: `frontend/src/components/panels/GovernancePanel.vue`
- Modify: `frontend/src/components/panels/GovernancePanel.test.ts`

**Interfaces:**
- Consumes: `GovernancePropose.vue`; `useGovernanceStore().loadProposeKinds`.
- Produces: a third `Propose` sub-tab; `load()` also calls `gov.loadProposeKinds()`.

- [ ] **Step 1: Update the panel test `GovernancePanel.test.ts`**

Add `GetProposeKinds: vi.fn(() => Promise.resolve([]))` to the `vi.mock('../../../wailsjs/go/app/NomService', …)` object. Then add this test inside the existing `describe`:

```ts
  it('renders the Propose sub-tab and loads kinds on mount', async () => {
    setActivePinia(createPinia())
    const gov = useGovernanceStore()
    const loadProposeKinds = vi.spyOn(gov, 'loadProposeKinds')
    const w = mount(GovernancePanel)
    await new Promise((r) => setTimeout(r))
    expect(loadProposeKinds).toHaveBeenCalled()
    expect(w.find('button[aria-label="sub Propose"]').exists()).toBe(true)
  })
```

(The existing mock's `TabsTrigger` stub renders `aria-label="sub ${value}"`; if the existing test file's nom-ui `Button` stub lacks `emits: ['click']`, add it.)

- [ ] **Step 2: Run it — verify the new test fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernancePanel.test.ts`
Expected: FAIL — no `sub Propose` trigger / `loadProposeKinds` not called.

- [ ] **Step 3: Update `GovernancePanel.vue`**

Add the import: `import GovernancePropose from './GovernancePropose.vue'`
Add `gov.loadProposeKinds()` inside the existing `load()` function (alongside `loadActions`/`loadVotablePillars`/`loadActivePillarCount`).
Add the trigger + content (after the Actions ones):

```vue
        <TabsTrigger value="Propose">Propose</TabsTrigger>
```
```vue
      <TabsContent value="Propose"><GovernancePropose /></TabsContent>
```

- [ ] **Step 4: Run it — verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/GovernancePanel.test.ts`
Expected: PASS.

- [ ] **Step 5: Full gates (vertical slice must be green end-to-end)**

Run, from `frontend/`: `pnpm run typecheck && pnpm test && pnpm run build`
Expected: typecheck clean; all suites pass; build OK.
Run, from repo root: `GOWORK=off GOTOOLCHAIN=auto go vet ./... && GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run Governance`
Expected: vet clean; governance tests pass.

- [ ] **Step 6: Stage** (controller commits)

```bash
git add frontend/src/components/panels/GovernancePanel.vue frontend/src/components/panels/GovernancePanel.test.ts
```
Do NOT commit.

---

### Task 5: Add Bridge kinds (15) — catalog + builder cases

**Files:**
- Modify: `app/governance_propose.go` (extend `proposeKinds()` + `buildProposalPayloadWith` switch)
- Test: `app/governance_propose_test.go` (append)

**Interfaces:**
- Consumes: the parsing toolkit from Task 1; SDK Bridge `Payload…` helpers (signatures below).
- Produces: 15 new catalog entries + 15 new builder cases. No signature changes; no binding regen needed (the form is generic).

SDK Bridge helper signatures (call these exactly):
```
PayloadBridgeAddNetwork(networkClass, chainId uint32, name, contractAddress, metadata string)
PayloadBridgeRemoveNetwork(networkClass, chainId uint32)
PayloadBridgeSetTokenPair(networkClass, chainId uint32, tokenStandard types.ZenonTokenStandard, tokenAddress string, bridgeable, redeemable, owned bool, minAmount *big.Int, fee, redeemDelay uint32, metadata string)
PayloadBridgeRemoveTokenPair(networkClass, chainId uint32, tokenStandard types.ZenonTokenStandard, tokenAddress string)
PayloadBridgeHalt(signature string)
PayloadBridgeUnhalt()
PayloadBridgeEmergency()
PayloadBridgeChangeAdministrator(administrator types.Address)
PayloadBridgeChangeTssECDSAPubKey(pubKey, signature, newSignature string)
PayloadBridgeSetAllowKeygen(allowKeygen bool)
PayloadBridgeSetOrchestratorInfo(windowSize uint64, keyGenThreshold, confirmationsToFinality, estimatedMomentumTime uint32)
PayloadBridgeSetMetadata(metadata string)
PayloadBridgeSetNetworkMetadata(networkClass, chainId uint32, metadata string)
PayloadBridgeRevokeUnwrapRequest(transactionHash types.Hash, logIndex uint32)
PayloadBridgeNominateGuardians(guardians []types.Address)
```

- [ ] **Step 1: Append the failing tests** to `app/governance_propose_test.go`

```go
func TestBuildProposalPayload_BridgeKinds(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	cases := []struct {
		kind   string
		params map[string]string
		want   embedded.ProposalPayload
	}{
		{"bridge.addNetwork", map[string]string{"networkClass": "1", "chainId": "2", "name": "eth", "contractAddress": "0xabc", "metadata": "{}"}, api.PayloadBridgeAddNetwork(1, 2, "eth", "0xabc", "{}")},
		{"bridge.removeNetwork", map[string]string{"networkClass": "1", "chainId": "2"}, api.PayloadBridgeRemoveNetwork(1, 2)},
		{"bridge.unhalt", map[string]string{}, api.PayloadBridgeUnhalt()},
		{"bridge.emergency", map[string]string{}, api.PayloadBridgeEmergency()},
		{"bridge.halt", map[string]string{"signature": "sig"}, api.PayloadBridgeHalt("sig")},
		{"bridge.setAllowKeygen", map[string]string{"allowKeygen": "true"}, api.PayloadBridgeSetAllowKeygen(true)},
		{"bridge.changeAdministrator", map[string]string{"administrator": types.SporkContract.String()}, api.PayloadBridgeChangeAdministrator(types.SporkContract)},
		{"bridge.setOrchestratorInfo", map[string]string{"windowSize": "10", "keyGenThreshold": "2", "confirmationsToFinality": "3", "estimatedMomentumTime": "10"}, api.PayloadBridgeSetOrchestratorInfo(10, 2, 3, 10)},
		{"bridge.setMetadata", map[string]string{"metadata": "{}"}, api.PayloadBridgeSetMetadata("{}")},
		{"bridge.setNetworkMetadata", map[string]string{"networkClass": "1", "chainId": "2", "metadata": "{}"}, api.PayloadBridgeSetNetworkMetadata(1, 2, "{}")},
		{"bridge.revokeUnwrapRequest", map[string]string{"transactionHash": "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20", "logIndex": "0"}, api.PayloadBridgeRevokeUnwrapRequest(types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"), 0)},
		{"bridge.nominateGuardians", map[string]string{"guardians": types.SporkContract.String() + "," + types.PillarContract.String()}, api.PayloadBridgeNominateGuardians([]types.Address{types.SporkContract, types.PillarContract})},
		{"bridge.changeTssECDSAPubKey", map[string]string{"pubKey": "pk", "signature": "s", "newSignature": "ns"}, api.PayloadBridgeChangeTssECDSAPubKey("pk", "s", "ns")},
		{"bridge.removeTokenPair", map[string]string{"networkClass": "1", "chainId": "2", "tokenStandard": "zts1znnxxxxxxxxxxxxx9z4ulx", "tokenAddress": "0xabc"}, api.PayloadBridgeRemoveTokenPair(1, 2, types.ParseZTSPanic("zts1znnxxxxxxxxxxxxx9z4ulx"), "0xabc")},
		{"bridge.setTokenPair", map[string]string{"networkClass": "1", "chainId": "2", "tokenStandard": "zts1znnxxxxxxxxxxxxx9z4ulx", "tokenAddress": "0xabc", "bridgeable": "true", "redeemable": "true", "owned": "false", "minAmount": "100", "fee": "5", "redeemDelay": "10", "metadata": "{}"}, api.PayloadBridgeSetTokenPair(1, 2, types.ParseZTSPanic("zts1znnxxxxxxxxxxxxx9z4ulx"), "0xabc", true, true, false, big.NewInt(100), 5, 10, "{}")},
	}
	for _, c := range cases {
		got, err := buildProposalPayloadWith(api, c.kind, c.params)
		if err != nil {
			t.Fatalf("%s: unexpected err %v", c.kind, err)
		}
		if got.Destination != c.want.Destination || got.Data != c.want.Data {
			t.Fatalf("%s: payload mismatch got %+v want %+v", c.kind, got, c.want)
		}
	}
	// a representative bad-params case
	if _, err := buildProposalPayloadWith(api, "bridge.addNetwork", map[string]string{"networkClass": "x", "chainId": "2", "name": "e", "contractAddress": "c", "metadata": "m"}); err == nil {
		t.Fatal("non-numeric networkClass must error")
	}
}

func TestProposeKinds_IncludesAllBridge(t *testing.T) {
	have := map[string]bool{}
	for _, k := range proposeKinds() {
		have[k.Kind] = true
	}
	for _, want := range []string{"bridge.addNetwork", "bridge.removeNetwork", "bridge.setTokenPair", "bridge.removeTokenPair", "bridge.halt", "bridge.unhalt", "bridge.emergency", "bridge.changeAdministrator", "bridge.changeTssECDSAPubKey", "bridge.setAllowKeygen", "bridge.setOrchestratorInfo", "bridge.setMetadata", "bridge.setNetworkMetadata", "bridge.revokeUnwrapRequest", "bridge.nominateGuardians"} {
		if !have[want] {
			t.Fatalf("missing bridge kind %q", want)
		}
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestBuildProposalPayload_BridgeKinds|TestProposeKinds_IncludesAllBridge' -v`
Expected: FAIL — bridge kinds unknown.

- [ ] **Step 3: Add the 15 builder cases** to the `buildProposalPayloadWith` switch (before the final `return … unknown`)

```go
	case "bridge.addNetwork":
		nc, err := parseU32Param(p, "networkClass"); if err != nil { return embedded.ProposalPayload{}, err }
		cid, err := parseU32Param(p, "chainId"); if err != nil { return embedded.ProposalPayload{}, err }
		name, err := reqParam(p, "name"); if err != nil { return embedded.ProposalPayload{}, err }
		ca, err := reqParam(p, "contractAddress"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeAddNetwork(nc, cid, name, ca, optParam(p, "metadata")), nil
	case "bridge.removeNetwork":
		nc, err := parseU32Param(p, "networkClass"); if err != nil { return embedded.ProposalPayload{}, err }
		cid, err := parseU32Param(p, "chainId"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeRemoveNetwork(nc, cid), nil
	case "bridge.setTokenPair":
		nc, err := parseU32Param(p, "networkClass"); if err != nil { return embedded.ProposalPayload{}, err }
		cid, err := parseU32Param(p, "chainId"); if err != nil { return embedded.ProposalPayload{}, err }
		zts, err := parseZtsParam(p, "tokenStandard"); if err != nil { return embedded.ProposalPayload{}, err }
		ta, err := reqParam(p, "tokenAddress"); if err != nil { return embedded.ProposalPayload{}, err }
		bridgeable, err := parseBoolParam(p, "bridgeable"); if err != nil { return embedded.ProposalPayload{}, err }
		redeemable, err := parseBoolParam(p, "redeemable"); if err != nil { return embedded.ProposalPayload{}, err }
		owned, err := parseBoolParam(p, "owned"); if err != nil { return embedded.ProposalPayload{}, err }
		minAmt, err := parseBigIntParam(p, "minAmount"); if err != nil { return embedded.ProposalPayload{}, err }
		fee, err := parseU32Param(p, "fee"); if err != nil { return embedded.ProposalPayload{}, err }
		rd, err := parseU32Param(p, "redeemDelay"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeSetTokenPair(nc, cid, zts, ta, bridgeable, redeemable, owned, minAmt, fee, rd, optParam(p, "metadata")), nil
	case "bridge.removeTokenPair":
		nc, err := parseU32Param(p, "networkClass"); if err != nil { return embedded.ProposalPayload{}, err }
		cid, err := parseU32Param(p, "chainId"); if err != nil { return embedded.ProposalPayload{}, err }
		zts, err := parseZtsParam(p, "tokenStandard"); if err != nil { return embedded.ProposalPayload{}, err }
		ta, err := reqParam(p, "tokenAddress"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeRemoveTokenPair(nc, cid, zts, ta), nil
	case "bridge.halt":
		sig, err := reqParam(p, "signature"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeHalt(sig), nil
	case "bridge.unhalt":
		return api.PayloadBridgeUnhalt(), nil
	case "bridge.emergency":
		return api.PayloadBridgeEmergency(), nil
	case "bridge.changeAdministrator":
		a, err := parseAddrParam(p, "administrator"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeChangeAdministrator(a), nil
	case "bridge.changeTssECDSAPubKey":
		pk, err := reqParam(p, "pubKey"); if err != nil { return embedded.ProposalPayload{}, err }
		sig, err := reqParam(p, "signature"); if err != nil { return embedded.ProposalPayload{}, err }
		ns, err := reqParam(p, "newSignature"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeChangeTssECDSAPubKey(pk, sig, ns), nil
	case "bridge.setAllowKeygen":
		b, err := parseBoolParam(p, "allowKeygen"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeSetAllowKeygen(b), nil
	case "bridge.setOrchestratorInfo":
		ws, err := parseU64Param(p, "windowSize"); if err != nil { return embedded.ProposalPayload{}, err }
		kt, err := parseU32Param(p, "keyGenThreshold"); if err != nil { return embedded.ProposalPayload{}, err }
		cf, err := parseU32Param(p, "confirmationsToFinality"); if err != nil { return embedded.ProposalPayload{}, err }
		et, err := parseU32Param(p, "estimatedMomentumTime"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeSetOrchestratorInfo(ws, kt, cf, et), nil
	case "bridge.setMetadata":
		m, err := reqParam(p, "metadata"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeSetMetadata(m), nil
	case "bridge.setNetworkMetadata":
		nc, err := parseU32Param(p, "networkClass"); if err != nil { return embedded.ProposalPayload{}, err }
		cid, err := parseU32Param(p, "chainId"); if err != nil { return embedded.ProposalPayload{}, err }
		m, err := reqParam(p, "metadata"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeSetNetworkMetadata(nc, cid, m), nil
	case "bridge.revokeUnwrapRequest":
		h, err := parseHashParam(p, "transactionHash"); if err != nil { return embedded.ProposalPayload{}, err }
		li, err := parseU32Param(p, "logIndex"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeRevokeUnwrapRequest(h, li), nil
	case "bridge.nominateGuardians":
		gs, err := parseAddrList(p, "guardians"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadBridgeNominateGuardians(gs), nil
```

- [ ] **Step 4: Add the 15 catalog entries** to `proposeKinds()` (after the Spork entries, before `custom`)

```go
		{Kind: "bridge.addNetwork", Label: "Bridge — Add Network", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "name", Label: "Name", Type: "text", Placeholder: "Ethereum", Required: true},
			{Key: "contractAddress", Label: "Contract address", Type: "text", Placeholder: "0x…", Required: true},
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: false},
		}},
		{Kind: "bridge.removeNetwork", Label: "Bridge — Remove Network", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
		}},
		{Kind: "bridge.setTokenPair", Label: "Bridge — Set Token Pair", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "tokenStandard", Label: "Token standard (ZTS)", Type: "text", Placeholder: "zts1…", Required: true},
			{Key: "tokenAddress", Label: "Foreign token address", Type: "text", Placeholder: "0x…", Required: true},
			{Key: "bridgeable", Label: "Bridgeable", Type: "bool", Placeholder: "", Required: true},
			{Key: "redeemable", Label: "Redeemable", Type: "bool", Placeholder: "", Required: true},
			{Key: "owned", Label: "Owned", Type: "bool", Placeholder: "", Required: true},
			{Key: "minAmount", Label: "Min amount", Type: "amount", Placeholder: "0", Required: true},
			{Key: "fee", Label: "Fee (per-ten-thousand)", Type: "number", Placeholder: "0", Required: true},
			{Key: "redeemDelay", Label: "Redeem delay (momentums)", Type: "number", Placeholder: "0", Required: true},
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: false},
		}},
		{Kind: "bridge.removeTokenPair", Label: "Bridge — Remove Token Pair", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "tokenStandard", Label: "Token standard (ZTS)", Type: "text", Placeholder: "zts1…", Required: true},
			{Key: "tokenAddress", Label: "Foreign token address", Type: "text", Placeholder: "0x…", Required: true},
		}},
		{Kind: "bridge.halt", Label: "Bridge — Halt", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "signature", Label: "Signature", Type: "text", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.unhalt", Label: "Bridge — Unhalt", Group: "Bridge", Fields: []ProposeFieldDTO{}},
		{Kind: "bridge.emergency", Label: "Bridge — Emergency", Group: "Bridge", Fields: []ProposeFieldDTO{}},
		{Kind: "bridge.changeAdministrator", Label: "Bridge — Change Administrator", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "administrator", Label: "New administrator", Type: "address", Placeholder: "z1…", Required: true},
		}},
		{Kind: "bridge.changeTssECDSAPubKey", Label: "Bridge — Change TSS Pubkey", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "pubKey", Label: "New TSS pubkey", Type: "text", Placeholder: "", Required: true},
			{Key: "signature", Label: "Old-key signature", Type: "text", Placeholder: "", Required: true},
			{Key: "newSignature", Label: "New-key signature", Type: "text", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.setAllowKeygen", Label: "Bridge — Set Allow Keygen", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "allowKeygen", Label: "Allow keygen", Type: "bool", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.setOrchestratorInfo", Label: "Bridge — Set Orchestrator Info", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "windowSize", Label: "Window size", Type: "number", Placeholder: "", Required: true},
			{Key: "keyGenThreshold", Label: "Keygen threshold", Type: "number", Placeholder: "", Required: true},
			{Key: "confirmationsToFinality", Label: "Confirmations to finality", Type: "number", Placeholder: "", Required: true},
			{Key: "estimatedMomentumTime", Label: "Estimated momentum time", Type: "number", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.setMetadata", Label: "Bridge — Set Metadata", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: true},
		}},
		{Kind: "bridge.setNetworkMetadata", Label: "Bridge — Set Network Metadata", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: true},
		}},
		{Kind: "bridge.revokeUnwrapRequest", Label: "Bridge — Revoke Unwrap Request", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "transactionHash", Label: "Transaction hash", Type: "hash", Placeholder: "0x…", Required: true},
			{Key: "logIndex", Label: "Log index", Type: "number", Placeholder: "0", Required: true},
		}},
		{Kind: "bridge.nominateGuardians", Label: "Bridge — Nominate Guardians", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "guardians", Label: "Guardian addresses", Type: "list", Placeholder: "z1…,z1…", Required: true},
		}},
```

- [ ] **Step 5: Run — verify pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestBuildProposalPayload|TestProposeKinds|TestGetProposeKinds' -v`
Expected: PASS (all governance-propose tests).

- [ ] **Step 6: Vet + build, then stage** (controller commits)

Run: `GOWORK=off GOTOOLCHAIN=auto go vet ./app/... && GOWORK=off GOTOOLCHAIN=auto go build ./...`
```bash
git add app/governance_propose.go app/governance_propose_test.go
```
Do NOT commit.

---

### Task 6: Add Liquidity kinds (9) — catalog + builder cases

**Files:**
- Modify: `app/governance_propose.go`
- Test: `app/governance_propose_test.go` (append)

**Interfaces:**
- Consumes: the parsing toolkit; SDK Liquidity `Payload…` helpers (signatures below).
- Produces: 9 new catalog entries + 9 new builder cases.

SDK Liquidity helper signatures:
```
PayloadLiquidityFund(znnReward, qsrReward *big.Int)
PayloadLiquidityBurnZnn(burnAmount *big.Int)
PayloadLiquiditySetTokenTuple(tokenStandards []string, znnPercentages, qsrPercentages []uint32, minAmounts []*big.Int)
PayloadLiquiditySetIsHalted(value bool)
PayloadLiquidityUnlockStakeEntries(zts types.ZenonTokenStandard)
PayloadLiquiditySetAdditionalReward(znnReward, qsrAmount *big.Int)
PayloadLiquidityChangeAdministrator(administrator types.Address)
PayloadLiquidityNominateGuardians(guardians []types.Address)
PayloadLiquidityEmergency()
```

- [ ] **Step 1: Append the failing tests**

```go
func TestBuildProposalPayload_LiquidityKinds(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	cases := []struct {
		kind   string
		params map[string]string
		want   embedded.ProposalPayload
	}{
		{"liquidity.fund", map[string]string{"znnReward": "10", "qsrReward": "20"}, api.PayloadLiquidityFund(big.NewInt(10), big.NewInt(20))},
		{"liquidity.burnZnn", map[string]string{"burnAmount": "5"}, api.PayloadLiquidityBurnZnn(big.NewInt(5))},
		{"liquidity.setIsHalted", map[string]string{"value": "true"}, api.PayloadLiquiditySetIsHalted(true)},
		{"liquidity.unlockStakeEntries", map[string]string{"zts": "zts1znnxxxxxxxxxxxxx9z4ulx"}, api.PayloadLiquidityUnlockStakeEntries(types.ParseZTSPanic("zts1znnxxxxxxxxxxxxx9z4ulx"))},
		{"liquidity.setAdditionalReward", map[string]string{"znnReward": "1", "qsrAmount": "2"}, api.PayloadLiquiditySetAdditionalReward(big.NewInt(1), big.NewInt(2))},
		{"liquidity.changeAdministrator", map[string]string{"administrator": types.SporkContract.String()}, api.PayloadLiquidityChangeAdministrator(types.SporkContract)},
		{"liquidity.nominateGuardians", map[string]string{"guardians": types.SporkContract.String() + "," + types.PillarContract.String()}, api.PayloadLiquidityNominateGuardians([]types.Address{types.SporkContract, types.PillarContract})},
		{"liquidity.emergency", map[string]string{}, api.PayloadLiquidityEmergency()},
		{"liquidity.setTokenTuple", map[string]string{"tokenStandards": "zts1znnxxxxxxxxxxxxx9z4ulx", "znnPercentages": "5000", "qsrPercentages": "5000", "minAmounts": "100"}, api.PayloadLiquiditySetTokenTuple([]string{"zts1znnxxxxxxxxxxxxx9z4ulx"}, []uint32{5000}, []uint32{5000}, []*big.Int{big.NewInt(100)})},
	}
	for _, c := range cases {
		got, err := buildProposalPayloadWith(api, c.kind, c.params)
		if err != nil {
			t.Fatalf("%s: unexpected err %v", c.kind, err)
		}
		if got.Destination != c.want.Destination || got.Data != c.want.Data {
			t.Fatalf("%s: payload mismatch got %+v want %+v", c.kind, got, c.want)
		}
	}
	if _, err := buildProposalPayloadWith(api, "liquidity.fund", map[string]string{"znnReward": "-1", "qsrReward": "2"}); err == nil {
		t.Fatal("negative znnReward must error")
	}
}

func TestProposeKinds_IncludesAllLiquidity(t *testing.T) {
	have := map[string]bool{}
	for _, k := range proposeKinds() {
		have[k.Kind] = true
	}
	for _, want := range []string{"liquidity.fund", "liquidity.burnZnn", "liquidity.setTokenTuple", "liquidity.setIsHalted", "liquidity.unlockStakeEntries", "liquidity.setAdditionalReward", "liquidity.changeAdministrator", "liquidity.nominateGuardians", "liquidity.emergency"} {
		if !have[want] {
			t.Fatalf("missing liquidity kind %q", want)
		}
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestBuildProposalPayload_LiquidityKinds|TestProposeKinds_IncludesAllLiquidity' -v`
Expected: FAIL — liquidity kinds unknown.

- [ ] **Step 3: Add the 9 builder cases** to the switch (before the final unknown return)

```go
	case "liquidity.fund":
		znn, err := parseBigIntParam(p, "znnReward"); if err != nil { return embedded.ProposalPayload{}, err }
		qsr, err := parseBigIntParam(p, "qsrReward"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquidityFund(znn, qsr), nil
	case "liquidity.burnZnn":
		amt, err := parseBigIntParam(p, "burnAmount"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquidityBurnZnn(amt), nil
	case "liquidity.setTokenTuple":
		zs, err := parseStrList(p, "tokenStandards"); if err != nil { return embedded.ProposalPayload{}, err }
		zp, err := parseU32List(p, "znnPercentages"); if err != nil { return embedded.ProposalPayload{}, err }
		qp, err := parseU32List(p, "qsrPercentages"); if err != nil { return embedded.ProposalPayload{}, err }
		ma, err := parseBigIntList(p, "minAmounts"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquiditySetTokenTuple(zs, zp, qp, ma), nil
	case "liquidity.setIsHalted":
		v, err := parseBoolParam(p, "value"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquiditySetIsHalted(v), nil
	case "liquidity.unlockStakeEntries":
		z, err := parseZtsParam(p, "zts"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquidityUnlockStakeEntries(z), nil
	case "liquidity.setAdditionalReward":
		znn, err := parseBigIntParam(p, "znnReward"); if err != nil { return embedded.ProposalPayload{}, err }
		qsr, err := parseBigIntParam(p, "qsrAmount"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquiditySetAdditionalReward(znn, qsr), nil
	case "liquidity.changeAdministrator":
		a, err := parseAddrParam(p, "administrator"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquidityChangeAdministrator(a), nil
	case "liquidity.nominateGuardians":
		gs, err := parseAddrList(p, "guardians"); if err != nil { return embedded.ProposalPayload{}, err }
		return api.PayloadLiquidityNominateGuardians(gs), nil
	case "liquidity.emergency":
		return api.PayloadLiquidityEmergency(), nil
```

- [ ] **Step 4: Add the 9 catalog entries** to `proposeKinds()` (after the Bridge entries, before `custom`)

```go
		{Kind: "liquidity.fund", Label: "Liquidity — Fund", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "znnReward", Label: "ZNN reward", Type: "amount", Placeholder: "0", Required: true},
			{Key: "qsrReward", Label: "QSR reward", Type: "amount", Placeholder: "0", Required: true},
		}},
		{Kind: "liquidity.burnZnn", Label: "Liquidity — Burn ZNN", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "burnAmount", Label: "Burn amount", Type: "amount", Placeholder: "0", Required: true},
		}},
		{Kind: "liquidity.setTokenTuple", Label: "Liquidity — Set Token Tuple", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "tokenStandards", Label: "Token standards", Type: "list", Placeholder: "zts1…,zts1…", Required: true},
			{Key: "znnPercentages", Label: "ZNN percentages", Type: "list", Placeholder: "5000,5000", Required: true},
			{Key: "qsrPercentages", Label: "QSR percentages", Type: "list", Placeholder: "5000,5000", Required: true},
			{Key: "minAmounts", Label: "Min amounts", Type: "list", Placeholder: "100,100", Required: true},
		}},
		{Kind: "liquidity.setIsHalted", Label: "Liquidity — Set Halted", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "value", Label: "Halted", Type: "bool", Placeholder: "", Required: true},
		}},
		{Kind: "liquidity.unlockStakeEntries", Label: "Liquidity — Unlock Stake Entries", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "zts", Label: "Token standard (ZTS)", Type: "text", Placeholder: "zts1…", Required: true},
		}},
		{Kind: "liquidity.setAdditionalReward", Label: "Liquidity — Set Additional Reward", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "znnReward", Label: "ZNN reward", Type: "amount", Placeholder: "0", Required: true},
			{Key: "qsrAmount", Label: "QSR amount", Type: "amount", Placeholder: "0", Required: true},
		}},
		{Kind: "liquidity.changeAdministrator", Label: "Liquidity — Change Administrator", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "administrator", Label: "New administrator", Type: "address", Placeholder: "z1…", Required: true},
		}},
		{Kind: "liquidity.nominateGuardians", Label: "Liquidity — Nominate Guardians", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "guardians", Label: "Guardian addresses", Type: "list", Placeholder: "z1…,z1…", Required: true},
		}},
		{Kind: "liquidity.emergency", Label: "Liquidity — Emergency", Group: "Liquidity", Fields: []ProposeFieldDTO{}},
```

- [ ] **Step 5: Run — verify pass + final gates**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run Governance -v` (all governance tests)
Expected: PASS.
Run: `GOWORK=off GOTOOLCHAIN=auto go vet ./... && GOWORK=off GOTOOLCHAIN=auto go build ./...`
Expected: clean.
Run, from `frontend/`: `pnpm run typecheck && pnpm test && pnpm run build`
Expected: all green (the dynamic form already handles every field type; no frontend change needed for Bridge/Liquidity).

- [ ] **Step 6: Stage** (controller commits)

```bash
git add app/governance_propose.go app/governance_propose_test.go
```
Do NOT commit.

---

## Self-Review notes

- **Spec coverage:** GetProposeKinds catalog (Tasks 1,5,6) ✓; PrepareProposeAction dispatcher + confirm-from-built-block (Task 1) ✓; Spork+Custom vertical slice testable (Tasks 1–4) ✓; Bridge 15 (Task 5) ✓; Liquidity 9 (Task 6) ✓; schema-driven dynamic form handling all field types (Task 3) ✓; Propose tab, no pillar gate (Tasks 3–4) ✓; bindings (Task 2) ✓; 1 ZNN from template (Task 1) ✓.
- **Type consistency:** `buildProposalPayloadWith(api, kind, params)` and `buildProposalPayload(client, kind, params)` signatures stable across Tasks 1/5/6; `ProposeKindDTO`/`ProposeFieldDTO` fields identical Go↔TS; the form reads `f.key/f.label/f.type/f.placeholder/f.required` matching the DTO json tags.
- **Field-type completeness:** Task 3's renderer must implement `bool` (checkbox) and the text-input fallback for `text|number|address|hash|amount|base64|list`; `number` uses `<input type="number">`; `list` shows a "comma-separated" hint. Bridge/Liquidity introduce no new type beyond these.
- **No binding regen for Tasks 5/6:** signatures unchanged; only catalog data + builder cases grow.
- **Deferred-cut path:** to drop Liquidity, omit Task 6 entirely (nothing depends on it).
