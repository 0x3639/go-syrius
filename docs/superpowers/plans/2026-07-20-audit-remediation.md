# Security Audit Remediation (2026-07-20) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remediate audit findings GS-03, GS-04, GS-05, GS-06, GS-07, GS-10, GS-11, GS-12 plus the stale-doc fix from `docs/security-audit-2026-07-20.md` (the spec — read the finding's §8 entry before its task).

**Architecture:** Small, independent hardening changes: a generic bounded-pagination helper; routing every first-party NoM write through the existing `decodeContractCall` fail-closed effect decoder; manifest write atomicity + mutex; input bounds (decimals, URLs, decimal strings); CI supply-chain pinning. No new subsystems; every fix mirrors an existing in-repo pattern named in its task.

**Tech Stack:** Go (Wails v2 bound services), Vue 3 + TypeScript, vitest, go test, GitHub Actions.

## Global Constraints

- Every Go command MUST be prefixed `GOWORK=off GOTOOLCHAIN=auto`; frontend commands run in `frontend/` (pnpm 10.17.1).
- Spec: `docs/security-audit-2026-07-20.md`. Each task names its finding (GS-xx); the finding's **Remediation** and **Regression test** lines in §8 are binding requirements.
- Fail closed everywhere: a value that cannot be validated/decoded must produce an error, never a partial render or silent normalization.
- Do not change any signing, publishing, or callExpect semantics — GS-04 only *adds* decoded effects to previews; `assertMatches` and all `callExpect` literals stay byte-identical.
- Do not touch GS-01 (CSP), GS-02 (signing), GS-08, GS-09, GS-13 — explicitly out of this batch's scope.
- Commit messages end with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.

---

### Task 1: GS-03 — bounded pagination helper + convert the four unbounded loops

**Files:**
- Create: `app/paging.go`, `app/paging_test.go`
- Modify: `app/nom_service.go:340-356` (`GetPillarList` loop), `:866-881` (`GetMyTokens` loop); `app/nom_accelerator.go:432-446` (`GetVotableItems` loop), `:512-528` (`GetMyProjects` loop)

**Interfaces:**
- Consumes: nothing new.
- Produces: `func collectPaged[T any](fetch func(pageIndex uint32) (page []T, total int, err error)) ([]T, error)` with unexported `const maxPagedPages = 50`. Later tasks do not depend on it.

- [ ] **Step 1: Write the failing test**

Create `app/paging_test.go`:

```go
package app

import (
	"errors"
	"testing"
)

// A malicious node can return an inflated Count with endless non-empty pages
// (GS-03); collectPaged must stop at the hard cap instead of looping forever.
func TestCollectPaged_CapsMaliciousCount(t *testing.T) {
	calls := 0
	out, err := collectPaged(func(pageIndex uint32) ([]int, int, error) {
		calls++
		return []int{1, 2, 3}, 1 << 30, nil // always-full page, absurd total
	})
	if err != nil {
		t.Fatalf("capped collection must not error: %v", err)
	}
	if calls != maxPagedPages {
		t.Fatalf("must stop at the page cap: %d calls, want %d", calls, maxPagedPages)
	}
	if len(out) != maxPagedPages*3 {
		t.Fatalf("unexpected item count %d", len(out))
	}
}

func TestCollectPaged_NormalTermination(t *testing.T) {
	// Terminates when the claimed total is reached.
	pages := [][]int{{1, 2}, {3}}
	out, err := collectPaged(func(i uint32) ([]int, int, error) {
		if int(i) >= len(pages) {
			t.Fatal("fetched past the final page")
		}
		return pages[i], 3, nil
	})
	if err != nil || len(out) != 3 {
		t.Fatalf("got %v (err %v), want 3 items", out, err)
	}
	// Terminates on an empty page even when the claimed total is never reached.
	out, err = collectPaged(func(i uint32) ([]int, int, error) {
		if i == 0 {
			return []int{9}, 100, nil
		}
		return nil, 100, nil
	})
	if err != nil || len(out) != 1 {
		t.Fatalf("empty page must terminate: %v (err %v)", out, err)
	}
}

func TestCollectPaged_PropagatesError(t *testing.T) {
	boom := errors.New("rpc failed")
	if _, err := collectPaged(func(uint32) ([]int, int, error) { return nil, 0, boom }); !errors.Is(err, boom) {
		t.Fatalf("want fetch error, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run TestCollectPaged -v`
Expected: FAIL to compile — `undefined: collectPaged` / `undefined: maxPagedPages`.

- [ ] **Step 3: Implement the helper**

Create `app/paging.go`:

```go
package app

// maxPagedPages is the hard safety cap on node-driven pagination, mirroring
// SearchTokens' maxPages (nom_service.go). At the standard pageSize of 50 it
// admits 2500 items — far above any legitimate on-chain list.
const maxPagedPages = 50

// collectPaged pages through fetch until the node's claimed total is reached,
// an empty page arrives, or the hard maxPagedPages cap trips. The total AND
// the page contents are NODE-SUPPLIED and untrusted: without the cap, a
// malicious node returning an inflated total with endless non-empty pages
// drives the loop — and the wallet's memory — unbounded (audit GS-03).
// On cap, the items collected so far are returned (mirrors SearchTokens).
func collectPaged[T any](fetch func(pageIndex uint32) (page []T, total int, err error)) ([]T, error) {
	out := []T{}
	for pageIndex := uint32(0); pageIndex < maxPagedPages; pageIndex++ {
		page, total, err := fetch(pageIndex)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(out) >= total || len(page) == 0 {
			break
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run TestCollectPaged -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Convert the four loops**

Each conversion replaces the whole `for { ... }` block. The surrounding validation/client checks stay untouched.

`app/nom_service.go` `GetPillarList` (loop at :340-356; it appends `pillarSummaryDTO(p)` from `client.PillarApi.GetAll(pageIndex, pageSize)`):

```go
	raw, err := collectPaged(func(pageIndex uint32) ([]*embedded.PillarInfo, int, error) {
		list, err := client.PillarApi.GetAll(pageIndex, 50)
		if err != nil {
			return nil, 0, err
		}
		return list.List, list.Count, nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]PillarSummary, 0, len(raw))
	for _, p := range raw {
		out = append(out, pillarSummaryDTO(p))
	}
```

`app/nom_service.go` `GetMyTokens` (loop at :866-881, over `client.TokenApi.GetByOwner(addr, pageIndex, pageSize)` mapping `tokenInfoDTO`):

```go
	raw, err := collectPaged(func(pageIndex uint32) ([]*api.Token, int, error) {
		list, err := client.TokenApi.GetByOwner(addr, pageIndex, 50)
		if err != nil {
			return nil, 0, err
		}
		return list.List, list.Count, nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]TokenInfo, 0, len(raw))
	for _, t := range raw {
		out = append(out, tokenInfoDTO(t))
	}
```

(Use the actual element type of `list.List` — check the existing code's imports; if `GetByOwner` returns `*embedded.TokenList` with `List []*api.Token` or similar, name that type. Compile is the check.)

`app/nom_accelerator.go` `GetVotableItems` (loop at :432-446 collecting raw `list.List...` into `all`):

```go
	all, err := collectPaged(func(pageIndex uint32) ([]*embedded.Project, int, error) {
		list, err := client.AcceleratorApi.GetAll(pageIndex, 50)
		if err != nil {
			return nil, 0, err
		}
		return list.List, list.Count, nil
	})
	if err != nil {
		return nil, err
	}
```

(The post-processing over `all` is unchanged.)

`app/nom_accelerator.go` `GetMyProjects` (loop at :512-528; it filtered per page with `myActiveProjects(list.List, addr)` and tracked `seen`): collect raw pages with the same `collectPaged` call as `GetVotableItems`, then replace the loop's output with a single post-filter:

```go
	out := myActiveProjects(all, addr)
```

(`myActiveProjects` is a pure filter/map — verify by reading it; filtering once over the concatenated pages is equivalent to filtering per page.)

- [ ] **Step 6: Run the full package + vet**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ && GOWORK=off GOTOOLCHAIN=auto go vet ./app/`
Expected: PASS, vet clean. Verify no unbounded loops remain: `grep -n "pageIndex++" app/nom_service.go app/nom_accelerator.go` → only the `SearchTokens` bounded loop (if written with `pageIndex++`) or zero hits.

- [ ] **Step 7: Commit**

```bash
git add app/paging.go app/paging_test.go app/nom_service.go app/nom_accelerator.go
git commit -m "fix(security): bound node-driven pagination loops (GS-03)"
```

---

### Task 2: GS-05 — manifest write atomicity + mutex

**Files:**
- Modify: `app/wallet_manifest.go:60-74` (`saveManifest`); `app/wallet_service.go` (add `manifestMu` field; wrap the three load→mutate→save regions at `wallet_service.go:120-163`, `:205-210`, `:220-227`)
- Test: `app/wallet_manifest_test.go` (append or create)

**Interfaces:**
- Consumes: existing `loadManifest`/`saveManifest`/`manifestPath`, `newTestWalletService(t)` (wallet_service_test.go:48).
- Produces: nothing later tasks use.

- [ ] **Step 1: Write the failing test**

Find the three enclosing functions first: `grep -n "loadManifest" app/wallet_service.go` and read each enclosing function (they are the manifest read-modify-write callers, e.g. the `ListWallets` reconcile and `RenameWallet`). Then append to `app/wallet_manifest_test.go` (create the file with `package app` + imports if absent):

```go
// Two concurrent manifest read-modify-writes must both survive (GS-05: the
// fixed .tmp path + missing mutex could drop one), and no temp file may
// remain. -race does not see this file-level race; the assertion does.
func TestManifest_ConcurrentRenamesBothSurvive(t *testing.T) {
	w := newTestWalletService(t)
	seed := walletManifest{Wallets: map[string]string{"a.dat": "A", "b.dat": "B"}}
	if err := w.saveManifest(seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var wg sync.WaitGroup
	for _, r := range []struct{ id, name string }{{"a.dat", "A2"}, {"b.dat", "B2"}} {
		wg.Add(1)
		go func(id, name string) {
			defer wg.Done()
			if err := w.RenameWallet(id, name); err != nil {
				t.Errorf("RenameWallet(%s): %v", id, err)
			}
		}(r.id, r.name)
	}
	wg.Wait()
	m, err := w.loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Wallets["a.dat"] != "A2" || m.Wallets["b.dat"] != "B2" {
		t.Fatalf("a rename was lost: %+v", m.Wallets)
	}
	dir, _ := w.walletsDirOfManifest() // use whatever helper manifestPath's dir comes from
	tmps, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(tmps) != 0 {
		t.Fatalf("temp files left behind: %v", tmps)
	}
}
```

Adapt the seeding and the temp-dir lookup to the REAL `walletManifest` struct shape and `manifestPath()` (read `app/wallet_manifest.go:1-58` first — the struct/field names above are illustrative and MUST be corrected to the actual definitions; the assertions' substance is binding: both renames survive, zero `*.tmp` files). If `RenameWallet` validates that the keystore file exists, seed dummy `0600` files in the wallets dir first.

- [ ] **Step 2: Run test to verify it fails (or races)**

Run: `GOWORK=off GOTOOLCHAIN=auto go test -race ./app/ -run TestManifest_Concurrent -count=20 -v`
Expected: intermittent FAIL (lost rename) — file-level races are probabilistic; `-count=20` makes the loss overwhelmingly likely. If it happens to pass, proceed anyway (the fix is still mandated by the spec) but note it in the report.

- [ ] **Step 3: Implement**

Add to the `WalletService` struct (near `selMu`):

```go
	// manifestMu serializes every manifest read-modify-write. The manifest is
	// mutated from multiple bound methods (import/rename/list-reconcile) and an
	// unsynchronized load→mutate→save loses one writer's update (audit GS-05).
	manifestMu sync.Mutex
```

Replace `saveManifest` (`wallet_manifest.go:60-74`) with the unique-temp + fsync pattern of `setSettingsLocked` (`config_service.go:119-151` — read it and mirror exactly):

```go
func (w *WalletService) saveManifest(m walletManifest) error {
	p, err := w.manifestPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "manifest-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}
```

Then wrap each of the three load→mutate→save regions: add `w.manifestMu.Lock()` / `defer w.manifestMu.Unlock()` at the top of the enclosing function (or immediately before the `loadManifest` call if the function does unrelated work first). Lock-ordering note: `manifestMu` is a leaf — never call a method that takes `w.mu`/`w.selMu` while holding it; the manifest regions only do file I/O, so this holds naturally. Verify by reading each wrapped region.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test -race ./app/ -run 'TestManifest|TestImport|TestRename' -count=20 -v`
Expected: PASS consistently, no races.

- [ ] **Step 5: Commit**

```bash
git add app/wallet_manifest.go app/wallet_service.go app/wallet_manifest_test.go
git commit -m "fix(security): serialize + fsync wallet manifest writes (GS-05)"
```

---

### Task 3: GS-07 + GS-11 — node-input bounds (decimals, URL query/fragment, error scrubbing)

**Files:**
- Modify: `app/decimals.go:74-85` (`clientTokenDecimals`); `app/node_service.go:413-416` (`SetNodeURL` validation), `:93,:99,:108` (the three connect-error wraps)
- Test: `app/decimals_test.go` (append), `app/node_service_test.go` (append)

**Interfaces:**
- Consumes: existing `ztsDecimalsLookup`, `errTokenNotFound`, `defaultDecimals` (read `app/decimals.go:1-73`).
- Produces: `func boundTokenDecimals(d int, zts types.ZenonTokenStandard) (int, error)`; `func redactURLUserinfo(msg, rawURL string) string`.

- [ ] **Step 1: Write the failing tests**

Append to `app/decimals_test.go`:

```go
// GS-07: node-supplied token decimals must be bounded to the protocol range
// [0,18] (issuance enforces it on-chain; a lying node must not skew display).
func TestBoundTokenDecimals(t *testing.T) {
	zts := types.ZnnTokenStandard
	for _, ok := range []int{0, 8, 18} {
		if d, err := boundTokenDecimals(ok, zts); err != nil || d != ok {
			t.Fatalf("valid decimals %d rejected: %d, %v", ok, d, err)
		}
	}
	for _, bad := range []int{-1, 19, 200} {
		if _, err := boundTokenDecimals(bad, zts); err == nil {
			t.Fatalf("implausible decimals %d must be rejected", bad)
		}
	}
}
```

Append to `app/node_service_test.go`:

```go
// GS-11: node URLs must not persist query/fragment; userinfo stays allowed
// (legitimate basic-auth to the user's own node) but is scrubbed from errors.
func TestSetNodeURL_RejectsQueryAndFragment(t *testing.T) {
	n := newTestNode(t)
	for _, bad := range []string{"wss://h:35998?apikey=x", "wss://h:35998#frag", "ws://h:35998/path?a=b"} {
		if err := n.SetNodeURL("remote", bad); err == nil {
			t.Fatalf("url %q must be rejected", bad)
		}
	}
	if err := n.SetNodeURL("remote", "wss://user:pass@h:35998"); err != nil {
		t.Fatalf("basic-auth userinfo must remain allowed: %v", err)
	}
}

func TestRedactURLUserinfo(t *testing.T) {
	got := redactURLUserinfo("dial ws://user:pass@h:1/ failed", "ws://user:pass@h:1")
	if strings.Contains(got, "pass") {
		t.Fatalf("credentials leaked: %q", got)
	}
	if !strings.Contains(got, "***@") {
		t.Fatalf("redaction marker missing: %q", got)
	}
	// URLs without userinfo pass through untouched.
	if msg := redactURLUserinfo("connect refused", "wss://h:1"); msg != "connect refused" {
		t.Fatalf("no-userinfo message altered: %q", msg)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestBoundTokenDecimals|TestSetNodeURL_Rejects|TestRedactURLUserinfo' -v`
Expected: FAIL to compile — `undefined: boundTokenDecimals` / `undefined: redactURLUserinfo`; the SetNodeURL test fails on the query/fragment cases.

- [ ] **Step 3: Implement**

In `app/decimals.go`, add above `clientTokenDecimals` and use it inside:

```go
// boundTokenDecimals rejects node-reported decimals outside the protocol range
// [0,18] (issuance validates 0..18 on-chain, nom_service.go; a node reporting
// anything else is lying and must not skew the human-readable amount — GS-07).
func boundTokenDecimals(d int, zts types.ZenonTokenStandard) (int, error) {
	if d < 0 || d > 18 {
		return 0, fmt.Errorf("node reports implausible decimals %d for %s (valid range 0-18)", d, zts)
	}
	return d, nil
}
```

and change `clientTokenDecimals`'s final `return int(tok.Decimals), nil` to `return boundTokenDecimals(int(tok.Decimals), zts)`.

In `app/node_service.go`, extend `SetNodeURL` validation (after the existing scheme/host check at :413-416):

```go
	// Query strings and fragments have no websocket-RPC meaning and would
	// persist tokens/keys into settings.json; reject them. Userinfo is
	// deliberately ALLOWED — it is legitimate basic-auth to the user's own
	// node — but is scrubbed from any error surfaced to the frontend (GS-11).
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("node url must not contain a query string or fragment")
	}
```

Add the redaction helper (same file):

```go
// redactURLUserinfo scrubs URL credentials from an error message bound for the
// frontend: the websocket dial error text embeds the full URL, userinfo
// included (GS-11).
func redactURLUserinfo(msg, rawURL string) string {
	u, err := neturl.Parse(rawURL)
	if err != nil || u.User == nil {
		return msg
	}
	return strings.ReplaceAll(msg, u.User.String()+"@", "***@")
}
```

Change the three connect-error wraps (`node_service.go:93,:99,:108`) from `fmt.Errorf("connect: %w", err)` style to redacted form, e.g.:

```go
		return fmt.Errorf("connect: %s", redactURLUserinfo(err.Error(), url))
```

(likewise `node unreachable:` and `connect governance transport:`; the `%w` chain is intentionally dropped — these errors go straight to the frontend, and the wrapped error's text is exactly what must be scrubbed). Confirm `strings` is imported.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run 'TestBoundTokenDecimals|TestSetNodeURL|TestRedactURLUserinfo' -v && GOWORK=off GOTOOLCHAIN=auto go test ./app/`
Expected: targeted PASS; full package PASS (existing SetNodeURL tests must still pass — if one persists a URL with a query, that test documents the old bug and should be updated to expect rejection, noted in the report).

- [ ] **Step 5: Commit**

```bash
git add app/decimals.go app/decimals_test.go app/node_service.go app/node_service_test.go
git commit -m "fix(security): bound node decimals, reject URL query/fragment, scrub userinfo from errors (GS-07, GS-11)"
```

---

### Task 4: GS-12 — strict `toBase` decimal parsing

**Files:**
- Modify: `frontend/src/lib/format.ts:1-8` (`toBase`)
- Test: `frontend/src/lib/format.test.ts` (append)
- Check (read, update only if needed): the four callers — `frontend/src/views/Transfer.vue:26`, `frontend/src/components/panels/PlasmaPanel.vue:38`, `frontend/src/components/panels/StakingPanel.vue:33`, `frontend/src/components/panels/PillarLaunch.vue:118`

**Interfaces:** consumes/produces nothing cross-task.

- [ ] **Step 1: Write the failing tests** (in `frontend/`)

Append to `frontend/src/lib/format.test.ts`:

```ts
// GS-12: malformed decimal strings must be rejected, not silently normalized
// ('1.2.3' used to become 1.2; '-0.5' used to become positive 0.5).
describe('toBase strictness', () => {
  it('rejects multiple dots', () => {
    expect(() => toBase('1.2.3', 8)).toThrow()
  })
  it('rejects signs', () => {
    expect(() => toBase('-0.5', 8)).toThrow()
    expect(() => toBase('+1', 8)).toThrow()
  })
  it('rejects non-numeric garbage and empty strings', () => {
    expect(() => toBase('abc', 8)).toThrow()
    expect(() => toBase('', 8)).toThrow()
    expect(() => toBase('.', 8)).toThrow()
  })
  it('still accepts well-formed values', () => {
    expect(toBase('1.5', 8)).toBe('150000000')
    expect(toBase('.5', 8)).toBe('50000000')
    expect(toBase('7', 2)).toBe('700')
    expect(toBase(' 1.5 ', 8)).toBe('150000000') // trimmed
  })
})
```

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `pnpm test src/lib/format.test.ts`
Expected: the reject cases FAIL (no throw today); accept cases pass.

- [ ] **Step 3: Implement**

Replace `toBase` in `frontend/src/lib/format.ts`:

```ts
// toBase converts a decimal string to its base-unit integer string at `decimals`
// precision. Inverse of formatAmountExact; used to build the amount for
// tx.prepare. STRICT (GS-12): digits with at most one dot, no sign — anything
// else throws instead of silently normalizing ('1.2.3'→1.2, '-0.5'→0.5 were
// the old bugs). The backend re-validates authoritatively. Excess fractional
// digits beyond `decimals` are truncated (unchanged behavior).
export function toBase(decimal: string, decimals: number): string {
  const s = decimal.trim()
  if (!/^(\d+(\.\d*)?|\.\d+)$/.test(s)) {
    throw new Error(`invalid amount: ${decimal}`)
  }
  const [i, f = ''] = s.split('.')
  const frac = (f + '0'.repeat(decimals)).slice(0, decimals)
  return (BigInt(i || '0') * 10n ** BigInt(decimals) + BigInt(frac || '0')).toString()
}
```

- [ ] **Step 4: Verify the four callers tolerate a throw**

Read each caller listed above: each must invoke `toBase` inside a code path whose rejection surfaces as a form error rather than an unhandled rejection (the tx stores' `prepare`/`awaitConfirm` flows have error handling; confirm the call is inside the same `try`/error-handled path). If any caller would let the throw escape unhandled, wrap that call site minimally (set the panel's existing error ref). Record what you found per caller in the report.

- [ ] **Step 5: Run tests + typecheck**

Run: `pnpm test src/lib/format.test.ts && pnpm run typecheck && pnpm test`
Expected: all green (full suite guards the caller behavior).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/format.ts frontend/src/lib/format.test.ts
git commit -m "fix(security): reject malformed decimal input in toBase (GS-12)"
```

(add any caller files you had to touch.)

---

### Task 5: GS-06 + GS-10 + doc hygiene — pin Actions/scanners, fix stale SDK version

**Files:**
- Modify: `.github/workflows/ci.yml` (uses: lines 25,26,29,41,42,66,67,86,87,90,93; scanner line 52), `.github/workflows/release.yml` (uses: lines 36,46,49,52,90,101,112)
- Modify: `AGENTS.md:18`, `CLAUDE.md:18`

**Interfaces:** none.

- [ ] **Step 1: Resolve each action tag to its current commit SHA**

For each distinct action (`actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-node@v4`, `actions/upload-artifact@v4`, `actions/download-artifact@v4`, `pnpm/action-setup@v4`, `softprops/action-gh-release@v2`):

```bash
gh api repos/<owner>/<repo>/commits/<tag> --jq .sha
```

(e.g. `gh api repos/actions/checkout/commits/v4 --jq .sha`). Record each SHA.

- [ ] **Step 2: Rewrite every `uses:` line to the SHA with a version comment**

Format (keep alignment/indentation):

```yaml
      - uses: actions/checkout@<full-sha> # v4
```

Apply to ALL listed lines in both workflows. Then verify none remain: `grep -n "uses: .*@v[0-9]" .github/workflows/*.yml` → zero hits.

- [ ] **Step 3: Pin govulncheck**

Resolve the latest release: `gh api repos/golang/vuln/tags --jq '.[0].name'` (or `go list -m -versions golang.org/x/vuln` and take the newest). Change `ci.yml:52` from `golang.org/x/vuln/cmd/govulncheck@latest` to that exact version, with a comment:

```yaml
          # Pinned (GS-10): the vuln gate must not depend on whatever @latest
          # resolves to at run time. go-install version pins are immutable via
          # the Go module checksum DB (which is why gosec@v2.27.1 and
          # wails@v2.12.0 need no change).
          go install golang.org/x/vuln/cmd/govulncheck@<resolved-version>
```

- [ ] **Step 4: Fix the stale doc version**

In both `AGENTS.md:18` and `CLAUDE.md:18`, change `currently v0.1.19` to `currently v0.2.1` (verify against `go.mod` first: `grep znn-sdk-go go.mod`).

- [ ] **Step 5: Validate workflow syntax**

Run: `command -v actionlint >/dev/null && actionlint .github/workflows/*.yml || echo "actionlint not installed — rely on a YAML parse"`; at minimum `python3 -c "import yaml,glob; [yaml.safe_load(open(f)) for f in glob.glob('.github/workflows/*.yml')]" && echo YAML_OK`.
Expected: no errors / YAML_OK.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml AGENTS.md CLAUDE.md
git commit -m "ci(security): pin Actions and scanners to immutable refs; fix stale SDK version (GS-06, GS-10)"
```

---

### Task 6: GS-04 — decoded effects for every first-party NoM write

**Files:**
- Modify: `app/nom_service.go` (21 `s.tx.prepareCall(` sites: lines 134, 155, 259, 275, 397, 409, 422, 575, 588, 617, 656, 673, 743, 757, 769, 782, 795, 1012, 1057, 1078, 1108), `app/nom_accelerator.go` (2 sites: lines 268, 294)
- Test: `app/tx_effect_test.go` (append)

**Interfaces:**
- Consumes: `decodeContractCall(destination types.Address, data []byte) (*TransactionEffect, error)` (`tx_effect.go:44`, fails closed); `prepareCallWithEffect(template, expect, summary, effect)` (`tx_service.go:449`); the existing model conversion `PrepareCollectReward` (`nom_service.go:283-296`) and test pattern `TestDecodeAcceleratorTemplates`/`assertEffectHasValues` (`tx_effect_test.go:200-253`).
- Produces: after this task, `grep -c "s.tx.prepareCall(" app/` MUST be zero — every NoM write carries a block-decoded effect.

- [ ] **Step 1: Write the failing regression test**

Append to `app/tx_effect_test.go` — the priority templates from the audit (Mint, UpdateToken, RegisterPillar/UpdatePillar, IssueToken, Fuse), built with the SDK's pure template builders exactly as `TestDecodeAcceleratorTemplates` does (`embedded.NewXxxApi(nil)`); check the SDK package for the constructor names (`NewPlasmaApi`, `NewPillarApi`, `NewTokenApi` — mirror how `NewAcceleratorApi` is used at :201):

```go
// TestDecodeNomWriteTemplates decodes the exact templates the first-party NoM
// prepare paths hold, proving the MATERIAL ABI parameters (mint receiver,
// token owner, pillar producer/reward addresses) surface from the signed
// bytes, not just a form-derived summary (audit GS-04).
func TestDecodeNomWriteTemplates(t *testing.T) {
	addrA := "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz" // any valid z1 fixture already used in this package
	a, err := types.ParseAddress(addrA)
	if err != nil {
		t.Fatal(err)
	}
	zts := types.ZnnTokenStandard

	t.Run("Plasma.Fuse", func(t *testing.T) {
		tmpl := embedded.NewPlasmaApi(nil).Fuse(a, mustBig(t, "5000000000"))
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, addrA)
	})
	t.Run("Pillar.Register", func(t *testing.T) {
		tmpl := embedded.NewPillarApi(nil).Register("MyPillar", a, a, 10, 90)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "MyPillar", addrA)
	})
	t.Run("Pillar.UpdatePillar", func(t *testing.T) {
		tmpl := embedded.NewPillarApi(nil).UpdatePillar("MyPillar", a, a, 10, 90)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "MyPillar", addrA)
	})
	t.Run("Token.IssueToken", func(t *testing.T) {
		tmpl := embedded.NewTokenApi(nil).IssueToken("My Token", "MYT", "example.com", mustBig(t, "1000"), mustBig(t, "2000"), 8, true, true, false)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "My Token", "MYT", "1000", "2000")
	})
	t.Run("Token.Mint", func(t *testing.T) {
		tmpl := embedded.NewTokenApi(nil).Mint(zts, mustBig(t, "777"), a)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "777", addrA)
	})
	t.Run("Token.UpdateToken", func(t *testing.T) {
		tmpl := embedded.NewTokenApi(nil).UpdateToken(zts, a, true, false)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, addrA)
	})
}
```

Adjust constructor/method names to the real SDK API (check imports at the top of `nom_service.go` / how `client.PlasmaApi.Fuse` is typed; the `embedded.NewXxxApi(nil)` pattern is proven at `tx_effect_test.go:201`). This test does not require any Task-6 code change to pass — it proves the decoder handles every priority template (a decode failure here is a blocking finding: report it, do not paper over it).

- [ ] **Step 2: Run the new test**

Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ -run TestDecodeNomWriteTemplates -v`
Expected: PASS (the decoder + ABI map already cover all five contracts). If any subtest fails to decode, STOP and report — that method cannot be converted and the audit's GS-04 remediation needs a design decision.

- [ ] **Step 3: Convert all 23 sites**

Mechanical, identical transformation at every listed site — model: `PrepareCollectReward` (`nom_service.go:283-296`). At each site, after the `template := client.XxxApi.Yyy(...)` line insert:

```go
	effect, err := decodeContractCall(template.ToAddress, template.Data)
	if err != nil {
		return CallPreview{}, fmt.Errorf("cannot render the exact contract call: %w", err)
	}
```

and change `return s.tx.prepareCall(template, callExpect{...}, <summary>)` to `return s.tx.prepareCallWithEffect(template, callExpect{...}, <summary>, effect)`.

RULES: (a) the `callExpect{...}` literal and the summary string stay byte-identical — do not "improve" them; (b) if the enclosing function already binds `err`, use `effect, derr :=` and adjust; (c) keep existing comments. Full example for the `Fuse` site (`nom_service.go:130-135`):

```go
	template := client.PlasmaApi.Fuse(addr, amt)
	effect, err := decodeContractCall(template.ToAddress, template.Data)
	if err != nil {
		return CallPreview{}, fmt.Errorf("cannot render the exact contract call: %w", err)
	}
	// The callExpect zts MUST match the SDK template's TokenStandard or
	// TxService.ConfirmPublish's assertMatches rejects the block. The SDK's
	// PlasmaApi.Fuse builds the block with TokenStandard: types.QsrTokenStandard.
	return s.tx.prepareCallWithEffect(template, callExpect{to: types.PlasmaContract, zts: types.QsrTokenStandard, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Fuse %s QSR for %s", formatBaseAmount(qsrAmount, 8), beneficiary), effect)
```

- [ ] **Step 4: Verify completeness + run the full suites**

Run: `grep -c "s.tx.prepareCall(" app/nom_service.go app/nom_accelerator.go` → expected `0` and `0`.
Run: `GOWORK=off GOTOOLCHAIN=auto go test ./app/ && GOWORK=off GOTOOLCHAIN=auto go vet ./app/`
Expected: PASS. Existing prepare-path tests (validation, not-connected) are unaffected because the decode runs after the client check and only on success paths those tests never reach.
Then frontend (in `frontend/`): `pnpm test` — expected all green (`CallPreview.Effect` is an existing optional field; `NomConfirm` already renders effects when present).

- [ ] **Step 5: Commit**

```bash
git add app/nom_service.go app/nom_accelerator.go app/tx_effect_test.go
git commit -m "fix(security): render every NoM write confirmation from the decoded built block (GS-04)"
```

---

### Task 7: Full verification gates

**Files:** none (fix-forward only).

- [ ] **Step 1: Backend** — `GOWORK=off GOTOOLCHAIN=auto go build ./... && GOWORK=off GOTOOLCHAIN=auto go vet ./... && GOWORK=off GOTOOLCHAIN=auto go test -race ./...` → all PASS (known gopsutil/IOKit warning only).
- [ ] **Step 2: Security gates** — `export PATH="$PATH:$(go env GOPATH)/bin"; GOWORK=off GOTOOLCHAIN=auto "$(go env GOPATH)/bin/gosec" -conf .gosec.json ./...` → 0 issues; `GOWORK=off GOTOOLCHAIN=auto bash scripts/govulncheck-gate.sh` → OK (allowlisted only).
- [ ] **Step 3: Frontend** (in `frontend/`) — `pnpm run typecheck && pnpm test && pnpm run build` → clean/green.
- [ ] **Step 4: Straggler check** — `git status --short` clean apart from untracked `.agents/`, `.claude/`.
