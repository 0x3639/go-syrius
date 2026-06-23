# Phase 5e — Tokens (ZTS) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** View the tokens the active address owns + look up any token by ZTS, issue a new token, mint, burn, and update a token (transfer ownership / permanently disable mint/burn) — reusing the 5a–5d shared contract-call path.

**Architecture:** `NomService` gains token reads (`GetMyTokens`/`GetTokenByZts`) + four action builders (`PrepareIssueToken`/`PrepareMint`/`PrepareBurn`/`PrepareUpdateToken`) that build `TokenApi` templates and delegate to `TxService.prepareCall` (confirm-what-you-sign, mainnet-gated). A Tokens route drives the shared `tx` flow. No new backend pipeline — 5a's `prepareCall`/`assertMatches` is reused unchanged.

**Tech Stack:** Go 1.24+, `znn-sdk-go` (`TokenApi`), `go-zenon/common/types` + `vm/constants`, Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- Templates via `client.TokenApi.{IssueToken,Mint,Burn,UpdateToken}`; publish via the existing `prepareCall`. No SDK/go-zenon forks.
- **Token-standard split is correctness-critical (the 5a "Cancel=ZNN" lesson):** `IssueToken`/`Mint`/`UpdateToken` use `types.ZnnTokenStandard`; **`Burn` uses the token's own ZTS and carries the burn amount**. All use `types.TokenContract`. A regression test locks each against the real SDK templates.
- **Amounts:** `IssueToken` amount is read from the template (`constants.TokenIssueAmount` = 1 ZNN = base `100000000`), never hardcoded. `Mint`/`UpdateToken` are Amount 0. `Burn` amount = the caller-supplied burn amount. Supplies + mint/burn amounts are base-unit decimal strings.
- **Issue field validation (mirror on-chain, in Go before any node use):** name length 1–40 + regex `^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$`; symbol length 1–10 + regex `^[A-Z0-9]+$`; domain empty OR (length ≤ 128 AND regex `^([A-Za-z0-9][A-Za-z0-9-]{0,61}[A-Za-z0-9]\.)+[A-Za-z]{2,}$`); decimals in [0, 18]; supplies parse to non-negative big.Int; maxSupply > 0 and ≤ `constants.TokenMaxSupplyBig`; maxSupply ≥ totalSupply; if **not** mintable, maxSupply must **equal** totalSupply.
- `prepareCall` binds to/zts/amount AND ABI `Data`; mainnet stays behind `AllowMainnetSend`; no key material in NomService; inputs validated in Go before any node use.
- `GOWORK=off go test ./...` offline (a parent `/Users/dfriestedt/Github/go.work` references a missing module — always prefix Go/wails commands with `GOWORK=off`). Frontend `pnpm test` + `pnpm run check` (svelte-check 0 errors) + `pnpm run build` pass.
- ENV HAZARD (iCloud repo): `" 2"` collision copies break builds (`find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} +`); stale `node_modules` (`rm -rf frontend/node_modules && pnpm install`); codesign xattrs (`xattr -cr build/bin`). Commits GPG-signed (if signing times out, the gpg-agent cache needs re-warming — report it, don't disable signing).

## File structure

```
app/dto.go                 # MOD: TokenInfo DTO
app/nom_service.go         # MOD: token reads + actions + tokenInfoDTO mapper + issue validation + regex vars
app/nom_service_test.go    # MOD: mapper + issue validation table + action input validation + template-token-standard tests
internal/spike/readonly_integration_test.go  # MOD: TestReadOnlyTokens live read smoke
frontend/wailsjs/...       # regenerated bindings
frontend/src/lib/stores/token.ts    # NEW: token store + refresh + lookup
frontend/src/lib/stores/nav.ts      # MOD: add 'tokens' view
frontend/src/routes/Tokens.svelte   # NEW: my-tokens (mint/update) + lookup/burn + issue
frontend/src/routes/Tokens.test.ts  # NEW
frontend/src/routes/Dashboard.svelte # MOD: link to Tokens
frontend/src/App.svelte             # MOD: route 'tokens'
```

---

## Task 1: NomService token reads + DTO

**Files:** Modify `app/dto.go`, `app/nom_service.go`; Test `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.TokenApi.GetByOwner/GetByZts`, `s.node.currentClient()`, `s.wallet.activeAddress()`, `errLocked`, `types.ParseZTS`.
- Produces: DTO `TokenInfo{Name,Symbol,Domain,TokenStandard,Owner string; TotalSupply,MaxSupply string; Decimals int; IsMintable,IsBurnable,IsUtility bool}`; `GetMyTokens() ([]TokenInfo, error)`; `GetTokenByZts(zts string) (TokenInfo, error)`; pure `tokenInfoDTO(t *embedded.Token) TokenInfo`.

- [ ] **Step 1: Write the failing test**

In `app/dto.go` add (next to the other NoM DTOs):
```go
// TokenInfo is one ZTS token's metadata. An empty TokenStandard means not found.
type TokenInfo struct {
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	Domain        string `json:"domain"`
	TokenStandard string `json:"tokenStandard"`
	Owner         string `json:"owner"`
	TotalSupply   string `json:"totalSupply"`
	MaxSupply     string `json:"maxSupply"`
	Decimals      int    `json:"decimals"`
	IsMintable    bool   `json:"isMintable"`
	IsBurnable    bool   `json:"isBurnable"`
	IsUtility     bool   `json:"isUtility"`
}
```
Add to `app/nom_service_test.go`:
```go
func TestTokenInfoDTO(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	zts, _ := types.ParseZTS("zts1znnxxxxxxxxxxxxx9z4ulx")
	tok := &embedded.Token{
		Name: "Test Token", Symbol: "TEST", Domain: "test.org",
		TotalSupply: big.NewInt(1000), MaxSupply: big.NewInt(2000),
		Decimals: 8, Owner: owner, TokenStandard: zts,
		IsMintable: true, IsBurnable: false, IsUtility: true,
	}
	d := tokenInfoDTO(tok)
	if d.Name != "Test Token" || d.Symbol != "TEST" || d.Domain != "test.org" {
		t.Fatalf("bad strings: %+v", d)
	}
	if d.TotalSupply != "1000" || d.MaxSupply != "2000" || d.Decimals != 8 {
		t.Fatalf("bad supply/decimals: %+v", d)
	}
	if d.Owner != owner.String() || d.TokenStandard != zts.String() {
		t.Fatalf("bad owner/zts: %+v", d)
	}
	if !d.IsMintable || d.IsBurnable || !d.IsUtility {
		t.Fatalf("bad flags: %+v", d)
	}
	// nil supplies → "0"
	z := tokenInfoDTO(&embedded.Token{Name: "X"})
	if z.TotalSupply != "0" || z.MaxSupply != "0" {
		t.Fatalf("nil supplies should be 0: %+v", z)
	}
}
```
(If the example ZTS string `zts1znnxxxxxxxxxxxxx9z4ulx` does not parse, use `types.ZnnTokenStandard` as the test's zts and assert against `types.ZnnTokenStandard.String()`.)

- [ ] **Step 2: Run to verify failure**

Run: `GOWORK=off go test ./app/ -run 'TestTokenInfoDTO' -v`
Expected: FAIL — `tokenInfoDTO` undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go` (the `embedded`, `errors`, `types` imports already exist):
```go
// tokenInfoDTO maps an SDK Token to the DTO. nil supplies map to "0".
func tokenInfoDTO(t *embedded.Token) TokenInfo {
	total, max := "0", "0"
	if t.TotalSupply != nil {
		total = t.TotalSupply.String()
	}
	if t.MaxSupply != nil {
		max = t.MaxSupply.String()
	}
	return TokenInfo{
		Name:          t.Name,
		Symbol:        t.Symbol,
		Domain:        t.Domain,
		TokenStandard: t.TokenStandard.String(),
		Owner:         t.Owner.String(),
		TotalSupply:   total,
		MaxSupply:     max,
		Decimals:      int(t.Decimals),
		IsMintable:    t.IsMintable,
		IsBurnable:    t.IsBurnable,
		IsUtility:     t.IsUtility,
	}
}

// GetMyTokens returns the tokens owned by the active address.
func (s *NomService) GetMyTokens() ([]TokenInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	out := []TokenInfo{}
	var pageIndex uint32 = 0
	const pageSize uint32 = 50
	for {
		list, err := client.TokenApi.GetByOwner(addr, pageIndex, pageSize)
		if err != nil {
			return nil, err
		}
		for _, t := range list.List {
			out = append(out, tokenInfoDTO(t))
		}
		if len(out) >= list.Count || len(list.List) == 0 {
			break
		}
		pageIndex++
	}
	return out, nil
}

// GetTokenByZts returns one token's metadata. zts is validated before any node
// use; an empty TokenStandard in the result means not found.
func (s *NomService) GetTokenByZts(zts string) (TokenInfo, error) {
	parsed, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return TokenInfo{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return TokenInfo{}, errors.New("not connected")
	}
	tok, err := client.TokenApi.GetByZts(parsed)
	if err != nil {
		return TokenInfo{}, err
	}
	if tok == nil {
		return TokenInfo{}, nil
	}
	return tokenInfoDTO(tok), nil
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `GOWORK=off go test ./app/ -run 'TestTokenInfoDTO' -v && GOWORK=off go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/dto.go app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService token reads (my tokens, by-ZTS lookup) + DTO"
```

---

## Task 2: NomService IssueToken action + field validation

**Files:** Modify `app/nom_service.go`, `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.TokenApi.IssueToken`, `tx.prepareCall`, `callExpect`, `types.{TokenContract, ZnnTokenStandard}`, `constants.TokenMaxSupplyBig`, `regexp`, `math/big`.
- Produces: `PrepareIssueToken(name, symbol, domain, totalSupply, maxSupply string, decimals int, isMintable, isBurnable, isUtility bool) (CallPreview, error)`; package regex vars `tokenNameRe`, `tokenSymbolRe`, `tokenDomainRe`.

- [ ] **Step 1: Write the failing test**

Add to `app/nom_service_test.go`:
```go
func TestPrepareIssueTokenValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// each call must be rejected BEFORE any node use (node is not connected in this test,
	// but validation runs first, so we assert a validation error, not "not connected").
	cases := []struct {
		name                          string
		tn, ts, td, total, max        string
		decimals                      int
		mintable                      bool
	}{
		{"empty name", "", "TEST", "", "100", "100", 8, false},
		{"bad name char", "bad name", "TEST", "", "100", "100", 8, false},
		{"empty symbol", "Tok", "", "", "100", "100", 8, false},
		{"lowercase symbol", "Tok", "test", "", "100", "100", 8, false},
		{"bad domain", "Tok", "TEST", "not_a_domain", "100", "100", 8, false},
		{"decimals too high", "Tok", "TEST", "", "100", "100", 19, false},
		{"decimals negative", "Tok", "TEST", "", "100", "100", -1, false},
		{"maxSupply zero", "Tok", "TEST", "", "0", "0", 8, true},
		{"max < total", "Tok", "TEST", "", "200", "100", 8, true},
		{"non-mintable max != total", "Tok", "TEST", "", "100", "200", 8, false},
		{"unparseable total", "Tok", "TEST", "", "abc", "100", 8, true},
	}
	for _, c := range cases {
		if _, err := s.PrepareIssueToken(c.tn, c.ts, c.td, c.total, c.max, c.decimals, c.mintable, true, false); err == nil {
			t.Fatalf("%s: expected validation error", c.name)
		}
	}
	// a valid set must pass validation and fail only on the not-connected node.
	_, err := s.PrepareIssueToken("Valid-Token", "VALID", "valid.org", "100", "100", 8, false, true, false)
	if err == nil || err.Error() != "not connected" {
		t.Fatalf("valid input should pass validation and hit not-connected; got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareIssueToken' -v`
Expected: FAIL — `PrepareIssueToken` undefined.

- [ ] **Step 3: Implement**

Add `"regexp"` to the import block and `constants "github.com/zenon-network/go-zenon/vm/constants"`. Add to `app/nom_service.go`:
```go
var (
	tokenNameRe   = regexp.MustCompile(`^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$`)
	tokenSymbolRe = regexp.MustCompile(`^[A-Z0-9]+$`)
	tokenDomainRe = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]{0,61}[A-Za-z0-9]\.)+[A-Za-z]{2,}$`)
)

// PrepareIssueToken validates every field against the on-chain rules (before any
// node use), then builds an IssueToken template. The 1 ZNN fee is read from the
// template, never hardcoded.
func (s *NomService) PrepareIssueToken(name, symbol, domain, totalSupply, maxSupply string, decimals int, isMintable, isBurnable, isUtility bool) (CallPreview, error) {
	if l := len(name); l == 0 || l > 40 || !tokenNameRe.MatchString(name) {
		return CallPreview{}, errors.New("invalid token name (1-40 chars, letters/digits with single -._ separators)")
	}
	if l := len(symbol); l == 0 || l > 10 || !tokenSymbolRe.MatchString(symbol) {
		return CallPreview{}, errors.New("invalid token symbol (1-10 chars, A-Z and 0-9 only)")
	}
	if len(domain) != 0 && (len(domain) > 128 || !tokenDomainRe.MatchString(domain)) {
		return CallPreview{}, errors.New("invalid token domain")
	}
	if decimals < 0 || decimals > 18 {
		return CallPreview{}, errors.New("decimals must be 0 to 18")
	}
	total, ok := new(big.Int).SetString(totalSupply, 10)
	if !ok || total.Sign() < 0 {
		return CallPreview{}, errors.New("invalid total supply")
	}
	max, ok := new(big.Int).SetString(maxSupply, 10)
	if !ok || max.Sign() <= 0 {
		return CallPreview{}, errors.New("max supply must be greater than 0")
	}
	if max.Cmp(constants.TokenMaxSupplyBig) > 0 {
		return CallPreview{}, errors.New("max supply exceeds the maximum")
	}
	if max.Cmp(total) < 0 {
		return CallPreview{}, errors.New("max supply must be >= total supply")
	}
	if !isMintable && max.Cmp(total) != 0 {
		return CallPreview{}, errors.New("non-mintable token requires max supply == total supply")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.IssueToken(name, symbol, domain, total, max, uint8(decimals), isMintable, isBurnable, isUtility)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Issue token %s", symbol))
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareIssueToken' -v && GOWORK=off go build ./... && GOWORK=off go vet ./app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService PrepareIssueToken with on-chain field validation"
```

---

## Task 3: NomService Mint / Burn / Update actions

**Files:** Modify `app/nom_service.go`, `app/nom_service_test.go`.

**Interfaces:**
- Consumes: `client.TokenApi.{Mint,Burn,UpdateToken}`, `tx.prepareCall`, `callExpect`, `types.{TokenContract, ZnnTokenStandard, ParseZTS, ParseAddress}`, `embedded.NewTokenApi`, `constants.TokenIssueAmount`, `nom.AccountBlock`.
- Produces: `PrepareMint(zts, amount, receiver string) (CallPreview, error)`; `PrepareBurn(zts, amount string) (CallPreview, error)`; `PrepareUpdateToken(zts, newOwner string, isMintable, isBurnable bool) (CallPreview, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/nom_service_test.go`:
```go
func TestPrepareMintBurnUpdateValidateInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	good := types.ZnnTokenStandard.String()
	addr := "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	// Mint: bad zts, non-positive amount, bad receiver
	if _, err := s.PrepareMint("bad", "1", addr); err == nil {
		t.Fatal("mint: bad zts must error")
	}
	if _, err := s.PrepareMint(good, "0", addr); err == nil {
		t.Fatal("mint: zero amount must error")
	}
	if _, err := s.PrepareMint(good, "1", "notanaddr"); err == nil {
		t.Fatal("mint: bad receiver must error")
	}
	// Burn: bad zts, non-positive amount
	if _, err := s.PrepareBurn("bad", "1"); err == nil {
		t.Fatal("burn: bad zts must error")
	}
	if _, err := s.PrepareBurn(good, "-1"); err == nil {
		t.Fatal("burn: negative amount must error")
	}
	// Update: bad zts, bad owner
	if _, err := s.PrepareUpdateToken("bad", addr, true, true); err == nil {
		t.Fatal("update: bad zts must error")
	}
	if _, err := s.PrepareUpdateToken(good, "notanaddr", true, true); err == nil {
		t.Fatal("update: bad owner must error")
	}
}

func TestTokenTemplateTokenStandards(t *testing.T) {
	api := embedded.NewTokenApi(nil) // builders construct blocks from args/constants; no client deref
	zts := types.ZnnTokenStandard
	recv, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	amt := big.NewInt(123)

	issue := api.IssueToken("Tok", "TEST", "", big.NewInt(100), big.NewInt(100), 8, true, true, false)
	if issue.ToAddress != types.TokenContract || issue.TokenStandard != types.ZnnTokenStandard {
		t.Fatalf("issue: wrong to/zts: %+v", issue)
	}
	if issue.Amount.String() != constants.TokenIssueAmount.String() {
		t.Fatalf("issue amount=%v want %v", issue.Amount, constants.TokenIssueAmount)
	}

	mint := api.Mint(zts, amt, recv)
	if mint.ToAddress != types.TokenContract || mint.TokenStandard != types.ZnnTokenStandard || mint.Amount.Sign() != 0 {
		t.Fatalf("mint: wrong to/zts/amount: %+v", mint)
	}

	update := api.UpdateToken(zts, recv, true, true)
	if update.ToAddress != types.TokenContract || update.TokenStandard != types.ZnnTokenStandard || update.Amount.Sign() != 0 {
		t.Fatalf("update: wrong to/zts/amount: %+v", update)
	}

	// BURN is the dynamic one: zts = the token being burned, amount = the burn amount.
	burn := api.Burn(zts, amt)
	if burn.ToAddress != types.TokenContract {
		t.Fatalf("burn: wrong to: %+v", burn)
	}
	if burn.TokenStandard != zts {
		t.Fatalf("burn: TokenStandard=%v want the burned token %v", burn.TokenStandard, zts)
	}
	if burn.Amount.Cmp(amt) != 0 {
		t.Fatalf("burn: Amount=%v want %v", burn.Amount, amt)
	}
}
```
(Confirm `embedded.NewTokenApi(nil)`'s builders construct `*nom.AccountBlock` from args/constants only and don't deref the nil client — mirrors 5a–5d.)

- [ ] **Step 2: Run to verify failure**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareMintBurnUpdate|TestTokenTemplate' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement**

Add to `app/nom_service.go`:
```go
// PrepareMint builds a Mint template (owner-only on-chain). Inputs validated first.
func (s *NomService) PrepareMint(zts, amount, receiver string) (CallPreview, error) {
	parsedZts, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("mint amount must be greater than 0")
	}
	recv, err := types.ParseAddress(strings.TrimSpace(receiver))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid receiver: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.Mint(parsedZts, amt, recv)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Mint %s %s to %s", amt.String(), parsedZts.String(), recv.String()))
}

// PrepareBurn builds a Burn template. The burned token IS the block's token
// standard and the amount is carried by the block.
func (s *NomService) PrepareBurn(zts, amount string) (CallPreview, error) {
	parsedZts, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("burn amount must be greater than 0")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.Burn(parsedZts, amt)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: parsedZts, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Burn %s %s", amt.String(), parsedZts.String()))
}

// PrepareUpdateToken builds an UpdateToken template (transfer owner / one-way
// disable mint/burn). Inputs validated first.
func (s *NomService) PrepareUpdateToken(zts, newOwner string, isMintable, isBurnable bool) (CallPreview, error) {
	parsedZts, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	owner, err := types.ParseAddress(strings.TrimSpace(newOwner))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid owner: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.UpdateToken(parsedZts, owner, isMintable, isBurnable)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Update token %s", parsedZts.String()))
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `GOWORK=off go test ./app/ -run 'TestPrepareMintBurnUpdate|TestTokenTemplate' -v && GOWORK=off go build ./... && GOWORK=off go vet ./app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_service_test.go
git commit -m "feat(app): NomService Mint/Burn/Update token actions via prepareCall"
```

---

## Task 4: Bindings + token store + nav

**Files:** Modify `frontend/wailsjs/` (generated), `frontend/src/lib/stores/nav.ts`; Create `frontend/src/lib/stores/token.ts`.

**Interfaces:**
- Consumes: bound `NomService.GetMyTokens`/`GetTokenByZts`/`PrepareIssueToken`/`PrepareMint`/`PrepareBurn`/`PrepareUpdateToken`; generated `app.TokenInfo`.
- Produces: `token` store (`myTokens` + `lookedUpToken` writables) + `refreshTokens()` + `lookupToken(zts)`; `nav` `'tokens'` view.

- [ ] **Step 1: Regenerate bindings**

```bash
GOWORK=off "$(go env GOPATH)/bin/wails" generate module
git checkout -- frontend/wailsjs/runtime/ 2>/dev/null || true
ls frontend/wailsjs/go/app/NomService.d.ts   # GetMyTokens/GetTokenByZts/PrepareIssueToken/PrepareMint/PrepareBurn/PrepareUpdateToken present; TokenInfo in models.ts
```
Revert any `frontend/wailsjs/runtime/*` churn.

- [ ] **Step 2: Add the token store + nav view**

`frontend/src/lib/stores/token.ts`:
```ts
import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const myTokens = writable<app.TokenInfo[]>([])
export const lookedUpToken = writable<app.TokenInfo | null>(null)

export async function refreshTokens(): Promise<void> {
  try {
    myTokens.set(await Nom.GetMyTokens())
  } catch { /* not connected / locked — leave as-is */ }
}

export async function lookupToken(zts: string): Promise<void> {
  const t = await Nom.GetTokenByZts(zts)
  lookedUpToken.set(t && t.tokenStandard !== '' ? t : null)
}
```
Add `'tokens'` to the `View` union in `frontend/src/lib/stores/nav.ts` (currently `'dashboard' | 'send' | 'create' | 'import' | 'unlock' | 'settings' | 'plasma' | 'stake' | 'pillars' | 'sentinels'`).

- [ ] **Step 3: Build to verify**

Run: `cd frontend && pnpm run check && pnpm run build`
Expected: `svelte-check` 0 errors; clean build (run `rm -rf node_modules && pnpm install` first if node_modules is stale).

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs frontend/src/lib/stores/token.ts frontend/src/lib/stores/nav.ts
git commit -m "feat(frontend): token bindings + store + nav view"
```

---

## Task 5: Tokens route UI

**Files:** Create `frontend/src/routes/Tokens.svelte`, `frontend/src/routes/Tokens.test.ts`; Modify `frontend/src/routes/Dashboard.svelte`, `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: `token` store, `NomService.PrepareIssueToken`/`PrepareMint`/`PrepareBurn`/`PrepareUpdateToken`, the `tx` store + `awaitConfirm`/`TxModal`/`TxResult`, `formatAmount`, `nav`, `wallet` (active address for the mint-receiver prefill).
- Produces: Tokens route (my-tokens with mint/update + lookup/burn + issue form) + dashboard link + App route.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/Tokens.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'

const mocks = vi.hoisted(() => ({
  GetMyTokens: vi.fn(),
  GetTokenByZts: vi.fn(),
  PrepareIssueToken: vi.fn(), PrepareMint: vi.fn(), PrepareBurn: vi.fn(), PrepareUpdateToken: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NomService', () => mocks)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))
vi.mock('../lib/stores/wallet', () => ({ wallet: { subscribe: (fn: any) => { fn({ activeAddress: 'z1qme' }); return () => {} } } }))

import Tokens from './Tokens.svelte'

const TOK = { name: 'Alpha', symbol: 'ALPHA', domain: '', tokenStandard: 'zts1alpha', owner: 'z1qme', totalSupply: '100', maxSupply: '200', decimals: 0, isMintable: true, isBurnable: true, isUtility: false }

describe('Tokens', () => {
  it('lists my tokens with a Mint control for mintable tokens', async () => {
    mocks.GetMyTokens.mockResolvedValue([TOK])
    render(Tokens)
    expect(await screen.findByText(/ALPHA/)).toBeTruthy()
    expect(screen.getByRole('button', { name: /mint alpha/i })).toBeTruthy()
  })

  it('shows Burn only after a successful lookup of a burnable token', async () => {
    mocks.GetMyTokens.mockResolvedValue([])
    mocks.GetTokenByZts.mockResolvedValue({ ...TOK, owner: 'z1other' })
    render(Tokens)
    // no burn button before lookup
    expect(screen.queryByRole('button', { name: /^burn$/i })).toBeNull()
    const input = screen.getByLabelText('lookup zts') as HTMLInputElement
    await fireEvent.input(input, { target: { value: 'zts1alpha' } })
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }))
    expect(await screen.findByRole('button', { name: /^burn$/i })).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Tokens.test.ts`
Expected: FAIL — cannot resolve `./Tokens.svelte`.

- [ ] **Step 3: Implement**

READ `frontend/src/routes/Pillars.svelte` first and mirror its exact tx-flow wiring (`awaitConfirm`, the four `$tx.status` blocks, `TxModal`/`TxResult`, refresh-on-done). `frontend/src/routes/Tokens.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { myTokens, lookedUpToken, refreshTokens, lookupToken } from '../lib/stores/token'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { wallet } from '../lib/stores/wallet'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let error = ''
  let lookupZts = ''
  // issue form
  let iName = '', iSymbol = '', iDomain = '', iTotal = '', iMax = ''
  let iDecimals = 8, iMintable = true, iBurnable = true, iUtility = false
  // mint form (per token, keyed by zts)
  let mintZts = '', mintAmount = '', mintReceiver = ''
  // burn form
  let burnAmount = ''
  // update form
  let updZts = '', updOwner = '', updDisableMint = false, updDisableBurn = false

  onMount(refreshTokens)
  $: if ($tx.status === 'done') refreshTokens()

  function fail(e: any) { error = e?.message ?? String(e) }

  async function issue() {
    error = ''
    try { awaitConfirm((await Nom.PrepareIssueToken(iName, iSymbol, iDomain, iTotal, iMax, iDecimals, iMintable, iBurnable, iUtility)) as any) } catch (e) { fail(e) }
  }
  function startMint(zts: string) { mintZts = zts; mintAmount = ''; mintReceiver = $wallet.activeAddress ?? '' }
  async function mint() {
    error = ''
    try { awaitConfirm((await Nom.PrepareMint(mintZts, mintAmount, mintReceiver)) as any) } catch (e) { fail(e) }
  }
  async function doLookup() { error = ''; try { await lookupToken(lookupZts) } catch (e) { fail(e) } }
  async function burn(zts: string) {
    error = ''
    try { awaitConfirm((await Nom.PrepareBurn(zts, burnAmount)) as any) } catch (e) { fail(e) }
  }
  function startUpdate(zts: string, owner: string) { updZts = zts; updOwner = owner; updDisableMint = false; updDisableBurn = false }
  async function update(t: any) {
    error = ''
    try {
      awaitConfirm((await Nom.PrepareUpdateToken(updZts, updOwner, t.isMintable && !updDisableMint, t.isBurnable && !updDisableBurn)) as any)
    } catch (e) { fail(e) }
  }
</script>

<div class="mx-auto mt-8 w-[44rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Tokens</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">My tokens</h2>
    {#each $myTokens as t}
      <div class="border-b border-muted/20 py-2 text-sm space-y-1">
        <p class="font-mono">{t.symbol} · {t.name} · {formatAmount(t.totalSupply, t.decimals)}/{formatAmount(t.maxSupply, t.decimals)} · dec {t.decimals}{#if !t.isMintable} · fixed{/if}</p>
        <p class="text-xs text-muted">{t.tokenStandard}</p>
        <div class="flex flex-wrap gap-2 items-center">
          {#if t.isMintable}
            <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => startMint(t.tokenStandard)} aria-label={`mint ${t.symbol}`}>Mint</button>
          {/if}
          <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => startUpdate(t.tokenStandard, t.owner)} aria-label={`update ${t.symbol}`}>Update</button>
        </div>
        {#if mintZts === t.tokenStandard}
          <div class="flex flex-wrap gap-2 items-center pt-1">
            <input class="rounded bg-bg px-2 py-1 text-xs" placeholder="amount (base units)" bind:value={mintAmount} aria-label="mint amount" />
            <input class="rounded bg-bg px-2 py-1 text-xs w-72" placeholder="receiver" bind:value={mintReceiver} aria-label="mint receiver" />
            <button class="rounded bg-accent px-3 py-1 text-bg text-xs" on:click={mint}>Confirm mint</button>
          </div>
        {/if}
        {#if updZts === t.tokenStandard}
          <div class="flex flex-wrap gap-2 items-center pt-1">
            <input class="rounded bg-bg px-2 py-1 text-xs w-72" placeholder="new owner" bind:value={updOwner} aria-label="update owner" />
            {#if t.isMintable}<label class="text-xs"><input type="checkbox" bind:checked={updDisableMint} /> disable minting</label>{/if}
            {#if t.isBurnable}<label class="text-xs"><input type="checkbox" bind:checked={updDisableBurn} /> disable burning</label>{/if}
            <button class="rounded bg-accent px-3 py-1 text-bg text-xs" on:click={() => update(t)}>Confirm update</button>
          </div>
        {/if}
      </div>
    {/each}
    {#if $myTokens.length === 0}<p class="text-xs text-muted">No tokens owned.</p>{/if}
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Look up / burn</h2>
    <div class="flex gap-2">
      <input class="flex-1 rounded bg-bg px-3 py-2" placeholder="zts1…" bind:value={lookupZts} aria-label="lookup zts" />
      <button class="rounded border border-muted/40 px-3 py-1 text-xs" on:click={doLookup}>Look up</button>
    </div>
    {#if $lookedUpToken}
      <p class="text-sm font-mono">{$lookedUpToken.symbol} · {$lookedUpToken.name} · {$lookedUpToken.tokenStandard}</p>
      {#if $lookedUpToken.isBurnable}
        <div class="flex gap-2 items-center">
          <input class="rounded bg-bg px-2 py-1 text-xs" placeholder="amount (base units)" bind:value={burnAmount} aria-label="burn amount" />
          <button class="rounded bg-accent px-3 py-1 text-bg text-xs" on:click={() => burn($lookedUpToken.tokenStandard)}>Burn</button>
        </div>
      {:else}
        <p class="text-xs text-muted">Token is not burnable.</p>
      {/if}
    {/if}
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Issue a token (1 ZNN fee)</h2>
    <div class="grid grid-cols-2 gap-2">
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="name" bind:value={iName} aria-label="issue name" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="symbol (A-Z0-9)" bind:value={iSymbol} aria-label="issue symbol" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="domain (optional)" bind:value={iDomain} aria-label="issue domain" />
      <input class="rounded bg-bg px-2 py-1 text-sm" type="number" min="0" max="18" placeholder="decimals" bind:value={iDecimals} aria-label="issue decimals" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="total supply (base units)" bind:value={iTotal} aria-label="issue total" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="max supply (base units)" bind:value={iMax} aria-label="issue max" />
    </div>
    <div class="flex gap-4 text-xs">
      <label><input type="checkbox" bind:checked={iMintable} /> mintable</label>
      <label><input type="checkbox" bind:checked={iBurnable} /> burnable</label>
      <label><input type="checkbox" bind:checked={iUtility} /> utility</label>
    </div>
    <button class="rounded bg-accent px-3 py-1 text-bg" on:click={issue} aria-label="issue token">Issue token</button>
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
```
(If Pillars.svelte wires `TxModal`/`TxResult` or the `tx` status values differently, or the `wallet` store field for the active address is named differently than `activeAddress`, match the working code — read `frontend/src/lib/stores/wallet.ts` to confirm the field name and adjust the mint-receiver prefill.)

In `Dashboard.svelte` add a "Tokens" button → `view.set('tokens')` (next to the existing "Pillars"/"Sentinels" buttons). In `App.svelte` import `Tokens` and add an `{:else if $view === 'tokens'}<Tokens />` branch (next to the `'sentinels'` branch).

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run check && pnpm run build`
Expected: full suite PASS; `svelte-check` 0 errors; clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Tokens.svelte frontend/src/routes/Tokens.test.ts frontend/src/routes/Dashboard.svelte frontend/src/App.svelte
git commit -m "feat(frontend): Tokens route (my tokens mint/update + lookup/burn + issue)"
```

---

## Task 6: Verification + live smoke + acceptance

**Files:** Modify `internal/spike/readonly_integration_test.go`; Create `docs/phase5e-acceptance.md`.

- [ ] **Step 1: Add the live read smoke**

Add to `internal/spike/readonly_integration_test.go` (same `//go:build integration` tag; `rpc_client`, `types`, `os`, `testing` already imported):
```go
// TestReadOnlyTokens exercises the Phase-5e token read path against a live node
// (proves the embedded namespace + the exact TokenApi calls NomService uses).
// Read-only: no PoW, no signing.
//
// Env:
//   ZNN_NODE_URL  — ws:// or wss:// node URL (required)
//   ZNN_TEST_ADDR — a z1… address (required; reads its owned tokens)
func TestReadOnlyTokens(t *testing.T) {
	url := os.Getenv("ZNN_NODE_URL")
	addrStr := os.Getenv("ZNN_TEST_ADDR")
	if url == "" || addrStr == "" {
		t.Skip("set ZNN_NODE_URL and ZNN_TEST_ADDR to run")
	}
	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}
	defer client.Stop()
	addr := types.ParseAddressPanic(addrStr)

	owned, err := client.TokenApi.GetByOwner(addr, 0, 50)
	if err != nil {
		t.Fatalf("GetByOwner (embedded namespace enabled?): %v", err)
	}
	t.Logf("owned tokens: count=%d returned=%d", owned.Count, len(owned.List))
	for i, tok := range owned.List {
		if i >= 5 {
			break
		}
		t.Logf("  %s (%s) zts=%s supply=%v/%v mintable=%v burnable=%v", tok.Symbol, tok.Name, tok.TokenStandard, tok.TotalSupply, tok.MaxSupply, tok.IsMintable, tok.IsBurnable)
	}

	// GetByZts on a well-known token (ZNN) proves the single-token read path.
	znn, err := client.TokenApi.GetByZts(types.ZnnTokenStandard)
	if err != nil {
		t.Fatalf("GetByZts(ZNN): %v", err)
	}
	t.Logf("ZNN token: %s (%s) decimals=%d totalSupply=%v", znn.Symbol, znn.Name, znn.Decimals, znn.TotalSupply)
}
```

- [ ] **Step 2: Full automated verification**

```bash
find . -path ./.git -prune -o -path ./frontend/node_modules -prune -o -name '* 2.*' -print -exec rm -rf {} + 2>/dev/null
GOWORK=off go test ./...
GOWORK=off go build -tags integration ./...   # opt-in integration test still compiles
cd frontend && pnpm test && pnpm run check && pnpm run build && cd ..
xattr -cr build/bin 2>/dev/null; GOWORK=off "$(go env GOPATH)/bin/wails" build
```
Expected: backend green, frontend green (`svelte-check` 0 errors), app builds.

- [ ] **Step 3: Live read smoke (if a testnet node is available)**

```bash
ZNN_NODE_URL=ws://172.245.236.40:35998 ZNN_TEST_ADDR=z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg \
  GOWORK=off go test -tags integration ./internal/spike -run TestReadOnlyTokens -v -count=1
```
Expected: PASS; logs owned tokens (may be 0) + the ZNN token info. Record the real output.

- [ ] **Step 4: Manual acceptance (Phase 5e gate)**

On a testnet node **with the `embedded` namespace enabled** (Tokens route):
1. Open the Tokens route → see your owned tokens (or "No tokens owned").
2. Issue a token → TxModal shows "Issue token <SYMBOL>" (zts = ZNN, amount 1 ZNN fee) → Confirm → after a momentum the token appears in "My tokens".
3. Mint a mintable token to an address → TxModal shows the mint summary (zts = ZNN, amount 0) → Confirm → total supply increases.
4. Look up a token by ZTS → info renders; Burn (if burnable) → TxModal shows **zts = the token, amount = the burn amount** (NOT ZNN) → Confirm → supply decreases.
5. Update a token → transfer ownership and/or disable a flag → Confirm → change reflected.
6. Mainnet guard: with `AllowMainnetSend` false on a mainnet node, the actions are blocked.

- [ ] **Step 5: Record the result**

`docs/phase5e-acceptance.md`: automated results + live smoke output + the manual checks (issue/mint/burn/update, issue-validation, confirm-modal token-standard correctness — **Burn = token zts vs others = ZNN**, mainnet-gated), with testnet tx hashes where captured. Note the `embedded`-namespace node prerequisite (as in 5b–5d). Mirror the structure of `docs/phase5d-acceptance.md`.

- [ ] **Step 6: Commit**

```bash
git add internal/spike/readonly_integration_test.go docs/phase5e-acceptance.md
git commit -m "docs: Phase 5e acceptance record (+ live token read smoke)"
```

---

## Self-Review

**Spec coverage:** token reads + DTO + by-ZTS lookup + nil-supply mapping (T1); IssueToken + full on-chain field validation (T2); Mint/Burn/Update actions + input validation + per-call token-standard regression test incl. Burn's dynamic zts and Issue's 1 ZNN fee (T3); bindings + store + nav (T4); Tokens route UI with my-tokens (mint/update) + lookup/burn + issue form (T5); verification + live read smoke + manual acceptance (T6). All spec sections mapped.

**Placeholder scan:** No TBD/TODO in product code. All steps carry full code. Bindings regen is environment-run with the revert caution. The test ZTS-string fallback (T1) and the wallet-field confirmation note (T5) are explicit verify-against-reality instructions, not placeholders.

**Type consistency:** Go `TokenInfo` fields ↔ camelCase TS (`name/symbol/domain/tokenStandard/owner/totalSupply/maxSupply/decimals/isMintable/isBurnable/isUtility`). `PrepareIssueToken(name,symbol,domain,totalSupply,maxSupply,decimals,isMintable,isBurnable,isUtility)` / `PrepareMint(zts,amount,receiver)` / `PrepareBurn(zts,amount)` / `PrepareUpdateToken(zts,newOwner,isMintable,isBurnable)` match the TS store/route calls. `callExpect{to: TokenContract, zts: ZnnTokenStandard|the-burned-zts, amount, data}` matches the real SDK templates (T3 regression test locks token standards + Issue amount + Burn dynamic zts/amount). Reuses `prepareCall`/`assertMatches`/`awaitConfirm`/`formatAmount` from 5a–5d unchanged.

**Known follow-up (not 5e):** global all-tokens browser (`GetAll`); token-holdings list for burn discovery; Accelerator-Z (next sub-phase); deeper ABI `Data` semantic decode (Phase-5/7 hardening).
