# Vue B4 — Settings + Tokens + Merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Port the Settings and Tokens-management routes to Vue, clear the carried-forward review minors, then merge the Vue migration to `main` (replacing the Svelte frontend).

**Architecture:** `Settings.vue`/`Tokens.vue` replace the placeholder routes; node-config + `changePassword`/`revealMnemonic` actions extend the existing Pinia stores; token ops reuse the B3 confirm-what-you-sign flow (`Nom.Prepare* → tx.awaitConfirm → NomConfirm → ConfirmPublish`). Tokens mounts its own `NomConfirm` (Home keeps its own). Frontend-only.

**Tech Stack:** Vue 3.4 + Pinia + vue-router, Tailwind 4 + nom-ui, Vitest + @vue/test-utils.

## Global Constraints

- **Branch `frontend-vue-migration`** (after B3 `5a3539f`); **B4 ends by merging to `main`.**
- **Frontend-only:** NO `app/*.go`/`internal/*` changes. Bindings consumed as-is.
- **Faithful port** of `main:frontend/src/routes/{Settings,Tokens}.svelte`: same fields, validation, copy, dirty-tracking, confirms.
- **Funds-safety:** token ops go through the confirm flow (`Prepare* → tx.awaitConfirm → TxModal(formatAmountExact) → ConfirmPublish`); reveal-mnemonic is password-gated (`RevealMnemonic`) and cleared on hide; change-password via `ChangePassword`; no key material in the frontend.
- **Amounts:** `formatAmount`/`formatAmountExact`. Token amount inputs are **base units** (the Svelte forms label them "base units" and pass them raw to `PrepareMint`/`PrepareBurn`/`PrepareIssueToken` — do NOT `toBase` them).
- **Mappings (from B2/B3):** Svelte `$wallet.walletName` → `wallet.active` (Vue wallet store's wallet name); `$wallet.active` (account index) → `wallet.activeIndex`; active address → `wallet.activeAddress()`. Theme map: `text-text→text-foreground`, `text-muted→text-muted-foreground`, `bg-surface→bg-card`, `bg-bg→bg-background`, green/accent→`primary`, `text-error→text-destructive`, `text-success→text-primary`, `text-warn→text-destructive` (no warn token). nom-ui `Button` has NO `variant="primary"`.
- **`tx` store** (B2/B3): `awaitConfirm(preview)`/`status`/`preview`/`error`/`reset`. `NomConfirm` (B3) renders the confirm dialog from tx state.
- Commands in `frontend/`: `pnpm test`/`pnpm run typecheck`/`pnpm run build`. wails=`~/go/bin/wails`. Commits GPG-signed: implementers STAGE only; keep `go.mod` 2.12.0 churn out.

## File Structure

- `src/stores/node.ts` (+config actions + `sync` object), `src/stores/wallet.ts` (+changePassword/revealMnemonic).
- `src/views/Settings.vue` (replace placeholder), `src/views/Tokens.vue` (new, replaces the shared placeholder).
- `src/stores/tx.ts` (widen `awaitConfirm` type — Task 4).
- tests colocated.

---

## Task 1: Store extensions (node config + wallet change-password/reveal-mnemonic)

**Files:** Modify `src/stores/node.ts`, `src/stores/wallet.ts`; tests `src/stores/settings-stores.test.ts`.

**Interfaces:**
- `useNodeStore` add: state `sync: SyncStatus | null` (init null); actions `getConfig()` → `NodeService.GetNodeConfig()` (`{mode, remoteUrl, localUrl}`), `setMode(mode)` → `SetNodeMode`, `setUrl(mode, url)` → `SetNodeURL`, `getEmbeddedInfo()` → `GetEmbeddedInfo()` (`{running, dataDir, sizeBytes}`), `deleteEmbeddedData()` → `DeleteEmbeddedData()`. Extend `initEvents` so `node:sync` sets `this.sync = s` (in addition to the existing `syncing`). `SyncStatus = {state, currentHeight, targetHeight, percent, etaSeconds, peers}`. Also expose `mode` (the connected mode) — already present via status; ensure `node:status` sets `this.mode`.
- `useWalletStore` add: `changePassword(oldPw, newPw)` → `WalletService.ChangePassword(this.active, oldPw, newPw)`; `revealMnemonic(password): Promise<string>` → `WalletService.RevealMnemonic(password)`.

- [ ] **Step 1: Extend `src/stores/node.ts`** — add the config actions + `sync` state. Keep `connect`/`initEvents`/`height`/`syncing`/`connected`. Add `mode` to state if not present and set it from `node:status`. (Read the Svelte `node.ts` helpers `git show main:frontend/src/lib/stores/node.ts` for the exact binding calls.)

```ts
// node.ts additions (sketch)
import * as N from '../../wailsjs/go/app/NodeService'
// state adds: sync: null as SyncStatus | null, mode: 'remote'
    async getConfig() { return (await N.GetNodeConfig()) as { mode: string; remoteUrl: string; localUrl: string } },
    async setMode(mode: string) { await N.SetNodeMode(mode) },
    async setUrl(mode: string, url: string) { await N.SetNodeURL(mode, url) },
    async getEmbeddedInfo() { return (await N.GetEmbeddedInfo()) as { running: boolean; dataDir: string; sizeBytes: number } },
    async deleteEmbeddedData() { await N.DeleteEmbeddedData() },
// in initEvents: EventsOn('node:sync', (s) => { this.sync = s; this.syncing = s?.state !== 'synced' })
//                EventsOn('node:status', (s) => { this.connected = !!s?.connected; this.height = s?.height ?? this.height; this.mode = s?.mode ?? this.mode })
```

- [ ] **Step 2: Extend `src/stores/wallet.ts`** — add the two actions:

```ts
    async changePassword(oldPw: string, newPw: string) { await W.ChangePassword(this.active, oldPw, newPw) },
    async revealMnemonic(password: string): Promise<string> { return await W.RevealMnemonic(password) },
```

- [ ] **Step 3: Tests** `src/stores/settings-stores.test.ts` — node `setMode`/`setUrl`/`getEmbeddedInfo`/`deleteEmbeddedData` call the right bindings; wallet `revealMnemonic('pw')` returns the mocked mnemonic; `changePassword('a','b')` calls `ChangePassword('<active>','a','b')`. (vi.hoisted + vi.mock.)
- [ ] **Step 4: Run** `pnpm test -- src/stores && pnpm run typecheck` → pass + clean. **Stage** node.ts, wallet.ts, test. No commit.

---

## Task 2: `Settings.vue` (faithful port)

**Files:** Modify `src/views/Settings.vue` (replace the placeholder); test `src/views/Settings.test.ts`.

**Interfaces:** Consumes `useNodeStore` (connect state + config actions + sync), `useWalletStore` (changePassword/revealMnemonic/active), `useRouter`, nom-ui `Input`/`Button` + local `Field.vue`.

- [ ] **Step 1: Port `Settings.vue`** ← `main:frontend/src/routes/Settings.svelte` (read it; faithful port), applying:
  - Svelte stores → Pinia: `$node`→`node` (connected/mode/height), `$sync`→`node.sync`, `getConfig/setMode/setUrl/getEmbeddedInfo/deleteEmbeddedData` → `node.*`, `changePassword/revealMnemonic` → `wallet.*` (changePassword uses `wallet.active`, not a separate walletName arg in the call — the store action already passes `this.active`).
  - Keep the THREE sections: **Change password** (old/new/confirm, `canChange = old && new && new===confirm`, `doChange` → `wallet.changePassword(old,new)`, clear on success), **Reveal mnemonic** (password → `wallet.revealMnemonic` → show the words → Hide clears; the warning copy), **Node** (radio mode remote/local/embedded with `modeDirty`; remote/local URL inputs with `remoteDirty`/`localDirty`; `applyNode` with the EXACT dirty-tracking logic; the embedded-switch confirm (`showEmbeddedConfirm` → `confirmStartEmbedded`); embedded sync display with `fmtEta`/percent bar (when `node.mode==='embedded' && node.sync`); Delete-embedded-data button (when not embedded) with the GB size; Apply + Retry (when `!node.connected`); connected/disconnected status line; nodeMsg/nodeErr).
  - `onMounted`: `getConfig()` → seed mode/urls (respecting the dirty flags) + `refreshEmbedded()`.
  - Back button → `router.push('/home')`. Svelte reactivity → Vue `ref`/`computed`; `on:input`/`on:change`→`@input`/`@change`; theme map; drop `variant="primary"`.
- [ ] **Step 2: Test** `src/views/Settings.test.ts`: (a) Apply with an edited remote URL + changed mode calls `setUrl('remote', url)` then `setMode(mode)`; (b) reveal with a password shows the mnemonic then Hide clears it; (c) change-password with matching new/confirm calls `wallet.changePassword(old,new)`. Mock the bindings + a test router; seed `getConfig`/`getEmbeddedInfo`/`RevealMnemonic`.
- [ ] **Step 3: Run** `pnpm test -- src/views/Settings && pnpm run typecheck`. **Stage** Settings.vue + test. No commit.

---

## Task 3: `Tokens.vue` (faithful port) + NomConfirm

**Files:** Create `src/views/Tokens.vue` (the `/tokens` route currently reuses the Settings placeholder — point it at this new view); modify `src/router/index.ts` (route `tokens` → `Tokens.vue`); test `src/views/Tokens.test.ts`.

**Interfaces:** Consumes `useTokenStore` (myTokens/lookedUp/refresh/lookup), `useTxStore` (awaitConfirm/status), `useWalletStore` (activeAddress), `NomService.Prepare{IssueToken,Mint,Burn,UpdateToken}`, `formatAmount`, nom-ui `Input`/`Button` + local `Field` + `NomConfirm`.

- [ ] **Step 1: Point the `/tokens` route at `Tokens.vue`** in `src/router/index.ts` — change the `tokens` route `component` from the Settings placeholder to `() => import('../views/Tokens.vue')`.
- [ ] **Step 2: Port `Tokens.vue`** ← `main:frontend/src/routes/Tokens.svelte` (read it; faithful port), applying:
  - `$myTokens`/`$lookedUpToken`/`refreshTokens`/`lookupToken` → `token.myTokens`/`token.lookedUp`/`token.refresh`/`token.lookup`; `onMounted(refreshTokens)` → `token.refresh()`.
  - `activeAddress = $wallet.accounts.find(a=>a.index===$wallet.active)?.address` → `wallet.activeAddress()`.
  - `import { tx, awaitConfirm }` → `const tx = useTxStore()`; `awaitConfirm(preview)` → `tx.awaitConfirm(preview)`. Each op: `tx.awaitConfirm(await Nom.PrepareX(...))` — Issue (`PrepareIssueToken(name,symbol,domain,total,max,decimals,mintable,burnable,utility)`), Mint (`PrepareMint(zts,amount,receiver)`), Burn (`PrepareBurn(zts,amount)`), Update (`PrepareUpdateToken(zts,owner, t.isMintable && !disableMint, t.isBurnable && !disableBurn)`), lookup (`token.lookup(zts)`).
  - `$: if ($tx.status==='done') refreshTokens()` → `watch(() => tx.status, s => { if (s==='done') token.refresh() })`.
  - Keep all 3 sections (My tokens w/ inline Mint/Update forms; Look up / burn; Issue) + the error + `preparing` lines.
  - REPLACE the Svelte's inline `<TxModal/>`/`<TxResult/>` with `<NomConfirm/>` (mount it once in this view) — the B3 dialog renders the confirm/result.
  - Back → `router.push('/home')`. Token amounts are **base units** (no toBase). Theme map; drop `variant="primary"`.
- [ ] **Step 3: Test** `src/views/Tokens.test.ts`: clicking "Issue token" calls `Nom.PrepareIssueToken(...)` then `tx.awaitConfirm`; a lookup calls `token.lookup(zts)`; Mint (after startMint) calls `Nom.PrepareMint(zts, amount, receiver)` then awaitConfirm. Mock NomService + token store + tx store + a router; stub NomConfirm/nom-ui.
- [ ] **Step 4: Run** `pnpm test -- src/views/Tokens && pnpm run typecheck`. **Stage** Tokens.vue, router/index.ts, test. No commit.

---

## Task 4: Cleanup carryovers (widen awaitConfirm type; fix dead test query)

**Files:** Modify `src/stores/tx.ts` + the panels/Tokens that cast; modify `src/components/panels/AcceleratorPanel.test.ts`.

- [ ] **Step 1: Widen `tx.awaitConfirm`'s param type** in `src/stores/tx.ts` so the panels/Tokens can pass a `CallPreview` (the `PrepareX` return) without `as never`/`as any`. Make `SendPreview` the structural superset (it already has optional `summary`), and type the param to accept the generated preview shape:

```ts
// tx.ts — accept either the SendPreview or the NoM CallPreview (superset with summary)
import type { app } from '../../wailsjs/go/models'
// ...
    awaitConfirm(preview: SendPreview | app.CallPreview) {
      this.preview = preview as SendPreview
      this.status = 'awaiting'; this.hash = ''; this.error = ''
    },
```
(If `app.CallPreview` is structurally assignable to `SendPreview` already, simpler: keep one param type and just remove the casts at the call sites. Verify against `wailsjs/go/models.ts`.)

- [ ] **Step 2: Remove the casts** at every `tx.awaitConfirm(... as never)`/`as any` call site (the 6 panels + Tokens) now that the type accepts the preview directly. Grep: `grep -rn "awaitConfirm(" frontend/src`.
- [ ] **Step 3: Fix the dead test query** in `src/components/panels/AcceleratorPanel.test.ts` — the `option[value="Pillar-Two"]` selector that never selects; make the Vote test actually select a votable pillar before asserting `PrepareVote(id, pillar, vote)`.
- [ ] **Step 4: Run** `pnpm test && pnpm run typecheck` → all pass + clean (the cast removal must not break types). **Stage** tx.ts + the de-cast panels/Tokens + AcceleratorPanel.test.ts. No commit.

---

## Task 5: Integration + full gate

- [ ] **Step 1: Full gate** — `cd frontend && pnpm test && pnpm run typecheck && pnpm run build` → ALL pass + clean + build OK.
- [ ] **Step 2: Go sanity** — `cd /Users/dfriestedt/Github/go-syrius && GOWORK=off go build ./...` → compiles.
- [ ] **Step 3: Confirm no placeholders remain** — `/settings` → `Settings.vue`, `/tokens` → `Tokens.vue`; the `PanelPlaceholder.vue` + the Settings placeholder text are gone from the routes (the `PanelPlaceholder.vue` file may remain unused — harmless, or delete it). **Stage** any glue. No commit.

---

## Self-Review / Verification (B4)

- `pnpm test` green (settings stores + Settings + Tokens + de-cast panels + all prior); `pnpm run typecheck` clean; `pnpm run build` OK; `go build ./...` OK.
- **Live `wails dev` gate (controller):** change node mode/URL + Apply (status updates); switch to embedded (confirm shown); reveal a mnemonic (shows then hides); change the password; issue/mint/burn a testnet token (each confirming via the modal). The Home + all 7 tabs + Send/Receive still work.
- No `app/*.go`/`internal/*` changes; `go.mod` 2.12.0 churn not committed.

## The MERGE (controller closeout — after Task 5 green)

1. **Final whole-branch review** of the entire Vue migration: `review-package $(git merge-base main HEAD) HEAD` → dispatch the final reviewer (funds-safety + parity over the whole migration).
2. Address any Critical/Important findings; record minors.
3. Discard the `wails dev` `go.mod`/`go.sum` churn; ensure the tree is clean.
4. `git checkout main && git pull` (confirm `main` still at `b515834`, no divergence); `git merge --no-ff frontend-vue-migration` (signed) with a summary message; verify tests on the merge result (`pnpm test` + `go build ./...`); push; delete the branch (local + remote if pushed).
5. Confirm CI green (it already runs `vue-tsc`/vitest from sub-project A).
6. **The Vue migration is complete.** Update memory. Next: the deferred **Network Configuration (Chain ID + Network ID + embedded testnet)** feature gets its own brainstorm→spec→plan→implement cycle.
