# Sentinel Launch Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the single-panel Sentinel registration into a stepped wizard (Deposit 50,000 QSR → wait → Register/Deposit 5,000 ZNN → wait → Active) with explicit "clearing" states, a withdraw escape hatch, and a polished active view.

**Architecture:** Split `SentinelsPanel.vue` into a container that picks `SentinelActive` vs `SentinelLaunch`. The wizard step derives from chain state (`depositedQsr`, `sentinel.active`); a `pendingStep` flag in the sentinel store drives transient "clearing" states by polling `refresh()` until the chain reflects the step. No backend changes — `NomService` already exposes every call.

**Tech Stack:** Vue 3 + TypeScript, Pinia, nom-ui, Vitest + @vue/test-utils, pnpm.

## Global Constraints

- Frontend-only; **no Go/Wails changes**. All commands run from `frontend/`: `pnpm run typecheck`, `pnpm test`, `pnpm run build`.
- Collateral amounts (verbatim): **50,000 QSR** = `5000000000000n` base units (1e8); **5,000 ZNN** (sent by `Register`, never separately queried).
- Poll cadence `POLL_INTERVAL_MS = 3000`; slow-hint threshold `SLOW_AFTER_POLLS = 6`.
- Use the existing **NoM-confirm pattern**: `tx.awaitConfirm(await Nom.Prepare…())` — components render no modal of their own (the global `NomConfirm` handles it).
- `tx` store status union: `'idle'|'preparing'|'awaiting'|'publishing'|'done'|'error'`.
- `SentinelInfo` fields: `owner: string`, `active: boolean`, `isRevocable: boolean`, `revokeCooldown: number`. `RewardInfo`: `znn: string`, `qsr: string`.
- Match existing nom-ui test stubs (Button forwards `disabled` + `@click`).

---

### Task 1: StepHeader (3-dot progress indicator)

**Files:**
- Create: `frontend/src/components/panels/StepHeader.vue`
- Test: `frontend/src/components/panels/StepHeader.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `StepHeader` — props `{ current: 1 | 2 | 3 }`. Each step span has `data-state` = `'done'` (n < current), `'current'` (n === current), or `'todo'` (n > current). Used by `SentinelLaunch` (Task 4).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/panels/StepHeader.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import StepHeader from './StepHeader.vue'

describe('StepHeader', () => {
  it('marks earlier steps done, the current step current, later steps todo', () => {
    const w = mount(StepHeader, { props: { current: 2 } })
    const states = w.findAll('[data-state]').map((n) => n.attributes('data-state'))
    expect(states).toEqual(['done', 'current', 'todo'])
  })

  it('labels the three sentinel launch stages', () => {
    const w = mount(StepHeader, { props: { current: 1 } })
    expect(w.text()).toContain('Deposit 50,000 QSR')
    expect(w.text()).toContain('Deposit 5,000 ZNN')
    expect(w.text()).toContain('Sentinel active')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `frontend/`: `pnpm test -- StepHeader`
Expected: FAIL — cannot resolve `./StepHeader.vue`.

- [ ] **Step 3: Write the component**

Create `frontend/src/components/panels/StepHeader.vue`:

```vue
<script setup lang="ts">
defineProps<{ current: 1 | 2 | 3 }>()
const STEPS = [
  { n: 1, label: 'Deposit 50,000 QSR' },
  { n: 2, label: 'Deposit 5,000 ZNN' },
  { n: 3, label: 'Sentinel active' },
] as const
</script>

<template>
  <ol class="flex flex-wrap items-center gap-2" aria-label="Sentinel launch progress">
    <li v-for="(s, i) in STEPS" :key="s.n" class="flex items-center gap-2">
      <span
        class="grid h-6 w-6 shrink-0 place-items-center rounded-full border text-xs font-medium"
        :data-state="s.n < current ? 'done' : s.n === current ? 'current' : 'todo'"
        :class="s.n < current
          ? 'border-primary bg-primary text-primary-foreground'
          : s.n === current
            ? 'border-primary text-primary'
            : 'border-border text-muted-foreground'"
      >
        <svg v-if="s.n < current" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
        <template v-else>{{ s.n }}</template>
      </span>
      <span
        class="whitespace-nowrap text-xs"
        :class="s.n === current ? 'font-medium text-foreground' : 'text-muted-foreground'"
      >{{ s.label }}</span>
      <span v-if="i < STEPS.length - 1" class="mx-1 hidden h-px w-6 bg-border sm:block" />
    </li>
  </ol>
</template>
```

- [ ] **Step 4: Run the test to verify it passes**

Run from `frontend/`: `pnpm test -- StepHeader`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/panels/StepHeader.vue frontend/src/components/panels/StepHeader.test.ts
git commit -m "feat(vue): StepHeader progress indicator for sentinel wizard"
```

---

### Task 2: Sentinel store — pending/poll machinery

**Files:**
- Modify: `frontend/src/stores/sentinel.ts`
- Test: `frontend/src/stores/sentinel.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `useSentinelStore` gains — exported `QSR_REQUIRED = 5000000000000n`; state `pendingStep: 'deposit'|'register'|null`, `pollCount: number`; getters `active: boolean`, `qsrCleared: boolean`; actions `beginPending(step: 'deposit'|'register')`, `settleCheck()`, `stopPolling()`. `refresh()` unchanged. Consumed by Tasks 3, 4, 5.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/stores/sentinel.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSentinelStore } from './sentinel'

// Don't touch the (unmocked) backend; refresh is stubbed per test.
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetSentinel: vi.fn(), GetDepositedQsr: vi.fn(), GetSentinelReward: vi.fn(),
}))

beforeEach(() => setActivePinia(createPinia()))

describe('sentinel store pending/poll', () => {
  it('beginPending(deposit) clears once deposited reaches 50,000 QSR', async () => {
    vi.useFakeTimers()
    const s = useSentinelStore()
    vi.spyOn(s, 'refresh').mockImplementation(async () => { s.depositedQsr = '5000000000000' })
    s.beginPending('deposit')
    expect(s.pendingStep).toBe('deposit')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('keeps polling (pendingStep stays) until the chain reflects the step', async () => {
    vi.useFakeTimers()
    const s = useSentinelStore()
    let credited = false
    vi.spyOn(s, 'refresh').mockImplementation(async () => { if (credited) s.depositedQsr = '5000000000000' })
    s.beginPending('deposit')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe('deposit')
    expect(s.pollCount).toBe(1)
    credited = true
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('beginPending(register) clears once the sentinel is active', async () => {
    vi.useFakeTimers()
    const s = useSentinelStore()
    vi.spyOn(s, 'refresh').mockImplementation(async () => { s.sentinel = { owner: 'z1own', active: true } as never })
    s.beginPending('register')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('stopPolling clears the pending state', () => {
    const s = useSentinelStore()
    s.beginPending('deposit')
    s.stopPolling()
    expect(s.pendingStep).toBe(null)
    expect(s.pollCount).toBe(0)
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `frontend/`: `pnpm test -- stores/sentinel`
Expected: FAIL — `beginPending`/`pendingStep` undefined.

- [ ] **Step 3: Rewrite the store**

Replace `frontend/src/stores/sentinel.ts` entirely with:

```ts
import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

// 50,000 QSR in base units (1e8) — the Sentinel QSR collateral.
export const QSR_REQUIRED = 5000000000000n
const POLL_INTERVAL_MS = 3000

export const useSentinelStore = defineStore('sentinel', {
  state: () => ({
    sentinel: null as app.SentinelInfo | null,
    depositedQsr: '0',
    reward: null as app.RewardInfo | null,
    // Transient "clearing" flag: a just-published step we're polling to settle.
    pendingStep: null as 'deposit' | 'register' | null,
    pollCount: 0,
    pollHandle: null as number | null,
  }),
  getters: {
    active(s): boolean {
      return !!s.sentinel && s.sentinel.owner !== ''
    },
    qsrCleared(s): boolean {
      try {
        return BigInt(s.depositedQsr || '0') >= QSR_REQUIRED
      } catch {
        return false
      }
    },
  },
  actions: {
    async refresh() {
      try {
        this.sentinel = await Nom.GetSentinel()
        this.depositedQsr = await Nom.GetDepositedQsr()
        this.reward = await Nom.GetSentinelReward()
      } catch {
        /* not connected / locked — leave as-is */
      }
    },
    // Start polling for a just-published step to settle on-chain, then advance.
    beginPending(step: 'deposit' | 'register') {
      this.stopPolling()
      this.pendingStep = step
      this.pollCount = 0
      this.pollHandle = window.setInterval(async () => {
        this.pollCount++
        await this.refresh()
        this.settleCheck()
      }, POLL_INTERVAL_MS)
    },
    // Clear the pending state once the chain reflects the step.
    settleCheck() {
      if (this.pendingStep === 'deposit' && this.qsrCleared) {
        this.stopPolling()
      } else if (this.pendingStep === 'register' && this.active) {
        this.stopPolling()
      }
    },
    // Stop polling and clear the pending state (settle, unmount, or cancel).
    stopPolling() {
      if (this.pollHandle !== null) {
        clearInterval(this.pollHandle)
        this.pollHandle = null
      }
      this.pendingStep = null
      this.pollCount = 0
    },
  },
})
```

- [ ] **Step 4: Run the test to verify it passes**

Run from `frontend/`: `pnpm test -- stores/sentinel`
Expected: PASS (4 tests).

- [ ] **Step 5: Typecheck**

Run from `frontend/`: `pnpm run typecheck`
Expected: no errors. (The old `SentinelsPanel.vue` still imports `useSentinelStore` and uses `sentinel`/`depositedQsr`/`reward`, all still present — it compiles until Task 5 rewrites it.)

- [ ] **Step 6: Commit**

```bash
git add frontend/src/stores/sentinel.ts frontend/src/stores/sentinel.test.ts
git commit -m "feat(vue): sentinel store pending-step polling for the launch wizard"
```

---

### Task 3: SentinelActive (polished management view)

**Files:**
- Create: `frontend/src/components/panels/SentinelActive.vue`
- Test: `frontend/src/components/panels/SentinelActive.test.ts`

**Interfaces:**
- Consumes: `useSentinelStore` (`sentinel`, `reward`), `useTxStore` (`awaitConfirm`, `status`, `error`), `NomService.PrepareCollectSentinelReward`/`PrepareRevokeSentinel`.
- Produces: `SentinelActive` — no props; renders the active-sentinel card. Used by `SentinelsPanel` (Task 5).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/panels/SentinelActive.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareCollectSentinelReward: vi.fn(() => Promise.resolve({ kind: 'collect' })),
  PrepareRevokeSentinel: vi.fn(() => Promise.resolve({ kind: 'revoke' })),
}))

import SentinelActive from './SentinelActive.vue'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'

function setup(reward: unknown, sentinel: unknown) {
  setActivePinia(createPinia())
  const s = useSentinelStore()
  const tx = useTxStore()
  vi.spyOn(s, 'refresh').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.reward = reward as never
  s.sentinel = sentinel as never
  return { s, awaitConfirm }
}

const REVOCABLE = { owner: 'z1own', active: true, isRevocable: true, revokeCooldown: 0 }

describe('SentinelActive', () => {
  it('disables Collect when the reward is zero', () => {
    setup({ znn: '0', qsr: '0' }, REVOCABLE)
    const w = mount(SentinelActive)
    const collect = w.findAll('button').find((b) => b.text() === 'Collect')!
    expect(collect.attributes('disabled')).toBeDefined()
  })

  it('disables Revoke with a cooldown note when not revocable', () => {
    setup({ znn: '0', qsr: '0' }, { owner: 'z1own', active: true, isRevocable: false, revokeCooldown: 42 })
    const w = mount(SentinelActive)
    const revoke = w.find('button[aria-label="revoke sentinel"]')
    expect(revoke.attributes('disabled')).toBeDefined()
    expect(revoke.text()).toContain('42')
  })

  it('forwards the collect call to tx.awaitConfirm', async () => {
    const { awaitConfirm } = setup({ znn: '100', qsr: '0' }, REVOCABLE)
    const w = mount(SentinelActive)
    await w.findAll('button').find((b) => b.text() === 'Collect')!.trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'collect' })
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `frontend/`: `pnpm test -- SentinelActive`
Expected: FAIL — cannot resolve `./SentinelActive.vue`.

- [ ] **Step 3: Write the component**

Create `frontend/src/components/panels/SentinelActive.vue`:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'

const sentinelStore = useSentinelStore()
const tx = useTxStore()
const { sentinel, reward } = storeToRefs(sentinelStore)
const error = ref('')

const rewardZero = computed(
  () => !reward.value || (reward.value.znn === '0' && reward.value.qsr === '0'),
)

async function collect() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCollectSentinelReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function revoke() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareRevokeSentinel())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// Refresh after a collect/revoke settles (reward updates; revoke flips active).
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') sentinelStore.refresh()
  },
)
</script>

<template>
  <section v-if="sentinel" class="space-y-3 rounded-lg border border-border bg-card p-4">
    <div class="flex items-center gap-2">
      <svg class="text-primary" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg>
      <h2 class="text-sm font-medium text-foreground">Your Sentinel</h2>
      <span class="rounded-full bg-primary/15 px-2 py-0.5 text-xs font-medium text-primary">
        {{ sentinel.active ? 'Active' : 'Inactive' }}
      </span>
    </div>
    <p class="text-sm text-muted-foreground">
      Your Sentinel is {{ sentinel.active ? 'active and earning rewards.' : 'registered.' }}
    </p>
    <p v-if="reward" class="text-sm text-muted-foreground">
      Uncollected reward
      <span class="font-mono text-foreground"
        >{{ formatAmount(reward.znn, 8) }} ZNN · {{ formatAmount(reward.qsr, 8) }} QSR</span
      >
    </p>
    <div class="flex flex-wrap items-center gap-2">
      <Button :disabled="rewardZero" @click="collect">Collect</Button>
      <Button
        variant="outline"
        :disabled="!sentinel.isRevocable"
        aria-label="revoke sentinel"
        @click="revoke"
        >Revoke<template v-if="!sentinel.isRevocable">
          (cooldown {{ sentinel.revokeCooldown }}s)</template
        ></Button
      >
    </div>
    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
  </section>
</template>
```

- [ ] **Step 4: Run the test to verify it passes**

Run from `frontend/`: `pnpm test -- SentinelActive`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/panels/SentinelActive.vue frontend/src/components/panels/SentinelActive.test.ts
git commit -m "feat(vue): SentinelActive management view"
```

---

### Task 4: SentinelLaunch (the stepped wizard)

**Files:**
- Create: `frontend/src/components/panels/SentinelLaunch.vue`
- Test: `frontend/src/components/panels/SentinelLaunch.test.ts`

**Interfaces:**
- Consumes: `StepHeader` (Task 1); `useSentinelStore` (`depositedQsr`, `pendingStep`, `pollCount`, `beginPending`, `refresh`, exported `QSR_REQUIRED`); `useTxStore` (`awaitConfirm`, `status`, `error`); `NomService.PrepareDepositQsr`/`PrepareRegisterSentinel`/`PrepareWithdrawQsr`.
- Produces: `SentinelLaunch` — no props; the launch wizard body. Used by `SentinelsPanel` (Task 5).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/panels/SentinelLaunch.test.ts`:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDepositQsr: vi.fn(() => Promise.resolve({ kind: 'deposit' })),
  PrepareRegisterSentinel: vi.fn(() => Promise.resolve({ kind: 'register' })),
  PrepareWithdrawQsr: vi.fn(() => Promise.resolve({ kind: 'withdraw' })),
}))

import SentinelLaunch from './SentinelLaunch.vue'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'

const CLEARED = '5000000000000' // 50,000 QSR

function setup(depositedQsr = '0', pendingStep: 'deposit' | 'register' | null = null) {
  setActivePinia(createPinia())
  const s = useSentinelStore()
  const tx = useTxStore()
  vi.spyOn(s, 'refresh').mockResolvedValue()
  const begin = vi.spyOn(s, 'beginPending').mockImplementation(() => {})
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.depositedQsr = depositedQsr
  s.pendingStep = pendingStep
  return { s, tx, begin, awaitConfirm }
}

describe('SentinelLaunch', () => {
  it('step 1: shows the deposit action when no QSR is deposited', () => {
    setup('0')
    const w = mount(SentinelLaunch)
    expect(w.find('button[aria-label="deposit qsr"]').exists()).toBe(true)
    expect(w.find('button[aria-label="register sentinel"]').exists()).toBe(false)
    expect(w.find('[data-state="current"]').text()).toContain('Deposit 50,000 QSR')
  })

  it('step 2: shows Register + the withdraw escape hatch once QSR clears', () => {
    setup(CLEARED)
    const w = mount(SentinelLaunch)
    expect(w.find('button[aria-label="register sentinel"]').exists()).toBe(true)
    expect(w.find('button[aria-label="withdraw qsr"]').exists()).toBe(true)
    expect(w.find('button[aria-label="deposit qsr"]').exists()).toBe(false)
  })

  it('clearing: shows the waiting message and hides actions while a step is pending', () => {
    setup('0', 'deposit')
    const w = mount(SentinelLaunch)
    expect(w.text()).toContain('Waiting for the Sentinel contract to credit it')
    expect(w.find('button[aria-label="deposit qsr"]').exists()).toBe(false)
  })

  it('forwards the deposit call and begins polling when it completes', async () => {
    const { tx, begin, awaitConfirm } = setup('0')
    const w = mount(SentinelLaunch)
    await w.find('button[aria-label="deposit qsr"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'deposit' })
    tx.status = 'done'
    await w.vm.$nextTick()
    expect(begin).toHaveBeenCalledWith('deposit')
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `frontend/`: `pnpm test -- SentinelLaunch`
Expected: FAIL — cannot resolve `./SentinelLaunch.vue`.

- [ ] **Step 3: Write the component**

Create `frontend/src/components/panels/SentinelLaunch.vue`:

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useSentinelStore, QSR_REQUIRED } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import StepHeader from './StepHeader.vue'

const SLOW_AFTER_POLLS = 6

const sentinelStore = useSentinelStore()
const tx = useTxStore()
const { depositedQsr, pendingStep, pollCount } = storeToRefs(sentinelStore)
const error = ref('')

const deposited = computed(() => {
  try {
    return BigInt(depositedQsr.value || '0')
  } catch {
    return 0n
  }
})
const shortfall = computed(() => (QSR_REQUIRED > deposited.value ? QSR_REQUIRED - deposited.value : 0n))
const cleared = computed(() => deposited.value >= QSR_REQUIRED)
const clearing = computed(() => pendingStep.value !== null)
const slow = computed(() => pendingStep.value !== null && pollCount.value >= SLOW_AFTER_POLLS)
const currentStep = computed<1 | 2 | 3>(() => (cleared.value ? 2 : 1))

// Remember which action we initiated so the tx-done watcher can begin polling.
let lastAction: 'deposit' | 'register' | null = null

async function depositQsr() {
  error.value = ''
  lastAction = 'deposit'
  try {
    tx.awaitConfirm(await Nom.PrepareDepositQsr(shortfall.value.toString()))
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function register() {
  error.value = ''
  lastAction = 'register'
  try {
    tx.awaitConfirm(await Nom.PrepareRegisterSentinel())
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function withdrawQsr() {
  // Withdraw returns to step 1 — no clearing wait, just refresh.
  error.value = ''
  lastAction = null
  try {
    tx.awaitConfirm(await Nom.PrepareWithdrawQsr())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// When the user's deposit/register publishes, poll for it to settle on-chain.
watch(
  () => tx.status,
  (s) => {
    if (s !== 'done') return
    if (lastAction === 'deposit' || lastAction === 'register') {
      sentinelStore.beginPending(lastAction)
    } else {
      sentinelStore.refresh()
    }
    lastAction = null
  },
)
</script>

<template>
  <section class="space-y-4 rounded-lg border border-border bg-card p-4">
    <StepHeader :current="currentStep" />

    <!-- Clearing (transient): waiting for the contract to credit/activate. -->
    <div v-if="clearing" class="space-y-2">
      <div class="flex items-center gap-2 text-sm font-medium text-info">
        <svg class="animate-spin" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        <span>{{
          pendingStep === 'deposit'
            ? 'Your QSR deposit is on-chain. Waiting for the Sentinel contract to credit it…'
            : 'Launching your Sentinel — waiting for activation…'
        }}</span>
      </div>
      <p class="text-xs text-muted-foreground">This usually takes a few momentums.</p>
      <div v-if="slow" class="flex items-center gap-2">
        <p class="text-xs text-muted-foreground">Taking longer than usual — the network may be busy.</p>
        <button
          type="button"
          class="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          @click="sentinelStore.refresh()"
        >
          Refresh
        </button>
      </div>
    </div>

    <!-- Step 1: deposit the QSR shortfall. -->
    <template v-else-if="!cleared">
      <p class="text-xs text-muted-foreground">
        A Sentinel needs 50,000 QSR + 5,000 ZNN collateral (returned on revocation).
      </p>
      <p class="text-sm text-muted-foreground">
        Deposited
        <span class="font-mono text-foreground">{{ formatAmount(depositedQsr, 8) }} / 50,000 QSR</span>
      </p>
      <Button class="w-full" aria-label="deposit qsr" @click="depositQsr"
        >Deposit {{ formatAmount(shortfall.toString(), 8) }} QSR</Button
      >
    </template>

    <!-- Step 2: register (sends 5,000 ZNN), with a withdraw escape hatch. -->
    <template v-else>
      <p class="text-sm text-foreground">✓ 50,000 QSR cleared. Ready to launch.</p>
      <Button class="w-full" aria-label="register sentinel" @click="register"
        >Deposit 5,000 ZNN &amp; Launch Sentinel</Button
      >
      <Button variant="outline" class="w-full" aria-label="withdraw qsr" @click="withdrawQsr"
        >Changed your mind? Withdraw your 50,000 QSR</Button
      >
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </section>
</template>
```

- [ ] **Step 4: Run the test to verify it passes**

Run from `frontend/`: `pnpm test -- SentinelLaunch`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/panels/SentinelLaunch.vue frontend/src/components/panels/SentinelLaunch.test.ts
git commit -m "feat(vue): SentinelLaunch stepped wizard with clearing states"
```

---

### Task 5: SentinelsPanel container + update its test

**Files:**
- Modify: `frontend/src/components/panels/SentinelsPanel.vue` (full rewrite)
- Modify: `frontend/src/components/panels/SentinelsPanel.test.ts` (rewrite for the container)

**Interfaces:**
- Consumes: `SentinelActive` (Task 3), `SentinelLaunch` (Task 4), `useSentinelStore` (`active` getter, `refresh`, `stopPolling`).
- Produces: the container rendered by Home's Sentinels tab — `SentinelActive` when `active`, else `SentinelLaunch`.

- [ ] **Step 1: Write the failing test**

Replace `frontend/src/components/panels/SentinelsPanel.test.ts` entirely with:

```ts
import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Stub the children so the container test asserts routing, not their internals.
vi.mock('./SentinelLaunch.vue', () => ({
  default: { name: 'SentinelLaunch', template: '<div data-test="launch" />' },
}))
vi.mock('./SentinelActive.vue', () => ({
  default: { name: 'SentinelActive', template: '<div data-test="active" />' },
}))

import SentinelsPanel from './SentinelsPanel.vue'
import { useSentinelStore } from '../../stores/sentinel'

function setup(sentinel: unknown) {
  setActivePinia(createPinia())
  const s = useSentinelStore()
  vi.spyOn(s, 'refresh').mockResolvedValue()
  s.sentinel = sentinel as never
  return s
}

describe('SentinelsPanel container', () => {
  it('renders the launch wizard when there is no active sentinel', () => {
    setup(null)
    const w = mount(SentinelsPanel)
    expect(w.find('[data-test="launch"]').exists()).toBe(true)
    expect(w.find('[data-test="active"]').exists()).toBe(false)
  })

  it('renders the active view when a sentinel is owned', () => {
    setup({ owner: 'z1own', active: true, isRevocable: true, revokeCooldown: 0 })
    const w = mount(SentinelsPanel)
    expect(w.find('[data-test="active"]').exists()).toBe(true)
    expect(w.find('[data-test="launch"]').exists()).toBe(false)
  })

  it('stops polling on unmount', () => {
    const s = setup(null)
    const stop = vi.spyOn(s, 'stopPolling')
    mount(SentinelsPanel).unmount()
    expect(stop).toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run from `frontend/`: `pnpm test -- panels/SentinelsPanel`
Expected: FAIL — current panel renders inline sections, not the stubbed children.

- [ ] **Step 3: Rewrite the container**

Replace `frontend/src/components/panels/SentinelsPanel.vue` entirely with:

```vue
<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useSentinelStore } from '../../stores/sentinel'
import SentinelLaunch from './SentinelLaunch.vue'
import SentinelActive from './SentinelActive.vue'

// Container: refresh on mount, then show the active view or the launch wizard.
// The step within the wizard is derived from chain state by the children.
const sentinelStore = useSentinelStore()
const { active } = storeToRefs(sentinelStore)

onMounted(() => sentinelStore.refresh())
onUnmounted(() => sentinelStore.stopPolling())
</script>

<template>
  <div class="space-y-4 p-4">
    <SentinelActive v-if="active" />
    <SentinelLaunch v-else />
  </div>
</template>
```

- [ ] **Step 4: Run the test to verify it passes**

Run from `frontend/`: `pnpm test -- panels/SentinelsPanel`
Expected: PASS (3 tests).

- [ ] **Step 5: Full verification**

Run from `frontend/`:

```bash
pnpm run typecheck && pnpm test && pnpm run build
```

Expected: typecheck clean, full vitest suite green (including the new StepHeader / sentinel store / SentinelActive / SentinelLaunch / SentinelsPanel tests), Vite build succeeds.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/panels/SentinelsPanel.vue frontend/src/components/panels/SentinelsPanel.test.ts
git commit -m "feat(vue): SentinelsPanel routes to launch wizard vs active view"
```

---

## Self-Review

**Spec coverage:**
- Stepped wizard (Deposit QSR → wait → Register → wait → Active) → Tasks 4 (wizard) + 1 (header). ✓
- Clearing states derived from chain + `pendingStep`, poll each interval, auto-advance → Task 2 (store) + Task 4 (bodies). ✓
- Withdraw escape hatch only in Step 2 → Task 4 (`withdraw qsr` button under the `cleared` branch). ✓
- Slow hint after `SLOW_AFTER_POLLS` polls with manual Refresh → Task 4 (`slow` computed). ✓
- Resume across restart (no `pendingStep` persistence; step from chain) → Task 2 state default `null` + Task 4 `cleared`/`currentStep` computeds. ✓
- Polished active view (status, reward, Collect/Revoke) → Task 3. ✓
- Component split (Panel container / Launch / Active / StepHeader) → Tasks 1,3,4,5. ✓
- No Go changes → respected. ✓
- Tests for each unit → Tasks 1–5 each ship a test. ✓
- Acceptance typecheck/test/build → Task 5 Step 5. ✓

**Placeholder scan:** No TBD/TODO/"handle edge cases"; every code step shows full code. ✓

**Type consistency:** `QSR_REQUIRED` (bigint, exported from `sentinel.ts`, imported in Task 4); `pendingStep: 'deposit'|'register'|null`, `pollCount`, `beginPending`/`settleCheck`/`stopPolling`, getters `active`/`qsrCleared` defined in Task 2 and used identically in Tasks 3/4/5; `tx.awaitConfirm`, `tx.status` `'done'`/`'error'`, `tx.error` consistent; `SentinelInfo.owner/active/isRevocable/revokeCooldown` and `RewardInfo.znn/qsr` used as defined; aria-labels (`deposit qsr`, `register sentinel`, `withdraw qsr`, `revoke sentinel`) match between components and tests. ✓
