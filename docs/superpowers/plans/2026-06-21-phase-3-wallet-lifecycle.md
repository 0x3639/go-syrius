# Phase 3 — Wallet Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create new wallets (24-word, forced 3-word backup verify), import from mnemonic, change password, reveal mnemonic, and label accounts — producing syrius-compatible keystores.

**Architecture:** All keystore work uses go-zenon's `wallet` package (its `KeyStore` struct has exported fields; `Encrypt`/`Write`/`ReadKeyFile`/`Decrypt`/`DeriveForIndexPath` are public) plus BIP-39 directly — no custom crypto, no dependency forks. A single writer (`ImportMnemonic`) serves create + import. The mnemonic crosses to the Svelte frontend only via `GenerateMnemonic` (once) and password-gated `RevealMnemonic`.

**Tech Stack:** Go 1.24+, `github.com/zenon-network/go-zenon/wallet`, `github.com/tyler-smith/go-bip39`, Wails v2, Svelte + TS + Tailwind, Vitest.

## Global Constraints

- No SDK / no dependency forks. Keystore via go-zenon `wallet`; mnemonics via `go-bip39`. No custom crypto.
- New-wallet mnemonic: **24 words / 256-bit** (`bip39.NewEntropy(256)`).
- **Secret boundary:** the mnemonic reaches the WebView ONLY via `GenerateMnemonic` (once, at creation) and `RevealMnemonic(password)` (password-gated). No other method returns a key/seed/mnemonic. Never log secrets.
- Keystores written in go-zenon's canonical format (raw entropy, AES-256-GCM, AAD `"zenon"`, Argon2id `(1,64*1024,4,32)`, `argon2Params:{salt}`); acceptance = opens in real syrius.
- `ChangePassword` re-encrypt is atomic (write `<path>.tmp` then `os.Rename`); never corrupt the keystore.
- WalletService already has a `sync.Mutex mu` (Phase 2) — protect new shared state; avoid reentrant locking (use the existing `…Locked` helpers).
- Tests: offline `go test ./...` network-free; secrets never committed; temp data dirs.

## File structure

```
app/wallet_service.go     # MOD: GenerateMnemonic, ImportMnemonic(+writeKeystoreFromMnemonic), ChangePassword, RevealMnemonic, SetAccountLabel, activeWallet tracking, AccountInfo.Label
app/dto.go                # MOD: AccountInfo.Label; Settings.AccountLabels
app/config_service.go     # MOD: nil-safe AccountLabels default
app/wallet_service_test.go# MOD: unit tests
app/app.go                # (unchanged — Wallet already bound; new methods auto-exposed on regenerate)
frontend/wailsjs/...      # regenerated bindings
frontend/src/lib/stores/wallet.ts        # MOD: generateMnemonic/importMnemonic/changePassword/revealMnemonic/setLabel
frontend/src/lib/stores/nav.ts           # MOD: add 'create','import','settings' views
frontend/src/routes/Create.svelte        # NEW: 3-step wizard
frontend/src/routes/ImportMnemonic.svelte# NEW
frontend/src/routes/Settings.svelte      # NEW: change password + reveal mnemonic
frontend/src/routes/Unlock.svelte        # MOD: add Create / Import entry points
frontend/src/lib/components/AccountSwitcher.svelte # MOD: inline label edit
```

---

## Task 1: Create + import (GenerateMnemonic, ImportMnemonic)

**Files:** Modify `app/wallet_service.go`; Test `app/wallet_service_test.go`; Modify `go.mod` (promote bip39 to direct).

**Interfaces:**
- Consumes: `*ConfigService.walletsDir()`, go-zenon `wallet.KeyStore`/`Encrypt`/`Write`/`DeriveForIndexPath`/`Zero`, `bip39`.
- Produces: `(*WalletService) GenerateMnemonic() (string, error)`; `ImportMnemonic(name, password, mnemonic string) (WalletMeta, error)`; unexported `writeKeystoreFromMnemonic(name, password, mnemonic string) (WalletMeta, error)`.

- [ ] **Step 1: Write the failing tests**

Add to `app/wallet_service_test.go`:
```go
func TestGenerateMnemonic24Words(t *testing.T) {
	w := newTestWalletService(t)
	m, err := w.GenerateMnemonic()
	if err != nil {
		t.Fatalf("GenerateMnemonic: %v", err)
	}
	if n := len(strings.Fields(m)); n != 24 {
		t.Fatalf("expected 24 words, got %d", n)
	}
	m2, _ := w.GenerateMnemonic()
	if m == m2 {
		t.Fatal("expected distinct mnemonics")
	}
}

func TestImportMnemonicRoundTrip(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()

	meta, err := w.ImportMnemonic("created.dat", "pw123", m)
	if err != nil {
		t.Fatalf("ImportMnemonic: %v", err)
	}
	if !strings.HasPrefix(meta.BaseAddress, "z1") {
		t.Fatalf("bad baseAddress %q", meta.BaseAddress)
	}

	// The written keystore must open via go-zenon and derive the same address.
	if err := w.Unlock("created.dat", "pw123"); err != nil {
		t.Fatalf("Unlock created wallet: %v", err)
	}
	accts, err := w.CurrentAccounts()
	if err != nil || accts[0].Address != meta.BaseAddress {
		t.Fatalf("round-trip address mismatch: %v / %v", accts, err)
	}

	// Refuse overwrite; reject invalid mnemonic.
	if _, err := w.ImportMnemonic("created.dat", "pw123", m); err == nil {
		t.Fatal("expected overwrite to be refused")
	}
	if _, err := w.ImportMnemonic("bad.dat", "pw", "not a valid mnemonic phrase"); err == nil {
		t.Fatal("expected invalid mnemonic to be rejected")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestGenerateMnemonic|TestImportMnemonic' -v`
Expected: FAIL — `w.GenerateMnemonic undefined`.

- [ ] **Step 3: Implement**

Add to `app/wallet_service.go` imports `"github.com/tyler-smith/go-bip39"` and ensure `"os"`, `"path/filepath"`, `"strings"`, `"fmt"`, `"errors"` present. Add:
```go
// GenerateMnemonic returns a fresh 24-word (256-bit) BIP-39 mnemonic. It
// persists nothing — the create wizard shows it for backup before calling
// ImportMnemonic.
func (w *WalletService) GenerateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	return bip39.NewMnemonic(entropy)
}

// ImportMnemonic creates a keystore from a BIP-39 mnemonic (used for both
// "create new" after backup-verify and "import existing").
func (w *WalletService) ImportMnemonic(name, password, mnemonic string) (WalletMeta, error) {
	return w.writeKeystoreFromMnemonic(name, password, strings.TrimSpace(mnemonic))
}

// writeKeystoreFromMnemonic assembles a go-zenon KeyStore from the mnemonic and
// writes it as a syrius-compatible keyfile, refusing to overwrite an existing file.
func (w *WalletService) writeKeystoreFromMnemonic(name, password, mnemonic string) (WalletMeta, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return WalletMeta{}, errors.New("invalid mnemonic")
	}
	entropy, err := bip39.EntropyFromMnemonic(mnemonic)
	if err != nil {
		return WalletMeta{}, fmt.Errorf("invalid mnemonic: %w", err)
	}
	ks := &wallet.KeyStore{
		Entropy:  entropy,
		Seed:     bip39.NewSeed(mnemonic, ""),
		Mnemonic: mnemonic,
	}
	defer ks.Zero()
	_, kp, err := ks.DeriveForIndexPath(0)
	if err != nil {
		return WalletMeta{}, err
	}
	ks.BaseAddress = kp.Address

	dir, err := w.config.walletsDir()
	if err != nil {
		return WalletMeta{}, err
	}
	dst := filepath.Join(dir, name)
	if _, err := os.Stat(dst); err == nil {
		return WalletMeta{}, fmt.Errorf("a wallet named %q already exists", name)
	}
	kf, err := ks.Encrypt(password)
	if err != nil {
		return WalletMeta{}, err
	}
	kf.Path = dst
	if err := kf.Write(); err != nil {
		return WalletMeta{}, err
	}
	return WalletMeta{Name: name, BaseAddress: ks.BaseAddress.String()}, nil
}
```
(The existing file imports go-zenon `wallet` unaliased — reuse it. `bip39` is currently indirect; the next step promotes it.)

- [ ] **Step 4: Promote bip39 + run to verify pass**

Run: `go mod tidy && go test ./app/ -run 'TestGenerateMnemonic|TestImportMnemonic' -v && go build ./...`
Expected: PASS; `go.mod` now lists `github.com/tyler-smith/go-bip39` as direct.

- [ ] **Step 5: Commit**

```bash
git add app/wallet_service.go app/wallet_service_test.go go.mod go.sum
git commit -m "feat(app): create/import wallet from mnemonic (go-zenon canonical keystore)"
```

---

## Task 2: ChangePassword (atomic) + active-wallet tracking

**Files:** Modify `app/wallet_service.go`; Test `app/wallet_service_test.go`.

**Interfaces:**
- Consumes: `wallet.ReadKeyFile`/`Decrypt`/`Encrypt`/`Write`, `os.Rename`.
- Produces: `(*WalletService) ChangePassword(name, oldPassword, newPassword string) error`; unexported field `activeWallet string` set in `Unlock` (under `mu`), cleared in `Lock`.

- [ ] **Step 1: Write the failing test**

Add to `app/wallet_service_test.go`:
```go
func TestChangePassword(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	if _, err := w.ImportMnemonic("cp.dat", "old-pw", m); err != nil {
		t.Fatal(err)
	}

	if err := w.ChangePassword("cp.dat", "wrong", "new-pw"); err == nil {
		t.Fatal("expected wrong old password to fail")
	}
	if err := w.ChangePassword("cp.dat", "old-pw", "new-pw"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if err := w.Unlock("cp.dat", "old-pw"); err == nil {
		t.Fatal("old password should no longer work")
	}
	if err := w.Unlock("cp.dat", "new-pw"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestChangePassword -v`
Expected: FAIL — `w.ChangePassword undefined`.

- [ ] **Step 3: Implement**

In `app/wallet_service.go`: add `activeWallet string` to the struct. In `Unlock`, after the keystore is set (under `mu`), set `w.activeWallet = name`; in `Lock`, after zeroing, set `w.activeWallet = ""`. Add:
```go
// ChangePassword re-encrypts the named keystore under a new password, writing
// atomically (temp file + rename) so a failure never corrupts the original.
func (w *WalletService) ChangePassword(name, oldPassword, newPassword string) error {
	dir, err := w.config.walletsDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name)
	kf, err := wallet.ReadKeyFile(path)
	if err != nil {
		return fmt.Errorf("cannot read keystore: %w", err)
	}
	ks, err := kf.Decrypt(oldPassword)
	if err != nil {
		return errors.New("incorrect password")
	}
	defer ks.Zero()
	newKf, err := ks.Encrypt(newPassword)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	newKf.Path = tmp
	if err := newKf.Write(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run TestChangePassword -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/wallet_service.go app/wallet_service_test.go
git commit -m "feat(app): atomic ChangePassword + track active wallet name"
```

---

## Task 3: RevealMnemonic

**Files:** Modify `app/wallet_service.go`; Test `app/wallet_service_test.go`.

**Interfaces:**
- Consumes: `mu`, `keystore`, `activeWallet` (Task 2), `wallet.ReadKeyFile`/`Decrypt`.
- Produces: `(*WalletService) RevealMnemonic(password string) (string, error)`.

- [ ] **Step 1: Write the failing test**

Add to `app/wallet_service_test.go`:
```go
func TestRevealMnemonic(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	if _, err := w.ImportMnemonic("rv.dat", "pw", m); err != nil {
		t.Fatal(err)
	}

	if _, err := w.RevealMnemonic("pw"); err == nil {
		t.Fatal("expected RevealMnemonic to fail when locked")
	}
	if err := w.Unlock("rv.dat", "pw"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.RevealMnemonic("wrong"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
	got, err := w.RevealMnemonic("pw")
	if err != nil {
		t.Fatalf("RevealMnemonic: %v", err)
	}
	if got != m {
		t.Fatalf("revealed mnemonic mismatch")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestRevealMnemonic -v`
Expected: FAIL — `w.RevealMnemonic undefined`.

- [ ] **Step 3: Implement**

Add to `app/wallet_service.go`:
```go
// RevealMnemonic returns the active wallet's mnemonic after re-verifying the
// password against the keystore file. Requires an unlocked wallet. The mnemonic
// is never logged.
func (w *WalletService) RevealMnemonic(password string) (string, error) {
	w.mu.Lock()
	locked := w.keystore == nil
	name := w.activeWallet
	mnemonic := ""
	if w.keystore != nil {
		mnemonic = w.keystore.Mnemonic
	}
	w.mu.Unlock()
	if locked {
		return "", errLocked
	}
	dir, err := w.config.walletsDir()
	if err != nil {
		return "", err
	}
	kf, err := wallet.ReadKeyFile(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	ks, err := kf.Decrypt(password)
	if err != nil {
		return "", errors.New("incorrect password")
	}
	ks.Zero()
	return mnemonic, nil
}
```

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run TestRevealMnemonic -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/wallet_service.go app/wallet_service_test.go
git commit -m "feat(app): password-gated RevealMnemonic for the active wallet"
```

---

## Task 4: Account labels

**Files:** Modify `app/dto.go`, `app/wallet_service.go`; Test `app/wallet_service_test.go`.

**Interfaces:**
- Consumes: `*ConfigService.GetSettings/SetSettings`, `activeWallet`, `accountRange`.
- Produces: `Settings.AccountLabels map[string]string`; `AccountInfo.Label string`; `(*WalletService) SetAccountLabel(index int, label string) error`; `CurrentAccounts` populates `Label`.

- [ ] **Step 1: Write the failing test**

Add to `app/wallet_service_test.go`:
```go
func TestAccountLabels(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	if _, err := w.ImportMnemonic("lbl.dat", "pw", m); err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock("lbl.dat", "pw"); err != nil {
		t.Fatal(err)
	}

	if err := w.SetAccountLabel(0, "Savings"); err != nil {
		t.Fatalf("SetAccountLabel: %v", err)
	}
	accts, err := w.CurrentAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if accts[0].Label != "Savings" {
		t.Fatalf("label not applied: %+v", accts[0])
	}
	if err := w.SetAccountLabel(99, "x"); err == nil {
		t.Fatal("expected out-of-range index to fail")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestAccountLabels -v`
Expected: FAIL — `w.SetAccountLabel undefined` / `AccountInfo.Label` missing.

- [ ] **Step 3: Implement**

In `app/dto.go`: add `Label string \`json:"label"\`` to `AccountInfo`; add `AccountLabels map[string]string \`json:"accountLabels"\`` to `Settings`.

In `app/wallet_service.go`:
```go
func labelKey(wallet string, index int) string { return fmt.Sprintf("%s:%d", wallet, index) }

// SetAccountLabel persists a human label for the active wallet's account index.
func (w *WalletService) SetAccountLabel(index int, label string) error {
	if index < 0 || index >= accountRange {
		return fmt.Errorf("account index %d out of range", index)
	}
	w.mu.Lock()
	name := w.activeWallet
	w.mu.Unlock()
	if name == "" {
		return errLocked
	}
	s, err := w.config.GetSettings()
	if err != nil {
		return err
	}
	if s.AccountLabels == nil {
		s.AccountLabels = map[string]string{}
	}
	s.AccountLabels[labelKey(name, index)] = label
	return w.config.SetSettings(s)
}
```
Modify `CurrentAccounts` to set each `AccountInfo.Label` from settings: after building the list, load settings once and for each account `out[i].Label = s.AccountLabels[labelKey(w.activeWallet, i)]` (nil-safe map read returns ""). Read `w.activeWallet` under `mu` consistent with the existing locking.

- [ ] **Step 4: Run to verify pass + build**

Run: `go test ./app/ -run TestAccountLabels -v && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app/dto.go app/wallet_service.go app/wallet_service_test.go
git commit -m "feat(app): per-account labels persisted in settings"
```

---

## Task 5: Regenerate bindings

**Files:** Modify `frontend/wailsjs/` (generated).

**Interfaces:** Produces TS bindings for the new WalletService methods (GenerateMnemonic, ImportMnemonic, ChangePassword, RevealMnemonic, SetAccountLabel) and the `AccountInfo.label` / `Settings.accountLabels` models.

- [ ] **Step 1: Regenerate**

```bash
"$(go env GOPATH)/bin/wails" generate module
ls frontend/wailsjs/go/app   # WalletService.d.ts should list the new methods
```
If `wails generate module` alters unrelated `frontend/wailsjs/runtime/*` due to CLI-version skew, revert those (keep only the WalletService/models additions) — same caution as prior phases.

- [ ] **Step 2: Commit**

```bash
git add frontend/wailsjs
git commit -m "chore: regenerate TS bindings for wallet lifecycle methods"
```

---

## Task 6: Create wizard (frontend)

**Files:** Create `frontend/src/routes/Create.svelte`, `frontend/src/routes/Create.test.ts`; Modify `frontend/src/lib/stores/wallet.ts`, `frontend/src/lib/stores/nav.ts`.

**Interfaces:**
- Consumes: `WalletService.GenerateMnemonic`/`ImportMnemonic`, `unlock` store action.
- Produces: store actions `generateMnemonic()`/`importMnemonic(name,password,mnemonic)`; `Create.svelte` 3-step wizard; `nav` view `'create'`.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/Create.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/WalletService', () => ({
  GenerateMnemonic: vi.fn().mockResolvedValue('alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra tango uniform victor whiskey xray'),
  ImportMnemonic: vi.fn().mockResolvedValue({ name: 'w.dat', baseAddress: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg', label: '' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))

import Create from './Create.svelte'

describe('Create', () => {
  it('shows the generated mnemonic on step 1', async () => {
    render(Create)
    expect(await screen.findByText(/foxtrot/)).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Create.test.ts`
Expected: FAIL — cannot resolve `./Create.svelte`.

- [ ] **Step 3: Implement store actions + wizard**

Add to `frontend/src/lib/stores/wallet.ts`:
```ts
import * as W from '../../../wailsjs/go/app/WalletService'

export async function generateMnemonic(): Promise<string> {
  return (await W.GenerateMnemonic()) as string
}

export async function importMnemonic(name: string, password: string, mnemonic: string): Promise<void> {
  await W.ImportMnemonic(name, password, mnemonic)
}
```
(If `W` is already imported in wallet.ts, reuse it.)

Add `'create'` and `'import'` to the `View` union in `frontend/src/lib/stores/nav.ts`.

`frontend/src/routes/Create.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import { generateMnemonic, importMnemonic, unlock } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'

  let step = 1
  let mnemonic = ''
  let words: string[] = []
  let error = ''

  // backup-verify: 3 random positions
  let positions: number[] = []
  let answers: Record<number, string> = {}

  let name = ''
  let password = ''
  let confirm = ''

  onMount(async () => {
    try {
      mnemonic = await generateMnemonic()
      words = mnemonic.split(/\s+/)
      const idx = new Set<number>()
      while (idx.size < 3) idx.add(Math.floor((words.length) * (idx.size + 1) / 4)) // deterministic-ish spread
      positions = [...idx].sort((a, b) => a - b)
    } catch (e: any) { error = e?.message ?? String(e) }
  })

  $: verifyOk = positions.length === 3 && positions.every((p) => (answers[p] ?? '').trim() === words[p])
  $: canCreate = name.trim() !== '' && password.length > 0 && password === confirm

  async function finish() {
    error = ''
    try {
      await importMnemonic(name.endsWith('.dat') ? name : name + '.dat', password, mnemonic)
      await unlock(name.endsWith('.dat') ? name : name + '.dat', password)
    } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-12 w-[32rem] space-y-4">
  <h1 class="text-xl">Create wallet</h1>

  {#if step === 1}
    <p class="text-warn text-sm">Write these 24 words down and store them safely. Anyone with them controls your funds. They are shown only once.</p>
    <div class="grid grid-cols-3 gap-2 rounded bg-surface p-3 font-mono text-sm">
      {#each words as wd, i}<div><span class="text-muted">{i + 1}.</span> {wd}</div>{/each}
    </div>
    <button class="w-full rounded bg-accent py-2 text-bg" on:click={() => (step = 2)}>I've backed it up</button>
  {:else if step === 2}
    <p class="text-sm text-muted">Confirm your backup — enter these words:</p>
    {#each positions as p}
      <label class="block text-sm text-muted">Word #{p + 1}
        <input class="mt-1 w-full rounded bg-surface px-3 py-2 font-mono" bind:value={answers[p]} aria-label={`word ${p + 1}`} />
      </label>
    {/each}
    <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!verifyOk} on:click={() => (step = 3)}>Continue</button>
  {:else}
    <label class="block text-sm text-muted">Wallet name<input class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={name} aria-label="wallet name" /></label>
    <label class="block text-sm text-muted">Password<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={password} aria-label="password" /></label>
    <label class="block text-sm text-muted">Confirm password<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={confirm} aria-label="confirm password" /></label>
    <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!canCreate} on:click={finish}>Create wallet</button>
  {/if}

  <button class="text-xs text-muted" on:click={() => view.set('unlock')}>Cancel</button>
  {#if error}<p class="text-error" role="alert">{error}</p>{/if}
</div>
```
(If the `nav` store's default/unlock view differs, match its existing value names; the Cancel action returns to the unlock screen.)

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test src/routes/Create.test.ts && pnpm run build`
Expected: PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Create.svelte frontend/src/routes/Create.test.ts frontend/src/lib/stores/wallet.ts frontend/src/lib/stores/nav.ts
git commit -m "feat(frontend): create-wallet wizard with forced 3-word backup verify"
```

---

## Task 7: Import-mnemonic route + unlock entry points

**Files:** Create `frontend/src/routes/ImportMnemonic.svelte`, `frontend/src/routes/ImportMnemonic.test.ts`; Modify `frontend/src/routes/Unlock.svelte`, `frontend/src/App.svelte` (route to create/import).

**Interfaces:**
- Consumes: `importMnemonic` store action, `view` nav store.
- Produces: `ImportMnemonic.svelte`; Unlock screen buttons for "Create wallet" and "Import mnemonic"; App routes `'create'`/`'import'` views.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/ImportMnemonic.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ImportMnemonic: vi.fn().mockResolvedValue({ name: 'i.dat', baseAddress: 'z1q' }),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1q', label: '' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))
import ImportMnemonic from './ImportMnemonic.svelte'

describe('ImportMnemonic', () => {
  it('disables Import until name+password+mnemonic provided', async () => {
    render(ImportMnemonic)
    expect((screen.getByRole('button', { name: /^import$/i }) as HTMLButtonElement).disabled).toBe(true)
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/ImportMnemonic.test.ts`
Expected: FAIL — cannot resolve `./ImportMnemonic.svelte`.

- [ ] **Step 3: Implement**

`frontend/src/routes/ImportMnemonic.svelte`:
```svelte
<script lang="ts">
  import { importMnemonic, unlock } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'
  let mnemonic = ''
  let name = ''
  let password = ''
  let confirm = ''
  let error = ''
  $: wordCount = mnemonic.trim().split(/\s+/).filter(Boolean).length
  $: looksValid = wordCount === 12 || wordCount === 24
  $: canImport = looksValid && name.trim() !== '' && password.length > 0 && password === confirm
  async function doImport() {
    error = ''
    const file = name.endsWith('.dat') ? name : name + '.dat'
    try { await importMnemonic(file, password, mnemonic.trim()); await unlock(file, password) }
    catch (e: any) { error = e?.message ?? String(e) }
  }
</script>
<div class="mx-auto mt-12 w-[32rem] space-y-4">
  <h1 class="text-xl">Import from mnemonic</h1>
  <textarea class="w-full rounded bg-surface p-3 font-mono text-sm" rows="3" placeholder="word1 word2 …" bind:value={mnemonic} aria-label="mnemonic"></textarea>
  {#if mnemonic && !looksValid}<p class="text-xs text-error">Expected 12 or 24 words ({wordCount})</p>{/if}
  <label class="block text-sm text-muted">Wallet name<input class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={name} aria-label="wallet name" /></label>
  <label class="block text-sm text-muted">Password<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={password} aria-label="password" /></label>
  <label class="block text-sm text-muted">Confirm<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={confirm} aria-label="confirm password" /></label>
  <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!canImport} on:click={doImport} aria-label="Import">Import</button>
  <button class="text-xs text-muted" on:click={() => view.set('unlock')}>Cancel</button>
  {#if error}<p class="text-error" role="alert">{error}</p>{/if}
</div>
```

In `frontend/src/routes/Unlock.svelte`, add two buttons that set the nav view:
```svelte
<button class="w-full rounded border border-muted/40 py-2 text-muted" on:click={() => view.set('create')}>Create wallet</button>
<button class="w-full rounded border border-muted/40 py-2 text-muted" on:click={() => view.set('import')}>Import mnemonic</button>
```
(import `view` from `../lib/stores/nav` in Unlock.svelte.)

In `frontend/src/App.svelte`, extend the routing so `$view === 'create'` renders `Create`, `'import'` renders `ImportMnemonic`, while keeping the `$wallet.locked` gate (these are pre-unlock screens):
```svelte
{#if $wallet.locked}
  {#if $view === 'create'}<Create />
  {:else if $view === 'import'}<ImportMnemonic />
  {:else}<Unlock />{/if}
{:else}
  ...existing dashboard/send routing...
{/if}
```
(Match the existing App.svelte structure; import Create/ImportMnemonic.)

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/ImportMnemonic.svelte frontend/src/routes/ImportMnemonic.test.ts frontend/src/routes/Unlock.svelte frontend/src/App.svelte
git commit -m "feat(frontend): import-mnemonic route + create/import entry points"
```

---

## Task 8: Settings — change password + reveal mnemonic

**Files:** Create `frontend/src/routes/Settings.svelte`, `frontend/src/routes/Settings.test.ts`; Modify `frontend/src/lib/stores/wallet.ts`, `frontend/src/lib/stores/nav.ts`, `frontend/src/routes/Dashboard.svelte` (link to settings), `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: `WalletService.ChangePassword`/`RevealMnemonic`, `wallet` store (active wallet name), `view` nav.
- Produces: store actions `changePassword`/`revealMnemonic`; `Settings.svelte`; `'settings'` view; Dashboard "Settings" link.

- [ ] **Step 1: Write the failing test**

`frontend/src/routes/Settings.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ChangePassword: vi.fn().mockResolvedValue(undefined),
  RevealMnemonic: vi.fn().mockResolvedValue('alpha bravo charlie'),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))
import Settings from './Settings.svelte'
import { wallet } from '../lib/stores/wallet'

describe('Settings', () => {
  it('reveals the mnemonic after entering a password', async () => {
    wallet.set({ locked: false, walletName: 'w.dat', accounts: [], active: 0 } as any)
    render(Settings)
    await fireEvent.input(screen.getByLabelText(/reveal password/i), { target: { value: 'pw' } })
    await fireEvent.click(screen.getByRole('button', { name: /reveal/i }))
    expect(await screen.findByText(/alpha bravo charlie/)).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Settings.test.ts`
Expected: FAIL — cannot resolve `./Settings.svelte`.

- [ ] **Step 3: Implement**

Add to `frontend/src/lib/stores/wallet.ts`:
```ts
export async function changePassword(name: string, oldPassword: string, newPassword: string): Promise<void> {
  await W.ChangePassword(name, oldPassword, newPassword)
}
export async function revealMnemonic(password: string): Promise<string> {
  return (await W.RevealMnemonic(password)) as string
}
```
Add `'settings'` to the nav `View` union.

`frontend/src/routes/Settings.svelte`:
```svelte
<script lang="ts">
  import { wallet, changePassword, revealMnemonic } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'

  let oldP = '', newP = '', confirmP = '', cpMsg = '', cpErr = ''
  $: canChange = oldP.length > 0 && newP.length > 0 && newP === confirmP
  async function doChange() {
    cpMsg = ''; cpErr = ''
    try { await changePassword($wallet.walletName, oldP, newP); cpMsg = 'Password changed'; oldP = newP = confirmP = '' }
    catch (e: any) { cpErr = e?.message ?? String(e) }
  }

  let revealP = '', revealed = '', revErr = ''
  async function doReveal() {
    revErr = ''; revealed = ''
    try { revealed = await revealMnemonic(revealP) } catch (e: any) { revErr = e?.message ?? String(e) }
    revealP = ''
  }
  function hide() { revealed = '' }
</script>

<div class="mx-auto mt-8 w-[32rem] space-y-6">
  <div class="flex items-center justify-between"><h1 class="text-xl">Settings</h1>
    <button class="text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button></div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Change password</h2>
    <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="Current password" bind:value={oldP} aria-label="current password" />
    <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="New password" bind:value={newP} aria-label="new password" />
    <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="Confirm new password" bind:value={confirmP} aria-label="confirm new password" />
    <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-50" disabled={!canChange} on:click={doChange}>Change</button>
    {#if cpMsg}<p class="text-success text-sm">{cpMsg}</p>{/if}
    {#if cpErr}<p class="text-error text-sm" role="alert">{cpErr}</p>{/if}
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Reveal mnemonic</h2>
    <p class="text-warn text-xs">Anyone who sees these words controls your funds. Reveal only in private.</p>
    {#if revealed}
      <div class="rounded bg-bg p-3 font-mono text-sm break-words">{revealed}</div>
      <button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={hide}>Hide</button>
    {:else}
      <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="Password" bind:value={revealP} aria-label="reveal password" />
      <button class="rounded bg-accent px-3 py-1 text-bg" on:click={doReveal}>Reveal</button>
    {/if}
    {#if revErr}<p class="text-error text-sm" role="alert">{revErr}</p>{/if}
  </section>
</div>
```

In `Dashboard.svelte`, add a "Settings" button: `<button on:click={() => view.set('settings')}>Settings</button>` (import `view`). In `App.svelte` unlocked branch, route `$view === 'settings'` → `Settings`.

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/Settings.svelte frontend/src/routes/Settings.test.ts frontend/src/lib/stores/wallet.ts frontend/src/lib/stores/nav.ts frontend/src/routes/Dashboard.svelte frontend/src/App.svelte
git commit -m "feat(frontend): settings — change password + reveal mnemonic"
```

---

## Task 9: Account labels (frontend)

**Files:** Modify `frontend/src/lib/components/AccountSwitcher.svelte`, `frontend/src/lib/stores/wallet.ts`; Test `frontend/src/lib/components/AccountSwitcher.test.ts`.

**Interfaces:**
- Consumes: `WalletService.SetAccountLabel`, `wallet` store (`accounts[].label`, `select`, `refreshAccounts`).
- Produces: store action `setLabel(index, label)`; an editable label in the switcher.

- [ ] **Step 1: Write the failing test**

`frontend/src/lib/components/AccountSwitcher.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/WalletService', () => ({
  SelectAccount: vi.fn().mockResolvedValue(undefined),
  SetAccountLabel: vi.fn().mockResolvedValue(undefined),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1q', label: 'Savings' }]),
}))
import AccountSwitcher from './AccountSwitcher.svelte'
import { wallet } from '../stores/wallet'

describe('AccountSwitcher', () => {
  it('shows the account label', async () => {
    wallet.set({ locked: false, walletName: 'w.dat', active: 0,
      accounts: [{ index: 0, address: 'z1q', label: 'Savings' }] } as any)
    render(AccountSwitcher)
    expect(await screen.findByText(/Savings/)).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/lib/components/AccountSwitcher.test.ts`
Expected: FAIL (current switcher renders "Account N", not the label).

- [ ] **Step 3: Implement**

Add to `frontend/src/lib/stores/wallet.ts`:
```ts
export async function setLabel(index: number, label: string): Promise<void> {
  await W.SetAccountLabel(index, label)
  await refreshAccounts()
}
```
(`refreshAccounts` exists from Phase 1.)

Update `frontend/src/lib/components/AccountSwitcher.svelte` so each option shows the label when present, with an inline edit:
```svelte
<script lang="ts">
  import { wallet, select, setLabel } from '../stores/wallet'
  let editing = false
  let draft = ''
  function labelFor(a: { index: number; label?: string }) { return a.label && a.label.trim() ? a.label : `Account ${a.index}` }
  async function onChange(e: Event) { await select(Number((e.target as HTMLSelectElement).value)) }
  function startEdit() { draft = $wallet.accounts.find((a) => a.index === $wallet.active)?.label ?? ''; editing = true }
  async function saveEdit() { await setLabel($wallet.active, draft.trim()); editing = false }
</script>
<div class="flex items-center gap-2">
  <select class="rounded bg-surface px-2 py-1 text-sm" on:change={onChange} value={$wallet.active}>
    {#each $wallet.accounts as a}<option value={a.index}>{labelFor(a)}</option>{/each}
  </select>
  {#if editing}
    <input class="rounded bg-surface px-2 py-1 text-sm" bind:value={draft} aria-label="account label" />
    <button class="text-xs text-accent" on:click={saveEdit}>Save</button>
  {:else}
    <button class="text-xs text-muted" on:click={startEdit} aria-label="edit label">✎</button>
  {/if}
</div>
```

- [ ] **Step 4: Run to verify pass + build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: full suite PASS and clean build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/components/AccountSwitcher.svelte frontend/src/lib/components/AccountSwitcher.test.ts frontend/src/lib/stores/wallet.ts
git commit -m "feat(frontend): editable per-account labels in the account switcher"
```

---

## Task 10: Round-trip acceptance + verification

**Files:** Create `docs/phase3-roundtrip-acceptance.md`.

**Interfaces:** Produces the manual acceptance record (created-here opens in syrius).

- [ ] **Step 1: Full automated verification**

```bash
go test ./...                 # offline backend green incl. new wallet tests
cd frontend && pnpm test && pnpm run build && cd ..
"$(go env GOPATH)/bin/wails" build   # produces build/bin/syrius
```

- [ ] **Step 2: Manual round-trip acceptance (Phase 3 Gate)**

1. Run the app; from the unlock screen choose **Create wallet**; record the 24 words; pass the 3-word verify; set a name + password; land on the dashboard with correct (zero) balances.
2. Locate the written keystore under the app data dir (`os.UserConfigDir()/go-syrius/wallets/<name>.dat`).
3. **Open that keystore in real syrius** with the same password; confirm syrius shows the **same index-0 `z1…` address**. (This is the acceptance gate.)
4. Reverse direction is already covered (Phase 0 opened a syrius keystore here).
5. In the app: change the password, re-unlock with the new password; reveal the mnemonic (confirm it matches what was recorded); set an account label and confirm it persists across a lock/unlock.

- [ ] **Step 3: Record the result**

`docs/phase3-roundtrip-acceptance.md`: note the syrius version tested, the address shown in both apps (must match), and pass/fail for create-in-syrius-opens-here / created-here-opens-in-syrius / change-password / reveal / labels. If the created keystore does NOT open in syrius, capture the exact error and the two keystores' JSON field diff (esp. `argon2Params`) — that would indicate go-zenon's salt-only params need syrius's full set, to be addressed before Phase 3 closes.

- [ ] **Step 4: Commit**

```bash
git add docs/phase3-roundtrip-acceptance.md
git commit -m "docs: Phase 3 round-trip acceptance record"
```

---

## Self-Review

**Spec coverage:** GenerateMnemonic + ImportMnemonic (T1), ChangePassword + active-wallet tracking (T2), RevealMnemonic (T3), account labels backend (T4), bindings (T5), create wizard with 3-word backup verify (T6), import-mnemonic route + entry points (T7), settings change-password + reveal (T8), account labels frontend (T9), round-trip acceptance (T10). All spec sections map to a task.

**Placeholder scan:** No TBD/TODO. The Create wizard's word-position selection uses a deterministic spread (not `Math.random`, which is unavailable/forbidden in some contexts and would make tests flaky) — concrete code, not a placeholder. Bindings regeneration (T5) is environment-run with the exact command + the revert caution from prior phases.

**Type consistency:** `GenerateMnemonic`/`ImportMnemonic`/`ChangePassword`/`RevealMnemonic`/`SetAccountLabel` Go signatures match the TS store wrappers and component calls. `AccountInfo.label` (camelCase) matches the store/component `label` usage. `Settings.AccountLabels`/`accountLabels` consistent. `writeKeystoreFromMnemonic` is the single writer used by `ImportMnemonic`. `activeWallet` set in `Unlock` (T2) is consumed by `RevealMnemonic` (T3) and `SetAccountLabel` (T4).

**Known dependency (flagged):** the "created-here opens in syrius" gate (T10) depends on syrius accepting go-zenon's `argon2Params:{salt}`-only keystore; both use the same fixed Argon2 params so it should, but T10 Step 3 captures the field diff if not — the one residual compatibility risk, surfaced rather than assumed.
