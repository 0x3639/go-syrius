# Frontend migration: Svelte → Vue 3 + nom-ui — design

**Date:** 2026-06-24
**Branch:** `frontend-vue-migration` (off `main` `b515834`)
**Scope:** Replace the Svelte frontend with **Vue 3 + nom-ui**, in two sub-projects (A: stack foundation/de-risk; B: full wallet port). This spec covers the overall migration and decomposition, with **sub-project A** specified in detail (the part we build first); B is outlined and gets its own spec/plan after A lands.

## Context

The just-merged UI redesign (commit `b515834`) is a **hand-ported Svelte copy** of the [nom-ui](https://github.com/digitalSloth/nom-ui) design. nom-ui is actually a **Vue 3 + Tailwind 4 + shadcn-vue** component library (MIT) — including blockchain primitives (`Address`, `Amount`, `TxStatus`, `TxDirection`, `TokenIcon`) and `useTheme`/`useToast`. Maintaining a hand-ported Svelte version is redundant; switching the frontend to **Vue 3** lets us consume nom-ui directly and get its components + theme + primitives for free.

The migration is a **`frontend/` rebuild only**. The Go backend (`app/`, `internal/`), the Wails v2 shell, and the Wails ↔ frontend binding contract are **framework-agnostic and stay** — Vue calls the same generated `window.go.app.*` bindings the Svelte app did. The merged Svelte app is the **design/UX reference** (the Home IA: account bar, 4-card row, status strip, 7 tabs, tx history, Send/Receive modals; the auto-receive/receive behavior; the confirm-what-you-sign flow).

## What stays vs what gets rebuilt

| Stays (untouched) | Rebuilt in Vue |
|---|---|
| Go backend: `app/*.go`, `internal/*` (services, embedded node, signer/PoW) | `frontend/src/**` (Svelte components/routes → Vue) |
| Wails binding contract (the `*Service` methods + events) | State: Svelte stores → **Pinia** |
| `wails.json` app config (frontend cmds updated) | Styling: hand-ported Tailwind 3 → **Tailwind 4 + nom-ui** |
| CI (`.github/workflows/ci.yml`) — frontend job cmds may need a tweak | `frontend/wailsjs` regenerated for the Vue app (same Go API) |

**Funds-path invariant carries over unchanged:** the frontend still sends *intent* only; Go builds → PoWs → signs → publishes; the confirm modal renders the effect from the built block. No Go/binding changes in this migration.

## Target stack

- **Vue 3.4+** (Composition API) + **Vite**, **TypeScript**
- **Tailwind CSS 4** via `@tailwindcss/vite` (CSS-first config; no `tailwind.config.js`)
- **nom-ui** (npm) + its peer/runtime deps (`reka-ui`, `@lucide/vue`, `vue-sonner`, `@vueuse/core`, `@tanstack/vue-table`, `class-variance-authority`, `clsx`, `tailwind-merge`) + the bundled Space Grotesk / JetBrains Mono fonts
- **Pinia** for state
- **Vitest** + **@vue/test-utils** for component tests
- **pnpm** (keep the package manager)
- Wails v2 runtime (`@wailsapp/runtime` / generated `wailsjs/runtime`)

## Strategy

Wails has a single frontend, so this is a **swap, not a parallel run**: we rebuild `frontend/` on this branch and merge only when the Vue app reaches parity. To keep `main` shippable, the branch is not merged until sub-project B completes. Sub-project A proves the stack end-to-end first so B is a port, not a research project.

## Decomposition

- **Sub-project A — Vue stack foundation (this spec).** Scaffold Vue+Vite+Tailwind4, add + wire nom-ui, regenerate the Wails bindings, set up Pinia, and get **one screen working end-to-end** via `wails dev` (Unlock → connect → a minimal balance view). Deliverable: the stack is proven (Wails + Vue + nom-ui + Tailwind 4 + Pinia interop), green tests, runs in `wails dev`.
- **Sub-project B — full wallet port (own spec/plan after A).** Rebuild every screen/flow to parity with the merged Svelte app — wallet lifecycle (Unlock/Create/Import), the Home (account bar, 4 cards, status strip, 7 tab panels, tx history), Send/Receive modals, Settings — porting each Svelte store to a Pinia store and each component to Vue + nom-ui. Ends with feature parity, CI green, and the branch ready to merge (replacing the Svelte frontend).

---

# Sub-project A — Vue stack foundation (detailed)

**Goal:** stand up the Vue + nom-ui + Tailwind 4 + Pinia frontend inside the Wails shell and prove the full interop with one end-to-end screen, before porting the rest.

## A.1 — Scaffold

- Build the Vue frontend in `frontend/` on this branch. The cleanest path: scaffold a Vue-TS Vite app (via `wails init -t vue-ts` into a temp dir, or `create-vue`), and bring its `src/`, `index.html`, `package.json`, `vite.config.ts`, `tsconfig*` into `frontend/`, **removing the Svelte-specific config** (`svelte.config.js`, `tailwind.config.js` (Tailwind 3), Svelte `package.json` deps) as the Vue setup replaces them.
- **Regenerate the Wails bindings:** `GOWORK=off wails generate module` rewrites `frontend/wailsjs/go/**` + `models.ts` from the Go services — Vue imports the same `NodeService`/`WalletService`/`TxService`/`NomService`/`ConfigService` methods.
- Update `wails.json`: `frontend:install`/`frontend:build`/`frontend:dev:watcher` stay `pnpm install`/`pnpm run build`/`pnpm run dev` (Vue Vite scripts use the same names).
- `main.go`'s `//go:embed all:frontend/dist` is unchanged (Vite still outputs `frontend/dist`).

## A.2 — Tailwind 4 + nom-ui

- Add Tailwind 4: `pnpm add -D tailwindcss @tailwindcss/vite`; register the Vite plugin in `vite.config.ts`.
- Add nom-ui: `pnpm add nom-ui`. **Risk/contingency:** if `nom-ui` is not published to the public npm registry under that name, install from GitHub: `pnpm add github:digitalSloth/nom-ui` (and pin a commit). Verifying this is part of A.
- App CSS (per nom-ui docs): `@import "tailwindcss";` + `@import "nom-ui/style.css";` + `@source "../node_modules/nom-ui/src";` (so Tailwind generates the utility classes nom-ui's components use). Apply nom-ui's dark theme (`useTheme` / the `.dark` class) — the app is dark by default, matching the merged design.

## A.3 — Pinia + bindings access

- Add Pinia (`pnpm add pinia`), install it on the app in `main.ts`.
- Create a thin **bindings layer** the stores call, mirroring the Svelte stores' boundaries: a `wallet` store (unlock/lock/active account) and a `node` store (connect/status) are enough for A. They call `wailsjs/go/app/*` and hold reactive state — the Vue analogue of the Svelte stores.

## A.4 — The end-to-end screen

One route/component tree proving every seam:
- **Unlock view:** a nom-ui `Card` + `Input` (password) + `Button`; it lists wallets via `WalletService.ListWallets()` and on submit calls `WalletService.Unlock(name, password)` (default to the first/only wallet for A) via the `wallet` store. On success, transitions to Home.
- **Minimal Home view:** calls `NodeService.Connect()` + a balance read (`NodeService.GetBalances()` via a store), and renders ZNN/QSR using nom-ui's **`Card`** + **`Amount`** (and/or `Address` for the active address). No tabs/panels yet — just enough to prove nom-ui components render with the theme and real binding data.
- A simple top-level `App.vue` switches Unlock ↔ Home on the wallet's locked state (the Vue analogue of the current Svelte `App.svelte`).

## A.5 — Testing & verification

- **Vitest + @vue/test-utils** smoke tests: the Unlock view renders + submitting calls the (mocked) `Unlock` binding; the Home view renders a nom-ui `Amount` with a mocked balance. (Mirrors how the Svelte tests mock `wailsjs/go/app/*`.)
- **Type/build:** `pnpm run build` (Vite) succeeds; `vue-tsc`/`tsc` typechecks clean.
- **Live:** `GOWORK=off ~/go/bin/wails dev` — unlock the real wallet, connect, see balances rendered by nom-ui. This is the gate that proves Wails + Vue + nom-ui + Tailwind 4 + Pinia all interop.
- **CI:** update `.github/workflows/ci.yml`'s `frontend` job if the scripts/check command changed (e.g. `vue-tsc` instead of `svelte-check`); keep the `build-test` matrix + security jobs as-is. (Wails-version/webkit specifics from Phase 7a still apply.)

## A.6 — Out of scope for A (→ sub-project B / later)

- The full Home (account bar, 4-card row, status strip, 7 tab panels, tx history), Send/Receive modals, Settings, wallet create/import — all of B.
- nom-ui `TxStatus`/`TxDirection`/`TokenIcon`/`useToast` adoption beyond what A needs.
- **Deferred, framework-agnostic:** embedded-node **testnet** support (bundle testnet `genesis.json` carrying chain id 73404 + seeders as a Mainnet/Testnet preset; per-network data dirs). Independent of the migration; do it in the Vue version or its own branch.

## Risks

- **nom-ui registry availability** (A.2) — resolved during A by trying the npm name, then GitHub install.
- **Tailwind 4 newness** — different config model than our Tailwind 3 Svelte setup; A's whole point is to validate it early.
- **Wails + Vue HMR / bindings regen** — standard (Wails ships a vue-ts template), but verified by A's `wails dev` gate.
- **Bindings drift** — regenerating `wailsjs` must not change the Go API; verify the generated method set matches what B will consume.
