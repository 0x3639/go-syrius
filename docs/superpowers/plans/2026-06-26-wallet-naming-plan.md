# Wallet Naming & Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Decouple a wallet's editable display name from its keystore filename — store new keystores under UUID filenames + a `wallets/wallets.json` manifest, add rename, allow same-named imports, and restyle the Unlock picker (the approved collapsible `WalletPicker`).

**Architecture:** A manifest (`wallets/wallets.json`) is the wallet registry (`id → {name, baseAddress}`, `id` = keystore filename). `ListWallets` reconciles the manifest with the dir (auto-registering legacy files — the safe migration). New keystores get `<uuid>.dat` filenames; `RenameWallet` edits the manifest. The frontend operates by `id`, shows `name`, and uses the collapsible `WalletPicker`.

**Tech Stack:** Go 1.25.11 (Wails v2); Vue 3 + Pinia + nom-ui.

## Global Constraints

- **Branch `wallet-naming`** (off `main` `447f92f`; carries the Unlock-branding commit `79f4c6e`). **DO NOT MERGE TO MAIN** — the user reviews the code first; the closeout pushes the branch for review.
- **Funds-safety:** keystore **content** is untouched (Argon2/AES keystore, BIP39/44 derivation, hashing, signing, PoW unchanged) — only the local filename + a local `wallets/wallets.json` manifest change. Keystores stay syrius-byte-compatible. Existing wallets must keep unlocking (migration). The manifest holds only display names + public base addresses (no key material).
- **`id` = the keystore filename** (the stable storage id). New wallets: `<uuid>.dat`. Legacy wallets: their existing filename (preserved by migration — no file is renamed/moved).
- **Mappings:** `WalletMeta` becomes `{ id, name, baseAddress }`. Frontend displays `name`, calls everything by `id`. `Settings.AccountLabels` keys (`"<wallet>:<index>"`) use the wallet **id** (== old filename for migrated wallets, so labels survive).
- Local Go: `GOWORK=off GOTOOLCHAIN=auto go test ./... && go vet ./...`; bindings regen `GOWORK=off ~/go/bin/wails generate module` (keep `models.ts`, revert any go.mod 2.12.0 churn). Frontend in `frontend/`: `pnpm test`/`typecheck`/`build`. Commits GPG-signed: implementers STAGE only.

## File Structure

- `app/wallet_manifest.go` (new) — manifest type + load/save/reconcile + `newWalletID()`.
- `app/dto.go` — `WalletMeta` gains `ID`.
- `app/wallet_service.go` — `ListWallets`/`ImportKeystore`/`writeKeystoreFromMnemonic` use uuid + manifest; new `RenameWallet`.
- `frontend/wailsjs/**` — regenerated.
- `frontend/src/components/WalletPicker.vue` (new) — the collapsible picker.
- `frontend/src/stores/wallet.ts`, `frontend/src/views/{Unlock,Create,ImportMnemonic,Settings}.vue` — new shape + name fields + rename.

---

## Task 1: Manifest module + `WalletMeta.ID` + `ListWallets` reconcile/migration

**Files:** Create `app/wallet_manifest.go`, `app/wallet_manifest_test.go`; Modify `app/dto.go`, `app/wallet_service.go` (`ListWallets`).

**Interfaces:**
- Produces: `WalletMeta{ ID, Name, BaseAddress }`; manifest helpers; `ListWallets()` returns reconciled `[]WalletMeta`.

- [ ] **Step 1: `WalletMeta` gains `ID`** in `app/dto.go`:
```go
type WalletMeta struct {
	ID          string `json:"id"`   // keystore filename (stable storage id)
	Name        string `json:"name"` // editable display name
	BaseAddress string `json:"baseAddress"`
}
```

- [ ] **Step 2: Write `app/wallet_manifest.go`** — the manifest + a uuid filename helper:
```go
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

const manifestFile = "wallets.json"

type walletManifest struct {
	Wallets []WalletMeta `json:"wallets"`
}

// newWalletID returns an opaque, collision-free keystore filename.
func newWalletID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ".dat", nil
}

func (w *WalletService) manifestPath() (string, error) {
	dir, err := w.config.walletsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, manifestFile), nil
}

func (w *WalletService) loadManifest() (walletManifest, error) {
	p, err := w.manifestPath()
	if err != nil {
		return walletManifest{}, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return walletManifest{Wallets: []WalletMeta{}}, nil
	}
	if err != nil {
		return walletManifest{}, err
	}
	var m walletManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return walletManifest{}, err
	}
	if m.Wallets == nil {
		m.Wallets = []WalletMeta{}
	}
	return m, nil
}

func (w *WalletService) saveManifest(m walletManifest) error {
	p, err := w.manifestPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p) // atomic
}
```

- [ ] **Step 3: Rewrite `ListWallets`** in `app/wallet_service.go` to reconcile manifest↔dir (the migration lives here). It loads the manifest, drops entries whose keystore is gone, registers keystore files not yet in the manifest (id=filename, name=filename without extension, baseAddress from `wallet.ReadKeyFile`), saves if changed, and returns the manifest order. Skip the manifest file itself when scanning. (Hold `w.mu` if the service has one; otherwise the manifest read-modify-write is serialized by the single Wails caller.)
```go
func (w *WalletService) ListWallets() ([]WalletMeta, error) {
	dir, err := w.config.walletsDir()
	if err != nil {
		return nil, err
	}
	m, err := w.loadManifest()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// Valid keystore filenames present on disk.
	files := map[string]string{} // filename -> baseAddress
	for _, e := range entries {
		if e.IsDir() || e.Name() == manifestFile {
			continue
		}
		kf, err := wallet.ReadKeyFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // not a keystore
		}
		files[e.Name()] = kf.BaseAddress.String()
	}
	changed := false
	// Drop manifest entries whose keystore is gone.
	kept := m.Wallets[:0]
	known := map[string]bool{}
	for _, e := range m.Wallets {
		if _, ok := files[e.ID]; ok {
			kept = append(kept, e)
			known[e.ID] = true
		} else {
			changed = true
		}
	}
	m.Wallets = kept
	// Register keystore files not yet in the manifest (migration of legacy files).
	for name, addr := range files {
		if known[name] {
			continue
		}
		display := name[:len(name)-len(filepath.Ext(name))] // filename stem
		m.Wallets = append(m.Wallets, WalletMeta{ID: name, Name: display, BaseAddress: addr})
		changed = true
	}
	if changed {
		if err := w.saveManifest(m); err != nil {
			return nil, err
		}
	}
	return m.Wallets, nil
}
```

- [ ] **Step 4: Tests** `app/wallet_manifest_test.go` — manifest load (missing file → empty), save→load round-trip; `ListWallets` against a temp wallets dir with a real (or fixture) keystore: a legacy file is registered (id=filename, name=stem); a manifest entry whose file is removed is dropped; `newWalletID()` returns distinct `*.dat` names. (Use the existing keystore test fixtures / the `internal/compat` real `.dat` if available, or write a keystore via the existing create path.)
- [ ] **Step 5: Verify** `GOWORK=off GOTOOLCHAIN=auto go test ./app/ && go vet ./app/` → pass. **Stage** dto.go, wallet_manifest.go, wallet_service.go, the test. No commit.

---

## Task 2: `ImportKeystore` (uuid/name/dup) + create/import write (uuid/name) + `RenameWallet`

**Files:** Modify `app/wallet_service.go`; test `app/wallet_service_test.go`; regenerate bindings.

**Interfaces:**
- `ImportKeystore(srcPath, name string) (WalletMeta, error)` — writes `<uuid>.dat`, manifest entry.
- `ImportMnemonic(name, password, mnemonic string) (WalletMeta, error)` — unchanged signature; now writes `<uuid>.dat` + manifest.
- `RenameWallet(id, newName string) error` — updates the manifest name.
- A duplicate-baseAddress signal on import.

- [ ] **Step 1: Rewrite `ImportKeystore(srcPath, name string)`** — copy the source to a fresh `<uuid>.dat` (ignore the source filename); the display `name` defaults to the source filename stem when empty; append a manifest entry; allow same-named imports (no name collision since the filename is a uuid). Duplicate detection: if the manifest already has an entry with the same baseAddress, still import but return a non-fatal indication (simplest: include the existing duplicate's name in an error-free path the frontend can detect, OR add `AlreadyImportedAs string` to the returned meta — **decision: the frontend warns** by checking the returned `baseAddress` against its existing list, so the backend just imports; document this).
```go
func (w *WalletService) ImportKeystore(srcPath, name string) (WalletMeta, error) {
	kf, err := wallet.ReadKeyFile(srcPath)
	if err != nil {
		return WalletMeta{}, fmt.Errorf("not a valid syrius keystore: %w", err)
	}
	if name == "" {
		base := filepath.Base(srcPath)
		name = base[:len(base)-len(filepath.Ext(base))]
	}
	id, err := newWalletID()
	if err != nil {
		return WalletMeta{}, err
	}
	dst, err := w.walletPath(id)
	if err != nil {
		return WalletMeta{}, err
	}
	if err := copyFile(srcPath, dst); err != nil {
		return WalletMeta{}, err
	}
	meta := WalletMeta{ID: id, Name: name, BaseAddress: kf.BaseAddress.String()}
	if err := w.addToManifest(meta); err != nil {
		return WalletMeta{}, err
	}
	return meta, nil
}
```
Add `addToManifest(meta WalletMeta) error` (load → append → save).

- [ ] **Step 2: `writeKeystoreFromMnemonic`** — generate `id` via `newWalletID()`, write the keystore to `walletPath(id)`, append the manifest entry `{id, name, baseAddress}`, drop the old "already exists" check (uuid can't collide). Return `WalletMeta{ID: id, Name: name, BaseAddress: ...}`. (Validation of name non-empty stays.)

- [ ] **Step 3: New `RenameWallet(id, newName string) error`** — validate `newName != ""`; load manifest; find the entry by `id`; set `Name = newName`; save. Error if `id` not found.
```go
func (w *WalletService) RenameWallet(id, newName string) error {
	if strings.TrimSpace(newName) == "" {
		return errors.New("wallet name must not be empty")
	}
	m, err := w.loadManifest()
	if err != nil {
		return err
	}
	for i := range m.Wallets {
		if m.Wallets[i].ID == id {
			m.Wallets[i].Name = strings.TrimSpace(newName)
			return w.saveManifest(m)
		}
	}
	return fmt.Errorf("wallet %q not found", id)
}
```

- [ ] **Step 4: Tests** — `ImportKeystore` twice from two same-named sources → two distinct `<uuid>.dat` files + two manifest entries (no collision); the written filename is NOT the source name. `ImportMnemonic` writes `<uuid>.dat` + a manifest entry with the given name. `RenameWallet` updates the name and rejects empty / unknown id. (Unlock by the returned `id` still works.)
- [ ] **Step 5: Regenerate bindings** — `GOWORK=off ~/go/bin/wails generate module` (updates `WalletMeta` + adds `ImportKeystore(srcPath,name)`/`RenameWallet` to `frontend/wailsjs/go/app/WalletService.*`). Revert any go.mod churn.
- [ ] **Step 6: Verify** `go test ./app/ && go vet ./app/` → pass. **Stage** wallet_service.go, test, frontend/wailsjs. No commit.

---

## Task 3: Frontend wallet store + `WalletPicker.vue` + wire into Unlock

**Files:** Modify `frontend/src/stores/wallet.ts`; Create `frontend/src/components/WalletPicker.vue` (+ test); Modify `frontend/src/views/Unlock.vue` (+ test).

**Interfaces:**
- `useWalletStore`: `wallets: WalletMeta[]` (`{id,name,baseAddress}`); state `active` becomes the active wallet **id**; `loadWallets()` (new shape), `unlock(id, password)`, `rename(id, name)` → `WalletService.RenameWallet`. Keep `accounts`/`activeIndex` (per-account).

- [ ] **Step 1: Update `wallet.ts`** — `WalletMeta = { id: string; name: string; baseAddress: string }`; `wallets: WalletMeta[]`; `loadWallets()` maps `W.ListWallets()` to the array (no `.name`-only mapping); `unlock(id, password)` → `W.Unlock(id, password)` (the id is the filename, the binding is unchanged) then load accounts; add `rename(id, name)` → `W.RenameWallet(id, name)` then `loadWallets()`. Update the existing `active` usage (it held the wallet name; now holds the id).
- [ ] **Step 2: Write `WalletPicker.vue`** — the approved collapsible combobox (mockup in `.superpowers/brainstorm/40031-…/content/unlock-wallet-collapsible.html`). Props: `modelValue` (selected id), `wallets: WalletMeta[]`. Emits `update:modelValue` (select), and calls the store `rename` on inline edit. Behavior: collapsed shows the selected wallet (avatar = first letter of name, name, short address via `shortAddress(baseAddress)`) + a chevron; clicking toggles a panel listing all wallets; a row click selects + collapses; the ✎ enters inline-rename (text field + ✓/✕ → `rename(id, value)`); clicking outside closes. Use the nom-ui theme classes (`bg-background`/`bg-card`/`text-foreground`/`text-muted-foreground`/`border-border`, accent=`primary`, avatar gradient `from-primary to-info`) — match the mockup's look. No password needed to rename.
- [ ] **Step 3: Wire into `Unlock.vue`** — replace the native `<select>` of wallet names with `<WalletPicker v-model="selected" :wallets="wallet.wallets" />` (`selected` = the chosen id; default to the first wallet's id on load). `doUnlock` calls `wallet.unlock(selected, password)`. Keep the logo + the Import/Create/Import-mnemonic buttons. `doImport` (keystore) → `wallet.pickKeystoreFile()` then `wallet.importKeystore(path, '')` (name defaults backend-side; or prompt — see Task 4).
- [ ] **Step 4: Tests** — `WalletPicker.test.ts`: collapsed shows the selected wallet; expanding shows all; clicking a row emits `update:modelValue` with that id; ✎ + typing + ✓ calls `rename(id, newName)`. `Unlock.test.ts` (update): unlock calls `Unlock(selectedId, password)`.
- [ ] **Step 5: Verify** `cd frontend && pnpm test -- "src/components/WalletPicker" "src/views/Unlock" && pnpm run typecheck`. **Stage** the 3 files + tests. No commit.

---

## Task 4: Name fields on Create/Import + Settings rename

**Files:** Modify `frontend/src/views/{Create,ImportMnemonic,Settings}.vue` (+ their tests).

- [ ] **Step 1: Create.vue / ImportMnemonic.vue** — these already collect a "wallet name" (the Svelte port used it as the filename). Now that name is the **display name** (free text, NO `.dat` suffix — drop the `fileName()`/`.dat` logic): pass the raw name to `wallet.importMnemonic(name, password, mnemonic)` (the backend assigns a uuid filename). Update the field label/placeholder to "Wallet name" if needed. For ImportMnemonic keep the 12/24-word validation.
- [ ] **Step 2: Settings.vue** — add a **"Wallet name"** field to the security section for the currently-unlocked wallet: load the current wallet's name (from `wallet.wallets.find(w => w.id === wallet.active)?.name`), an input + a Rename button → `wallet.rename(wallet.active, newName)` with a success/error message. (No password.)
- [ ] **Step 3: Tests** — Create/Import: finishing passes the typed name to `ImportMnemonic` (no `.dat`); Settings: Rename calls `wallet.rename(activeId, name)`.
- [ ] **Step 4: Verify** `cd frontend && pnpm test && pnpm run typecheck && pnpm run build` → all pass + clean. **Stage** the 3 views + tests. No commit.

---

## Task 5: Integration + full gate

- [ ] **Step 1: Backend** — `GOWORK=off GOTOOLCHAIN=auto go test ./... && go vet ./... && go build ./...` → pass.
- [ ] **Step 2: Security (Go changed)** — `bash scripts/govulncheck-gate.sh` + `gosec -conf .gosec.json ./...` → pass.
- [ ] **Step 3: Frontend** — `cd frontend && pnpm test && pnpm run typecheck && pnpm run build` → all pass + clean.
- [ ] **Step 4: Stage** any glue. No commit.

---

## Self-Review / Verification

- `go test ./...` + vet + govulncheck/gosec; frontend `pnpm test`/`typecheck`/`build` green.
- New keystores get `<uuid>.dat`; legacy keystores still unlock (migration registers them); two same-named imports coexist as distinct named wallets; `RenameWallet` works from the picker + Settings; keystore content unchanged (syrius-compatible).
- **Live `wails dev` gate (controller):** import the same `wallet.json` twice → two named wallets; rename one in the picker and one in Settings; create a named wallet; unlock an existing (migrated) wallet; verify the collapsible picker looks/behaves like the mockup.

## Closeout — NO auto-merge

After Task 5 green + a final review subagent: **DO NOT merge to main.** Push the `wallet-naming` branch to origin and hand the diff to the user for code review (per their request). Merge only after they approve. Update memory.
