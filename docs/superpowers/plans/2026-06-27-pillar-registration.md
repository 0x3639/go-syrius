# Pillar Registration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the ability to register and manage a Pillar from the wallet, modeled on the existing Sentinel launch wizard (3-step flow + on-chain "clearing" polling) with a minimal owned-pillar view.

**Architecture:** A new `NomService` set of pillar methods (reads + `Prepare*` calls routed through the existing `tx.prepareCall` confirm-what-you-sign pipeline); the existing Pinia `pillar` store extended with registration state mirroring `sentinel.ts`; three new Vue components (`PillarLaunch`, `PillarActive`, and an extracted `PillarDelegate`) wired into a restructured `PillarPanel` container with "Delegate" / "Run a Pillar" sub-tabs. Delegation behavior is unchanged — only relocated.

**Tech Stack:** Go 1.25.11 + Wails v2, `znn-sdk-go` v0.1.19 (`PillarApi`), Vue 3 + TypeScript + Pinia + nom-ui, Vitest + @vue/test-utils.

## Global Constraints

- All `go`/`wails` commands run with `GOWORK=off GOTOOLCHAIN=auto` (local `go.work` hazard, per CLAUDE.md).
- **Binding invariant:** the frontend never receives key material; it sends intent. Every `Prepare*` method routes through `s.tx.prepareCall(template, callExpect{...}, summary)` so the confirm dialog renders the effect from the *built block*.
- **Re-validate server-side:** never trust frontend validation. Validate name (regex + length), reward percentages (0–100), and address parsing in Go independently.
- **Amounts from the SDK template, never hardcoded:** the 15,000 ZNN Register collateral comes from `template.Amount`.
- **go-zenon pillar name rule (authoritative):** non-empty, ≤ 40 chars, regex `^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$`.
- **Reward percentages:** `uint8`, range 0–100 inclusive (both momentum and delegate).
- **Register template Amount:** `PillarStakeAmount` = 15,000 ZNN = base-unit `"1500000000000"`.
- **Plasma gate:** `PILLAR_PLASMA_REQUIRED = 105000n` (a Register block costs `2 * EmbeddedSimplePlasma`); recommend fusing `500` QSR (→ ~1,050,000 plasma).
- **SDK `PillarApi` signatures (verified, v0.1.19):**
  - `GetByOwner(address types.Address) ([]*PillarInfo, error)` — returns a slice; take `[0]`.
  - `GetQsrRegistrationCost() (*big.Int, error)`
  - `GetDepositedQsr(address types.Address) (*big.Int, error)`
  - `CheckNameAvailability(name string) (*bool, error)`
  - `Register(name string, producerAddress, rewardAddress types.Address, blockProducingPercentage, delegationPercentage uint8) *nom.AccountBlock`
  - `DepositQsr(amount *big.Int) *nom.AccountBlock`, `WithdrawQsr() *nom.AccountBlock`, `Revoke(name string) *nom.AccountBlock`, `CollectReward() *nom.AccountBlock`
  - `PillarInfo` fields: `Name`, `OwnerAddress`, `ProducerAddress`, `WithdrawAddress` (= reward address), `GiveMomentumRewardPercentage int32`, `GiveDelegateRewardPercentage int32`, `IsRevocable`, `RevokeCooldown int64`.
- **Reuse (already present):** `PrepareCollectPillarReward()`, `GetPillarReward()`, `GetPlasmaInfo()`, `PrepareFuse(beneficiary, qsrAmount)`.
- **Commit message trailer (every commit):**
  ```
  Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
  ```

## File Structure

- `app/dto.go` — add `OwnedPillarInfo` struct.
- `app/nom_service.go` — add `ownedPillarDTO`, `validatePillarName`, and the pillar read/write methods.
- `app/nom_service_test.go` — add DTO/validation/template tests.
- `frontend/wailsjs/go/app/NomService.d.ts` / `NomService.js` / `frontend/wailsjs/go/models.ts` — add the new method bindings + `OwnedPillarInfo` model.
- `frontend/src/lib/format.ts` — add `isValidPillarName`.
- `frontend/src/lib/format.test.ts` — add `isValidPillarName` tests.
- `frontend/src/components/panels/StepHeader.vue` — parameterize step labels.
- `frontend/src/stores/pillar.ts` — extend with registration state.
- `frontend/src/stores/pillar.test.ts` — new store test.
- `frontend/src/components/panels/PillarLaunch.vue` (+ `.test.ts`) — registration wizard.
- `frontend/src/components/panels/PillarActive.vue` (+ `.test.ts`) — owned-pillar view.
- `frontend/src/components/panels/PillarDelegate.vue` — extracted existing delegation UI.
- `frontend/src/components/panels/PillarPanel.vue` (+ `.test.ts`) — container with sub-tabs (replaces current delegation-only panel).

---

### Task 1: Backend — `OwnedPillarInfo` DTO, name validator, and pillar read methods

**Files:**
- Modify: `app/dto.go` (add struct after `SentinelInfo`, around line 256)
- Modify: `app/nom_service.go` (add `regexp` import; add helpers + methods near the existing pillar section ~line 420)
- Test: `app/nom_service_test.go`

**Interfaces:**
- Produces: `OwnedPillarInfo` struct; `ownedPillarDTO([]*embedded.PillarInfo) OwnedPillarInfo`; `validatePillarName(string) error`; `(*NomService) GetMyPillar() (OwnedPillarInfo, error)`, `GetPillarDepositedQsr() (string, error)`, `GetPillarQsrCost() (string, error)`, `CheckPillarName(string) (bool, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/nom_service_test.go`:

```go
func TestOwnedPillarDTO(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	producer, _ := types.ParseAddress("z1qr4pexnnfaexqqz8nscjjcsajy5hdqfkgadvwx")
	p := &embedded.PillarInfo{
		Name:                         "My-Pillar",
		OwnerAddress:                 owner,
		ProducerAddress:              producer,
		WithdrawAddress:              owner,
		GiveMomentumRewardPercentage: 0,
		GiveDelegateRewardPercentage: 100,
		IsRevocable:                  true,
		RevokeCooldown:               42,
	}
	d := ownedPillarDTO([]*embedded.PillarInfo{p})
	if d.Name != "My-Pillar" || d.OwnerAddress != owner.String() {
		t.Fatalf("bad mapping: %+v", d)
	}
	if d.ProducerAddress != producer.String() || d.RewardAddress != owner.String() {
		t.Fatalf("bad addresses: %+v", d)
	}
	if d.GiveMomentumRewardPct != 0 || d.GiveDelegateRewardPct != 100 {
		t.Fatalf("bad percentages: %+v", d)
	}
	if !d.IsRevocable || d.RevokeCooldown != 42 {
		t.Fatalf("bad flags: %+v", d)
	}
	// empty slice → empty Name (no pillar owned)
	if ownedPillarDTO(nil).Name != "" {
		t.Fatal("nil should map to empty Name")
	}
	if ownedPillarDTO([]*embedded.PillarInfo{}).Name != "" {
		t.Fatal("empty slice should map to empty Name")
	}
}

func TestValidatePillarName(t *testing.T) {
	valid := []string{"Pillar", "my-pillar", "a.b_c", "P1", "Node-01.eu", "ab"}
	for _, n := range valid {
		if err := validatePillarName(n); err != nil {
			t.Fatalf("expected %q valid, got %v", n, err)
		}
	}
	invalid := []string{
		"",                // empty
		"-leading",        // leading separator
		"trailing-",       // trailing separator
		"double--dash",    // consecutive separators
		"has space",       // space
		"bad!",            // symbol
		"a",               // too short? (1 char IS allowed) -- see note
	}
	// NOTE: single-char "a" IS valid per the regex; drop it from the invalid set.
	invalid = invalid[:len(invalid)-1]
	for _, n := range invalid {
		if err := validatePillarName(n); err == nil {
			t.Fatalf("expected %q invalid", n)
		}
	}
	// 41 chars → too long
	long := ""
	for i := 0; i < 41; i++ {
		long += "a"
	}
	if err := validatePillarName(long); err == nil {
		t.Fatal("expected 41-char name to be rejected")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestOwnedPillarDTO|TestValidatePillarName' -v`
Expected: FAIL — `undefined: ownedPillarDTO` / `undefined: validatePillarName`.

- [ ] **Step 3: Add the DTO**

In `app/dto.go`, after the `SentinelInfo` struct (~line 256):

```go
// OwnedPillarInfo describes the pillar owned by the active address. An empty
// Name means the address owns no pillar.
type OwnedPillarInfo struct {
	Name                  string `json:"name"`
	OwnerAddress          string `json:"ownerAddress"`
	ProducerAddress       string `json:"producerAddress"`
	RewardAddress         string `json:"rewardAddress"`
	GiveMomentumRewardPct int    `json:"giveMomentumRewardPct"`
	GiveDelegateRewardPct int    `json:"giveDelegateRewardPct"`
	IsRevocable           bool   `json:"isRevocable"`
	RevokeCooldown        int64  `json:"revokeCooldown"`
}
```

- [ ] **Step 4: Add the helpers + read methods**

In `app/nom_service.go`, add `"regexp"` to the import block (alphabetically, near `"math/big"`/`"strings"`). Then add near the existing pillar section (after `GetPillarReward`, ~line 435):

```go
// pillarNameRe matches go-zenon's pillar name rule: alphanumerics with single
// '-', '.', or '_' allowed only between alphanumerics.
var pillarNameRe = regexp.MustCompile(`^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$`)

// validatePillarName mirrors go-zenon's checkPillarNameStatic (1–40 chars + regex).
// The node re-validates authoritatively; this is the first gate.
func validatePillarName(name string) error {
	if len(name) == 0 || len(name) > 40 {
		return errors.New("pillar name must be 1–40 characters")
	}
	if !pillarNameRe.MatchString(name) {
		return errors.New("pillar name may use only letters, digits, and single - . _ between them")
	}
	return nil
}

// ownedPillarDTO maps the first pillar owned by the address to the DTO. An empty
// slice (or nil first element) maps to an empty Name (= owns no pillar).
func ownedPillarDTO(list []*embedded.PillarInfo) OwnedPillarInfo {
	if len(list) == 0 || list[0] == nil {
		return OwnedPillarInfo{}
	}
	p := list[0]
	return OwnedPillarInfo{
		Name:                  p.Name,
		OwnerAddress:          p.OwnerAddress.String(),
		ProducerAddress:       p.ProducerAddress.String(),
		RewardAddress:         p.WithdrawAddress.String(),
		GiveMomentumRewardPct: int(p.GiveMomentumRewardPercentage),
		GiveDelegateRewardPct: int(p.GiveDelegateRewardPercentage),
		IsRevocable:           p.IsRevocable,
		RevokeCooldown:        p.RevokeCooldown,
	}
}

// GetMyPillar returns the pillar owned by the active address (empty Name = none).
func (s *NomService) GetMyPillar() (OwnedPillarInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return OwnedPillarInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return OwnedPillarInfo{}, errLocked
	}
	list, err := client.PillarApi.GetByOwner(addr)
	if err != nil {
		return OwnedPillarInfo{}, err
	}
	return ownedPillarDTO(list), nil
}

// GetPillarDepositedQsr returns the active address's QSR escrowed toward pillar
// registration (base-unit decimal string; "0" if none).
func (s *NomService) GetPillarDepositedQsr() (string, error) {
	client := s.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return "", errLocked
	}
	q, err := client.PillarApi.GetDepositedQsr(addr)
	if err != nil {
		return "", err
	}
	if q == nil {
		return "0", nil
	}
	return q.String(), nil
}

// GetPillarQsrCost returns the current QSR cost to register the next pillar
// (base-unit decimal string). This QSR is burned on registration.
func (s *NomService) GetPillarQsrCost() (string, error) {
	client := s.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	cost, err := client.PillarApi.GetQsrRegistrationCost()
	if err != nil {
		return "", err
	}
	if cost == nil {
		return "0", nil
	}
	return cost.String(), nil
}

// CheckPillarName validates the name locally then asks the node whether it is
// available (true = free to register).
func (s *NomService) CheckPillarName(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if err := validatePillarName(name); err != nil {
		return false, err
	}
	client := s.node.currentClient()
	if client == nil {
		return false, errors.New("not connected")
	}
	avail, err := client.PillarApi.CheckNameAvailability(name)
	if err != nil {
		return false, err
	}
	if avail == nil {
		return false, nil
	}
	return *avail, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestOwnedPillarDTO|TestValidatePillarName' -v`
Expected: PASS. Also run `GOWORK=off GOTOOLCHAIN=auto go build ./app/` → no errors.

- [ ] **Step 6: Commit**

```bash
git add app/dto.go app/nom_service.go app/nom_service_test.go
git commit -m "$(cat <<'EOF'
feat(app): pillar registration read methods + name validator

Adds OwnedPillarInfo DTO, validatePillarName (go-zenon regex/length),
and GetMyPillar/GetPillarDepositedQsr/GetPillarQsrCost/CheckPillarName.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Backend — pillar write `Prepare*` methods

**Files:**
- Modify: `app/nom_service.go` (after the read methods from Task 1)
- Test: `app/nom_service_test.go`

**Interfaces:**
- Consumes: `validatePillarName` (Task 1).
- Produces: `(*NomService) PreparePillarDepositQsr(qsr string) (CallPreview, error)`, `PreparePillarWithdrawQsr() (CallPreview, error)`, `PrepareRegisterPillar(name, producer, reward string, momentumPct, delegatePct uint8) (CallPreview, error)`, `PrepareRevokePillar(name string) (CallPreview, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/nom_service_test.go`:

```go
func TestPreparePillarDepositQsrValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	for _, bad := range []string{"0", "-1", "", "abc"} {
		if _, err := s.PreparePillarDepositQsr(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestPrepareRegisterPillarValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	good := "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	// invalid name
	if _, err := s.PrepareRegisterPillar("bad name!", good, good, 50, 50); err == nil {
		t.Fatal("expected invalid name to be rejected")
	}
	// invalid producer address
	if _, err := s.PrepareRegisterPillar("Pillar-A", "nope", good, 50, 50); err == nil {
		t.Fatal("expected invalid producer to be rejected")
	}
	// invalid reward address
	if _, err := s.PrepareRegisterPillar("Pillar-A", good, "nope", 50, 50); err == nil {
		t.Fatal("expected invalid reward to be rejected")
	}
	// out-of-range percentage
	if _, err := s.PrepareRegisterPillar("Pillar-A", good, good, 101, 50); err == nil {
		t.Fatal("expected momentum pct > 100 to be rejected")
	}
	if _, err := s.PrepareRegisterPillar("Pillar-A", good, good, 50, 101); err == nil {
		t.Fatal("expected delegate pct > 100 to be rejected")
	}
}

func TestPrepareRevokePillarValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareRevokePillar("   "); err == nil {
		t.Fatal("expected empty name to be rejected")
	}
}

func TestPillarRegisterTemplateTokenStandards(t *testing.T) {
	api := embedded.NewPillarApi(nil) // builders construct blocks from args/constants; no client deref
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	znn := types.ZnnTokenStandard.String()
	qsr := types.QsrTokenStandard.String()

	deposit := api.DepositQsr(big.NewInt(123))
	if deposit.ToAddress != types.PillarContract || deposit.TokenStandard.String() != qsr {
		t.Fatalf("deposit: to=%v zts=%v", deposit.ToAddress, deposit.TokenStandard.String())
	}
	reg := api.Register("Pillar-A", addr, addr, 0, 100)
	if reg.ToAddress != types.PillarContract || reg.TokenStandard.String() != znn {
		t.Fatalf("register: to=%v zts=%v", reg.ToAddress, reg.TokenStandard.String())
	}
	// Register must carry the 15,000 ZNN collateral (15000 * 1e8).
	if reg.Amount == nil || reg.Amount.String() != "1500000000000" {
		t.Fatalf("register amount=%v want 1500000000000", reg.Amount)
	}
	for name, b := range map[string]*nom.AccountBlock{"withdraw": api.WithdrawQsr(), "revoke": api.Revoke("Pillar-A")} {
		if b.ToAddress != types.PillarContract || b.TokenStandard.String() != znn {
			t.Fatalf("%s: to=%v zts=%v", name, b.ToAddress, b.TokenStandard.String())
		}
		if b.Amount == nil || b.Amount.Sign() != 0 {
			t.Fatalf("%s: amount=%v want 0", name, b.Amount)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestPreparePillarDepositQsrValidatesInput|TestPrepareRegisterPillarValidatesInput|TestPrepareRevokePillarValidatesInput|TestPillarRegisterTemplateTokenStandards' -v`
Expected: FAIL — undefined `PreparePillarDepositQsr` etc. (`TestPillarRegisterTemplateTokenStandards` may pass already since it calls the SDK directly — that's fine, it's a regression guard.)

- [ ] **Step 3: Add the write methods**

In `app/nom_service.go`, after the Task 1 read methods:

```go
// PreparePillarDepositQsr builds a DepositQsr template (escrows QSR toward pillar
// registration; this QSR is BURNED on registration). qsr is a base-unit decimal
// string, validated before any node use.
func (s *NomService) PreparePillarDepositQsr(qsr string) (CallPreview, error) {
	amt, ok := new(big.Int).SetString(strings.TrimSpace(qsr), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("deposit amount must be a positive QSR value")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.DepositQsr(amt)
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.QsrTokenStandard, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Deposit %s QSR for pillar (burned on registration)", formatBaseAmount(amt.String(), 8)))
}

// PreparePillarWithdrawQsr builds a WithdrawQsr template (recovers escrowed QSR
// not yet consumed by registration).
func (s *NomService) PreparePillarWithdrawQsr() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.WithdrawQsr()
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Withdraw deposited pillar QSR")
}

// PrepareRegisterPillar builds a Register template (sends the 15,000 ZNN
// collateral; requires the QSR cost already deposited). Amount is read from the
// SDK template, never hardcoded. All inputs are validated before any node use.
func (s *NomService) PrepareRegisterPillar(name, producer, reward string, momentumPct, delegatePct uint8) (CallPreview, error) {
	name = strings.TrimSpace(name)
	if err := validatePillarName(name); err != nil {
		return CallPreview{}, err
	}
	producerAddr, err := types.ParseAddress(strings.TrimSpace(producer))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid producer address: %w", err)
	}
	rewardAddr, err := types.ParseAddress(strings.TrimSpace(reward))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid reward address: %w", err)
	}
	if momentumPct > 100 || delegatePct > 100 {
		return CallPreview{}, errors.New("reward percentages must be between 0 and 100")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.Register(name, producerAddr, rewardAddr, momentumPct, delegatePct)
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Register pillar %q (15,000 ZNN)", name))
}

// PrepareRevokePillar builds a Revoke template (returns the 15,000 ZNN collateral
// after the lock window).
func (s *NomService) PrepareRevokePillar(name string) (CallPreview, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return CallPreview{}, errors.New("pillar name is required")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.Revoke(name)
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Revoke pillar %q", name))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestPreparePillarDepositQsrValidatesInput|TestPrepareRegisterPillarValidatesInput|TestPrepareRevokePillarValidatesInput|TestPillarRegisterTemplateTokenStandards' -v`
Expected: PASS. Then run the full backend suite: `GOWORK=off GOTOOLCHAIN=auto go test ./app/` → PASS, and `GOWORK=off GOTOOLCHAIN=auto go vet ./app/` → clean.

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go
git commit -m "$(cat <<'EOF'
feat(app): pillar registration write methods

Adds PreparePillarDepositQsr/WithdrawQsr/RegisterPillar/RevokePillar,
each routed through tx.prepareCall with server-side validation; Register
collateral (15,000 ZNN) read from the SDK template.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Wails bindings — expose the new methods + model to the frontend

**Files:**
- Modify: `frontend/wailsjs/go/app/NomService.d.ts`
- Modify: `frontend/wailsjs/go/app/NomService.js`
- Modify: `frontend/wailsjs/go/models.ts`

**Interfaces:**
- Consumes: the Task 1–2 Go methods.
- Produces: TS bindings `GetMyPillar`, `GetPillarDepositedQsr`, `GetPillarQsrCost`, `CheckPillarName`, `PreparePillarDepositQsr`, `PreparePillarWithdrawQsr`, `PrepareRegisterPillar`, `PrepareRevokePillar`; `app.OwnedPillarInfo`.

> **Canonical method:** `GOWORK=off GOTOOLCHAIN=auto wails generate module` regenerates these. If that fails in the local environment, apply the exact hand-edits below (they match Wails' output format). After either path, the diff must contain only the additions described here.

- [ ] **Step 1: Add type declarations**

In `frontend/wailsjs/go/app/NomService.d.ts`, add (keep file's existing alphabetical-ish grouping; placement is not load-bearing):

```ts
export function CheckPillarName(arg1:string):Promise<boolean>;
export function GetMyPillar():Promise<app.OwnedPillarInfo>;
export function GetPillarDepositedQsr():Promise<string>;
export function GetPillarQsrCost():Promise<string>;
export function PreparePillarDepositQsr(arg1:string):Promise<app.CallPreview>;
export function PreparePillarWithdrawQsr():Promise<app.CallPreview>;
export function PrepareRegisterPillar(arg1:string,arg2:string,arg3:string,arg4:number,arg5:number):Promise<app.CallPreview>;
export function PrepareRevokePillar(arg1:string):Promise<app.CallPreview>;
```

- [ ] **Step 2: Add the JS bindings**

In `frontend/wailsjs/go/app/NomService.js`, add:

```js
export function CheckPillarName(arg1) {
  return window['go']['app']['NomService']['CheckPillarName'](arg1);
}

export function GetMyPillar() {
  return window['go']['app']['NomService']['GetMyPillar']();
}

export function GetPillarDepositedQsr() {
  return window['go']['app']['NomService']['GetPillarDepositedQsr']();
}

export function GetPillarQsrCost() {
  return window['go']['app']['NomService']['GetPillarQsrCost']();
}

export function PreparePillarDepositQsr(arg1) {
  return window['go']['app']['NomService']['PreparePillarDepositQsr'](arg1);
}

export function PreparePillarWithdrawQsr() {
  return window['go']['app']['NomService']['PreparePillarWithdrawQsr']();
}

export function PrepareRegisterPillar(arg1, arg2, arg3, arg4, arg5) {
  return window['go']['app']['NomService']['PrepareRegisterPillar'](arg1, arg2, arg3, arg4, arg5);
}

export function PrepareRevokePillar(arg1) {
  return window['go']['app']['NomService']['PrepareRevokePillar'](arg1);
}
```

- [ ] **Step 3: Add the model class**

In `frontend/wailsjs/go/models.ts`, inside the `export namespace app {` block (e.g. right after the `PlasmaInfo` class), add:

```ts
	export class OwnedPillarInfo {
	    name: string;
	    ownerAddress: string;
	    producerAddress: string;
	    rewardAddress: string;
	    giveMomentumRewardPct: number;
	    giveDelegateRewardPct: number;
	    isRevocable: boolean;
	    revokeCooldown: number;

	    static createFrom(source: any = {}) {
	        return new OwnedPillarInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ownerAddress = source["ownerAddress"];
	        this.producerAddress = source["producerAddress"];
	        this.rewardAddress = source["rewardAddress"];
	        this.giveMomentumRewardPct = source["giveMomentumRewardPct"];
	        this.giveDelegateRewardPct = source["giveDelegateRewardPct"];
	        this.isRevocable = source["isRevocable"];
	        this.revokeCooldown = source["revokeCooldown"];
	    }
	}
```

- [ ] **Step 4: Verify typecheck still passes**

Run: `cd frontend && pnpm run typecheck`
Expected: PASS (no new consumers yet; this only confirms the bindings are well-formed).

- [ ] **Step 5: Commit**

```bash
git add frontend/wailsjs/go/app/NomService.d.ts frontend/wailsjs/go/app/NomService.js frontend/wailsjs/go/models.ts
git commit -m "$(cat <<'EOF'
chore(bindings): regenerate Wails bindings for pillar registration

Exposes GetMyPillar/GetPillarDepositedQsr/GetPillarQsrCost/CheckPillarName
and the pillar Prepare* methods + OwnedPillarInfo model.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Frontend — `isValidPillarName` helper

**Files:**
- Modify: `frontend/src/lib/format.ts`
- Test: `frontend/src/lib/format.test.ts`

**Interfaces:**
- Produces: `isValidPillarName(name: string): boolean`.

- [ ] **Step 1: Write the failing tests**

Append to `frontend/src/lib/format.test.ts`:

```ts
import { isValidPillarName } from './format'

describe('isValidPillarName', () => {
  it('accepts alphanumerics with single separators between them', () => {
    for (const n of ['Pillar', 'my-pillar', 'a.b_c', 'P1', 'Node-01.eu', 'a']) {
      expect(isValidPillarName(n)).toBe(true)
    }
  })
  it('rejects empty, edge separators, doubles, spaces, symbols, and >40 chars', () => {
    for (const n of ['', '-x', 'x-', 'a--b', 'has space', 'bad!', 'a'.repeat(41)]) {
      expect(isValidPillarName(n)).toBe(false)
    }
  })
})
```

(If `format.test.ts` already imports from `./format` and declares `describe`, reuse those imports rather than redeclaring — merge the new `import { isValidPillarName }` into the existing import line and just add the `describe` block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/lib/format.test.ts`
Expected: FAIL — `isValidPillarName is not a function` / no export.

- [ ] **Step 3: Implement the helper**

Append to `frontend/src/lib/format.ts`:

```ts
// isValidPillarName mirrors go-zenon's pillar name rule (1–40 chars; alphanumerics
// with single - . _ allowed only between alphanumerics) for instant client-side
// feedback. The backend + CheckNameAvailability remain authoritative.
const PILLAR_NAME_RE = /^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$/
export function isValidPillarName(name: string): boolean {
  if (name.length === 0 || name.length > 40) return false
  return PILLAR_NAME_RE.test(name)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/lib/format.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/format.ts frontend/src/lib/format.test.ts
git commit -m "$(cat <<'EOF'
feat(vue): isValidPillarName helper mirroring go-zenon rule

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Frontend — parameterize `StepHeader` labels

**Files:**
- Modify: `frontend/src/components/panels/StepHeader.vue`

**Interfaces:**
- Produces: `StepHeader` props `current: 1|2|3`, optional `steps?: { n: number; label: string }[]` (defaults to the Sentinel labels), optional `ariaLabel?: string` (default `'Sentinel launch progress'`).
- Consumes (downstream): `PillarLaunch` (Task 7) passes pillar-specific `steps` + `ariaLabel`.

- [ ] **Step 1: Update the component**

Replace the entire contents of `frontend/src/components/panels/StepHeader.vue` with:

```vue
<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    current: 1 | 2 | 3
    steps?: { n: number; label: string }[]
    ariaLabel?: string
  }>(),
  {
    steps: () => [
      { n: 1, label: 'Deposit 50,000 QSR' },
      { n: 2, label: 'Deposit 5,000 ZNN' },
      { n: 3, label: 'Sentinel active' },
    ],
    ariaLabel: 'Sentinel launch progress',
  },
)
</script>

<template>
  <ol class="flex flex-wrap items-center gap-2" :aria-label="props.ariaLabel">
    <li
      v-for="(s, i) in props.steps"
      :key="s.n"
      class="flex items-center gap-2"
      :data-state="s.n < current ? 'done' : s.n === current ? 'current' : 'todo'"
    >
      <span
        class="grid h-6 w-6 shrink-0 place-items-center rounded-full border text-xs font-medium"
        :class="s.n < current
          ? 'border-primary bg-primary text-primary-foreground'
          : s.n === current
            ? 'border-primary text-primary'
            : 'border-border text-muted-foreground'"
      >
        <svg v-if="s.n < current" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
        <template v-else>{{ s.n }}</template>
      </span>
      <span
        class="whitespace-nowrap text-xs"
        :class="s.n === current ? 'font-medium text-foreground' : 'text-muted-foreground'"
      >{{ s.label }}</span>
      <span v-if="i < props.steps.length - 1" class="mx-1 hidden h-px w-6 bg-border sm:block" />
    </li>
  </ol>
</template>
```

- [ ] **Step 2: Verify Sentinel regression + typecheck**

Run: `cd frontend && pnpm exec vitest run src/components/panels/SentinelLaunch.test.ts && pnpm run typecheck`
Expected: PASS — the default `steps` preserve the Sentinel labels (the existing test that asserts `[data-state="current"]` contains `Deposit 50,000 QSR` still passes).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/panels/StepHeader.vue
git commit -m "$(cat <<'EOF'
refactor(vue): parameterize StepHeader labels for reuse

Defaults preserve the Sentinel labels; adds optional steps/ariaLabel props
so the pillar wizard can supply its own.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Frontend — extend the `pillar` store with registration state

**Files:**
- Modify: `frontend/src/stores/pillar.ts`
- Test: `frontend/src/stores/pillar.test.ts` (new)

**Interfaces:**
- Consumes: bindings `GetMyPillar`, `GetPillarDepositedQsr`, `GetPillarQsrCost`, `GetPlasmaInfo`, `GetPillarReward` (Task 3 + existing).
- Produces: exports `PILLAR_PLASMA_REQUIRED: bigint`, `FUSE_RECOMMENDED_QSR: string`; store getters `ownsPillar`, `qsrCleared`, `plasmaCleared`; actions `refreshRegistration()`, `beginPending(step)`, `settleCheck()`, `stopPolling()`; state `myPillar`, `depositedQsr`, `qsrCost`, `plasma`, `pendingStep`, `pollCount`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/stores/pillar.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePillarStore, PILLAR_PLASMA_REQUIRED } from './pillar'

// Don't touch the (unmocked) backend; refreshRegistration is stubbed per test.
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetMyPillar: vi.fn(), GetPillarDepositedQsr: vi.fn(), GetPillarQsrCost: vi.fn(),
  GetPlasmaInfo: vi.fn(), GetPillarReward: vi.fn(),
  GetPillarList: vi.fn(), GetDelegation: vi.fn(),
}))

beforeEach(() => setActivePinia(createPinia()))

describe('pillar store registration pending/poll', () => {
  it('beginPending(plasma) clears once plasma reaches the requirement', async () => {
    vi.useFakeTimers()
    const s = usePillarStore()
    vi.spyOn(s, 'refreshRegistration').mockImplementation(async () => {
      s.plasma = { currentPlasma: Number(PILLAR_PLASMA_REQUIRED), maxPlasma: 0, qsrFused: '0' } as never
    })
    s.beginPending('plasma')
    expect(s.pendingStep).toBe('plasma')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('beginPending(deposit) clears once deposited reaches the cost', async () => {
    vi.useFakeTimers()
    const s = usePillarStore()
    s.qsrCost = '15000000000000'
    vi.spyOn(s, 'refreshRegistration').mockImplementation(async () => { s.depositedQsr = '15000000000000' })
    s.beginPending('deposit')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('beginPending(register) clears once a pillar is owned', async () => {
    vi.useFakeTimers()
    const s = usePillarStore()
    vi.spyOn(s, 'refreshRegistration').mockImplementation(async () => { s.myPillar = { name: 'Pillar-A' } as never })
    s.beginPending('register')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('stopPolling clears the pending state', () => {
    const s = usePillarStore()
    s.beginPending('deposit')
    s.stopPolling()
    expect(s.pendingStep).toBe(null)
    expect(s.pollCount).toBe(0)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/stores/pillar.test.ts`
Expected: FAIL — `PILLAR_PLASMA_REQUIRED` / `beginPending` not exported/defined.

- [ ] **Step 3: Extend the store**

Replace the contents of `frontend/src/stores/pillar.ts` with:

```ts
import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

// A pillar Register block costs ~105,000 plasma (2 * EmbeddedSimple). We gate on
// this and recommend fusing 500 QSR (~1,050,000 plasma) for a comfortable buffer.
export const PILLAR_PLASMA_REQUIRED = 105000n
export const FUSE_RECOMMENDED_QSR = '500'
const POLL_INTERVAL_MS = 3000

export const usePillarStore = defineStore('pillar', {
  state: () => ({
    // delegation (existing)
    delegation: null as app.DelegationInfo | null,
    pillars: [] as app.PillarSummary[],
    reward: null as app.RewardInfo | null,
    // registration
    myPillar: null as app.OwnedPillarInfo | null,
    depositedQsr: '0',
    qsrCost: '0',
    plasma: null as app.PlasmaInfo | null,
    pendingStep: null as 'plasma' | 'deposit' | 'register' | null,
    pollCount: 0,
    pollHandle: null as number | null,
  }),
  getters: {
    ownsPillar(s): boolean {
      return !!s.myPillar && s.myPillar.name !== ''
    },
    qsrCleared(s): boolean {
      try {
        const cost = BigInt(s.qsrCost || '0')
        return cost > 0n && BigInt(s.depositedQsr || '0') >= cost
      } catch {
        return false
      }
    },
    plasmaCleared(s): boolean {
      try {
        return BigInt(s.plasma?.currentPlasma ?? 0) >= PILLAR_PLASMA_REQUIRED
      } catch {
        return false
      }
    },
  },
  actions: {
    async refreshDelegation() {
      try {
        this.delegation = await Nom.GetDelegation()
      } catch { /* not connected / locked — leave as-is */ }
    },
    async refresh() {
      try {
        this.pillars = await Nom.GetPillarList()
        this.delegation = await Nom.GetDelegation()
        this.reward = await Nom.GetPillarReward()
      } catch { /* not connected / locked — leave as-is */ }
    },
    // Refresh the registration view's chain state (owned pillar, deposit, cost,
    // plasma, reward).
    async refreshRegistration() {
      try {
        this.myPillar = await Nom.GetMyPillar()
        this.depositedQsr = await Nom.GetPillarDepositedQsr()
        this.qsrCost = await Nom.GetPillarQsrCost()
        this.plasma = await Nom.GetPlasmaInfo()
        this.reward = await Nom.GetPillarReward()
      } catch { /* not connected / locked — leave as-is */ }
    },
    // Start polling for a just-published step to settle on-chain, then advance.
    beginPending(step: 'plasma' | 'deposit' | 'register') {
      this.stopPolling()
      this.pendingStep = step
      this.pollCount = 0
      this.pollHandle = window.setInterval(async () => {
        this.pollCount++
        await this.refreshRegistration()
        this.settleCheck()
      }, POLL_INTERVAL_MS)
    },
    // Clear the pending state once the chain reflects the step.
    settleCheck() {
      if (this.pendingStep === 'plasma' && this.plasmaCleared) {
        this.stopPolling()
      } else if (this.pendingStep === 'deposit' && this.qsrCleared) {
        this.stopPolling()
      } else if (this.pendingStep === 'register' && this.ownsPillar) {
        this.stopPolling()
      }
    },
    // Stop polling and clear the pending state (settle, unmount, or cancel).
    stopPolling() {
      if (this.pollHandle !== null) {
        clearInterval(this.pollHandle)
        this.pollHandle = null
      }
      this.pendingStep = null
      this.pollCount = 0
    },
  },
})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/stores/pillar.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/pillar.ts frontend/src/stores/pillar.test.ts
git commit -m "$(cat <<'EOF'
feat(vue): extend pillar store with registration state + polling

Mirrors the sentinel store: myPillar/depositedQsr/qsrCost/plasma state,
ownsPillar/qsrCleared/plasmaCleared getters, and beginPending/settleCheck/
stopPolling for on-chain clearing. Delegation actions untouched.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Frontend — `PillarLaunch.vue` registration wizard

**Files:**
- Create: `frontend/src/components/panels/PillarLaunch.vue`
- Test: `frontend/src/components/panels/PillarLaunch.test.ts`

**Interfaces:**
- Consumes: pillar store (Task 6), `StepHeader` (Task 5), `isValidPillarName` (Task 4), bindings `PrepareFuse`, `PreparePillarDepositQsr`, `PreparePillarWithdrawQsr`, `PrepareRegisterPillar`, `CheckPillarName`, `useTxStore`, `useWalletStore`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/panels/PillarLaunch.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareFuse: vi.fn(() => Promise.resolve({ kind: 'fuse' })),
  PreparePillarDepositQsr: vi.fn(() => Promise.resolve({ kind: 'deposit' })),
  PreparePillarWithdrawQsr: vi.fn(() => Promise.resolve({ kind: 'withdraw' })),
  PrepareRegisterPillar: vi.fn(() => Promise.resolve({ kind: 'register' })),
  CheckPillarName: vi.fn(() => Promise.resolve(true)),
}))

import PillarLaunch from './PillarLaunch.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { useWalletStore } from '../../stores/wallet'

const COST = '15000000000000' // arbitrary QSR cost for tests
const ENOUGH_PLASMA = 105000

function setup(opts: { plasma?: number; deposited?: string; cost?: string; pendingStep?: 'plasma' | 'deposit' | 'register' | null } = {}) {
  setActivePinia(createPinia())
  const s = usePillarStore()
  const tx = useTxStore()
  const wallet = useWalletStore()
  wallet.accounts = [{ index: 0, address: 'z1qtest', label: '' }] as never
  wallet.activeIndex = 0
  vi.spyOn(s, 'refreshRegistration').mockResolvedValue()
  const begin = vi.spyOn(s, 'beginPending').mockImplementation(() => {})
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.plasma = { currentPlasma: opts.plasma ?? 0, maxPlasma: 0, qsrFused: '0' } as never
  s.depositedQsr = opts.deposited ?? '0'
  s.qsrCost = opts.cost ?? COST
  s.pendingStep = opts.pendingStep ?? null
  return { s, tx, begin, awaitConfirm }
}

describe('PillarLaunch wizard', () => {
  it('step 1: shows the fuse action when plasma is short', () => {
    setup({ plasma: 0 })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="fuse plasma"]').exists()).toBe(true)
    expect(w.find('[data-state="current"]').text()).toContain('Fuse plasma')
  })

  it('step 2: shows the deposit action + burn warning + withdraw escape once plasma clears', () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: '0', cost: COST })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="deposit pillar qsr"]').exists()).toBe(true)
    expect(w.find('button[aria-label="withdraw pillar qsr"]').exists()).toBe(true)
    expect(w.text().toLowerCase()).toContain('burned')
  })

  it('step 3: shows the register form once QSR clears', () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: COST, cost: COST })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="register pillar"]').exists()).toBe(true)
  })

  it('clearing: hides actions and shows the waiting message while pending', () => {
    setup({ plasma: 0, pendingStep: 'plasma' })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="fuse plasma"]').exists()).toBe(false)
    expect(w.text().toLowerCase()).toContain('waiting')
  })

  it('forwards the fuse call and begins polling when it completes', async () => {
    const { tx, begin, awaitConfirm } = setup({ plasma: 0 })
    const w = mount(PillarLaunch)
    await w.find('button[aria-label="fuse plasma"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareFuse).toHaveBeenCalledWith('z1qtest', '50000000000') // 500 QSR in base units
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'fuse' })
    tx.status = 'done'
    await w.vm.$nextTick()
    expect(begin).toHaveBeenCalledWith('plasma')
  })

  it('disables register when the name is invalid', async () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: COST, cost: COST })
    const w = mount(PillarLaunch)
    await w.find('input[aria-label="pillar name"]').setValue('bad name!')
    expect(w.find('button[aria-label="register pillar"]').attributes('disabled')).toBeDefined()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/PillarLaunch.test.ts`
Expected: FAIL — cannot resolve `./PillarLaunch.vue`.

- [ ] **Step 3: Create the component**

Create `frontend/src/components/panels/PillarLaunch.vue`:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore, PILLAR_PLASMA_REQUIRED, FUSE_RECOMMENDED_QSR } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { useWalletStore } from '../../stores/wallet'
import { formatAmount, toBase, isValidPillarName } from '../../lib/format'
import StepHeader from './StepHeader.vue'
import Field from '../Field.vue'

const SLOW_AFTER_POLLS = 6
const PILLAR_STEPS = [
  { n: 1, label: 'Fuse plasma' },
  { n: 2, label: 'Deposit QSR' },
  { n: 3, label: 'Configure & register' },
]

const pillarStore = usePillarStore()
const tx = useTxStore()
const wallet = useWalletStore()
const { depositedQsr, qsrCost, plasma, pendingStep, pollCount } = storeToRefs(pillarStore)
const error = ref('')

// Registration form.
const name = ref('')
const producer = ref(wallet.activeAddress)
const reward = ref(wallet.activeAddress)
const momentumPct = ref('100')
const delegatePct = ref('100')
const nameAvailable = ref<boolean | null>(null)

const plasmaCurrent = computed(() => {
  try {
    return BigInt(plasma.value?.currentPlasma ?? 0)
  } catch {
    return 0n
  }
})
const plasmaCleared = computed(() => plasmaCurrent.value >= PILLAR_PLASMA_REQUIRED)
const deposited = computed(() => {
  try {
    return BigInt(depositedQsr.value || '0')
  } catch {
    return 0n
  }
})
const cost = computed(() => {
  try {
    return BigInt(qsrCost.value || '0')
  } catch {
    return 0n
  }
})
const shortfall = computed(() => (cost.value > deposited.value ? cost.value - deposited.value : 0n))
const qsrCleared = computed(() => cost.value > 0n && deposited.value >= cost.value)
const clearing = computed(() => pendingStep.value !== null)
const slow = computed(() => pendingStep.value !== null && pollCount.value >= SLOW_AFTER_POLLS)
const currentStep = computed<1 | 2 | 3>(() => (!plasmaCleared.value ? 1 : !qsrCleared.value ? 2 : 3))

const nameValid = computed(() => isValidPillarName(name.value.trim()))
const pctValid = computed(() => {
  const m = Number(momentumPct.value)
  const d = Number(delegatePct.value)
  return Number.isInteger(m) && m >= 0 && m <= 100 && Number.isInteger(d) && d >= 0 && d <= 100
})
const canRegister = computed(
  () =>
    nameValid.value &&
    nameAvailable.value !== false &&
    producer.value.trim() !== '' &&
    reward.value.trim() !== '' &&
    pctValid.value,
)

// Check availability when the name becomes valid (best-effort; backend is final).
watch(name, async (n) => {
  nameAvailable.value = null
  if (!isValidPillarName(n.trim())) return
  try {
    nameAvailable.value = await Nom.CheckPillarName(n.trim())
  } catch {
    nameAvailable.value = null
  }
})

// Remember which action we initiated so the tx-done watcher can begin polling.
let lastAction: 'plasma' | 'deposit' | 'register' | null = null

async function fuse() {
  error.value = ''
  lastAction = 'plasma'
  try {
    tx.awaitConfirm(await Nom.PrepareFuse(wallet.activeAddress, toBase(FUSE_RECOMMENDED_QSR, 8)))
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function deposit() {
  error.value = ''
  lastAction = 'deposit'
  try {
    tx.awaitConfirm(await Nom.PreparePillarDepositQsr(shortfall.value.toString()))
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function withdraw() {
  // Recovers the escrowed QSR — no clearing wait, just refresh.
  error.value = ''
  lastAction = null
  try {
    tx.awaitConfirm(await Nom.PreparePillarWithdrawQsr())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function register() {
  error.value = ''
  lastAction = 'register'
  try {
    tx.awaitConfirm(
      await Nom.PrepareRegisterPillar(
        name.value.trim(),
        producer.value.trim(),
        reward.value.trim(),
        Number(momentumPct.value),
        Number(delegatePct.value),
      ),
    )
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// When a step publishes, poll for it to settle on-chain, then advance.
watch(
  () => tx.status,
  (s) => {
    if (s === 'idle' || s === 'error') {
      lastAction = null
      return
    }
    if (s !== 'done') return
    if (lastAction === 'plasma' || lastAction === 'deposit' || lastAction === 'register') {
      pillarStore.beginPending(lastAction)
    } else {
      pillarStore.refreshRegistration()
    }
    lastAction = null
  },
)
</script>

<template>
  <section class="space-y-4 rounded-lg border border-border bg-card p-4">
    <StepHeader :steps="PILLAR_STEPS" :current="currentStep" ariaLabel="Pillar registration progress" />

    <!-- Clearing (transient): waiting for the contract / fusion to settle. -->
    <div v-if="clearing" class="space-y-2">
      <div class="flex items-center gap-2 text-sm font-medium text-info">
        <svg class="animate-spin" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        <span>{{
          pendingStep === 'plasma'
            ? 'Fusing plasma — waiting for it to land on-chain…'
            : pendingStep === 'deposit'
              ? 'Your QSR deposit is on-chain. Waiting for the pillar contract to credit it…'
              : 'Registering your pillar — waiting for activation…'
        }}</span>
      </div>
      <p class="text-xs text-muted-foreground">This usually takes a few momentums.</p>
      <div v-if="slow" class="flex items-center gap-2">
        <p class="text-xs text-muted-foreground">Taking longer than usual — the network may be busy.</p>
        <button
          type="button"
          class="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          @click="pillarStore.refreshRegistration()"
        >
          Refresh
        </button>
        <button
          type="button"
          aria-label="stop waiting"
          class="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          @click="pillarStore.stopPolling()"
        >
          Stop waiting
        </button>
      </div>
    </div>

    <!-- Step 1: ensure enough fused plasma. -->
    <template v-else-if="!plasmaCleared">
      <p class="text-xs text-muted-foreground">
        Registering a pillar needs fused plasma. We recommend fusing 500 QSR (you can cancel the
        fusion later from the Plasma tab to reclaim it).
      </p>
      <p class="text-sm text-muted-foreground">
        Current plasma <span class="font-mono text-foreground">{{ plasma?.currentPlasma ?? 0 }}</span>
      </p>
      <Button class="w-full" aria-label="fuse plasma" @click="fuse">Fuse 500 QSR for plasma</Button>
    </template>

    <!-- Step 2: deposit the (dynamic) QSR registration cost. -->
    <template v-else-if="!qsrCleared">
      <p class="rounded border border-destructive/40 bg-destructive/10 p-2 text-xs text-destructive">
        ⚠ Deposited QSR is <strong>burned and unrecoverable</strong> once the pillar is registered.
        You can withdraw it before registering if you change your mind.
      </p>
      <p class="text-sm text-muted-foreground">
        Deposited
        <span class="font-mono text-foreground"
          >{{ formatAmount(depositedQsr, 8) }} / {{ formatAmount(qsrCost, 8) }} QSR</span
        >
      </p>
      <Button class="w-full" :disabled="shortfall === 0n" aria-label="deposit pillar qsr" @click="deposit"
        >Deposit {{ formatAmount(shortfall.toString(), 8) }} QSR</Button
      >
      <Button variant="outline" class="w-full" aria-label="withdraw pillar qsr" @click="withdraw"
        >Changed your mind? Withdraw deposited QSR</Button
      >
    </template>

    <!-- Step 3: configure + register (sends the 15,000 ZNN collateral). -->
    <template v-else>
      <p class="text-sm text-foreground">✓ QSR cleared. Configure and register your pillar.</p>
      <Field
        label="Pillar name"
        :error="name.length > 0 && !nameValid ? 'Letters, digits, and single - . _ between them (max 40).' : ''"
        :hint="nameValid && nameAvailable === false ? 'Name is already taken.' : nameValid && nameAvailable ? 'Available.' : 'Choose a unique name.'"
      >
        <Input v-model="name" placeholder="my-pillar" aria-label="pillar name" />
      </Field>
      <Field label="Producer address" hint="Your pillar node's block-producing address.">
        <Input v-model="producer" placeholder="z1…" aria-label="producer address" />
      </Field>
      <Field label="Reward address" hint="Where pillar rewards are collected.">
        <Input v-model="reward" placeholder="z1…" aria-label="reward address" />
      </Field>
      <Field label="Momentum reward % (to delegators)">
        <Input v-model="momentumPct" placeholder="0–100" aria-label="momentum percent" />
      </Field>
      <Field label="Delegate reward % (to delegators)">
        <Input v-model="delegatePct" placeholder="0–100" aria-label="delegate percent" />
      </Field>
      <Button class="w-full" :disabled="!canRegister" aria-label="register pillar" @click="register"
        >Deposit 15,000 ZNN &amp; Register Pillar</Button
      >
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </section>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/PillarLaunch.test.ts`
Expected: PASS. Then `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/panels/PillarLaunch.vue frontend/src/components/panels/PillarLaunch.test.ts
git commit -m "$(cat <<'EOF'
feat(vue): PillarLaunch registration wizard

3-step flow (fuse plasma -> deposit QSR -> configure & register) mirroring
SentinelLaunch: clearing/poll states, burn warning, withdraw escape hatch,
name/percentage validation, 15,000 ZNN register.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Frontend — `PillarActive.vue` owned-pillar view

**Files:**
- Create: `frontend/src/components/panels/PillarActive.vue`
- Test: `frontend/src/components/panels/PillarActive.test.ts`

**Interfaces:**
- Consumes: pillar store `myPillar`, `reward` (Task 6); bindings `PrepareCollectPillarReward` (existing), `PrepareRevokePillar` (Task 3); `useTxStore`; `formatAmount`, `shortAddress`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/panels/PillarActive.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareCollectPillarReward: vi.fn(() => Promise.resolve({ kind: 'collect' })),
  PrepareRevokePillar: vi.fn(() => Promise.resolve({ kind: 'revoke' })),
}))

import PillarActive from './PillarActive.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'

function setup(opts: { reward?: { znn: string; qsr: string }; isRevocable?: boolean; revokeCooldown?: number } = {}) {
  setActivePinia(createPinia())
  const s = usePillarStore()
  const tx = useTxStore()
  vi.spyOn(s, 'refreshRegistration').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.myPillar = {
    name: 'Pillar-A',
    ownerAddress: 'z1own',
    producerAddress: 'z1prod',
    rewardAddress: 'z1rew',
    giveMomentumRewardPct: 0,
    giveDelegateRewardPct: 100,
    isRevocable: opts.isRevocable ?? false,
    revokeCooldown: opts.revokeCooldown ?? 600,
  } as never
  s.reward = opts.reward ?? { znn: '0', qsr: '0' } as never
  return { s, tx, awaitConfirm }
}

describe('PillarActive', () => {
  it('disables Collect when reward is zero', () => {
    setup({ reward: { znn: '0', qsr: '0' } })
    const w = mount(PillarActive)
    expect(w.find('button[aria-label="collect pillar reward"]').attributes('disabled')).toBeDefined()
  })

  it('disables Revoke with a cooldown note when not revocable', () => {
    setup({ isRevocable: false, revokeCooldown: 600 })
    const w = mount(PillarActive)
    const btn = w.find('button[aria-label="revoke pillar"]')
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.text()).toContain('600')
  })

  it('forwards collect to tx.awaitConfirm', async () => {
    const { awaitConfirm } = setup({ reward: { znn: '100', qsr: '0' } })
    const w = mount(PillarActive)
    await w.find('button[aria-label="collect pillar reward"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareCollectPillarReward).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'collect' })
  })

  it('forwards revoke with the pillar name when revocable', async () => {
    setup({ isRevocable: true })
    const w = mount(PillarActive)
    await w.find('button[aria-label="revoke pillar"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareRevokePillar).toHaveBeenCalledWith('Pillar-A')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/PillarActive.test.ts`
Expected: FAIL — cannot resolve `./PillarActive.vue`.

- [ ] **Step 3: Create the component**

Create `frontend/src/components/panels/PillarActive.vue`:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { formatAmount, shortAddress } from '../../lib/format'

const pillarStore = usePillarStore()
const tx = useTxStore()
const { myPillar, reward } = storeToRefs(pillarStore)
const error = ref('')

const rewardZero = computed(
  () => !reward.value || (reward.value.znn === '0' && reward.value.qsr === '0'),
)

async function collect() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCollectPillarReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function revoke() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareRevokePillar(myPillar.value?.name ?? ''))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// Refresh after a collect/revoke settles (reward updates; revoke clears ownership).
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') pillarStore.refreshRegistration()
  },
)
</script>

<template>
  <section v-if="myPillar" class="space-y-3 rounded-lg border border-border bg-card p-4">
    <div class="flex items-center gap-2">
      <svg class="text-primary" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg>
      <h2 class="text-sm font-medium text-foreground">Your Pillar</h2>
      <span class="rounded-full bg-primary/15 px-2 py-0.5 text-xs font-medium text-primary">{{ myPillar.name }}</span>
    </div>
    <dl class="space-y-1 text-sm text-muted-foreground">
      <div class="flex justify-between">
        <dt>Producer</dt>
        <dd class="font-mono text-foreground">{{ shortAddress(myPillar.producerAddress) }}</dd>
      </div>
      <div class="flex justify-between">
        <dt>Reward address</dt>
        <dd class="font-mono text-foreground">{{ shortAddress(myPillar.rewardAddress) }}</dd>
      </div>
      <div class="flex justify-between">
        <dt>Momentum / Delegate %</dt>
        <dd class="font-mono text-foreground">{{ myPillar.giveMomentumRewardPct }}% / {{ myPillar.giveDelegateRewardPct }}%</dd>
      </div>
    </dl>
    <p v-if="reward" class="text-sm text-muted-foreground">
      Uncollected reward
      <span class="font-mono text-foreground"
        >{{ formatAmount(reward.znn, 8) }} ZNN · {{ formatAmount(reward.qsr, 8) }} QSR</span
      >
    </p>
    <div class="flex flex-wrap items-center gap-2">
      <Button :disabled="rewardZero" aria-label="collect pillar reward" @click="collect">Collect</Button>
      <Button
        variant="outline"
        :disabled="!myPillar.isRevocable"
        aria-label="revoke pillar"
        @click="revoke"
        >Revoke<template v-if="!myPillar.isRevocable">
          (cooldown {{ myPillar.revokeCooldown }}s)</template
        ></Button
      >
    </div>
    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
  </section>
</template>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/PillarActive.test.ts`
Expected: PASS. Then `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/panels/PillarActive.vue frontend/src/components/panels/PillarActive.test.ts
git commit -m "$(cat <<'EOF'
feat(vue): PillarActive owned-pillar view

Status + producer/reward/percentages, Collect (disabled when zero) and
Revoke (disabled with cooldown note), mirroring SentinelActive.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Frontend — restructure `PillarPanel` into a sub-tabbed container

**Files:**
- Create: `frontend/src/components/panels/PillarDelegate.vue` (the current delegation UI, moved verbatim)
- Modify: `frontend/src/components/panels/PillarPanel.vue` (becomes the container)
- Test: `frontend/src/components/panels/PillarPanel.test.ts` (new)

**Interfaces:**
- Consumes: pillar store `ownsPillar`, `refreshRegistration`, `stopPolling` (Task 6); `PillarLaunch` (Task 7), `PillarActive` (Task 8); nom-ui `Tabs`/`TabsList`/`TabsTrigger`/`TabsContent`.
- Produces: `PillarPanel` container (the `Home.vue` `<TabsContent value="Pillar">` slot — Home.vue needs no change, it still imports `PillarPanel`).

- [ ] **Step 1: Extract the delegation UI into `PillarDelegate.vue`**

Create `frontend/src/components/panels/PillarDelegate.vue` with the **current** contents of `PillarPanel.vue` (the existing delegation script + template — copy it verbatim; it already imports from `../../stores/pillar`, `../../stores/tx`, `../../lib/format`). This is a pure move: no behavior change. Its `onMounted(() => pillar.refresh())` and `watch(tx.status…)` stay as-is.

- [ ] **Step 2: Write the failing container test**

Create `frontend/src/components/panels/PillarPanel.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  // Render all tab content so we can assert routing without driving tab state.
  Tabs: { template: '<div><slot /></div>' },
  TabsList: { template: '<div><slot /></div>' },
  TabsTrigger: { template: '<button><slot /></button>' },
  TabsContent: { template: '<div><slot /></div>' },
}))
vi.mock('./PillarDelegate.vue', () => ({ default: { name: 'PillarDelegate', template: '<div data-test="delegate" />' } }))
vi.mock('./PillarLaunch.vue', () => ({ default: { name: 'PillarLaunch', template: '<div data-test="launch" />' } }))
vi.mock('./PillarActive.vue', () => ({ default: { name: 'PillarActive', template: '<div data-test="active" />' } }))

import PillarPanel from './PillarPanel.vue'
import { usePillarStore } from '../../stores/pillar'

function setup(myPillar: unknown) {
  setActivePinia(createPinia())
  const s = usePillarStore()
  vi.spyOn(s, 'refreshRegistration').mockResolvedValue()
  s.myPillar = myPillar as never
  return s
}

describe('PillarPanel container', () => {
  it('always renders the delegation sub-view', () => {
    setup(null)
    const w = mount(PillarPanel)
    expect(w.find('[data-test="delegate"]').exists()).toBe(true)
  })

  it('renders the launch wizard when no pillar is owned', () => {
    setup(null)
    const w = mount(PillarPanel)
    expect(w.find('[data-test="launch"]').exists()).toBe(true)
    expect(w.find('[data-test="active"]').exists()).toBe(false)
  })

  it('renders the active view when a pillar is owned', () => {
    setup({ name: 'Pillar-A' })
    const w = mount(PillarPanel)
    expect(w.find('[data-test="active"]').exists()).toBe(true)
    expect(w.find('[data-test="launch"]').exists()).toBe(false)
  })

  it('stops polling on unmount', () => {
    const s = setup(null)
    const stop = vi.spyOn(s, 'stopPolling')
    mount(PillarPanel).unmount()
    expect(stop).toHaveBeenCalled()
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd frontend && pnpm exec vitest run src/components/panels/PillarPanel.test.ts`
Expected: FAIL — current `PillarPanel.vue` renders delegation markup directly, not the mocked child components (`[data-test="delegate"]` absent).

- [ ] **Step 4: Replace `PillarPanel.vue` with the container**

Replace the entire contents of `frontend/src/components/panels/PillarPanel.vue` with:

```vue
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { usePillarStore } from '../../stores/pillar'
import PillarDelegate from './PillarDelegate.vue'
import PillarLaunch from './PillarLaunch.vue'
import PillarActive from './PillarActive.vue'

// Container: "Delegate" keeps the existing delegation flow; "Run a Pillar" shows
// the owned-pillar view if one exists, else the registration wizard. The wizard
// step is derived from chain state by the children.
const pillarStore = usePillarStore()
const { ownsPillar } = storeToRefs(pillarStore)
const sub = ref('Delegate')

onMounted(() => pillarStore.refreshRegistration())
onUnmounted(() => pillarStore.stopPolling())
</script>

<template>
  <div class="p-4">
    <Tabs v-model="sub">
      <TabsList class="w-full justify-start">
        <TabsTrigger value="Delegate">Delegate</TabsTrigger>
        <TabsTrigger value="Run a Pillar">Run a Pillar</TabsTrigger>
      </TabsList>
      <TabsContent value="Delegate"><PillarDelegate /></TabsContent>
      <TabsContent value="Run a Pillar">
        <PillarActive v-if="ownsPillar" />
        <PillarLaunch v-else />
      </TabsContent>
    </Tabs>
  </div>
</template>
```

> Note: `PillarDelegate.vue` keeps its own `p-4` wrapper from the original `PillarPanel`. If the double padding looks off in the running app, drop the outer `class="p-4"` here — purely cosmetic, verify in Task 10's manual check.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd frontend && pnpm exec vitest run src/components/panels/PillarPanel.test.ts`
Expected: PASS. Then `cd frontend && pnpm run typecheck` → PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/panels/PillarDelegate.vue frontend/src/components/panels/PillarPanel.vue frontend/src/components/panels/PillarPanel.test.ts
git commit -m "$(cat <<'EOF'
feat(vue): PillarPanel sub-tabs (Delegate / Run a Pillar)

Extracts the delegation UI into PillarDelegate and turns PillarPanel into a
container that routes to PillarActive (owned) or PillarLaunch (register).
Home.vue is unchanged.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Integration — full suites, typecheck, and build sanity

**Files:** none (verification only; commit only if a glue fix is needed).

- [ ] **Step 1: Full frontend test + typecheck**

Run: `cd frontend && pnpm test && pnpm run typecheck`
Expected: all suites PASS (including the existing Sentinel/Plasma/store tests — confirms no regression from the StepHeader and pillar-store changes), typecheck clean.

- [ ] **Step 2: Full backend test + vet + build**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./... && GOWORK=off GOTOOLCHAIN=auto go vet ./... && GOWORK=off GOTOOLCHAIN=auto go build ./...`
Expected: PASS / clean.

- [ ] **Step 3: Frontend production build**

Run: `cd frontend && pnpm run build`
Expected: Vite build succeeds (catches any template/type issue vitest's transform might miss).

- [ ] **Step 4 (manual, optional but recommended): live smoke test against testnet**

Run: `GOWORK=off wails dev`, unlock a wallet pointed at the testnet node, open the **Pillar → Run a Pillar** tab, and walk the wizard: confirm the plasma step shows current plasma and offers Fuse; the deposit step shows the dynamic cost + burn warning + withdraw; the configure step validates the name live and gates the register button. (Actual on-chain registration requires 15,000 ZNN + the QSR cost on the test address — treat as acceptance, not a required automated gate.)

- [ ] **Step 5: Commit (only if Step 1–3 required a fix)**

```bash
git add -A
git commit -m "$(cat <<'EOF'
test(vue): integration fixups for pillar registration

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**Spec coverage:**
- Deposit required (dynamic) QSR → Task 1 (`GetPillarQsrCost`), Task 2 (`PreparePillarDepositQsr`), Task 7 (step 2). ✓
- Warn QSR burned/unrecoverable → Task 7 step-2 warning banner + test asserts "burned". ✓
- Withdraw QSR escape hatch → Task 2 (`PreparePillarWithdrawQsr`), Task 7 (withdraw button + test). ✓
- Pillar name + go-zenon compliance → Task 1 (`validatePillarName`), Task 4 (`isValidPillarName`), Task 7 (live validation + `CheckPillarName`). ✓
- Name, reward address, reward percentages → Task 7 form; producer address added per decision (editable, default active). ✓
- Plasma check before proceeding + recommend 500 QSR → Task 6 (`PILLAR_PLASMA_REQUIRED`, `FUSE_RECOMMENDED_QSR`), Task 7 step 1 (inline fuse). ✓
- 15,000 ZNN collateral from template → Task 2 (`PrepareRegisterPillar` uses `template.Amount`) + Task 2 template test. ✓
- Minimal owned-pillar view (status/collect/revoke) → Task 8. ✓
- Keep our design/style (Sentinel parity) → StepHeader reuse, wizard pattern, sub-tab container. ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete, copy-pasteable content.

**Type consistency:** `OwnedPillarInfo` fields match across Go DTO (`giveMomentumRewardPct`/`giveDelegateRewardPct`/`rewardAddress`/`producerAddress`/`isRevocable`/`revokeCooldown`/`name`), the models.ts class, and frontend usage (`myPillar.giveMomentumRewardPct`, etc.). Store members (`myPillar`, `depositedQsr`, `qsrCost`, `plasma`, `pendingStep`, `ownsPillar`, `qsrCleared`, `plasmaCleared`, `beginPending`, `settleCheck`, `stopPolling`, `refreshRegistration`) are consistent between Task 6 definition and Tasks 7–9 consumers. `PrepareRegisterPillar` arg order/types (`string,string,string,number,number`) match across Go, bindings (Task 3), and the Task 7 call. `PrepareRevokePillar(name)` consistent in Task 2/3/8.
