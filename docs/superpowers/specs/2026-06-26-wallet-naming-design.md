# Wallet naming & management — design

**Date:** 2026-06-26
**Branch:** `wallet-naming` (off `main` `447f92f`; also carries the earlier app-logo/Unlock-branding commit `79f4c6e`, which this feature's Unlock restyle builds on).
**Type:** Backend (WalletService + a wallet manifest) + frontend (Unlock picker restyle, name fields, rename). Funds-adjacent (keystore handling) — keystore *content* stays syrius-byte-compatible; only the local filename + a local manifest change.

## Context / problem

Today a wallet's **name is literally its keystore filename**: `ListWallets` returns `e.Name()` (the file), `ImportKeystore` copies the source to `wallets/<source-filename>` (so two `wallet.json`s collide with "already exists"), and `Unlock`/`ChangePassword`/etc. key off that filename. There's no display name, no rename, and same-named files can't coexist. `wallet.json` / `pillar.json` are not intuitive names.

**Goal:** decouple a wallet's human **display name** (editable, renamable) from its on-disk **storage identity**, so names are friendly, renamable, and same-named keystore files can all be imported. Plus restyle the Unlock picker (an approved collapsible wallet combobox — see the mockups in `.superpowers/brainstorm/`).

## Design

### Storage identity: UUID filenames + a manifest

- New keystores are written under an opaque **UUID filename** (`<uuid>.dat`) in the wallets dir — never the source/display name. So nothing leaks public addresses via filenames, and same-named imports never collide.
- A **manifest** `wallets/wallets.json` is the source of truth for the wallet list + names. Shape (a list, order preserved for the UI):
  ```json
  { "wallets": [ { "id": "<keystore-filename>", "name": "Pillar wallet", "baseAddress": "z1qz372…u3d6g" } ] }
  ```
  - `id` = the keystore **filename** (the stable storage id used for unlock/operations). For new wallets it's `<uuid>.dat`; for pre-existing wallets it stays their original filename (migration, below).
  - `name` = the editable display name. `baseAddress` = cached so `ListWallets` needn't re-read every keystore (still reconciled with the dir).

### Backend (`app/wallet_service.go` + a small manifest module)

- **`WalletMeta`** DTO becomes `{ id, name, baseAddress }` (was `{ name, baseAddress }` where name==filename). The frontend displays `name`, operates by `id`.
- **Manifest load/save:** read/merge/write `wallets/wallets.json`. Tolerate a missing manifest (first run) and reconcile on every read.
- **`ListWallets()`** → read the manifest, **reconcile with the dir**: any keystore file present but not in the manifest is **registered** (id=filename, name=filename-stem, baseAddress read from the keystore) — this is the migration path; any manifest entry whose file is gone is dropped. Returns `[]WalletMeta` (id/name/baseAddress), manifest order.
- **`ImportKeystore(srcPath, name string)`** → validate it's a real syrius keystore, read its baseAddress, copy to a fresh **`<uuid>.dat`**, append a manifest entry (`name` = the provided display name, defaulting to the source filename stem if empty). **Duplicate detection:** if a keystore with the same baseAddress already exists, return a non-fatal signal/warning ("already imported as <name>") but still import (the user may want it).
- **`ImportMnemonic(name, password, mnemonic)`** / the Create flow's write → write the keystore to a fresh `<uuid>.dat`, append a manifest entry with `name`.
- **`Unlock(id, password)`**, **`ChangePassword(id, …)`**, **`RevealMnemonic`** → operate by `id` (the filename), via the existing `walletPath(id)` validation. `activeWallet` holds the id.
- **New `RenameWallet(id, newName string)`** → update the manifest entry's `name` (validate non-empty). No keystore access, **no password** (local metadata).
- **Account labels:** `Settings.AccountLabels` is keyed `"<wallet>:<index>"`. Switch the `<wallet>` key from the old filename to the wallet **id** (same value for migrated wallets, so existing labels survive). `CurrentAccounts`/`SelectAccount`/`SetAccountLabel` use the active wallet's id.

### Frontend (Vue)

- **Unlock picker → a collapsible wallet combobox** (`WalletPicker.vue`, the approved mockup): collapsed shows only the selected wallet (avatar initial + name + short address) + a chevron; expanding drops a panel listing all wallets, each **selectable** (✓) or **renamable** inline (✎ → text field → ✓/✕ → `RenameWallet(id, name)`). Replaces the native `<select>`. Keeps the Unlock-screen logo (from `79f4c6e`).
- **Create / Import** screens get a **name** field (display name, free text, no extension); Import defaults it to the source filename stem.
- **Settings** gets a **"Wallet name"** field for the currently-unlocked wallet (rename via `RenameWallet`).
- **`wallet` Pinia store:** `wallets: WalletMeta[]` (`{id,name,baseAddress}`); `active` becomes the active wallet **id**; `unlock(id, password)`, `rename(id, name)` actions; `loadWallets()` returns the new shape.
- Regenerate the Wails bindings for the changed DTO + the new `RenameWallet`.

### Data flow

Manifest is the wallet registry. ListWallets reconciles manifest↔dir (auto-registering legacy files). Create/Import write `<uuid>.dat` + a manifest entry. Unlock/ChangePassword/RevealMnemonic/account-ops key off the wallet `id`. Rename edits the manifest only.

## Migration (safe — no keystore moves)

Existing keystores are **left in place** (not renamed or moved — zero risk to funds). On the first `ListWallets` under the new code, each unregistered keystore file is added to the manifest with `id` = its current filename and `name` = the filename stem (e.g. `pillar.json` → name "pillar"). They keep working; the user can rename them. Only **new** wallets use UUID filenames. (Mixed ids — legacy filenames and new UUIDs — are fine; `id` is always just the keystore filename.)

## Funds-safety / compatibility

- Keystore **content** is unchanged → stays byte-compatible with syrius (a wallet created here still opens in syrius and vice-versa; only the local filename differs, which syrius treats as the wallet's name on its side).
- The manifest holds only **local, non-secret metadata** (display names + base addresses, which are public). No key material; nothing logged.
- No change to the crypto path (Argon2/AES keystore, BIP39/44 derivation, hashing, signing, PoW).
- Existing wallets keep working via the reconcile/migration; `Unlock`/`ChangePassword` re-validate inputs as today.

## Testing

- **Backend:** manifest load/save round-trip; `ListWallets` reconciles (registers a legacy file, drops a missing one); `ImportKeystore` writes a `<uuid>.dat` (not the source name) + manifest entry, and two same-named sources both import as distinct wallets; duplicate-baseAddress import returns the warning; `ImportMnemonic`/Create write `<uuid>.dat` + entry; `Unlock`/`ChangePassword`/`RevealMnemonic` work by id; `RenameWallet` updates the name (and rejects empty). Keep live-node bits behind `//go:build integration`.
- **Frontend:** `WalletPicker` collapses/expands, selects, and renames (calls `RenameWallet(id,name)`); Create/Import pass the name; Settings renames the current wallet; the store's new shape.
- **Gates:** `go test ./...` + vet, govulncheck/gosec (Go changed); `pnpm test`/`typecheck`/`build`; controller live `wails dev` — import the same `wallet.json` twice as two named wallets, rename one in the picker and in Settings, unlock, create, ensure existing wallets still unlock.

## Risks

- **Account-label key migration** — labels keyed by the old filename must map to the wallet id; for migrated wallets the id == the old filename, so they survive unchanged. Verify no label is orphaned.
- **Manifest/dir drift** — a keystore deleted outside the app, or a manifest hand-edit; the reconcile-on-read keeps them consistent (register unknown files, drop missing ids).
- **DTO change ripples** — `WalletMeta` gains `id`; every consumer (Unlock, AccountSwitcher, the store, Create/Import) must switch from name-as-id to the explicit `id`. Covered by the plan + tests.
- **Duplicate-base-address imports** — allowed but warned; the user could end up with two entries for the same wallet (acceptable; they named them).
