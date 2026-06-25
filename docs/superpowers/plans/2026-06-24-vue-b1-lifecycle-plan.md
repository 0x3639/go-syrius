# Vue B1 — Lifecycle + Router Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the vue-router foundation + faithful Vue/nom-ui ports of the Unlock, Create, and Import-mnemonic lifecycle screens, replacing sub-project A's `v-if` screen switch.

**Architecture:** `vue-router` with memory history drives screens; a global `beforeEach` guard gates on the Pinia `wallet` store's lock state; routes are lazy-loaded from one central array (so a future plugin can `addRoute`). The three lifecycle screens are 1:1 ports of the merged Svelte routes (`main:frontend/src/routes/*`), built from nom-ui primitives + a local `Field.vue`, calling extended `wallet`-store actions over the unchanged Wails bindings.

**Tech Stack:** Vue 3.4 + Vite + TS, vue-router 4, Pinia, Tailwind 4 + nom-ui, Vitest + @vue/test-utils.

## Global Constraints

- **Branch `frontend-vue-migration`** (continues after A `c9db198`); not merged until B4.
- **Frontend-only:** NO `app/*.go` / `internal/*` changes; bindings are consumed as-is from `frontend/wailsjs/go/app/*`.
- **Faithful 1:1 port** of the Svelte flows (`main:frontend/src/routes/Unlock.svelte`, `Create.svelte`, `ImportMnemonic.svelte`) — same steps, validation, copy, `.dat` filename rule. Not a redesign.
- **Funds-safety:** frontend never receives key material; mnemonic shown once at creation; every state-changing op goes through the Go bindings.
- **A-review minors to honor:** lifecycle views **surface** binding errors inline (`role="alert"`), never swallow; **password fields clear after a submit attempt**.
- **nom-ui** is `github:digitalSloth/nom-ui#63f755a…` (already installed); import `Button`, `Card`, `CardContent`, `Input` from `nom-ui` (verified in A). Wallet selector = a native styled `<select>` (no nom-ui `Select` dependency). Theme classes: `bg-background`, `text-foreground`, `text-muted-foreground`, `text-destructive`.
- **Navigation:** Svelte `view.set('x')` → `router.push('/x')`; Svelte `dispatch('unlocked')` → `router.push('/home')`.
- **Filename rule (verbatim from Svelte):** wallet name → keystore file = `name.endsWith('.dat') ? name : name + '.dat'`.
- Commands in `frontend/`: `pnpm test`, `pnpm run typecheck` (vue-tsc), `pnpm run build`. wails = `~/go/bin/wails` (not on PATH). Commits GPG-signed: **implementers STAGE only**; keep `wails dev` `go.mod` 2.12.0 churn out.

## File Structure

- `frontend/src/router/index.ts` — router + routes array + lock guard (Task 1).
- `frontend/src/main.ts` — install router (Task 1).
- `frontend/src/App.vue` — `<RouterView/>` + theme + `Connect` (Task 1).
- `frontend/src/components/Field.vue` — label/hint/error wrapper (Task 2).
- `frontend/src/stores/wallet.ts` — add `generateMnemonic`/`importMnemonic`/`importKeystore`/`pickKeystoreFile` (Task 2).
- `frontend/src/views/Unlock.vue` — replaces A's minimal Unlock (Task 3).
- `frontend/src/views/Create.vue` — new (Task 4).
- `frontend/src/views/ImportMnemonic.vue` — new (Task 5).
- `frontend/src/views/Home.vue` — UNCHANGED (A's de-risk Home; registered as the `home` route placeholder until B2).
- Tests colocated: `router/index.test.ts`, `stores/wallet.test.ts` (extend), `views/{Unlock,Create,ImportMnemonic}.test.ts`.

---

## Task 1: vue-router foundation + lock guard

**Files:** Create `frontend/src/router/index.ts`, `frontend/src/router/index.test.ts`; Modify `frontend/src/main.ts`, `frontend/src/App.vue`.

**Interfaces:**
- Produces: `router` (default export of `src/router/index.ts`); routes named `unlock`, `create`, `import`, `home`; `PUBLIC_ROUTES = ['unlock','create','import']`. The guard redirects locked→`unlock`, unlocked-on-public→`home`.

- [ ] **Step 1: Install vue-router**

Run: `cd frontend && pnpm add vue-router@4`
Expected: added to `dependencies`.

- [ ] **Step 2: Write `src/router/index.ts`**

```ts
import { createRouter, createMemoryHistory, type RouteRecordRaw } from 'vue-router'
import { useWalletStore } from '../stores/wallet'

// Public routes are reachable while the wallet is locked. Everything else is
// gated. Routes are lazy-loaded so each screen code-splits and a future plugin
// can register more via router.addRoute().
export const PUBLIC_ROUTES = ['unlock', 'create', 'import']

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: { name: 'unlock' } },
  { path: '/unlock', name: 'unlock', component: () => import('../views/Unlock.vue') },
  { path: '/create', name: 'create', component: () => import('../views/Create.vue') },
  { path: '/import', name: 'import', component: () => import('../views/ImportMnemonic.vue') },
  { path: '/home', name: 'home', component: () => import('../views/Home.vue') },
]

const router = createRouter({ history: createMemoryHistory(), routes })

router.beforeEach((to) => {
  // Instantiate the store inside the guard (after app.use(pinia) has run).
  const wallet = useWalletStore()
  const isPublic = PUBLIC_ROUTES.includes(to.name as string)
  if (wallet.locked && !isPublic) return { name: 'unlock' }
  if (!wallet.locked && isPublic) return { name: 'home' }
  return true
})

export default router
```

- [ ] **Step 3: Update `src/main.ts`**

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './style.css'

createApp(App).use(createPinia()).use(router).mount('#app')
```

- [ ] **Step 4: Update `src/App.vue`** (replace A's `v-if` switch)

```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { useTheme } from 'nom-ui'
import * as N from '../wailsjs/go/app/NodeService'
const { setTheme } = useTheme()
onMounted(async () => {
  setTheme?.('dark')
  try { await N.Connect() } catch { /* best-effort; screens work offline */ }
})
</script>

<template>
  <RouterView />
</template>
```

- [ ] **Step 5: Write `src/router/index.test.ts`**

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useWalletStore } from '../stores/wallet'

// Stub the lazy-loaded views so navigation in the guard test doesn't pull the
// real nom-ui components into jsdom — we're testing the guard, not the screens.
vi.mock('../views/Unlock.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('../views/Create.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('../views/ImportMnemonic.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('../views/Home.vue', () => ({ default: { template: '<div/>' } }))
import router, { PUBLIC_ROUTES } from './index'

beforeEach(() => setActivePinia(createPinia()))

describe('router lock guard', () => {
  it('redirects a locked wallet away from gated routes to unlock', async () => {
    useWalletStore().locked = true
    await router.push('/home')
    expect(router.currentRoute.value.name).toBe('unlock')
  })
  it('redirects an unlocked wallet away from public routes to home', async () => {
    useWalletStore().locked = false
    await router.push('/unlock')
    expect(router.currentRoute.value.name).toBe('home')
  })
  it('lists the public routes', () => {
    expect(PUBLIC_ROUTES).toEqual(['unlock', 'create', 'import'])
  })
})
```

- [ ] **Step 6: Run tests + typecheck + build**

Run: `cd frontend && pnpm test -- src/router && pnpm run typecheck && pnpm run build`
Expected: guard tests pass; vue-tsc clean; vite build OK. (A's `Unlock.vue`/`Home.vue` still exist, so the lazy imports resolve; Task 3 replaces Unlock.)

- [ ] **Step 7: Stage** — `git add frontend/src/router frontend/src/main.ts frontend/src/App.vue frontend/package.json frontend/pnpm-lock.yaml`. Do NOT commit (controller signs). Verify no `go.mod`/`go.sum` changes.

---

## Task 2: `Field.vue` + wallet-store lifecycle actions

**Files:** Create `frontend/src/components/Field.vue`; Modify `frontend/src/stores/wallet.ts`, `frontend/src/stores/wallet.test.ts`.

**Interfaces:**
- Produces:
  - `Field.vue` — props `label?: string`, `hint?: string`, `error?: string`; renders `<label>{label}</label>` + default slot + an error (`text-destructive`) or hint (`text-muted-foreground`) line.
  - `wallet` store actions: `generateMnemonic(): Promise<string>`; `importMnemonic(file: string, password: string, mnemonic: string): Promise<void>` (persist only — caller unlocks); `importKeystore(srcPath: string): Promise<void>` (persist + refresh wallets); `pickKeystoreFile(): Promise<string>` (`''` if cancelled). Existing `unlock`/`loadWallets`/`lock` unchanged.

- [ ] **Step 1: Write `src/components/Field.vue`**

```vue
<script setup lang="ts">
defineProps<{ label?: string; hint?: string; error?: string }>()
</script>

<template>
  <label class="block space-y-1">
    <span v-if="label" class="text-sm text-foreground">{{ label }}</span>
    <slot />
    <span v-if="error" class="block text-xs text-destructive" role="alert">{{ error }}</span>
    <span v-else-if="hint" class="block text-xs text-muted-foreground">{{ hint }}</span>
  </label>
</template>
```

- [ ] **Step 2: Extend `src/stores/wallet.ts`** (keep existing state + loadWallets/unlock/lock; add the 4 actions)

Add inside `actions: { ... }` (alongside the existing ones):

```ts
    async generateMnemonic(): Promise<string> {
      return await W.GenerateMnemonic()
    },
    // Persist a new keystore from a mnemonic. Does NOT unlock — the caller
    // unlocks afterward (mirrors the Svelte create/import flow). Throws on error.
    async importMnemonic(file: string, password: string, mnemonic: string): Promise<void> {
      await W.ImportMnemonic(file, password, mnemonic)
      await this.loadWallets()
    },
    // Import an existing keystore file; wallet stays locked (user then unlocks).
    async importKeystore(srcPath: string): Promise<void> {
      await W.ImportKeystore(srcPath)
      await this.loadWallets()
    },
    async pickKeystoreFile(): Promise<string> {
      return (await W.PickKeystoreFile()) || ''
    },
```

(`W` is the existing `import * as W from '../../wailsjs/go/app/WalletService'` at the top of the file.)

- [ ] **Step 3: Extend `src/stores/wallet.test.ts`** — add the lifecycle mocks + a test. Update the existing `vi.mock` factory to include the new bindings (keep `ListWallets`/`Unlock`/`Lock`):

```ts
const Lock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const GenerateMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('w1 w2 w3'))
const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'New.dat' }))
const ImportKeystore = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'Old.dat' }))
const PickKeystoreFile = vi.hoisted(() => vi.fn().mockResolvedValue('/tmp/k.dat'))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
  Lock,
  GenerateMnemonic,
  ImportMnemonic,
  ImportKeystore,
  PickKeystoreFile,
}))
```

Add a test (keep the existing `lists wallets and unlocks` + `lock()` tests):

```ts
  it('lifecycle actions call the bindings', async () => {
    const s = useWalletStore()
    expect(await s.generateMnemonic()).toBe('w1 w2 w3')
    await s.importMnemonic('New.dat', 'pw', 'w1 w2 w3')
    expect(ImportMnemonic).toHaveBeenCalledWith('New.dat', 'pw', 'w1 w2 w3')
    await s.importKeystore('/tmp/k.dat')
    expect(ImportKeystore).toHaveBeenCalledWith('/tmp/k.dat')
    expect(await s.pickKeystoreFile()).toBe('/tmp/k.dat')
  })
```

- [ ] **Step 4: Run tests + typecheck**

Run: `cd frontend && pnpm test -- src/stores/wallet && pnpm run typecheck`
Expected: wallet store tests pass (existing + new); vue-tsc clean.

- [ ] **Step 5: Stage** — `git add frontend/src/components/Field.vue frontend/src/stores/wallet.ts frontend/src/stores/wallet.test.ts`. Do NOT commit.

---

## Task 3: `Unlock.vue` (full — replaces A's minimal version)

**Files:** Modify `frontend/src/views/Unlock.vue` (overwrite A's); Modify `frontend/src/views/Unlock.test.ts` (overwrite A's).

**Interfaces:**
- Consumes: `useWalletStore` (`loadWallets`, `wallets`, `unlock`, `importKeystore`, `pickKeystoreFile`), `useRouter`, nom-ui `Button`/`Card`/`CardContent`/`Input`.

- [ ] **Step 1: Overwrite `src/views/Unlock.vue`**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'

const wallet = useWalletStore()
const router = useRouter()
const selected = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

onMounted(async () => {
  await wallet.loadWallets()
  if (!selected.value && wallet.wallets[0]) selected.value = wallet.wallets[0]
})

async function doUnlock() {
  error.value = ''
  busy.value = true
  try {
    await wallet.unlock(selected.value, password.value)
    router.push('/home')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    busy.value = false
    password.value = ''
  }
}

async function doImport() {
  error.value = ''
  try {
    const path = await wallet.pickKeystoreFile()
    if (!path) return
    await wallet.importKeystore(path)
    if (!selected.value && wallet.wallets[0]) selected.value = wallet.wallets[0]
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  }
}
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="w-96">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Unlock wallet</h1>
        <p v-if="wallet.wallets.length === 0" class="text-muted-foreground">
          No wallets yet. Import a keystore to begin.
        </p>
        <template v-else>
          <select
            v-model="selected"
            aria-label="wallet"
            class="w-full rounded border border-border bg-background px-3 py-2 text-foreground">
            <option v-for="w in wallet.wallets" :key="w" :value="w">{{ w }}</option>
          </select>
          <Input v-model="password" type="password" placeholder="Password" aria-label="password" @keyup.enter="doUnlock" />
          <Button class="w-full" :disabled="busy || !selected" aria-label="Unlock" @click="doUnlock">Unlock</Button>
        </template>
        <Button variant="outline" class="w-full" @click="doImport">Import keystore…</Button>
        <Button variant="outline" class="w-full" @click="router.push('/create')">Create new wallet</Button>
        <Button variant="outline" class="w-full" @click="router.push('/import')">Import mnemonic</Button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
```
(Verify nom-ui `Button` supports `variant="outline"`; if not, use the default variant — cosmetic only. Verify `Input` `type="password"` passes through; A confirmed `Input` + `v-model`.)

- [ ] **Step 2: Overwrite `src/views/Unlock.test.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]),
  Unlock: unlock,
  Lock: vi.fn(),
  ImportKeystore: vi.fn().mockResolvedValue({ name: 'Main' }),
  PickKeystoreFile: vi.fn().mockResolvedValue(''),
}))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { template: '<button @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import Unlock from './Unlock.vue'

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
})

describe('Unlock.vue', () => {
  it('unlocks the selected wallet and routes home', async () => {
    const w = mount(Unlock)
    await new Promise((r) => setTimeout(r)) // loadWallets
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('button[aria-label="Unlock"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(unlock).toHaveBeenCalledWith('Main', 'pw')
    expect(push).toHaveBeenCalledWith('/home')
  })
})
```

- [ ] **Step 3: Run + typecheck**

Run: `cd frontend && pnpm test -- src/views/Unlock && pnpm run typecheck`
Expected: pass; clean.

- [ ] **Step 4: Stage** — `git add frontend/src/views/Unlock.vue frontend/src/views/Unlock.test.ts`. Do NOT commit.

---

## Task 4: `Create.vue` (3-step create)

**Files:** Create `frontend/src/views/Create.vue`, `frontend/src/views/Create.test.ts`.

**Interfaces:**
- Consumes: `useWalletStore` (`generateMnemonic`, `importMnemonic`, `unlock`), `useRouter`, nom-ui `Card`/`CardContent`/`Input`/`Button`.

- [ ] **Step 1: Write `src/views/Create.vue`** (faithful port of `Create.svelte`: generate → backup-verify 3 random words → name/password)

```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'

const wallet = useWalletStore()
const router = useRouter()

const step = ref(1)
const mnemonic = ref('')
const words = ref<string[]>([])
const positions = ref<number[]>([])
const answers = ref<Record<number, string>>({})
const name = ref('')
const password = ref('')
const confirm = ref('')
const error = ref('')

onMounted(async () => {
  try {
    mnemonic.value = await wallet.generateMnemonic()
    words.value = mnemonic.value.split(/\s+/)
    const idx = new Set<number>()
    while (idx.size < 3) idx.add(Math.floor(Math.random() * words.value.length))
    positions.value = [...idx].sort((a, b) => a - b)
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  }
})

const verifyOk = computed(
  () => positions.value.length === 3 && positions.value.every((p) => (answers.value[p] ?? '').trim() === words.value[p]),
)
const canCreate = computed(() => name.value.trim() !== '' && password.value.length > 0 && password.value === confirm.value)

function fileName(): string {
  return name.value.endsWith('.dat') ? name.value : name.value + '.dat'
}

async function finish() {
  error.value = ''
  try {
    const fn = fileName()
    await wallet.importMnemonic(fn, password.value, mnemonic.value)
    await wallet.unlock(fn, password.value)
    router.push('/home')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    password.value = ''
    confirm.value = ''
  }
}
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="w-[32rem]">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Create wallet</h1>

        <template v-if="step === 1">
          <p class="text-sm text-destructive">
            Write these {{ words.length }} words down and store them safely. Anyone with them controls your funds.
            They are shown only once.
          </p>
          <div class="grid grid-cols-3 gap-2 rounded bg-background p-3 font-mono text-sm text-foreground">
            <div v-for="(wd, i) in words" :key="i"><span class="text-muted-foreground">{{ i + 1 }}.</span> {{ wd }}</div>
          </div>
          <Button class="w-full" @click="step = 2">I've backed it up</Button>
        </template>

        <template v-else-if="step === 2">
          <p class="text-sm text-muted-foreground">Confirm your backup — enter these words:</p>
          <label v-for="p in positions" :key="p" class="block text-sm text-muted-foreground">
            Word #{{ p + 1 }}
            <Input v-model="answers[p]" :aria-label="`word ${p + 1}`" class="mt-1 font-mono" />
          </label>
          <Button class="w-full" :disabled="!verifyOk" @click="step = 3">Continue</Button>
        </template>

        <template v-else>
          <Input v-model="name" placeholder="Wallet name" aria-label="wallet name" />
          <Input v-model="password" type="password" placeholder="Password" aria-label="password" />
          <Input v-model="confirm" type="password" placeholder="Confirm password" aria-label="confirm password" />
          <Button class="w-full" :disabled="!canCreate" @click="finish">Create wallet</Button>
        </template>

        <button class="text-xs text-muted-foreground" @click="router.push('/unlock')">Cancel</button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
```

- [ ] **Step 2: Write `src/views/Create.test.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const GenerateMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('alpha bravo charlie'))
const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'New.dat' }))
const Unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([]),
  GenerateMnemonic,
  ImportMnemonic,
  Unlock,
  Lock: vi.fn(),
}))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import Create from './Create.vue'

beforeEach(() => { setActivePinia(createPinia()); push.mockClear() })

describe('Create.vue', () => {
  it('generates a mnemonic and creates the wallet through the 3 steps', async () => {
    const w = mount(Create)
    await new Promise((r) => setTimeout(r)) // generateMnemonic
    expect(GenerateMnemonic).toHaveBeenCalled()
    expect(w.text()).toContain('alpha')

    // Step 1 -> 2
    await w.findAll('button').find((b) => b.text() === "I've backed it up")!.trigger('click')
    // Step 2: answer each prompted word position correctly
    const words = ['alpha', 'bravo', 'charlie']
    for (const input of w.findAll('input')) {
      const label = input.attributes('aria-label') || ''
      const m = label.match(/^word (\d+)$/)
      if (m) await input.setValue(words[Number(m[1]) - 1])
    }
    await w.findAll('button').find((b) => b.text() === 'Continue')!.trigger('click')
    // Step 3: name + matching passwords
    await w.find('input[aria-label="wallet name"]').setValue('New')
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('input[aria-label="confirm password"]').setValue('pw')
    await w.findAll('button').find((b) => b.text() === 'Create wallet')!.trigger('click')
    await new Promise((r) => setTimeout(r))

    expect(ImportMnemonic).toHaveBeenCalledWith('New.dat', 'pw', 'alpha bravo charlie')
    expect(Unlock).toHaveBeenCalledWith('New.dat', 'pw')
    expect(push).toHaveBeenCalledWith('/home')
  })
})
```

- [ ] **Step 3: Run + typecheck**

Run: `cd frontend && pnpm test -- src/views/Create && pnpm run typecheck`
Expected: pass; clean.

- [ ] **Step 4: Stage** — `git add frontend/src/views/Create.vue frontend/src/views/Create.test.ts`. Do NOT commit.

---

## Task 5: `ImportMnemonic.vue`

**Files:** Create `frontend/src/views/ImportMnemonic.vue`, `frontend/src/views/ImportMnemonic.test.ts`.

**Interfaces:**
- Consumes: `useWalletStore` (`importMnemonic`, `unlock`), `useRouter`, nom-ui `Card`/`CardContent`/`Input`/`Button`.

- [ ] **Step 1: Write `src/views/ImportMnemonic.vue`** (faithful port of `ImportMnemonic.svelte`)

```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'

const wallet = useWalletStore()
const router = useRouter()
const mnemonic = ref('')
const name = ref('')
const password = ref('')
const confirm = ref('')
const error = ref('')

const wordCount = computed(() => mnemonic.value.trim().split(/\s+/).filter(Boolean).length)
const looksValid = computed(() => wordCount.value === 12 || wordCount.value === 24)
const canImport = computed(
  () => looksValid.value && name.value.trim() !== '' && password.value.length > 0 && password.value === confirm.value,
)

async function doImport() {
  error.value = ''
  const file = name.value.endsWith('.dat') ? name.value : name.value + '.dat'
  try {
    await wallet.importMnemonic(file, password.value, mnemonic.value.trim())
    await wallet.unlock(file, password.value)
    router.push('/home')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    password.value = ''
    confirm.value = ''
  }
}
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="w-[32rem]">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Import from mnemonic</h1>
        <textarea
          v-model="mnemonic"
          rows="3"
          placeholder="word1 word2 …"
          aria-label="mnemonic"
          class="w-full rounded border border-border bg-background p-3 font-mono text-sm text-foreground"></textarea>
        <p v-if="mnemonic && !looksValid" class="text-xs text-destructive">Expected 12 or 24 words ({{ wordCount }})</p>
        <Input v-model="name" placeholder="Wallet name" aria-label="wallet name" />
        <Input v-model="password" type="password" placeholder="Password" aria-label="password" />
        <Input v-model="confirm" type="password" placeholder="Confirm password" aria-label="confirm password" />
        <Button class="w-full" :disabled="!canImport" aria-label="Import" @click="doImport">Import</Button>
        <button class="text-xs text-muted-foreground" @click="router.push('/unlock')">Cancel</button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
```

- [ ] **Step 2: Write `src/views/ImportMnemonic.test.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'Imp.dat' }))
const Unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([]),
  ImportMnemonic,
  Unlock,
  Lock: vi.fn(),
}))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import ImportMnemonic_ from './ImportMnemonic.vue'

beforeEach(() => { setActivePinia(createPinia()); push.mockClear() })

describe('ImportMnemonic.vue', () => {
  it('imports a 12-word mnemonic and routes home', async () => {
    const w = mount(ImportMnemonic_)
    const twelve = 'a b c d e f g h i j k l'
    await w.find('textarea[aria-label="mnemonic"]').setValue(twelve)
    await w.find('input[aria-label="wallet name"]').setValue('Imp')
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('input[aria-label="confirm password"]').setValue('pw')
    await w.find('button[aria-label="Import"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(ImportMnemonic).toHaveBeenCalledWith('Imp.dat', 'pw', twelve)
    expect(Unlock).toHaveBeenCalledWith('Imp.dat', 'pw')
    expect(push).toHaveBeenCalledWith('/home')
  })
})
```

- [ ] **Step 3: Run full suite + typecheck + build**

Run: `cd frontend && pnpm test && pnpm run typecheck && pnpm run build`
Expected: ALL tests pass (router + stores + 3 views + A's format/node tests); vue-tsc clean; vite build OK.

- [ ] **Step 4: Stage** — `git add frontend/src/views/ImportMnemonic.vue frontend/src/views/ImportMnemonic.test.ts`. Do NOT commit.

---

## Self-Review / Verification (B1)

- `cd frontend && pnpm test` green (router guard, wallet store lifecycle, Unlock/Create/Import views, plus A's tests); `pnpm run typecheck` clean; `pnpm run build` succeeds.
- **Live `wails dev` gate (controller):** Create a throwaway wallet (generate → verify 3 words → name/password → lands on Home), Lock, Unlock it again, and Import a known 12/24-word mnemonic — each lands on Home with the right account. The lock guard keeps `/home` unreachable while locked.
- No `app/*.go` / `internal/*` changes; `go.mod`/`go.sum` 2.12.0 churn not committed.
- Faithful to the Svelte flows (steps, validation, `.dat` rule, copy); errors surfaced inline; password fields cleared after submit.

## Hand-off to B2

B2 replaces the placeholder `home` route with the real Home (account bar, 4-card row, status strip, 7 tab panels, tx history) + Send/Receive modals (confirm-what-you-sign via `formatAmountExact`), registering panels as the Home's tabs and adding the balances/tx/txs/unreceived/token/plasma Pinia stores.
