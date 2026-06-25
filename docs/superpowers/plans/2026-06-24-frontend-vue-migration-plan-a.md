# Vue Migration — Sub-project A (stack foundation) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax. This sub-project is part de-risk research: several steps have **Verify → Fallback** branches where an unknown (nom-ui packaging, Tailwind 4 wiring, Wails+Vue interop) is resolved. Record what you find.

**Goal:** Stand up a Vue 3 + Vite + Tailwind 4 + nom-ui + Pinia frontend inside the Wails shell and prove full interop with one end-to-end screen (Unlock → connect → balance), before porting the rest of the wallet (sub-project B).

**Architecture:** Replace `frontend/`'s Svelte app with a Vue 3 + Vite + TS app; regenerate the Wails bindings (same Go API); style with Tailwind 4 + nom-ui; state in Pinia. One Unlock/Home route tree exercises every seam (Wails bindings ↔ Vue, Pinia, nom-ui components + theme, Tailwind 4 build, `wails dev`).

**Tech Stack:** Vue 3.4+, Vite, TypeScript, Tailwind CSS 4 (`@tailwindcss/vite`), nom-ui (+ peers), Pinia, Vitest + @vue/test-utils, pnpm, Wails v2.

## Global Constraints

- **Branch `frontend-vue-migration`** off main; NOT merged until sub-project B reaches parity. Wails has one frontend, so this replaces `frontend/`'s Svelte setup.
- **Presentation/frontend-only.** No `app/*.go` or `internal/*` changes. `frontend/wailsjs` is **regenerated** from the existing Go services (`GOWORK=off wails generate module`) — the Go API must not change.
- **Bindings the de-risk screen uses (exact, verified):** `WalletService.ListWallets()`, `WalletService.Unlock(name, password)`, `NodeService.Connect()`, `NodeService.GetBalances()` → `[]{ zts, symbol, decimals, amount }`.
- **Stack versions:** Vue `^3.4`, Tailwind `^4`, Pinia latest, pnpm `10.17.1`. nom-ui per its README (Vue 3.4 peer dep).
- **Dark-only**, matching the merged design (nom-ui `.dark` theme).
- Commands: `cd frontend`; `pnpm install`; `pnpm run build` (vite); `pnpm run test` (vitest); typecheck `pnpm run typecheck` (`vue-tsc --noEmit`). Bindings: from repo root `GOWORK=off ~/go/bin/wails generate module`. Live: `GOWORK=off ~/go/bin/wails dev` (FOREGROUND; `wails` not on PATH — use the full path; wails CLI is 2.12.0 locally).
- Commits GPG-signed: **implementers STAGE only**, controller commits. Keep the `wails dev` `go.mod`/`models.ts`-2.12.0 churn OUT of commits.

## File Structure (end state of A)

- `frontend/package.json` — Vue/Vite/Tailwind4/nom-ui/Pinia/Vitest deps + scripts.
- `frontend/vite.config.ts` — `@vitejs/plugin-vue` + `@tailwindcss/vite`; build to `frontend/dist`.
- `frontend/tsconfig.json` (+ `tsconfig.node.json`), `frontend/index.html`.
- `frontend/src/main.ts` — create app, install Pinia, mount.
- `frontend/src/App.vue` — switch Unlock ↔ Home on wallet locked state.
- `frontend/src/style.css` — `@import "tailwindcss"` + nom-ui style + `@source`.
- `frontend/src/stores/{wallet,node}.ts` — Pinia stores over the bindings.
- `frontend/src/views/{Unlock,Home}.vue` — the de-risk screens.
- `frontend/src/**/*.test.ts` — Vitest smoke tests.
- `frontend/wailsjs/**` — regenerated bindings.
- Removed: `frontend/svelte.config.js`, `frontend/tailwind.config.js`, all `frontend/src/**/*.svelte`, Svelte deps.
- `.github/workflows/ci.yml` — frontend job updated (Task 5).

---

## Task 1: Vue 3 + Vite + TS scaffold (replace Svelte) + regenerate bindings

**Files:** Replace `frontend/package.json`, `frontend/vite.config.ts`, `frontend/tsconfig.json`, `frontend/index.html`, `frontend/src/main.ts`, `frontend/src/App.vue`; remove Svelte config + `src/**/*.svelte`; regenerate `frontend/wailsjs`.

**Interfaces:**
- Produces: a minimal Vue app that builds to `frontend/dist` and runs in `wails dev`; regenerated `wailsjs/go/app/*` bindings.

- [ ] **Step 1: Remove the Svelte frontend (keep wailsjs + assets)**

```bash
cd frontend
git rm -r src/routes src/lib --quiet
git rm src/App.svelte src/main.ts src/app.css src/style.css svelte.config.js tailwind.config.js --quiet 2>/dev/null || true
# keep: frontend/wailsjs (regenerated next), frontend/index.html (replaced), package.json (replaced)
```
(If a path doesn't exist, drop it from the command. Do NOT remove `frontend/wailsjs`.)

- [ ] **Step 2: Write `frontend/package.json`**

```json
{
  "name": "syrius-frontend",
  "private": true,
  "type": "module",
  "packageManager": "pnpm@10.17.1",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "typecheck": "vue-tsc --noEmit",
    "test": "vitest run"
  },
  "dependencies": {
    "pinia": "^2.2.0",
    "vue": "^3.4.0"
  },
  "devDependencies": {
    "@tailwindcss/vite": "^4.0.0",
    "@vitejs/plugin-vue": "^5.1.0",
    "@vue/test-utils": "^2.4.6",
    "jsdom": "^25.0.0",
    "tailwindcss": "^4.0.0",
    "typescript": "^5.5.0",
    "vite": "^5.4.0",
    "vitest": "^2.1.0",
    "vue-tsc": "^2.1.0"
  }
}
```

- [ ] **Step 3: Write `frontend/vite.config.ts`**

```ts
// Import defineConfig from vitest/config (it extends Vite's) so the `test` field
// is typed; importing from 'vite' would reject the `test` key.
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  build: { outDir: 'dist', emptyOutDir: true },
  test: { environment: 'jsdom', globals: true },
})
```

- [ ] **Step 4: Write `frontend/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "esModuleInterop": true,
    "lib": ["ESNext", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "types": ["vite/client", "vitest/globals"]
  },
  "include": ["src/**/*.ts", "src/**/*.vue", "wailsjs/**/*.ts"]
}
```

- [ ] **Step 5: Write `frontend/index.html`**

```html
<!doctype html>
<html lang="en" class="dark">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>syrius</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 6: Write minimal `frontend/src/main.ts` + `frontend/src/App.vue`**

```ts
// src/main.ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

createApp(App).use(createPinia()).mount('#app')
```

```vue
<!-- src/App.vue -->
<script setup lang="ts"></script>
<template>
  <main class="p-8 text-white">syrius — Vue scaffold OK</main>
</template>
```

(Pinia is installed now so Task 3 only adds stores. Styling comes in Task 2.)

- [ ] **Step 7: Regenerate the Wails bindings**

Run from repo root: `GOWORK=off ~/go/bin/wails generate module`
Verify: `frontend/wailsjs/go/app/{NodeService,WalletService,TxService,NomService,ConfigService}.{js,d.ts}` + `models.ts` exist and expose `ListWallets`, `Unlock`, `Connect`, `GetBalances`. (If `wails generate module` errors, run `GOWORK=off ~/go/bin/wails dev` once — it regenerates bindings on start — then stop it.)

- [ ] **Step 8: Install + build + run**

Run: `cd frontend && pnpm install && pnpm run build`
Expected: vite build succeeds → `frontend/dist`.
Run from repo root: `GOWORK=off ~/go/bin/wails dev` (foreground) → the window shows "syrius — Vue scaffold OK". Stop after confirming. (This proves Wails embeds + serves the Vue build.)

- [ ] **Step 9: Stage** (controller commits; do NOT stage `go.mod`/`go.sum`/`models.ts`-2.12.0 churn — `models.ts` regen from Task 7 IS wanted, but the wails-2.12.0 go.mod bump is not; `git checkout HEAD -- go.mod go.sum` if it appears)

`git add frontend/package.json frontend/vite.config.ts frontend/tsconfig.json frontend/index.html frontend/src frontend/wailsjs frontend/pnpm-lock.yaml` + the `git rm`s.

---

## Task 2: Tailwind 4 + nom-ui

**Files:** Create `frontend/src/style.css`; modify `frontend/src/main.ts`, `frontend/src/App.vue`, `frontend/package.json`.

**Interfaces:**
- Produces: a working Tailwind 4 + nom-ui setup; nom-ui components import and render with the dark theme.

- [ ] **Step 1: Add nom-ui — VERIFY its source**

Run: `cd frontend && pnpm add nom-ui`
**Verify:** does it resolve from the public npm registry? `pnpm why nom-ui` / check it installed the digitalSloth library (has `src/index.ts`, exports `Button`/`Card`/`Amount`).
**Fallback:** if the npm name is taken by an unrelated package or 404s, install from GitHub and pin: `pnpm add github:digitalSloth/nom-ui` (note the resolved commit in your report). Record which path worked.

- [ ] **Step 2: Write `frontend/src/style.css`** (per nom-ui's documented setup)

```css
@import "tailwindcss";
@import "nom-ui/style.css";
/* Generate the utility classes nom-ui's component source uses: */
@source "../node_modules/nom-ui/src";
```
(If nom-ui was installed from GitHub, the path is still `node_modules/nom-ui/src` — pnpm materializes it there. Verify the dir exists; adjust the `@source` glob if the layout differs.)

- [ ] **Step 2b: Import the stylesheet** — in `src/main.ts` add `import './style.css'` at the top.

- [ ] **Step 3: Apply the dark theme + render a nom-ui component** — `src/App.vue`

```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { Button, Card, CardContent, useTheme } from 'nom-ui'
const { setTheme } = useTheme()
onMounted(() => setTheme?.('dark'))
</script>
<template>
  <main class="min-h-screen bg-background p-8">
    <Card>
      <CardContent class="space-y-3 p-6">
        <p class="text-foreground">nom-ui + Tailwind 4 OK</p>
        <Button>Primary</Button>
      </CardContent>
    </Card>
  </main>
</template>
```
**Verify the exact export/composable names against the installed nom-ui** (`node_modules/nom-ui/src/index.ts`) — the README lists `Button`, `Card`, `CardContent`, `useTheme`; if a name differs (e.g. theme set via a class on `<html>` rather than `setTheme`), adapt. The `<html class="dark">` in index.html already sets dark; `setTheme` is belt-and-suspenders.

- [ ] **Step 4: Build + visual verify**

Run: `cd frontend && pnpm run build` → succeeds (Tailwind 4 + nom-ui compile).
Run from repo root: `GOWORK=off ~/go/bin/wails dev` → the window shows a dark nom-ui `Card` with a green primary `Button`. Confirms nom-ui renders with its theme through the Wails+Vite pipeline. Stop after.

- [ ] **Step 5: Stage** — `git add frontend/src/style.css frontend/src/main.ts frontend/src/App.vue frontend/package.json frontend/pnpm-lock.yaml`

---

## Task 3: Pinia stores over the Wails bindings

**Files:** Create `frontend/src/stores/{wallet,node}.ts`; tests `stores/{wallet,node}.test.ts`.

**Interfaces:**
- Consumes: `wailsjs/go/app/WalletService` (`ListWallets`, `Unlock`), `wailsjs/go/app/NodeService` (`Connect`, `GetBalances`).
- Produces:
  - `useWalletStore` — state `{ locked: boolean, wallets: string[], active: string }`; actions `loadWallets()`, `unlock(name, password)` (sets `locked=false` on success), `lock()`.
  - `useNodeStore` — state `{ connected: boolean, balances: TokenBalance[] }`; actions `connect()`, `loadBalances()`. `TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }`.

- [ ] **Step 1: Write `src/stores/node.ts`**

```ts
import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }

export const useNodeStore = defineStore('node', {
  state: () => ({ connected: false, balances: [] as TokenBalance[] }),
  actions: {
    async connect() {
      try { await N.Connect(); this.connected = true } catch { this.connected = false }
    },
    async loadBalances() {
      try { this.balances = (await N.GetBalances()) as unknown as TokenBalance[] } catch { this.balances = [] }
    },
  },
})
```

- [ ] **Step 2: Write `src/stores/wallet.ts`**

```ts
import { defineStore } from 'pinia'
import * as W from '../../wailsjs/go/app/WalletService'

export const useWalletStore = defineStore('wallet', {
  state: () => ({ locked: true, wallets: [] as string[], active: '' }),
  actions: {
    async loadWallets() {
      try {
        const list = (await W.ListWallets()) as unknown as Array<{ name: string }>
        this.wallets = list.map((w) => w.name)
        if (!this.active && this.wallets.length) this.active = this.wallets[0]
      } catch { this.wallets = [] }
    },
    async unlock(name: string, password: string) {
      await W.Unlock(name, password)
      this.active = name
      this.locked = false
    },
    lock() { this.locked = true },
  },
})
```
(Verify the `ListWallets()` element shape against `wailsjs/go/models.ts` — it returns `WalletMeta` objects; use the `name` field. Adjust the mapped field if the generated type differs.)

- [ ] **Step 3: Write `stores/node.test.ts` + `stores/wallet.test.ts`**

```ts
// stores/node.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
}))
import { useNodeStore } from './node'
beforeEach(() => setActivePinia(createPinia()))
describe('node store', () => {
  it('connects and loads balances', async () => {
    const s = useNodeStore()
    await s.connect(); expect(s.connected).toBe(true)
    await s.loadBalances(); expect(s.balances[0].symbol).toBe('ZNN')
  })
})
```

```ts
// stores/wallet.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
}))
import { useWalletStore } from './wallet'
beforeEach(() => setActivePinia(createPinia()))
describe('wallet store', () => {
  it('lists wallets and unlocks', async () => {
    const s = useWalletStore()
    await s.loadWallets(); expect(s.wallets).toEqual(['Main']); expect(s.active).toBe('Main')
    await s.unlock('Main', 'pw'); expect(s.locked).toBe(false)
  })
})
```

- [ ] **Step 4: Run tests + typecheck**

Run: `cd frontend && pnpm run test -- src/stores && pnpm run typecheck`
Expected: both store tests pass; `vue-tsc` clean.

- [ ] **Step 5: Stage** — `git add frontend/src/stores`

---

## Task 4: The end-to-end de-risk screen (Unlock → Home)

**Files:** Create `frontend/src/views/{Unlock,Home}.vue`, tests `views/{Unlock,Home}.test.ts`; modify `frontend/src/App.vue`.

**Interfaces:**
- Consumes: `useWalletStore`, `useNodeStore` (Task 3); nom-ui `Card`/`CardContent`/`Input`/`Button`/`Amount`.
- Produces: a working Unlock → Home flow.

- [ ] **Step 1: Write `src/views/Unlock.vue`**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
const wallet = useWalletStore()
const password = ref('')
const error = ref('')
onMounted(() => wallet.loadWallets())
async function submit() {
  error.value = ''
  try { await wallet.unlock(wallet.active, password.value) }
  catch (e: any) { error.value = e?.message ?? String(e) }
}
</script>
<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="w-80">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-lg text-foreground">Unlock {{ wallet.active || 'wallet' }}</h1>
        <Input v-model="password" type="password" placeholder="Password" aria-label="password" @keyup.enter="submit" />
        <Button class="w-full" @click="submit">Unlock</Button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
```
(Verify nom-ui `Input` supports `v-model`; if it exposes a different model prop, adapt. `text-destructive`/`bg-background`/`text-foreground` are nom-ui theme classes — confirm the names from its style.)

- [ ] **Step 2: Write `src/views/Home.vue`**

```vue
<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { Card, CardContent, Button, Amount } from 'nom-ui'
import { useNodeStore } from '../stores/node'
import { useWalletStore } from '../stores/wallet'
const node = useNodeStore()
const wallet = useWalletStore()
const znn = computed(() => node.balances.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => node.balances.find((b) => b.symbol === 'QSR'))
onMounted(async () => { await node.connect(); await node.loadBalances() })
</script>
<template>
  <main class="min-h-screen space-y-4 bg-background p-8">
    <div class="flex items-center justify-between">
      <span class="text-foreground">{{ wallet.active }}</span>
      <Button variant="ghost" @click="wallet.lock()">Lock</Button>
    </div>
    <div class="grid grid-cols-2 gap-3">
      <Card><CardContent class="p-4">
        <div class="text-xs text-muted-foreground">ZNN</div>
        <Amount :value="znn?.amount ?? '0'" :decimals="znn?.decimals ?? 8" />
      </CardContent></Card>
      <Card><CardContent class="p-4">
        <div class="text-xs text-muted-foreground">QSR</div>
        <Amount :value="qsr?.amount ?? '0'" :decimals="qsr?.decimals ?? 8" />
      </CardContent></Card>
    </div>
  </main>
</template>
```
(Verify nom-ui `Amount`'s prop API — it may take `:value` as a base-unit string + `:decimals`, or a pre-formatted number. Adapt the props to match; the point is to render a balance via nom-ui's primitive. If `Amount`'s API is unclear, fall back to `{{ formatted }}` text and note it for B.)

- [ ] **Step 3: Update `src/App.vue` to switch on locked state**

```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { useTheme } from 'nom-ui'
import { useWalletStore } from './stores/wallet'
import Unlock from './views/Unlock.vue'
import Home from './views/Home.vue'
const wallet = useWalletStore()
const { setTheme } = useTheme()
onMounted(() => setTheme?.('dark'))
</script>
<template>
  <Unlock v-if="wallet.locked" />
  <Home v-else />
</template>
```

- [ ] **Step 4: Write `views/Unlock.test.ts` + `views/Home.test.ts`**

```ts
// views/Unlock.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
const unlock = vi.fn().mockResolvedValue(undefined)
vi.mock('../../wailsjs/go/app/WalletService', () => ({ ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]), Unlock: unlock }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' }, CardContent: { template: '<div><slot/></div>' },
  Button: { template: '<button @click="$emit(\'click\')"><slot/></button>' },
  Input: { props: ['modelValue'], template: '<input :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
import Unlock from './Unlock.vue'
beforeEach(() => setActivePinia(createPinia()))
describe('Unlock.vue', () => {
  it('unlocks with the entered password', async () => {
    const w = mount(Unlock)
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('button').trigger('click')
    await Promise.resolve()
    expect(unlock).toHaveBeenCalledWith('Main', 'pw')
  })
})
```

```ts
// views/Home.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
vi.mock('../../wailsjs/go/app/NodeService', () => ({ Connect: vi.fn().mockResolvedValue(undefined), GetBalances: vi.fn().mockResolvedValue([{ zts: 'z', symbol: 'ZNN', decimals: 8, amount: '150000000' }]) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' }, CardContent: { template: '<div><slot/></div>' },
  Button: { template: '<button><slot/></button>' },
  Amount: { props: ['value', 'decimals'], template: '<span class="amount">{{ value }}</span>' },
}))
import Home from './Home.vue'
beforeEach(() => setActivePinia(createPinia()))
describe('Home.vue', () => {
  it('connects and renders a balance', async () => {
    const w = mount(Home)
    await new Promise((r) => setTimeout(r))
    expect(w.find('.amount').exists()).toBe(true)
  })
})
```
(The `nom-ui` mock stubs the components so the view logic is tested without the real library. Adjust stub props/events if Task-1/2 verification showed different nom-ui APIs.)

- [ ] **Step 5: Run tests + typecheck + build**

Run: `cd frontend && pnpm run test && pnpm run typecheck && pnpm run build`
Expected: all tests pass; `vue-tsc` clean; vite build succeeds.

- [ ] **Step 6: Live end-to-end verify (the de-risk gate)**

Run from repo root: `GOWORK=off ~/go/bin/wails dev` (foreground). Unlock the REAL wallet (enter your password) → it should connect and show ZNN/QSR balances rendered by nom-ui `Amount` in dark-themed cards. **This is the gate that proves Wails + Vue + nom-ui + Tailwind 4 + Pinia all interop end-to-end.** Note anything that needed adapting. Stop after.

- [ ] **Step 7: Stage** — `git add frontend/src/views frontend/src/App.vue`

---

## Task 5: CI frontend job update + final gate

**Files:** Modify `.github/workflows/ci.yml`.

- [ ] **Step 1: Update the `frontend` job** — it currently runs `pnpm run check` (svelte-check) + `pnpm test`. Change the check step to the Vue typecheck:

```yaml
      - run: pnpm install --frozen-lockfile
      - run: pnpm run typecheck
      - run: pnpm test
```
(Keep `pnpm/action-setup@v4` + node 22 + the pnpm cache. The `build-test` matrix still runs `wails build`; the Linux `webkit2_41` tag + `go.mod` 1.25.11 toolchain are unchanged. `wails build` now builds the Vue frontend — same `frontend:install`/`frontend:build` pnpm scripts, so no workflow change there beyond the check step.)

- [ ] **Step 2: Local full gate**

Run: `cd frontend && pnpm install --frozen-lockfile && pnpm run typecheck && pnpm test && pnpm run build`
Expected: typecheck clean, tests pass, build succeeds.
Run: `cd /Users/dfriestedt/Github/go-syrius && GOWORK=off go build ./...` (the Go side is untouched; sanity that the embed still resolves — needs `frontend/dist`, which the build above produced).

- [ ] **Step 3: Stage** — `git add .github/workflows/ci.yml`

---

## Self-Review / Verification (end-to-end for A)

- `cd frontend && pnpm test` green (stores + views); `pnpm run typecheck` clean; `pnpm run build` succeeds.
- `wails dev`: unlock the real wallet → balances render via nom-ui with the dark theme. Stack interop proven.
- `frontend/wailsjs` regenerated; Go API unchanged (no `app/*.go` edits).
- CI frontend job updated to `vue-tsc`; the rest of the pipeline intact.
- The `wails dev` `go.mod`/`go.sum`-to-2.12.0 churn is NOT committed.

## Hand-off to Sub-project B

With the stack proven, B ports the full wallet to parity: wallet lifecycle (Create/Import + the real Unlock with wallet selection), the full Home (account bar, 4-card row, status strip, 7 tab panels: Tokens/Rewards/Plasma/Pillar/Staking/Sentinels/Accelerator, tx history), Send/Receive modals (confirm-what-you-sign), Settings — each Svelte store → Pinia, each component → Vue + nom-ui (adopting `Address`/`TxStatus`/`TxDirection`/`TokenIcon`/`useToast`), reusing the merged Svelte app as the UX reference. B ends with CI green and the branch ready to merge, replacing the Svelte frontend.
