# Intro Splash Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Play the `zn-logo.json` Lottie animation as a full-screen splash the first time the app is ever opened, persisting a flag so later launches show nothing.

**Architecture:** Inline the external PNG into the Lottie JSON so it is self-contained, move it into `frontend/src/assets/`, and delete it from the repo root. Add a single-purpose `IntroSplash.vue` overlay that lazily loads `lottie-web` + the JSON, plays once, and emits `done` on completion or skip. `App.vue` shows it only when `localStorage['zn:introSeen']` is unset and sets that flag on dismissal.

**Tech Stack:** Vue 3 + TypeScript, Vite, `lottie-web`, Vitest + @vue/test-utils, pnpm 10.17.1.

## Global Constraints

- All `go`/`wails` commands (not needed here) would require `GOWORK=off`; this feature is frontend-only — **no Go/Wails backend changes**.
- Frontend commands run from `frontend/` with pnpm: `pnpm run typecheck` (vue-tsc), `pnpm test` (vitest run), `pnpm run build` (Vite).
- `lottie-web` is loaded **only via dynamic `import()`** so it is code-split and never fetched on non-first launches.
- The `localStorage` flag key is exactly `zn:introSeen`, value `'1'` when seen.
- Match existing component style in `frontend/src/components/*.vue` (`<script setup lang="ts">`, scoped/Tailwind).
- Do not touch the `animation/` working folder.

---

### Task 1: Inline the PNG and relocate the Lottie asset

**Files:**
- Create: `frontend/src/assets/zn-logo.json` (self-contained Lottie)
- Delete: `zn-logo.json` (repo root), `zn-logo.png` (repo root)

**Interfaces:**
- Consumes: nothing.
- Produces: `frontend/src/assets/zn-logo.json` — a Lottie animation whose `assets[0]` carries the image inline (`assets[0].e === 1`, `assets[0].u === ''`, `assets[0].p` starts with `data:image/png;base64,`). Task 2 imports this file.

- [ ] **Step 1: Inline the PNG into the JSON and write it to the assets folder**

Run from the repo root (`/Users/dfriestedt/Github/go-syrius`):

```bash
node -e '
const fs = require("fs");
const j = JSON.parse(fs.readFileSync("zn-logo.json", "utf8"));
const b64 = fs.readFileSync("zn-logo.png").toString("base64");
const img = j.assets.find(a => a.p === "img_0.png");
if (!img) throw new Error("img_0.png asset not found in zn-logo.json");
img.p = "data:image/png;base64," + b64;
img.u = "";
img.e = 1;
fs.writeFileSync("frontend/src/assets/zn-logo.json", JSON.stringify(j));
console.log("wrote frontend/src/assets/zn-logo.json,", "p starts:", img.p.slice(0, 24));
'
```

Expected output includes: `p starts: data:image/png;base64,`

- [ ] **Step 2: Verify the new asset is self-contained and parses**

```bash
node -e '
const j = require("./frontend/src/assets/zn-logo.json");
const img = j.assets.find(a => a.e === 1);
if (!img) throw new Error("no embedded asset (e:1) found");
if (!img.p.startsWith("data:image/png;base64,")) throw new Error("p is not a data URI");
if (img.u !== "") throw new Error("u not cleared");
console.log("OK self-contained; frames op=", j.op, "fr=", j.fr);
'
```

Expected: `OK self-contained; frames op= 180 fr= 30`

- [ ] **Step 3: Delete the root copies**

```bash
rm zn-logo.json zn-logo.png
ls zn-logo.json zn-logo.png 2>&1 || echo "root copies gone"
```

Expected: `root copies gone` (the `ls` errors because the files no longer exist).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/assets/zn-logo.json
git add -u zn-logo.json zn-logo.png
git commit -m "feat(assets): inline png into zn-logo Lottie and move into frontend/src/assets

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: IntroSplash component (TDD)

**Files:**
- Create: `frontend/src/components/IntroSplash.vue`
- Test: `frontend/src/components/IntroSplash.test.ts`
- Modify: `frontend/package.json` (add `lottie-web` dependency)

**Interfaces:**
- Consumes: `frontend/src/assets/zn-logo.json` (from Task 1); `lottie-web` default export `lottie.loadAnimation({...})` returning an object with `.addEventListener(name, cb)` and `.destroy()`.
- Produces: `IntroSplash.vue` — a component with **no props** that emits `done` exactly once (on Lottie `complete`, on overlay click, or on Esc keydown). Task 3 mounts it as `<IntroSplash @done="..." />`.

- [ ] **Step 1: Add the lottie-web dependency**

Run from `frontend/`:

```bash
pnpm add lottie-web
```

Expected: `package.json` gains `lottie-web` under dependencies; lockfile updates.

- [ ] **Step 2: Write the failing test**

Create `frontend/src/components/IntroSplash.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Capture the registered Lottie event handlers so tests can fire them, and the
// destroy spy so we can assert cleanup. lottie-web is the default export.
const handlers: Record<string, () => void> = {}
const destroy = vi.fn()
const loadAnimation = vi.fn(() => ({
  addEventListener: (name: string, cb: () => void) => {
    handlers[name] = cb
  },
  destroy,
}))
vi.mock('lottie-web', () => ({ default: { loadAnimation } }))
// The JSON is large; stub it so the test stays fast and decoupled from the asset.
vi.mock('../assets/zn-logo.json', () => ({ default: { v: '5.1.13', op: 180 } }))

import IntroSplash from './IntroSplash.vue'

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  loadAnimation.mockClear()
  destroy.mockClear()
})

// loadAnimation runs in onMounted via a dynamic import; flush microtasks.
async function mountSplash() {
  const w = mount(IntroSplash, { attachTo: document.body })
  await new Promise((r) => setTimeout(r, 0))
  await w.vm.$nextTick()
  return w
}

describe('IntroSplash', () => {
  it('loads the Lottie animation on mount', async () => {
    await mountSplash()
    expect(loadAnimation).toHaveBeenCalledTimes(1)
    const arg = loadAnimation.mock.calls[0][0] as Record<string, unknown>
    expect(arg.loop).toBe(false)
    expect(arg.autoplay).toBe(true)
  })

  it('emits done when the Lottie complete event fires', async () => {
    const w = await mountSplash()
    handlers['complete']?.()
    expect(w.emitted('done')).toBeTruthy()
  })

  it('emits done on overlay click (skip)', async () => {
    const w = await mountSplash()
    await w.trigger('click')
    expect(w.emitted('done')).toBeTruthy()
  })

  it('emits done on Esc keydown (skip)', async () => {
    const w = await mountSplash()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await w.vm.$nextTick()
    expect(w.emitted('done')).toBeTruthy()
  })

  it('emits done only once even if complete and skip both fire', async () => {
    const w = await mountSplash()
    handlers['complete']?.()
    await w.trigger('click')
    expect(w.emitted('done')).toHaveLength(1)
  })
})
```

- [ ] **Step 3: Run the test to verify it fails**

Run from `frontend/`:

```bash
pnpm test -- IntroSplash
```

Expected: FAIL — cannot resolve `./IntroSplash.vue` (file does not exist yet).

- [ ] **Step 4: Write the component**

Create `frontend/src/components/IntroSplash.vue`:

```vue
<!-- src/components/IntroSplash.vue -->
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// Single job: play the Zenon logo Lottie once on first launch, then tell the
// parent we're done — whether the animation completed or the user skipped.
const emit = defineEmits<{ done: [] }>()

const container = ref<HTMLDivElement | null>(null)
const leaving = ref(false)
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let anim: { destroy: () => void } | null = null
let finished = false

function finish() {
  if (finished) return
  finished = true
  // Brief fade-out, then hand control back to the app underneath.
  leaving.value = true
  window.setTimeout(() => emit('done'), 250)
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') finish()
}

onMounted(async () => {
  // Dynamic imports: lottie-web + the 130KB asset are code-split, so launches
  // that skip the splash never fetch them.
  const [{ default: lottie }, { default: animationData }] = await Promise.all([
    import('lottie-web'),
    import('../assets/zn-logo.json'),
  ])
  if (!container.value) return
  anim = lottie.loadAnimation({
    container: container.value,
    renderer: 'svg',
    loop: false,
    autoplay: true,
    animationData,
  }) as unknown as { addEventListener: (n: string, cb: () => void) => void; destroy: () => void }
  ;(anim as unknown as { addEventListener: (n: string, cb: () => void) => void }).addEventListener(
    'complete',
    finish,
  )
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  anim?.destroy()
})
</script>

<template>
  <div
    class="fixed inset-0 z-[9999] flex items-center justify-center bg-black transition-opacity duration-200"
    :class="leaving ? 'opacity-0' : 'opacity-100'"
    role="presentation"
    @click="finish"
  >
    <div ref="container" class="h-full max-h-[80vh] w-full max-w-[80vw]" />
  </div>
</template>
```

- [ ] **Step 5: Run the test to verify it passes**

Run from `frontend/`:

```bash
pnpm test -- IntroSplash
```

Expected: PASS — all 5 tests green.

- [ ] **Step 6: Typecheck**

Run from `frontend/`:

```bash
pnpm run typecheck
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/IntroSplash.vue frontend/src/components/IntroSplash.test.ts frontend/package.json frontend/pnpm-lock.yaml
git commit -m "feat(vue): IntroSplash component playing zn-logo Lottie once

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Wire the splash into App.vue

**Files:**
- Modify: `frontend/src/App.vue`
- Test: `frontend/src/App.test.ts`

**Interfaces:**
- Consumes: `IntroSplash` (from Task 2), emitting `done`.
- Produces: nothing downstream. App shows `<IntroSplash>` when `localStorage['zn:introSeen'] !== '1'` and sets that flag on `done`.

- [ ] **Step 1: Extend the App test (failing)**

Add to `frontend/src/App.test.ts`. First, stub `IntroSplash` so jsdom never pulls in `lottie-web`. Add this mock alongside the existing `vi.mock` calls near the top of the file:

```ts
vi.mock('./components/IntroSplash.vue', () => ({
  default: { name: 'IntroSplash', emits: ['done'], template: '<div data-test="intro" />' },
}))
```

Then append this describe block at the end of the file:

```ts
describe('App — intro splash', () => {
  beforeEach(() => localStorage.clear())

  it('shows the intro splash when the seen flag is absent', () => {
    useWalletStore().locked = true
    const w = mount(App)
    expect(w.find('[data-test="intro"]').exists()).toBe(true)
  })

  it('hides the intro splash when the seen flag is set', () => {
    localStorage.setItem('zn:introSeen', '1')
    useWalletStore().locked = true
    const w = mount(App)
    expect(w.find('[data-test="intro"]').exists()).toBe(false)
  })

  it('sets the seen flag and removes the splash on done', async () => {
    useWalletStore().locked = true
    const w = mount(App)
    w.findComponent({ name: 'IntroSplash' }).vm.$emit('done')
    await w.vm.$nextTick()
    expect(localStorage.getItem('zn:introSeen')).toBe('1')
    expect(w.find('[data-test="intro"]').exists()).toBe(false)
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `frontend/`:

```bash
pnpm test -- App.test
```

Expected: FAIL — the intro splash assertions fail because `App.vue` does not render `IntroSplash` yet.

- [ ] **Step 3: Modify App.vue**

In `frontend/src/App.vue`, update the `<script setup>` imports and add the splash state. Change the import line:

```ts
import { onMounted, ref, watch } from 'vue'
```

Add the `IntroSplash` import after the `useWalletStore` import:

```ts
import IntroSplash from './components/IntroSplash.vue'
```

Add the splash state and handler after the `const wallet = useWalletStore()` line:

```ts
// Show the logo intro only on the very first launch on this machine. The flag
// lives in localStorage; lottie-web + the asset are dynamically imported by
// IntroSplash, so later launches never load them.
const showIntro = ref(localStorage.getItem('zn:introSeen') !== '1')
function dismissIntro() {
  localStorage.setItem('zn:introSeen', '1')
  showIntro.value = false
}
```

Update the template to render the splash over everything:

```vue
<template>
  <RouterView />
  <Toaster />
  <IntroSplash v-if="showIntro" @done="dismissIntro" />
</template>
```

- [ ] **Step 4: Run the test to verify it passes**

Run from `frontend/`:

```bash
pnpm test -- App.test
```

Expected: PASS — existing lock test plus the three new intro tests green.

- [ ] **Step 5: Full verification**

Run from `frontend/`:

```bash
pnpm run typecheck && pnpm test && pnpm run build
```

Expected: typecheck clean, full vitest suite green, Vite build succeeds.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/App.vue frontend/src/App.test.ts
git commit -m "feat(vue): show IntroSplash on first launch, persist seen flag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Render via lottie-web → Task 2 (`loadAnimation`, dynamic import). ✓
- Play once, dismiss on completion → Task 2 (`loop:false`, `complete`→`finish`). ✓
- First-launch-only + `localStorage['zn:introSeen']` → Task 3. ✓
- Skippable (click + Esc) → Task 2. ✓
- Inline png, self-contained, move to `frontend/src/assets/zn-logo.json`, delete root copies → Task 1. ✓
- Dynamic import so non-first launches don't load lottie-web → Task 2 component + spec note. ✓
- Tests for IntroSplash and App → Tasks 2 & 3. ✓
- Acceptance: typecheck/test/build pass → Task 3 Step 5. ✓
- No Go/Wails changes → respected throughout. ✓

**Placeholder scan:** No TBD/TODO/"handle edge cases"; every code step shows full code. ✓

**Type consistency:** `finish()`, `dismissIntro()`, `showIntro`, emit name `done`, flag key `zn:introSeen`/value `'1'`, asset path `../assets/zn-logo.json`, `assets[0].e===1` used identically across tasks. ✓
