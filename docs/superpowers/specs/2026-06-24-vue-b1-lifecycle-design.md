# Vue migration — Sub-project B, Phase B1: lifecycle + routing foundation — design

**Date:** 2026-06-24
**Branch:** `frontend-vue-migration` (continues after sub-project A, `c9db198`)
**Parent:** `docs/superpowers/specs/2026-06-24-frontend-vue-migration-design.md` (the overall Svelte→Vue migration). Sub-project B (full wallet port) is decomposed into phases B1–B4; **this spec covers B1**. B2 (Home + send/receive), B3 (NoM panels), B4 (Settings + parity/merge) get their own specs.

## Context

Sub-project A stood up and proved the Vue 3 + Vite + Tailwind 4 + nom-ui + Pinia stack inside the Wails shell, with a minimal Unlock→Home de-risk screen. B1 builds the **real wallet lifecycle** (Unlock with wallet selection, Create, Import) and replaces A's hand-rolled `v-if` screen-switch with a **`vue-router` foundation** designed for growth — the user plans to add pages and, later, a plugin system that contributes pages at runtime.

The Go backend + Wails bindings are unchanged (framework-agnostic). The merged Svelte app on `main` (`routes/Unlock.svelte`, `routes/Create.svelte`, `routes/ImportMnemonic.svelte`, `App.svelte`, the `nav` store) is the **UX reference** — B1 is a faithful 1:1 port of those flows, not a redesign.

**Funds-safety invariants carry over:** the frontend never receives key material; mnemonics surface only at creation and via password-gated `RevealMnemonic`; `lock()` re-locks the Go keystore (done in A). Every state-changing call goes through the Go bindings, which re-validate.

## Scope

In: vue-router foundation (memory history, lock guard, plugin-ready route registration), the Unlock / Create / Import screens, the nom-ui primitive mapping locked in for all later phases, `wallet`-store lifecycle actions, a local `Field.vue` wrapper.

Out (→ later phases): the real Home (account bar, 4-card row, status strip, 7 tab panels, tx history) — B2; Send/Receive modals — B2; the 6 NoM panels — B3; Settings, account management UI (rename/select/reveal-mnemonic screens), Tokens-management route, branch merge — B4. B1 registers a **placeholder** `home` route so the lock guard and post-unlock redirect work; B2 replaces it.

## Architecture

### Routing (`src/router/index.ts`)

```ts
createRouter({ history: createMemoryHistory(), routes })
```

- **Memory history** — no URL bar in a desktop app; the navigation stack lives in memory, nothing leaks to a URL. (Hash history is the fallback if deep-linking is ever wanted.)
- **Routes** are defined in one central array and **lazy-loaded** (`component: () => import('../views/Unlock.vue')`) so adding pages doesn't bloat the initial bundle and code-splits per screen.
- **Plugin-ready:** routes are registered through a small helper (or the central array) such that a future plugin can call `router.addRoute(...)` to contribute pages. B1 does not build the plugin system — it only structures routing so it is not blocked later.
- **Public routes** (allowed while locked): `unlock`, `create`, `import`. **Gated routes:** everything else (B1: `home` placeholder).
- **Global `beforeEach` lock guard:**
  - if `wallet.locked` and `to.name` is not a public route → redirect to `{ name: 'unlock' }`.
  - if `!wallet.locked` and `to.name` is a public route → redirect to `{ name: 'home' }`.
  - guard reads the Pinia `wallet` store (instantiated inside the guard, not at module top, to avoid Pinia-before-app init).

`App.vue` becomes `<RouterView />` + the dark-theme init (`useTheme().setTheme('dark')`), replacing A's `v-if` Unlock/Home switch. `main.ts` installs the router (`app.use(router)`) alongside Pinia. The initial `NodeService.Connect()` attempt (A had it in App/Home; the Svelte App did it `onMount`) moves to `App.vue` `onMounted` (best-effort, ignore failure) so the node connects regardless of screen.

### Screens (`src/views/`)

Faithful ports of the Svelte routes, built from nom-ui + the local `Field`:

- **`Unlock.vue`** — on mount, `wallet.loadWallets()`; a wallet selector (dropdown over `wallet.wallets`) + password `Input`; "Unlock" → `wallet.unlock(selected, password)` then `router.push('/home')`. Footer actions: "Create new wallet" → `router.push('/create')`, "Import mnemonic" → `router.push('/import')`, "Import keystore file" → `wallet.pickKeystoreFile()` → if a path returned, `wallet.importKeystore(path)`. Errors surfaced inline; password cleared after attempt.
- **`Create.vue`** — 3 steps in local state (`step: 1|2|3`):
  1. `wallet.generateMnemonic()` → show the words (read-only) + "I've backed it up" → step 2.
  2. Verify step (re-enter / confirm the mnemonic per the Svelte `verifyOk` logic) → step 3.
  3. name + password + confirm-password (`canCreate = name && password && password === confirm`) → `wallet.importMnemonic(name, password, mnemonic)` → on success `router.push('/home')` (the new wallet is created **and** unlocked, since `importMnemonic` sets `locked=false`). "Cancel" → `/unlock`.
- **`ImportMnemonic.vue`** — mnemonic `<textarea>` + name + password + confirm (`canImport = looksValid && name && password && password === confirm`) → `wallet.importMnemonic(name, password, mnemonic)`. "Cancel" → `/unlock`.

### nom-ui primitive mapping (locked in for B2–B4)

| App need | Use |
|---|---|
| Button | nom-ui `Button` (verified A) |
| Card / section | nom-ui `Card` / `CardContent` (verified A) |
| Text/password input | nom-ui `Input` (v-model verified A) |
| Labeled field (label + hint/error) | **local `src/components/Field.vue`** — app-specific wrapper (slot + `label`/`hint`/`error` props), mirrors the Svelte `ui/Field` |
| Wallet selector dropdown | nom-ui `Select` if exported (verify against `node_modules/nom-ui/src/index.ts`); else native `<select>` styled to match |
| Tabs (B2/B3) | nom-ui `Tabs` (verify export now; not used in B1) |

The verify-against-the-installed-package discipline from A applies: confirm each nom-ui export/prop before relying on it; adapt or fall back (and note it) if the real API differs.

### State (`src/stores/wallet.ts`)

Extend the existing Pinia `wallet` store (A: `state {locked, wallets, active}`; actions `loadWallets`, `unlock(name,password)`, `lock`) with lifecycle actions over `wailsjs/go/app/WalletService`:

- `generateMnemonic(): Promise<string>` → `W.GenerateMnemonic()`.
- `importMnemonic(name, password, mnemonic): Promise<void>` → `W.ImportMnemonic(name,password,mnemonic)`, then set `active=name`, `locked=false` (created/imported wallet is unlocked), and refresh `wallets`.
- `importKeystore(srcPath): Promise<void>` → `W.ImportKeystore(srcPath)`, then refresh `wallets` (stays locked — user then unlocks).
- `pickKeystoreFile(): Promise<string>` → `W.PickKeystoreFile()` (returns '' if cancelled).

Actions that can fail (unlock/import) **throw** so the view surfaces the error (do not swallow). The `node` store is unchanged in B1.

## Error handling

- Each lifecycle action that hits the backend rejects on error; each view catches and renders the message inline (`role="alert"`), matching the Svelte forms. No silent `catch {}` swallowing (an A-review minor).
- Password fields are cleared after a submit attempt (success or failure) — an A-review minor.
- The lock guard is the single gate; views do not separately check lock state.

## Testing

Vitest + @vue/test-utils, mocking the `wailsjs/go/app/WalletService` bindings and using a test router (`createRouter({ history: createMemoryHistory(), routes })`):

- **Router guard:** locked + navigate to `/home` → lands on `/unlock`; unlocked + navigate to `/unlock` → lands on `/home`.
- **Unlock.vue:** lists wallets; entering password + clicking Unlock calls `Unlock(selected, password)`; a thrown error renders inline and the password field clears.
- **Create.vue:** step 1 calls `GenerateMnemonic` and shows words; completing step 3 calls `ImportMnemonic(name, password, mnemonic)` with the generated mnemonic.
- **ImportMnemonic.vue:** valid mnemonic + matching passwords enables import; submit calls `ImportMnemonic(name, password, mnemonic)`.
- **wallet store:** `importMnemonic` sets `locked=false`/`active`; `pickKeystoreFile`/`importKeystore` call the bindings.
- Gates: `pnpm test`, `pnpm run typecheck` (vue-tsc) clean, `pnpm run build` succeeds. Controller runs the live `wails dev` gate (create a throwaway wallet, import a known mnemonic, unlock) before B1 is considered done.

## Risks

- **nom-ui `Select`/`Tabs` exports** — verify against the installed package; fall back to native + note it (same as A).
- **Pinia-in-guard init order** — instantiate the store inside the guard callback, not at module scope, so it runs after `app.use(pinia)`.
- **vue-router + Wails** — memory history avoids file-protocol URL issues; verified by the live `wails dev` gate.
- **Mnemonic-verify UX** — port the Svelte `verifyOk` step faithfully; the plan pins the exact verification interaction.
