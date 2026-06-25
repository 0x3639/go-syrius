# Vue migration — Sub-project B, Phase B4: Settings + Tokens + merge — design

**Date:** 2026-06-25
**Branch:** `frontend-vue-migration` (continues after B3, `5a3539f`) — **B4 ends with the merge to `main`.**
**Parent:** `docs/superpowers/specs/2026-06-24-frontend-vue-migration-design.md`. B4 is the **final phase** of sub-project B (and the migration).

## Context

B1–B3 delivered the lifecycle, Home, Send/Receive, and the 6 NoM panels. **B4 fills the last two placeholder routes** — **Settings** and **Tokens-management** — ports the merged Svelte `routes/Settings.svelte` + `routes/Tokens.svelte`, clears the carried-forward review minors, and then **merges the branch to `main`**, replacing the Svelte frontend. After B4 the Vue migration is done.

The Go backend + Wails bindings are unchanged (B4 stays **frontend-only** — the migration's invariant holds through the merge).

## Scope

**In:**
- `Settings.vue` (real, replaces the placeholder `/settings`): node configuration (mode remote/local/embedded + remote/local URLs, embedded data info + delete, sync status) + **reveal-mnemonic** (password-gated) + **change-password** + the auto-receive default toggle.
- `Tokens.vue` (real, replaces the placeholder `/tokens`): my tokens list + token lookup, and **issue / mint / burn / update** token operations (each through the confirm-what-you-sign flow).
- Store extensions: node-config actions on `useNodeStore`; `changePassword`/`revealMnemonic` on `useWalletStore`.
- **Cleanup carryovers** (A/B review minors): widen `tx.awaitConfirm`'s param type to the `CallPreview` superset (removes the ~17 `as never` casts in the panels + Tokens); fix the dead `option[value="Pillar-Two"]` query in `AcceleratorPanel.test.ts`.
- **The merge:** a final whole-branch review of the entire Vue migration, then merge `frontend-vue-migration` → `main` (replacing the Svelte frontend), and update CI if needed (already done in A — `vue-tsc`).

**Out (→ separate feature after merge):**
- **Network Configuration (Chain ID + Network ID + embedded-node testnet).** This is a **backend** change (add chain/network ID to `Settings`, thread into the tx/node path + a network-id concept, configure the embedded node's network/genesis). It gets its **own brainstorm→spec→plan→implement cycle** after the migration merges, to keep B4 frontend-only and the migration's "backend untouched" property intact. (Today: remote/local nodes already transact on whatever network the connected node reports — the chain id is derived from the frontier momentum; the embedded node still syncs mainnet, which the new feature will address.)

## Architecture

### Store extensions (`src/stores/`)

- **`useNodeStore`** (add, porting the Svelte `node.ts` helpers): `getConfig()` → `NodeService.GetNodeConfig()` (`{mode, remoteUrl, localUrl}`); `setMode(mode)` → `SetNodeMode`; `setUrl(mode, url)` → `SetNodeURL`; `getEmbeddedInfo()` → `GetEmbeddedInfo()` (`{running, dataDir, sizeBytes}`); `deleteEmbeddedData()` → `DeleteEmbeddedData`; `sync` state fed by the `node:sync` event (already wired in `initEvents`). Keep B2/B3 state (connect/initEvents/height).
- **`useWalletStore`** (add): `changePassword(old, new)` → `WalletService.ChangePassword(name, old, new)` (use the active wallet name); `revealMnemonic(password)` → `WalletService.RevealMnemonic(password)` (returns the mnemonic string; the view shows it then clears it). Funds-safety: the mnemonic surfaces only via this explicit, password-gated call (the binding-boundary invariant) and is cleared from view on hide.

### `Settings.vue` (faithful port of `routes/Settings.svelte`, nom-ui presentation)

Sections (the Svelte structure):
- **Node:** a mode selector (remote/local/embedded), editable remote/local URLs, an **Apply** action (`setUrl`/`setMode` only for the dirtied fields, with success/error messages); switching **to embedded** shows a confirm (it starts an in-process node); embedded mode shows the **data dir size + a Delete-data** action; sync status display.
- **Security:** **reveal-mnemonic** (password input → `revealMnemonic` → show the 24 words → hide/clear); **change-password** (old + new + confirm → `changePassword`).
- Back navigation to Home (`router.push('/home')`); errors surfaced inline. Use the local `Field.vue` + nom-ui `Input`/`Button`/`Select`.

### `Tokens.vue` (faithful port of `routes/Tokens.svelte`)

- **My tokens** (`useTokenStore().myTokens`, `refresh()` on mount) with per-token **Mint** (if mintable) / **Burn** / **Update** affordances; **lookup** a token by ZTS (`lookup(zts)` → `lookedUp`).
- **Issue** a new token (name/symbol/domain/total/max/decimals/mintable/burnable/utility).
- Each write goes `Nom.Prepare{IssueToken,Mint,Burn,UpdateToken}(...)` → `tx.awaitConfirm(preview)` → the **global NomConfirm** (built in B3) renders the confirm-what-you-sign modal → `ConfirmPublish`. Refresh tokens on `tx.status==='done'`. Reuses the exact NoM-confirm machinery — Tokens is a route (not a Home tab), so the global confirm must also render here (see wiring below).
- Amounts via `formatAmount`/`formatAmountExact`; the reserved-symbol rule (ZNN/QSR rejected) is enforced server-side (Go) — the form may also hint it, matching the Svelte.

### Global confirm on the Tokens route

The B3 `NomConfirm` lives in `Home.vue` (for the panel confirms). Tokens is a separate route whose issue/mint/burn/update ops also use `tx.awaitConfirm`, so it needs the confirm modal too. **Decision: mount a `NomConfirm` inside `Tokens.vue`** — NOT lift it to `App.vue`. Lifting to App and gating only on `tx.status` would double-render with `SendModal`'s inline TxModal (whose `sendOpen` flag lives in Home), and lifting `sendOpen`/`receiveOpen` to app scope is needless coupling. Home and Tokens are different routes and are never mounted together, so a `NomConfirm` in each is conflict-free; Tokens has no Send/Receive, so its `NomConfirm` needs no `!sendOpen && !receiveOpen` gating. Home's `NomConfirm` stays exactly as B3 left it.

## Funds-safety

- Token operations (issue/mint/burn/update) go through the **same confirm-what-you-sign** path as every other NoM call — `Prepare* → tx.awaitConfirm → TxModal (formatAmountExact) → ConfirmPublish`. No token op bypasses confirm.
- Reveal-mnemonic is the only path that surfaces the seed; it is password-gated (`RevealMnemonic`) and the value is cleared from view on hide. Never logged, never sent anywhere.
- Change-password goes through `WalletService.ChangePassword` (re-encrypts the keystore); the frontend never holds key material.

## Testing

Vitest + @vue/test-utils, mocking the bindings:
- **node store:** `setMode`/`setUrl`/`getEmbeddedInfo`/`deleteEmbeddedData` call the right bindings.
- **wallet store:** `revealMnemonic(pw)` returns the binding's mnemonic; `changePassword` calls `ChangePassword(name, old, new)`.
- **Settings.vue:** Apply calls the dirtied `setUrl`/`setMode`; reveal shows then hides the mnemonic; change-password validates match + calls the action; embedded-switch confirm.
- **Tokens.vue:** Issue calls `PrepareIssueToken(args)` → `tx.awaitConfirm`; Mint/Burn/Update likewise; lookup sets `lookedUp`.
- **App-level NomConfirm:** still renders the confirm for a panel/token-triggered tx; no double render with Send/Receive.
- Gates: `pnpm test`, `pnpm run typecheck`, `pnpm run build`; controller live `wails dev` gate — change node mode/URL, reveal a mnemonic, change a password, and issue/mint/burn a testnet token (each confirming via the modal).

## The merge (end of B4)

1. Full gate green + a **final whole-branch review** of the entire migration (`merge-base main HEAD`..HEAD), focused on funds-safety + parity.
2. Merge `frontend-vue-migration` → `main` (signed `--no-ff`), replacing the Svelte frontend. Push. Delete the branch.
3. CI already runs `vue-tsc`/vitest (set in A) — confirm green on the merge.

## Risks

- **NomConfirm on Tokens** — mount a second `NomConfirm` in `Tokens.vue` (Home keeps its own); since Home and Tokens are separate routes there's no double-render, and Tokens has no Send/Receive to gate against.
- **Settings node-mode apply logic** — the Svelte has careful dirty-tracking (only apply changed fields, embedded-switch confirm); port it faithfully (mis-applying node settings could disconnect the wallet).
- **nom-ui Select/Switch/Checkbox** for the Settings toggles/mode-picker — verify against `node_modules/nom-ui/src`; native fallback as elsewhere.
- **Merge** — the branch has ~50 commits; the final review reads the whole-migration diff. Ensure `main` is current (no divergence since `b515834`).
