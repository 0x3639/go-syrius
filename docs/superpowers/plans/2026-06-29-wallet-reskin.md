# Wallet Reskin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reskin the syrius wallet to the `zenon-design-system` sidebar + plasma-hero Dashboard layout, decompose the NoM tab strip into routed pages, add a USD price feed, migrate icons to Lucide, and fix the outstanding `/zenon-design` token violations.

**Architecture:** A persistent `AppShell` (Sidebar + TopBar + scrollable `<main>`) wraps all authenticated routes. `Home.vue`'s `Tabs` become individual routed pages reusing the existing panel components unchanged. A new `price` Pinia store fetches `api.zenon.info/price` for the portfolio total, degrading gracefully (ZNN-headline, no `≈$`) when the feed is unavailable. All inline `<svg>` icons are replaced with `@lucide/vue` components.

**Tech Stack:** Vue 3 + TypeScript, Vite, Tailwind CSS 4, Pinia, vue-router (memory history), nom-ui (tokens + primitives), `@lucide/vue@1.20.0`, vitest + @vue/test-utils.

## Global Constraints

- **No secrets in the WebView.** This branch is frontend-only; it touches no key material, no signing path, no Wails binding. The price fetch is a read-only public HTTPS GET.
- **Amounts:** always format balances with `lib/format.ts` (`formatAmount`/`formatAmountExact`) — **never** nom-ui `Amount` (it loses BigInt precision). Money is **not** colored.
- **Tokens only, no raw hex.** Brand green is `#00d557`. Semantic CSS vars / Tailwind token classes everywhere.
- **Plasma is reserved** to the Dashboard hero card + primary buttons. No `bg-gradient` on any other chrome.
- **Icons:** `@lucide/vue`, imported with the `*Icon` suffix (e.g. `import { SendIcon } from '@lucide/vue'`). No inline `<svg>`, no unicode-as-icon.
- **Ledger labels:** table headers / stat captions use the `.text-ledger` utility (mono, uppercase, tracked).
- **Theme:** every new surface must read correctly in both light and dark; dark is default.
- **Local commands need** `GOWORK=off` (and `GOTOOLCHAIN=auto` for Go). Frontend uses pnpm 10.17.1 in `frontend/`.
- **Bridge is omitted** from the nav entirely (no placeholder).
- Branch: `wallet-reskin`. Gates before any "done" claim: `pnpm run typecheck`, `pnpm test`, `pnpm run build`, and `GOWORK=off GOTOOLCHAIN=auto go build ./...`.

---

## Icon mapping (used across Tasks 3, 4, 9, 10, 11)

Replace inline SVGs with these `@lucide/vue` components:

| Concept | Lucide component | Used in |
|---|---|---|
| Dashboard | `LayoutDashboardIcon` | Sidebar |
| Transfer / Send | `SendIcon` | Sidebar, TopBar hero, Dashboard |
| Receive | `DownloadIcon` | Sidebar, Dashboard |
| Tokens | `CoinsIcon` | Sidebar |
| Plasma | `ZapIcon` | Sidebar, TopBar |
| Staking | `LayersIcon` | Sidebar |
| Pillars | `Building2Icon` | Sidebar |
| Sentinels | `ShieldCheckIcon` | Sidebar |
| Accelerator | `RocketIcon` | Sidebar, TopBar (votes badge) |
| Rewards | `GiftIcon` | Sidebar |
| Governance | `VoteIcon` | Sidebar |
| Settings | `SettingsIcon` | Sidebar |
| Address book | `BookUserIcon` | Sidebar, TopBar |
| Node-sync shield | `ShieldIcon` | Sidebar pill |
| Account address pill | `WalletIcon` | TopBar |
| Auto-receive | `ArrowDownCircleIcon` | TopBar |
| Theme (dark→light) | `SunIcon` / `MoonIcon` | TopBar |
| Notifications | `BellIcon` | TopBar |
| Lock | `LockIcon` | TopBar |
| Copy / copied | `CopyIcon` / `CheckIcon` | AccountSlotPicker, AddressDisplay |
| Chevron | `ChevronDownIcon` | AccountSlotPicker, WalletPicker |
| Add | `PlusIcon` | AccountSlotPicker |
| Rename | `PencilIcon` | AccountSlotPicker |
| Save / cancel (✓ / ✕) | `CheckIcon` / `XIcon` | AccountSlotPicker, WalletPicker, ContactPicker |
| QR | `QrCodeIcon` | Receive page |

---

## Task 1: Price store + fiat formatter

**Files:**
- Create: `frontend/src/lib/fiat.ts`
- Create: `frontend/src/stores/price.ts`
- Test: `frontend/src/lib/fiat.test.ts`
- Test: `frontend/src/stores/price.test.ts`

**Interfaces:**
- Produces: `formatFiat(n: number): string` → `"$1,234.56"`.
- Produces: `usePriceStore()` with reactive `znnUsd: number | null`, `qsrUsd: number | null`, `available: boolean`, `updatedAt: number`, and actions `start()` / `stop()` (poll lifecycle) and `portfolioUsd(balances: {symbol:string; amount:string; decimals:number}[]): number | null`.

- [ ] **Step 1: Write the failing fiat-formatter test**

```ts
// frontend/src/lib/fiat.test.ts
import { describe, it, expect } from 'vitest'
import { formatFiat } from './fiat'

describe('formatFiat', () => {
  it('formats thousands with two decimals', () => {
    expect(formatFiat(13639.07)).toBe('$13,639.07')
  })
  it('formats small unit prices to two decimals', () => {
    expect(formatFiat(0.118422)).toBe('$0.12')
  })
  it('formats zero', () => {
    expect(formatFiat(0)).toBe('$0.00')
  })
})
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/lib/fiat.test.ts`
Expected: FAIL — `formatFiat` is not exported / module not found.

- [ ] **Step 3: Implement `fiat.ts`**

```ts
// frontend/src/lib/fiat.ts
// USD display formatter for the dashboard hero + balance cards. Fiat is
// display-only — never used for any signing/amount path (amounts use lib/format).
export function formatFiat(n: number): string {
  return '$' + n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
```

- [ ] **Step 4: Run the fiat test and confirm it passes**

Run: `cd frontend && pnpm vitest run src/lib/fiat.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Write the failing price-store test**

```ts
// frontend/src/stores/price.test.ts
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePriceStore } from './price'

const OK = {
  data: {
    znn: { usd: 0.118422, timestamp: '2026-06-29T23:46:19Z' },
    qsr: { usd: 0.02343554, timestamp: '2026-06-29T23:46:19Z' },
    btc: { usd: 60172.0 }, eth: { usd: 1609.2 },
  },
}

describe('price store', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => vi.restoreAllMocks())

  it('parses a successful response and becomes available', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => OK }))
    const price = usePriceStore()
    await price.refresh()
    expect(price.available).toBe(true)
    expect(price.znnUsd).toBeCloseTo(0.118422)
    expect(price.qsrUsd).toBeCloseTo(0.02343554)
  })

  it('stays unavailable on HTTP 429', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 429, json: async () => ({}) }))
    const price = usePriceStore()
    await price.refresh()
    expect(price.available).toBe(false)
    expect(price.znnUsd).toBeNull()
  })

  it('stays unavailable when fetch throws', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network')))
    const price = usePriceStore()
    await price.refresh()
    expect(price.available).toBe(false)
  })

  it('computes the portfolio total from BigInt balances at full precision', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => OK }))
    const price = usePriceStore()
    await price.refresh()
    // 100 ZNN (8 decimals) * 0.118422 + 200 QSR * 0.02343554 = 11.8422 + 4.687108
    const total = price.portfolioUsd([
      { symbol: 'ZNN', amount: '10000000000', decimals: 8 },
      { symbol: 'QSR', amount: '20000000000', decimals: 8 },
    ])
    expect(total).toBeCloseTo(11.8422 + 4.687108, 4)
  })

  it('portfolioUsd returns null when unavailable', () => {
    const price = usePriceStore()
    expect(price.portfolioUsd([])).toBeNull()
  })
})
```

- [ ] **Step 6: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/stores/price.test.ts`
Expected: FAIL — `usePriceStore` not found.

- [ ] **Step 7: Implement `price.ts`**

```ts
// frontend/src/stores/price.ts
import { defineStore } from 'pinia'
import { formatAmountExact } from '../lib/format'

const PRICE_URL = 'https://api.zenon.info/price'
const POLL_MS = 60_000 // endpoint is rate-limited (observed HTTP 429) — poll gently

type PriceState = {
  znnUsd: number | null
  qsrUsd: number | null
  available: boolean
  updatedAt: number
  _timer: ReturnType<typeof setInterval> | null
  _inFlight: boolean
}

export const usePriceStore = defineStore('price', {
  state: (): PriceState => ({
    znnUsd: null, qsrUsd: null, available: false, updatedAt: 0, _timer: null, _inFlight: false,
  }),
  actions: {
    async refresh() {
      if (this._inFlight) return // single in-flight guard
      this._inFlight = true
      try {
        const res = await fetch(PRICE_URL, { method: 'GET' })
        if (!res.ok) { this.available = false; return } // 429 / 5xx → degrade
        const body = await res.json()
        const znn = body?.data?.znn?.usd
        const qsr = body?.data?.qsr?.usd
        if (typeof znn !== 'number' || typeof qsr !== 'number') { this.available = false; return }
        this.znnUsd = znn
        this.qsrUsd = qsr
        this.available = true
        this.updatedAt = Date.now()
      } catch {
        this.available = false // offline / CORS / parse error → degrade
      } finally {
        this._inFlight = false
      }
    },
    // portfolioUsd sums fiat across balances. Returns null when no price is
    // available so the Dashboard can fall back to a ZNN headline.
    portfolioUsd(balances: { symbol: string; amount: string; decimals: number }[]): number | null {
      if (!this.available) return null
      let total = 0
      for (const b of balances) {
        const price = b.symbol === 'ZNN' ? this.znnUsd : b.symbol === 'QSR' ? this.qsrUsd : null
        if (price == null) continue
        total += parseFloat(formatAmountExact(b.amount, b.decimals)) * price
      }
      return total
    },
    start() {
      this.refresh()
      if (this._timer) return
      this._timer = setInterval(() => this.refresh(), POLL_MS)
    },
    stop() {
      if (this._timer) { clearInterval(this._timer); this._timer = null }
    },
  },
})
```

> **Note on `Date.now()`:** allowed in app runtime code (the workflow-script restriction does not apply to the app).

- [ ] **Step 8: Run the price-store tests and confirm they pass**

Run: `cd frontend && pnpm vitest run src/stores/price.test.ts src/lib/fiat.test.ts`
Expected: PASS (8 tests total).

- [ ] **Step 9: Commit**

```bash
git add frontend/src/lib/fiat.ts frontend/src/lib/fiat.test.ts frontend/src/stores/price.ts frontend/src/stores/price.test.ts
git commit -m "feat(price): USD price store + fiat formatter with graceful degradation"
```

---

## Task 2: Add `@lucide/vue` as a direct dependency

**Files:**
- Modify: `frontend/package.json` (dependencies)

**Interfaces:**
- Produces: top-level resolvable `@lucide/vue@^1.20.0` so app code can `import { SendIcon } from '@lucide/vue'`.

- [ ] **Step 1: Add the dependency**

Edit `frontend/package.json` `dependencies` to add (alphabetical order, after `@types/qrcode`):

```json
    "@lucide/vue": "^1.20.0",
```

- [ ] **Step 2: Install**

Run: `cd frontend && pnpm install`
Expected: lockfile updates; `@lucide/vue` resolves at top level.

- [ ] **Step 3: Verify it imports and builds**

Create a throwaway check (do NOT commit this file):

```bash
cd frontend && cat > /tmp/lucide-check.ts <<'EOF'
import { SendIcon, LockIcon, LayoutDashboardIcon } from '@lucide/vue'
void SendIcon; void LockIcon; void LayoutDashboardIcon
EOF
cp /tmp/lucide-check.ts src/_lucide_check.ts && pnpm run typecheck; rm src/_lucide_check.ts
```

Expected: typecheck passes with the temp file present (no "cannot find module '@lucide/vue'").

- [ ] **Step 4: Commit**

```bash
git add frontend/package.json frontend/pnpm-lock.yaml
git commit -m "build(deps): add @lucide/vue@1.20.0 as a direct dependency"
```

---

## Task 3: Sidebar component

**Files:**
- Create: `frontend/src/components/Sidebar.vue`
- Test: `frontend/src/components/Sidebar.test.ts`

**Interfaces:**
- Consumes: `useNodeStore` (`connected`, `height`, `syncing`), `useUiStore` (`showGovernance`), `useNodeStore().chainId`.
- Produces: `<Sidebar />` — a `<aside>` with the wordmark, grouped `<router-link>` nav, and a node-sync pill. No props.

- [ ] **Step 1: Write the failing test**

```ts
// frontend/src/components/Sidebar.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Sidebar from './Sidebar.vue'
import { useNodeStore } from '../stores/node'
import { useUiStore } from '../stores/ui'

function mountSidebar() {
  return mount(Sidebar, { global: { stubs: { RouterLink: RouterLinkStub } } })
}

describe('Sidebar', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the core nav destinations', () => {
    const w = mountSidebar()
    const text = w.text()
    for (const label of ['Dashboard', 'Transfer', 'Receive', 'Tokens', 'Plasma', 'Staking', 'Pillars', 'Sentinels', 'Accelerator', 'Rewards', 'Settings']) {
      expect(text).toContain(label)
    }
  })

  it('hides Governance unless opted in on testnet', async () => {
    const w = mountSidebar()
    expect(w.text()).not.toContain('Governance')
    const ui = useUiStore(); const node = useNodeStore()
    ui.showGovernance = true; node.chainId = 2
    await w.vm.$nextTick()
    expect(w.text()).toContain('Governance')
  })

  it('shows the node-sync height when connected', async () => {
    const node = useNodeStore()
    node.connected = true; node.syncing = false; node.height = 3_420_000
    const w = mountSidebar()
    await w.vm.$nextTick()
    expect(w.text()).toContain('3,420,000')
  })
})
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/components/Sidebar.test.ts`
Expected: FAIL — `Sidebar.vue` not found.

- [ ] **Step 3: Implement `Sidebar.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import {
  LayoutDashboardIcon, SendIcon, DownloadIcon, CoinsIcon, ZapIcon, LayersIcon,
  Building2Icon, ShieldCheckIcon, RocketIcon, GiftIcon, VoteIcon, SettingsIcon,
  BookUserIcon, ShieldIcon,
} from '@lucide/vue'
import { useNodeStore } from '../stores/node'
import { useUiStore } from '../stores/ui'

const node = useNodeStore()
const ui = useUiStore()

const showGovernance = computed(() => ui.showGovernance && node.chainId !== 1)

const topNav = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboardIcon },
  { to: '/transfer', label: 'Transfer', icon: SendIcon },
  { to: '/receive', label: 'Receive', icon: DownloadIcon },
  { to: '/tokens', label: 'Tokens', icon: CoinsIcon },
]
const networkNav = computed(() => [
  { to: '/network/plasma', label: 'Plasma', icon: ZapIcon },
  { to: '/network/staking', label: 'Staking', icon: LayersIcon },
  { to: '/network/pillars', label: 'Pillars', icon: Building2Icon },
  { to: '/network/sentinels', label: 'Sentinels', icon: ShieldCheckIcon },
  { to: '/network/accelerator', label: 'Accelerator', icon: RocketIcon },
  { to: '/network/rewards', label: 'Rewards', icon: GiftIcon },
  ...(showGovernance.value ? [{ to: '/network/governance', label: 'Governance', icon: VoteIcon }] : []),
])
const bottomNav = [
  { to: '/settings', label: 'Settings', icon: SettingsIcon },
  { to: '/address-book', label: 'Address book', icon: BookUserIcon },
]

const heightLabel = computed(() => node.height.toLocaleString('en-US'))
const synced = computed(() => node.connected && !node.syncing)
</script>

<template>
  <aside class="flex w-58 flex-none flex-col border-r border-sidebar-border bg-sidebar px-3.5 py-5">
    <!-- Wordmark -->
    <div class="flex items-center gap-2.5 px-2 pb-5">
      <img src="../assets/images/syrius-logo.png" alt="" class="h-7 w-7 rounded-md" />
      <div class="flex flex-col leading-tight">
        <span class="text-base font-bold tracking-tight text-sidebar-foreground">syrius</span>
        <span class="text-ledger text-muted-foreground">Network of Momentum</span>
      </div>
    </div>

    <!-- Primary nav -->
    <nav class="flex flex-col gap-0.5">
      <RouterLink
        v-for="item in topNav" :key="item.to" :to="item.to" v-slot="{ isActive }" custom
      >
        <a
          :href="item.to"
          class="group flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors"
          :class="isActive
            ? 'bg-sidebar-accent font-semibold text-sidebar-accent-foreground'
            : 'font-medium text-muted-foreground hover:bg-sidebar-accent/60'"
          @click.prevent="$router.push(item.to)"
        >
          <component :is="item.icon" :size="18" :class="isActive ? 'text-primary' : ''" />
          {{ item.label }}
        </a>
      </RouterLink>
    </nav>

    <!-- Network section -->
    <div class="text-ledger mt-5 px-3 pb-1 text-muted-foreground">Network of Momentum</div>
    <nav class="flex flex-col gap-0.5">
      <RouterLink
        v-for="item in networkNav" :key="item.to" :to="item.to" v-slot="{ isActive }" custom
      >
        <a
          :href="item.to"
          class="group flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors"
          :class="isActive
            ? 'bg-sidebar-accent font-semibold text-sidebar-accent-foreground'
            : 'font-medium text-muted-foreground hover:bg-sidebar-accent/60'"
          @click.prevent="$router.push(item.to)"
        >
          <component :is="item.icon" :size="18" :class="isActive ? 'text-primary' : ''" />
          {{ item.label }}
        </a>
      </RouterLink>
    </nav>

    <!-- Bottom: settings, address book, node-sync pill -->
    <div class="mt-auto flex flex-col gap-0.5 pt-4">
      <RouterLink
        v-for="item in bottomNav" :key="item.to" :to="item.to" v-slot="{ isActive }" custom
      >
        <a
          :href="item.to"
          class="flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors"
          :class="isActive ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'text-muted-foreground hover:bg-sidebar-accent/60'"
          @click.prevent="$router.push(item.to)"
        >
          <component :is="item.icon" :size="18" />
          {{ item.label }}
        </a>
      </RouterLink>
      <div class="mt-1.5 flex items-center gap-2 rounded-md bg-sidebar-accent px-3 py-2.5">
        <ShieldIcon :size="16" :class="synced ? 'text-success' : 'text-warning'" />
        <span class="text-xs text-muted-foreground">{{ synced ? 'Node synced' : 'Syncing…' }}</span>
        <span class="ml-auto font-mono text-xs" :class="synced ? 'text-success' : 'text-warning'">#{{ heightLabel }}</span>
      </div>
    </div>
  </aside>
</template>
```

> `w-58` = 232px (Tailwind 4 arbitrary spacing). If your Tailwind config rejects `w-58`, use `class="w-[232px]"` — but prefer the spacing token; 232px is the kit's sidebar width.

- [ ] **Step 4: Run the test and confirm it passes**

Run: `cd frontend && pnpm vitest run src/components/Sidebar.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/Sidebar.vue frontend/src/components/Sidebar.test.ts
git commit -m "feat(chrome): Sidebar with grouped nav + node-sync pill"
```

---

## Task 4: TopBar rewrite

**Files:**
- Modify: `frontend/src/components/TopBar.vue` (full rewrite)
- Modify: `frontend/src/components/TopBar.test.ts` (update assertions)

**Interfaces:**
- Consumes: a `title` prop (current page name), `useWalletStore` (`locked`, `activeAddress()`), `usePlasmaStore`, `useAutoReceiveStore`, `usePillarStore`, `useAcceleratorStore`, `useUiStore` (theme), `lib/format.shortAddress`.
- Produces: `<TopBar :title="…" :locked="…" />` — header with page title + address pill + Lucide control buttons (theme/plasma/auto-receive/votes/lock). The account *picker dropdown* stays in `AccountSlotPicker` (rendered from the address pill).

**Theme persistence:** add a `theme` field to `useUiStore` in this task (see Step 3a).

- [ ] **Step 1: Update the test**

```ts
// frontend/src/components/TopBar.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import TopBar from './TopBar.vue'
import { useWalletStore } from '../stores/wallet'

const stubs = { AccountSlotPicker: true }

describe('TopBar', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the page title', () => {
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    expect(w.text()).toContain('Dashboard')
  })

  it('shows a lock button when unlocked', () => {
    const wallet = useWalletStore(); wallet.locked = false
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    expect(w.find('[aria-label="Lock wallet"]').exists()).toBe(true)
  })

  it('exposes a theme toggle', () => {
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    expect(w.find('[aria-label="Toggle theme"]').exists()).toBe(true)
  })
})
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/components/TopBar.test.ts`
Expected: FAIL — title not rendered / theme toggle missing (old TopBar has neither).

- [ ] **Step 3a: Add theme state to the ui store**

In `frontend/src/stores/ui.ts`, extend state + actions:

```ts
  state: () => ({
    showGovernance: false,
    theme: 'dark' as 'dark' | 'light',
  }),
```

Add to `actions` (and call `applyTheme()` at the end of `init()`):

```ts
    applyTheme() {
      document.documentElement.classList.toggle('dark', this.theme === 'dark')
    },
    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark'
      this.applyTheme()
      try { localStorage.setItem('syrius.theme', this.theme) } catch { /* ignore */ }
    },
```

And at the top of `init()` (before the try), restore persisted theme:

```ts
      try { const t = localStorage.getItem('syrius.theme'); if (t === 'light' || t === 'dark') this.theme = t } catch { /* ignore */ }
      this.applyTheme()
```

- [ ] **Step 3b: Rewrite `TopBar.vue`**

```vue
<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from 'nom-ui'
import {
  WalletIcon, SunIcon, MoonIcon, ZapIcon, ArrowDownCircleIcon, RocketIcon, LockIcon,
} from '@lucide/vue'
import { useWalletStore } from '../stores/wallet'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useUiStore } from '../stores/ui'
import { plasmaLevel, plasmaColorClass } from '../lib/plasma'
import { shortAddress } from '../lib/format'
import AccountSlotPicker from './AccountSlotPicker.vue'

const props = defineProps<{ title?: string; locked?: boolean }>()

const router = useRouter()
const wallet = useWalletStore()
const plasma = usePlasmaStore()
const pillar = usePillarStore()
const accelerator = useAcceleratorStore()
const autoReceive = useAutoReceiveStore()
const ui = useUiStore()

const plasmaLvl = computed(() => plasmaLevel(plasma.info?.currentPlasma ?? 0))
const plasmaColor = computed(() => plasmaColorClass(plasmaLvl.value))
const addr = computed(() => (props.locked ? '' : wallet.activeAddress()))

function gotoVotes() {
  router.push({ path: '/network/accelerator', query: { sub: 'Vote' } })
}

let toast: ReturnType<typeof useToast> | undefined
try { toast = useToast() } catch { /* no Toaster in tests */ }
watch(
  () => autoReceive.errorCount,
  () => { if (autoReceive.lastError) toast?.show(autoReceive.lastError, 'error') },
)
</script>

<template>
  <header class="flex h-15 flex-none items-center gap-4 border-b border-border px-7">
    <h1 class="text-lg font-semibold tracking-tight text-foreground">{{ title }}</h1>

    <div class="ml-auto flex items-center gap-2">
      <!-- Account/address pill: opens the account picker dropdown -->
      <AccountSlotPicker v-if="!locked" variant="pill" />
      <div v-else class="flex h-8.5 items-center gap-2 rounded-md border border-border bg-card px-3 text-muted-foreground">
        <WalletIcon :size="15" />
        <span class="font-mono text-xs">Locked</span>
      </div>

      <button type="button" aria-label="Toggle theme"
        class="grid h-8.5 w-8.5 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        @click="ui.toggleTheme()">
        <component :is="ui.theme === 'dark' ? SunIcon : MoonIcon" :size="16" />
      </button>

      <button type="button" :disabled="locked" aria-label="Plasma"
        :title="locked ? 'Plasma — unlock to use' : `Plasma: ${plasmaLvl}`"
        class="grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : `hover:bg-foreground/[0.06] ${plasmaColor}`"
        @click="router.push('/network/plasma')">
        <ZapIcon :size="16" />
      </button>

      <button type="button" :disabled="locked"
        :aria-label="autoReceive.enabled ? 'Auto-receive on' : 'Auto-receive off'"
        :aria-pressed="locked ? undefined : autoReceive.enabled"
        class="grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : `hover:bg-foreground/[0.06] ${autoReceive.enabled ? 'text-primary' : 'text-muted-foreground'}`"
        @click="autoReceive.toggle(wallet.activeIndex)">
        <ArrowDownCircleIcon :size="16" />
      </button>

      <button v-if="!locked && pillar.ownsPillar" type="button" aria-label="Accelerator votes"
        :title="accelerator.needsVoteCount > 0 ? `${accelerator.needsVoteCount} AZ item(s) to vote on` : 'Accelerator votes'"
        class="relative grid h-8.5 w-8.5 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        @click="gotoVotes">
        <RocketIcon :size="16" />
        <span v-if="accelerator.needsVoteCount > 0"
          class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[0.625rem] font-semibold text-primary-foreground">
          {{ accelerator.needsVoteCount }}
        </span>
      </button>

      <span class="mx-1 h-5 w-px bg-border"></span>

      <button type="button" :disabled="locked" aria-label="Lock wallet" title="Lock wallet"
        class="grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : 'text-muted-foreground hover:bg-foreground/[0.06] hover:text-foreground'"
        @click="wallet.lock()">
        <LockIcon :size="16" />
      </button>
    </div>
  </header>
</template>
```

> `h-15` = 60px, `h-8.5`/`w-8.5` = 34px, `min-w-4` = 16px (Tailwind 4 fractional spacing). If any are rejected by your config, fall back to `h-[60px]` / `h-[34px]` / `min-w-[1rem]`.
> `AccountSlotPicker` gains a `variant` prop in Task 11; until then it renders its current full form (the `variant` prop is additive and optional).

- [ ] **Step 4: Run the test and confirm it passes**

Run: `cd frontend && pnpm vitest run src/components/TopBar.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/TopBar.vue frontend/src/components/TopBar.test.ts frontend/src/stores/ui.ts
git commit -m "feat(chrome): rewrite TopBar (Lucide controls, address pill, theme toggle)"
```

---

## Task 5: AppShell layout

**Files:**
- Create: `frontend/src/components/AppShell.vue`
- Test: `frontend/src/components/AppShell.test.ts`

**Interfaces:**
- Consumes: `Sidebar`, `TopBar`, the current route's `meta.title`, `usePriceStore` (start/stop polling lifecycle).
- Produces: `<AppShell />` — `flex` row: `<Sidebar/>` + a column of `<TopBar :title/>` and a scrollable `<main><router-view/></main>`.

- [ ] **Step 1: Write the failing test**

```ts
// frontend/src/components/AppShell.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import AppShell from './AppShell.vue'

describe('AppShell', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the sidebar, a topbar title from route meta, and a router-view outlet', () => {
    const w = mount(AppShell, {
      global: {
        stubs: {
          RouterLink: RouterLinkStub,
          RouterView: { template: '<div class="rv-stub">page</div>' },
          AccountSlotPicker: true,
        },
        mocks: { $route: { meta: { title: 'Dashboard' }, path: '/dashboard' } },
      },
    })
    expect(w.find('aside').exists()).toBe(true)
    expect(w.find('header').text()).toContain('Dashboard')
    expect(w.find('.rv-stub').exists()).toBe(true)
  })
})
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/components/AppShell.test.ts`
Expected: FAIL — `AppShell.vue` not found.

- [ ] **Step 3: Implement `AppShell.vue`**

```vue
<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'
import { usePriceStore } from '../stores/price'

const route = useRoute()
const price = usePriceStore()
const title = computed(() => (route.meta.title as string) ?? '')

onMounted(() => price.start())
onBeforeUnmount(() => price.stop())
</script>

<template>
  <div class="flex h-screen bg-background">
    <Sidebar />
    <div class="flex min-w-0 flex-1 flex-col">
      <TopBar :title="title" />
      <main class="flex-1 overflow-y-auto p-7">
        <router-view />
      </main>
    </div>
  </div>
</template>
```

- [ ] **Step 4: Run the test and confirm it passes**

Run: `cd frontend && pnpm vitest run src/components/AppShell.test.ts`
Expected: PASS (1 test).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/AppShell.vue frontend/src/components/AppShell.test.ts
git commit -m "feat(chrome): AppShell layout (sidebar + topbar + routed main, price polling)"
```

---

## Task 6: Routed page wrappers (Transfer, Receive, Network/*)

**Files:**
- Create: `frontend/src/views/Transfer.vue`
- Create: `frontend/src/views/Receive.vue`
- Create: `frontend/src/views/NetworkPage.vue` (thin wrapper that renders a named panel + handles `tx.reset()` on route change)
- Test: `frontend/src/views/Transfer.test.ts`
- Test: `frontend/src/views/NetworkPage.test.ts`

**Interfaces:**
- Consumes: `SendForm`, `TxModal`, `TxResult` (Transfer); `AddressDisplay`, `UnreceivedPanel` (Receive); the panel components (`PlasmaPanel`, `StakingPanel`, `PillarPanel`, `SentinelsPanel`, `AcceleratorPanel`, `RewardsPanel`, `GovernancePanel`).
- Produces: route components named in Task 8's route table.

> **Transfer** reuses the exact tx flow from `SendModal.vue` (Task pre-read), minus the Dialog wrapper — same `onSend` → `tx.prepare(recipient, zts, toBase(...))` and the same status-driven sub-panels.

- [ ] **Step 1: Write the failing Transfer test**

```ts
// frontend/src/views/Transfer.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Transfer from './Transfer.vue'

describe('Transfer page', () => {
  beforeEach(() => setActivePinia(createPinia()))
  it('renders the send form while idle', () => {
    const w = mount(Transfer, { global: { stubs: { SendForm: true, TxModal: true, TxResult: true } } })
    expect(w.findComponent({ name: 'SendForm' }).exists() || w.find('send-form-stub').exists()).toBe(true)
  })
})
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/views/Transfer.test.ts`
Expected: FAIL — `Transfer.vue` not found.

- [ ] **Step 3: Implement `Transfer.vue`**

```vue
<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useToast } from 'nom-ui'
import { watch } from 'vue'
import { useBalancesStore } from '../stores/balances'
import { useTxStore } from '../stores/tx'
import { toBase } from '../lib/format'
import SendForm from '../components/SendForm.vue'
import TxModal from '../components/TxModal.vue'
import TxResult from '../components/TxResult.vue'

const balances = useBalancesStore()
const { items } = storeToRefs(balances)
const tx = useTxStore()
const { status, error } = storeToRefs(tx)

let toast: ReturnType<typeof useToast> | undefined
try { toast = useToast() } catch { toast = undefined }

async function onSend(intent: { recipient: string; zts: string; amountDecimal: string }) {
  const tok = items.value.find((b) => b.zts === intent.zts)
  await tx.prepare(intent.recipient, intent.zts, toBase(intent.amountDecimal, tok?.decimals ?? 8))
}

watch(status, (s) => { if (s === 'done') toast?.show('Transaction published', 'success') })
</script>

<template>
  <div class="mx-auto max-w-[34rem]">
    <div class="rounded-xl border border-border bg-card p-6">
      <SendForm v-if="status === 'idle' || status === 'error'" @send="onSend" />
      <p v-if="status === 'preparing'" class="text-sm text-muted-foreground">Preparing… (PoW if required)</p>
      <p v-if="status === 'error'" class="text-sm text-destructive" role="alert">{{ error }}</p>
      <TxModal v-if="status === 'awaiting' || status === 'publishing'" />
      <TxResult v-if="status === 'done'" @close="tx.reset()" />
    </div>
  </div>
</template>
```

- [ ] **Step 4: Implement `Receive.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useWalletStore } from '../stores/wallet'
import AddressDisplay from '../components/AddressDisplay.vue'
import UnreceivedPanel from '../components/UnreceivedPanel.vue'

const wallet = useWalletStore()
const address = computed(() => wallet.activeAddress())
</script>

<template>
  <div class="mx-auto max-w-[34rem] space-y-5">
    <div class="rounded-xl border border-border bg-card p-6">
      <AddressDisplay :address="address" />
    </div>
    <div class="rounded-xl border border-border bg-card p-6">
      <UnreceivedPanel />
    </div>
  </div>
</template>
```

- [ ] **Step 5: Implement `NetworkPage.vue`**

```vue
<script setup lang="ts">
// Thin wrapper for each NoM feature route. Renders the panel named by the
// route's meta.panel and resets the tx flow on enter so a half-built block
// can't leak in from another feature.
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useTxStore } from '../stores/tx'
import PlasmaPanel from '../components/panels/PlasmaPanel.vue'
import StakingPanel from '../components/panels/StakingPanel.vue'
import PillarPanel from '../components/panels/PillarPanel.vue'
import SentinelsPanel from '../components/panels/SentinelsPanel.vue'
import AcceleratorPanel from '../components/panels/AcceleratorPanel.vue'
import RewardsPanel from '../components/panels/RewardsPanel.vue'
import GovernancePanel from '../components/panels/GovernancePanel.vue'

const PANELS: Record<string, any> = {
  plasma: PlasmaPanel, staking: StakingPanel, pillars: PillarPanel,
  sentinels: SentinelsPanel, accelerator: AcceleratorPanel, rewards: RewardsPanel,
  governance: GovernancePanel,
}
const route = useRoute()
const tx = useTxStore()
const panelKey = computed(() => route.meta.panel as string)
const panel = computed(() => PANELS[panelKey.value])
// Accelerator deep-link: ?sub=Vote drives the panel's initial sub-view.
const initialSub = computed(() => (typeof route.query.sub === 'string' ? route.query.sub : ''))

watch(panelKey, () => tx.reset())
</script>

<template>
  <component :is="panel" v-bind="panelKey === 'accelerator' ? { initialSub } : {}" />
</template>
```

- [ ] **Step 6: Write + run the NetworkPage test**

```ts
// frontend/src/views/NetworkPage.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import NetworkPage from './NetworkPage.vue'

describe('NetworkPage', () => {
  beforeEach(() => setActivePinia(createPinia()))
  it('renders the panel named by route meta', () => {
    const w = mount(NetworkPage, {
      global: {
        stubs: { PlasmaPanel: { template: '<div class="plasma-stub"/>' } },
        mocks: { $route: { meta: { panel: 'plasma' }, query: {} } },
      },
    })
    expect(w.find('.plasma-stub').exists()).toBe(true)
  })
})
```

Run: `cd frontend && pnpm vitest run src/views/Transfer.test.ts src/views/NetworkPage.test.ts`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/views/Transfer.vue frontend/src/views/Receive.vue frontend/src/views/NetworkPage.vue frontend/src/views/Transfer.test.ts frontend/src/views/NetworkPage.test.ts
git commit -m "feat(routes): Transfer/Receive pages + NetworkPage panel wrapper"
```

---

## Task 7: Dashboard view

**Files:**
- Create: `frontend/src/views/Dashboard.vue`
- Test: `frontend/src/views/Dashboard.test.ts`

**Interfaces:**
- Consumes: `useBalancesStore` (`items`), `usePriceStore` (`portfolioUsd`, `znnUsd`, `qsrUsd`, `available`), `formatAmount`, `formatFiat`, `TxHistory`.
- Produces: route component for `/dashboard` (Task 8).

- [ ] **Step 1: Write the failing test**

```ts
// frontend/src/views/Dashboard.test.ts
import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Dashboard from './Dashboard.vue'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'

function setup() {
  const balances = useBalancesStore()
  balances.items = [
    { zts: 'zts1', symbol: 'ZNN', decimals: 8, amount: '1240850319000' },
    { zts: 'zts2', symbol: 'QSR', decimals: 8, amount: '12408500000000' },
  ] as any
  return mount(Dashboard, { global: { stubs: { TxHistory: true } } })
}

describe('Dashboard', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('shows a USD portfolio total when price is available', async () => {
    const price = usePriceStore()
    price.znnUsd = 0.118422; price.qsrUsd = 0.02343554; price.available = true
    const w = setup()
    await w.vm.$nextTick()
    expect(w.text()).toContain('TOTAL PORTFOLIO VALUE')
    expect(w.text()).toContain('$') // a formatted fiat total
  })

  it('falls back to a ZNN headline when price is unavailable', async () => {
    const price = usePriceStore(); price.available = false
    const w = setup()
    await w.vm.$nextTick()
    expect(w.text()).toContain('ZNN')
    expect(w.text()).not.toContain('≈ $')
  })
})
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/views/Dashboard.test.ts`
Expected: FAIL — `Dashboard.vue` not found.

- [ ] **Step 3: Implement `Dashboard.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { SendIcon, DownloadIcon } from '@lucide/vue'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'
import { formatAmount } from '../lib/format'
import { formatFiat } from '../lib/fiat'
import TxHistory from '../components/TxHistory.vue'

const router = useRouter()
const balances = useBalancesStore()
const price = usePriceStore()

const znn = computed(() => balances.items.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => balances.items.find((b) => b.symbol === 'QSR'))

const totalUsd = computed(() => price.portfolioUsd(balances.items as any))

// Per-token fiat line (≈ $X) — only when a price is available.
function fiatFor(symbol: string, amount: string, decimals: number): string | null {
  if (!price.available) return null
  const unit = symbol === 'ZNN' ? price.znnUsd : symbol === 'QSR' ? price.qsrUsd : null
  if (unit == null) return null
  return '≈ ' + formatFiat(parseFloat((Number(BigInt(amount)) / 10 ** decimals).toString()) * unit)
}

const cards = computed(() => [
  { name: 'Zenon', symbol: 'ZNN', amount: znn.value?.amount ?? '0', decimals: znn.value?.decimals ?? 8 },
  { name: 'Quasar', symbol: 'QSR', amount: qsr.value?.amount ?? '0', decimals: qsr.value?.decimals ?? 8 },
])
</script>

<template>
  <div class="mx-auto flex max-w-[60rem] flex-col gap-5">
    <!-- Plasma hero -->
    <div class="overflow-hidden rounded-xl bg-plasma p-7 text-[#0c1f12] shadow-md">
      <p class="text-ledger opacity-70">Total portfolio value</p>
      <div class="mb-4 mt-2 font-mono text-5xl font-bold tabular-nums tracking-tight">
        <template v-if="totalUsd !== null">{{ formatFiat(totalUsd) }}</template>
        <template v-else>{{ formatAmount(znn?.amount ?? '0', znn?.decimals ?? 8) }} <span class="text-2xl">ZNN</span></template>
      </div>
      <div class="flex gap-2.5">
        <button class="inline-flex h-9 items-center gap-1.5 rounded-md bg-[rgba(8,24,14,0.9)] px-4 text-sm font-semibold text-[#eafff1]" @click="router.push('/transfer')">
          <SendIcon :size="15" /> Send
        </button>
        <button class="inline-flex h-9 items-center gap-1.5 rounded-md bg-[rgba(8,24,14,0.9)] px-4 text-sm font-semibold text-[#eafff1]" @click="router.push('/receive')">
          <DownloadIcon :size="15" /> Receive
        </button>
      </div>
    </div>

    <!-- Token balances -->
    <div class="grid grid-cols-2 gap-4">
      <div v-for="c in cards" :key="c.symbol" class="flex items-center gap-3.5 rounded-xl border border-border bg-card p-5">
        <div>
          <div class="text-base font-semibold text-foreground">{{ c.name }}</div>
          <div class="font-mono text-xs text-muted-foreground">{{ c.symbol }}</div>
        </div>
        <div class="ml-auto text-right">
          <div class="font-mono text-xl tabular-nums text-foreground" :aria-label="`${c.symbol} balance`">
            {{ formatAmount(c.amount, c.decimals) }}
          </div>
          <div v-if="fiatFor(c.symbol, c.amount, c.decimals)" class="font-mono text-xs text-muted-foreground">
            {{ fiatFor(c.symbol, c.amount, c.decimals) }}
          </div>
        </div>
      </div>
    </div>

    <!-- Recent activity -->
    <div class="rounded-xl border border-border bg-card">
      <div class="flex items-center px-5 pb-1.5 pt-4">
        <span class="text-base font-semibold text-foreground">Recent activity</span>
        <button class="ml-auto text-sm text-muted-foreground transition-colors hover:text-foreground" @click="router.push('/tokens')">View all</button>
      </div>
      <div class="px-3 pb-3">
        <TxHistory />
      </div>
    </div>
  </div>
</template>
```

> The per-token fiat math uses `Number(BigInt(amount)) / 10**decimals` for a display approximation; precision-critical balances still render via `formatAmount`. Money text is **not** colored (foreground/muted only).

- [ ] **Step 4: Run the test and confirm it passes**

Run: `cd frontend && pnpm vitest run src/views/Dashboard.test.ts`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Dashboard.vue frontend/src/views/Dashboard.test.ts
git commit -m "feat(dashboard): plasma-hero Dashboard with USD total + balances + activity"
```

---

## Task 8: Router restructure + wire AppShell + remove Home

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/router/index.test.ts` (existing guard tests → new routes)
- Delete: `frontend/src/views/Home.vue`, `frontend/src/views/Home.test.ts`

**Interfaces:**
- Consumes: `AppShell`, `Dashboard`, `Transfer`, `Receive`, `NetworkPage`, existing `Settings`/`Tokens`/`AddressBook` views.
- Produces: the authenticated route tree under `AppShell`, default redirect `/dashboard`, preserved lock guard.

- [ ] **Step 1: Update the router test**

```ts
// frontend/src/router/index.test.ts  (replace the route-name assertions)
import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import router from './index'
import { useWalletStore } from '../stores/wallet'

describe('router guard', () => {
  beforeEach(async () => { setActivePinia(createPinia()); await router.replace('/unlock').catch(() => {}) })

  it('redirects to /unlock when locked and visiting a gated route', async () => {
    useWalletStore().locked = true
    await router.push('/dashboard').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/unlock')
  })

  it('redirects to /dashboard when unlocked and visiting a public route', async () => {
    useWalletStore().locked = false
    await router.push('/unlock').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('allows a gated route when unlocked', async () => {
    useWalletStore().locked = false
    await router.push('/network/plasma').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/network/plasma')
  })
})
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/router/index.test.ts`
Expected: FAIL — `/dashboard` not a route yet.

- [ ] **Step 3: Rewrite `router/index.ts`**

```ts
import { createRouter, createMemoryHistory, type RouteRecordRaw } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { useTxStore } from '../stores/tx'
import AppShell from '../components/AppShell.vue'

// Public routes are reachable while locked. Everything else is gated and lives
// under the AppShell (sidebar + topbar). Lazy-loaded so each screen code-splits.
export const PUBLIC_ROUTES = ['unlock', 'create', 'import']

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/dashboard' },
  { path: '/unlock', name: 'unlock', component: () => import('../views/Unlock.vue') },
  { path: '/create', name: 'create', component: () => import('../views/Create.vue') },
  { path: '/import', name: 'import', component: () => import('../views/ImportMnemonic.vue') },
  {
    path: '/',
    component: AppShell,
    children: [
      { path: 'dashboard', name: 'dashboard', meta: { title: 'Dashboard' }, component: () => import('../views/Dashboard.vue') },
      { path: 'transfer', name: 'transfer', meta: { title: 'Transfer' }, component: () => import('../views/Transfer.vue') },
      { path: 'receive', name: 'receive', meta: { title: 'Receive' }, component: () => import('../views/Receive.vue') },
      { path: 'tokens', name: 'tokens', meta: { title: 'Tokens' }, component: () => import('../views/Tokens.vue') },
      { path: 'network/plasma', name: 'net-plasma', meta: { title: 'Plasma', panel: 'plasma' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/staking', name: 'net-staking', meta: { title: 'Staking', panel: 'staking' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/pillars', name: 'net-pillars', meta: { title: 'Pillars', panel: 'pillars' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/sentinels', name: 'net-sentinels', meta: { title: 'Sentinels', panel: 'sentinels' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/accelerator', name: 'net-accelerator', meta: { title: 'Accelerator', panel: 'accelerator' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/rewards', name: 'net-rewards', meta: { title: 'Rewards', panel: 'rewards' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/governance', name: 'net-governance', meta: { title: 'Governance', panel: 'governance' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'settings', name: 'settings', meta: { title: 'Settings' }, component: () => import('../views/Settings.vue') },
      { path: 'address-book', name: 'address-book', meta: { title: 'Address book' }, component: () => import('../views/AddressBook.vue') },
    ],
  },
]

const router = createRouter({ history: createMemoryHistory(), routes })

router.beforeEach((to) => {
  const wallet = useWalletStore()
  const isPublic = PUBLIC_ROUTES.includes(to.name as string)
  if (wallet.locked && !isPublic) return { name: 'unlock' }
  if (!wallet.locked && isPublic) return { name: 'dashboard' }
  return true
})

router.afterEach(() => {
  // Discard any half-built/finished tx when navigating between screens.
  useTxStore().reset()
})

export default router
```

- [ ] **Step 4: Delete the obsolete Home view + test**

```bash
git rm frontend/src/views/Home.vue frontend/src/views/Home.test.ts
```

- [ ] **Step 5: Update redirects elsewhere (`/home` → `/dashboard`)**

Find and fix every `'/home'` / `name: 'home'` reference:

Run: `cd frontend && grep -rn "'/home'\|name: 'home'\|\"/home\"" src`
For each hit (notably `views/Unlock.vue:28` `router.push('/home')`, `views/Create.vue`, `views/ImportMnemonic.vue`, and any store), replace `'/home'` with `'/dashboard'`. (Unlock's is also touched in Task 9.)

- [ ] **Step 6: Run router + full suite**

Run: `cd frontend && pnpm vitest run src/router/index.test.ts && pnpm test`
Expected: router PASS; full suite green except any tests that imported `Home.vue` (there should be none after the delete) — fix stragglers if grep in Step 5 missed a `/home` push.

- [ ] **Step 7: Commit**

```bash
git add -A frontend/src
git commit -m "feat(routes): nest authenticated screens under AppShell; default /dashboard; drop Home"
```

---

## Task 9: Unlock / lock screen restyle

**Files:**
- Modify: `frontend/src/views/Unlock.vue`
- Modify: `frontend/src/views/Unlock.test.ts` (keep behavior assertions; drop any TopBar assertion)

**Interfaces:**
- Consumes: `useWalletStore`, `WalletPicker`, `lib/format` (none new). Removes the `TopBar locked` chrome (the shell-less lock screen owns its own framing).

- [ ] **Step 1: Update the test**

```ts
// frontend/src/views/Unlock.test.ts — ensure these still hold:
// - renders "Welcome back"
// - Unlock button disabled until a wallet is selected
// (Mount with stubs: { WalletPicker: true }; remove any TopBar reference.)
```

Concretely, set the test's expected heading text to `Welcome back` and assert `router.push` target is `/dashboard` on a successful unlock (mock `wallet.unlock`).

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/views/Unlock.test.ts`
Expected: FAIL on the new "Welcome back" / `/dashboard` expectations.

- [ ] **Step 3: Rewrite `Unlock.vue` template + redirect**

Replace the `<template>` and the `router.push('/home')` in `doUnlock` with:

```vue
<script setup lang="ts">
// ... keep the existing <script setup> imports EXCEPT drop `import TopBar`.
// In doUnlock, change router.push('/home') -> router.push('/dashboard').
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import WalletPicker from '../components/WalletPicker.vue'
import logoUrl from '../assets/images/syrius-logo.png'
// (rest of the existing script unchanged, with the /home → /dashboard edit)
</script>

<template>
  <!-- Shell-less lock screen with a soft radial plasma halo. -->
  <div
    class="grid min-h-screen place-items-center bg-background p-8"
    style="background-image: radial-gradient(circle at 50% 30%, rgba(0,213,87,.10), transparent 60%);"
  >
    <div class="flex w-[21rem] flex-col items-center gap-5">
      <img :src="logoUrl" alt="syrius" class="h-16 w-16 rounded-2xl" />
      <div class="text-center">
        <div class="text-xl font-bold tracking-tight text-foreground">Welcome back</div>
        <div class="mt-1 text-sm text-muted-foreground">Unlock your Syrius wallet</div>
      </div>

      <template v-if="wallet.wallets.length > 0">
        <WalletPicker v-model="selected" :wallets="wallet.wallets" class="w-full" />
        <Input v-model="password" type="password" placeholder="Password" aria-label="password" class="w-full" @keyup.enter="doUnlock" />
        <Button class="w-full" size="lg" :disabled="busy || !selected" aria-label="Unlock" @click="doUnlock">Unlock</Button>
      </template>
      <p v-else class="text-sm text-muted-foreground">No wallets yet. Import a keystore to begin.</p>

      <div class="flex w-full flex-col gap-2">
        <Button variant="outline" class="w-full" @click="doImport">Import keystore…</Button>
        <Button variant="outline" class="w-full" @click="router.push('/create')">Create new wallet</Button>
        <Button variant="outline" class="w-full" @click="router.push('/import')">Import mnemonic</Button>
      </div>

      <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      <p v-if="notice" class="text-sm text-muted-foreground">{{ notice }}</p>
    </div>
  </div>
</template>
```

> The plasma halo is the one decorative gradient permitted on the lock screen (per the standard). The Unlock button is a standard primary (plasma) `Button`; do not add a second plasma surface.

- [ ] **Step 4: Run the test and confirm it passes**

Run: `cd frontend && pnpm vitest run src/views/Unlock.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Unlock.vue frontend/src/views/Unlock.test.ts
git commit -m "feat(unlock): restyle lock screen to the design-system reference (plasma halo)"
```

---

## Task 10: Lucide migration of remaining inline SVGs

**Files (each modified to swap inline `<svg>` → Lucide):**
- `frontend/src/components/AddressDisplay.vue` (copy/copied icons)
- `frontend/src/components/ContactPicker.vue` (`✕` close → `XIcon`; "Manage address book →" keeps the `→` text glyph, which is a permitted functional glyph)
- `frontend/src/components/panels/PillarLaunch.vue` and `SentinelLaunch.vue` (`✓` status → `CheckIcon`)
- Any remaining inline `<svg>` surfaced by the grep in Step 1 (panels)

**Interfaces:** none changed — pure markup swaps. Behavior + aria-labels preserved.

- [ ] **Step 1: Enumerate the remaining inline SVGs and unicode glyphs**

Run:
```bash
cd frontend && grep -rn "<svg" src --include="*.vue" | grep -v node_modules
grep -rnE "[✓✔✗✕✎]" src --include="*.vue"
```
Expected: a finite list (TopBar/Sidebar/AccountSlotPicker/WalletPicker are handled in their own tasks; here, handle AddressDisplay, ContactPicker, PillarLaunch, SentinelLaunch, and any panel SVGs).

- [ ] **Step 2: Swap each, following this exact pattern**

For an inline icon like AddressDisplay's copy button:

```vue
<!-- before -->
<svg v-if="!copied" width="13" height="13" viewBox="0 0 24 24" ...><rect .../><path .../></svg>
<svg v-else class="text-primary" width="13" height="13" ...><path d="M20 6 9 17l-5-5"/></svg>
<!-- after -->
<CopyIcon v-if="!copied" :size="13" />
<CheckIcon v-else :size="13" class="text-primary" />
```
Add to that file's `<script setup>`: `import { CopyIcon, CheckIcon } from '@lucide/vue'`.

For status glyphs like PillarLaunch's `✓ Plasma is sufficient.`:

```vue
<!-- before -->
<p v-if="plasmaCleared" class="text-sm text-foreground">✓ Plasma is sufficient.</p>
<!-- after -->
<p v-if="plasmaCleared" class="flex items-center gap-1.5 text-sm text-foreground"><CheckIcon :size="15" class="text-success" /> Plasma is sufficient.</p>
```
Add `import { CheckIcon } from '@lucide/vue'` to that file.

For ContactPicker's close `✕`:

```vue
<!-- before --> <button ... @click="emit('close')">✕</button>
<!-- after -->  <button ... @click="emit('close')"><XIcon :size="16" /></button>
```
Add `import { XIcon } from '@lucide/vue'`.

- [ ] **Step 3: Confirm no stray inline SVG / glyph-as-icon remains (outside picker tasks)**

Run:
```bash
cd frontend && grep -rn "<svg" src --include="*.vue" | grep -vE "AccountSlotPicker|WalletPicker"
grep -rnE "[✓✔✗✕✎]" src --include="*.vue" | grep -vE "AccountSlotPicker|WalletPicker"
```
Expected: empty (everything migrated; pickers handled in Task 11).

- [ ] **Step 4: Typecheck + test**

Run: `cd frontend && pnpm run typecheck && pnpm test`
Expected: green (icon swaps don't change behavior; existing tests still pass).

- [ ] **Step 5: Commit**

```bash
git add -A frontend/src
git commit -m "refactor(icons): migrate remaining inline SVGs + status glyphs to @lucide/vue"
```

---

## Task 11: Picker fixes (review items #1–#4)

**Files:**
- Modify: `frontend/src/components/AccountSlotPicker.vue`
- Modify: `frontend/src/components/WalletPicker.vue`

**Interfaces:**
- AccountSlotPicker gains an optional `variant?: 'pill'` prop (used by TopBar). Default (no variant) renders today's name+address+dropdown; `pill` renders the compact address pill that opens the same dropdown.

Fix in BOTH files:
1. **Gradient avatars** `bg-gradient-to-br from-primary to-info` → solid `bg-sidebar-accent text-foreground` (or `bg-muted`). Plasma/gradients stay reserved for the hero + primary buttons.
2. **Arbitrary radii** `rounded-[7px]` → `rounded-lg`; `rounded-[10px]` → `rounded-xl`.
3. **Off-scale text** `text-[15px]` → `text-base`; `text-[13px]` → `text-sm`; `text-[10px]` → `text-xs`.
4. **Unicode glyphs** `✓`/`✕`/`✎` buttons → `CheckIcon`/`XIcon`/`PencilIcon`; the chevron + plus inline SVGs → `ChevronDownIcon`/`PlusIcon`.

- [ ] **Step 1: Add a focused test for the fixes**

```ts
// frontend/src/components/AccountSlotPicker.test.ts — add cases (keep existing):
import { mount } from '@vue/test-utils'
// ... existing setup ...
it('renders avatars without a gradient and without arbitrary radii', () => {
  // mount with one account; open the dropdown
  const html = /* mounted component */ wrapper.html()
  expect(html).not.toContain('from-primary')
  expect(html).not.toContain('rounded-[7px]')
  expect(html).not.toContain('rounded-[10px]')
  expect(html).not.toContain('text-[15px]')
})
```
(Adapt to the file's existing mount/open helper; the key assertions are the four `not.toContain` lines.)

- [ ] **Step 2: Run it and confirm it fails**

Run: `cd frontend && pnpm vitest run src/components/AccountSlotPicker.test.ts`
Expected: FAIL — the gradient/radii/text strings are still present.

- [ ] **Step 3: Apply the swaps in `AccountSlotPicker.vue`**

Replace, throughout the file:
- `bg-gradient-to-br from-primary to-info text-[13px]` → `bg-sidebar-accent text-sm text-foreground` (3 avatar spots: lines ~129, ~151, and the inline-rename avatar).
- `rounded-[7px]` → `rounded-lg` (4 spots); `rounded-[10px]` (none here).
- `text-[15px]` → `text-base` (2 spots); `text-[13px]` → `text-sm`.
- Save/cancel/rename buttons: replace `✓`/`✕`/`✎` text with `<CheckIcon :size="14"/>` / `<XIcon :size="14"/>` / `<PencilIcon :size="13"/>`.
- Chevron + add `<svg>` → `<ChevronDownIcon :size="16"/>` / `<PlusIcon :size="15"/>`.
- Add imports: `import { ChevronDownIcon, PlusIcon, CheckIcon, XIcon, PencilIcon, CopyIcon } from '@lucide/vue'` and swap the copy-icon `<svg>` pair for `CopyIcon`/`CheckIcon` (as in Task 10's pattern).

Add the `variant` prop + pill rendering:

```ts
const props = defineProps<{ variant?: 'pill' }>()
```
```vue
<!-- pill entry point (TopBar): compact address pill that toggles the same dropdown -->
<template v-if="props.variant === 'pill'">
  <button type="button" class="flex h-8.5 items-center gap-2 rounded-md border border-border bg-card px-3 transition-colors hover:border-muted-foreground/40"
    aria-label="Select account" :aria-expanded="open" @click="toggle">
    <WalletIcon :size="15" class="text-muted-foreground" />
    <span class="font-mono text-xs">{{ shortAddress(activeAddr) }}</span>
    <ChevronDownIcon :size="14" class="text-muted-foreground" :class="open ? 'rotate-180' : ''" />
  </button>
</template>
```
(Add `WalletIcon` to the import; render the existing dropdown markup for both variants — wrap the current name+chevron+address block in `<template v-else>`.)

- [ ] **Step 4: Apply the same swaps in `WalletPicker.vue`**

- `bg-gradient-to-br from-primary to-info text-[13px]` → `bg-sidebar-accent text-sm text-foreground` (3 spots).
- `rounded-[10px]` → `rounded-xl`; `rounded-b-[10px]` → `rounded-b-xl`; `rounded-[7px]` → `rounded-lg`.
- `text-[15px]` → `text-base`; `text-[13px]` → `text-sm`; `py-[11px]`/`py-[5px]` → `py-3`/`py-1.5`; `h-[30px] w-[30px]`/`h-[18px] w-[18px]` → `h-7.5 w-7.5`/`h-4.5 w-4.5`.
- `✓`/`✕` → `CheckIcon`/`XIcon`; chevron `<svg>` → `ChevronDownIcon`. Add `import { ChevronDownIcon, CheckIcon, XIcon } from '@lucide/vue'`.

- [ ] **Step 5: Run tests + grep to confirm the violations are gone**

Run:
```bash
cd frontend && pnpm vitest run src/components/AccountSlotPicker.test.ts src/components/WalletPicker.test.ts
grep -rnE "from-primary to-info|rounded-\[(7|10)px\]|text-\[(10|13|15)px\]|[✓✕✎]" src/components/AccountSlotPicker.vue src/components/WalletPicker.vue
```
Expected: tests PASS; grep returns **nothing**.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/AccountSlotPicker.vue frontend/src/components/WalletPicker.vue frontend/src/components/AccountSlotPicker.test.ts
git commit -m "fix(design): remove avatar gradients, arbitrary radii, off-scale text, glyph icons in pickers"
```

---

## Task 12: QR color fix + Nunito cleanup

**Files:**
- Modify: `frontend/src/components/AddressDisplay.vue` (QR hex)
- Delete: `frontend/src/assets/fonts/nunito-v16-latin-regular.woff2`, `frontend/src/assets/fonts/OFL.txt`

**Interfaces:** none.

- [ ] **Step 1: Fix the off-brand QR green**

In `AddressDisplay.vue`, change the QR module color `#00d659` → brand `#00d557`:

```ts
// before: color: { dark: '#00d659', light: '#0d0d0d' },
// after:  color: { dark: '#00d557', light: '#0d0d0d' },
```
(The `<canvas>`/qrcode API needs literal hex; use the corrected brand value. The dark `#0d0d0d` background is intentional QR contrast and stays.)

- [ ] **Step 2: Delete the unused Nunito font asset**

```bash
git rm frontend/src/assets/fonts/nunito-v16-latin-regular.woff2 frontend/src/assets/fonts/OFL.txt
```

- [ ] **Step 3: Confirm nothing references the deleted font**

Run: `cd frontend && grep -rn "nunito\|OFL" src` → expected: no `@font-face`/import hits (only the now-deleted files would have matched).

- [ ] **Step 4: Typecheck + build**

Run: `cd frontend && pnpm run typecheck && pnpm run build`
Expected: green; no missing-asset error.

- [ ] **Step 5: Commit**

```bash
git add -A frontend/src
git commit -m "fix(design): correct QR brand green (#00d557); drop unused Nunito font"
```

---

## Task 13: Full verification + main.ts theme init

**Files:**
- Modify: `frontend/src/main.ts` (ensure theme is applied at boot, default dark)

**Interfaces:** none new.

- [ ] **Step 1: Ensure dark is the boot default**

In `frontend/src/main.ts`, after Pinia is installed and before/at mount, apply the persisted theme so the first paint is correct. Add:

```ts
import { useUiStore } from './stores/ui'
// after app.use(pinia):
useUiStore().applyTheme() // default 'dark'; init() later restores persisted value
```
(If `main.ts` cannot create a store before mount in your setup, instead add `class="dark"` to `index.html`'s root element as the static default and let `ui.init()` reconcile.)

- [ ] **Step 2: Run the full frontend gate**

Run:
```bash
cd frontend && pnpm run typecheck && pnpm test && pnpm run build
```
Expected: typecheck clean, all tests pass, Vite build succeeds.

- [ ] **Step 3: Run the Go build (bindings untouched, prove embed compiles)**

Run: `cd /Users/dfriestedt/Github/go-syrius && GOWORK=off GOTOOLCHAIN=auto go build ./...`
Expected: success (the unrelated gopsutil/IOKit cgo deprecation warning is fine).

- [ ] **Step 4: Manual smoke (document result in the PR)**

Run: `cd /Users/dfriestedt/Github/go-syrius && GOWORK=off wails dev`
Verify by eye: lock screen → unlock → sidebar + plasma-hero Dashboard; nav to each Network page; Transfer/Receive pages; theme toggle flips light/dark; portfolio shows USD when the feed is up (and ZNN-headline when blocked). No gradient avatars; Lucide icons throughout.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/main.ts
git commit -m "chore(theme): apply persisted theme at boot (default dark)"
```

---

## Self-Review

**Spec coverage:**
- §1 App shell → Tasks 3, 4, 5. ✅
- §2 Nav & routing (decompose tabs, default /dashboard, guard, deep-link) → Tasks 6, 8. ✅
- §3 Dashboard (hero + balances + activity + fiat degradation) → Tasks 1, 7. ✅
- §4 Standards (Lucide, picker fixes #1–4, QR hex, ledger labels, Nunito) → Tasks 10, 11, 12 (ledger labels applied in Sidebar/Dashboard Tasks 3, 7). ✅
- §5 Lock screen → Task 9. ✅
- §6 Theme (light+dark, toggle, dark default) → Tasks 4, 13. ✅
- §7 Price feed (store, 60s poll, 429/offline degrade, BigInt total) → Task 1, wired in Tasks 5, 7. ✅
- §8 Testing (router, Dashboard, price, AppShell/Sidebar, Transfer/Receive) → Tasks 1,3,5,6,7,8. ✅
- Non-goals (no Bridge) → honored (no Bridge route/nav anywhere). ✅

**Placeholder scan:** Task 11's test snippet is intentionally adapted-to-existing-helper (the four `not.toContain` assertions are concrete); all code steps show real code. No TBD/TODO. ✅

**Type consistency:** `usePriceStore().portfolioUsd(balances)`, `available`, `znnUsd`, `qsrUsd` used consistently in Tasks 1/5/7. `formatFiat` signature consistent. Route `meta.panel` keys in Task 8 match `PANELS` map keys in Task 6 (`plasma/staking/pillars/sentinels/accelerator/rewards/governance`). `AccountSlotPicker` `variant: 'pill'` defined in Task 11, consumed in Task 4 (additive/optional, so Task 4 works before Task 11 lands). ✅

**Known sequencing note:** Task 4 (TopBar) references `AccountSlotPicker`'s `variant="pill"` before Task 11 adds it; because the prop is optional and ignored by the pre-Task-11 component, TopBar renders the full picker until Task 11 lands — no breakage, just a transient visual until the picker task completes.

**CORS risk (Task 1):** if the WebView blocks the cross-origin `fetch`, `available` stays false and the Dashboard shows the ZNN headline — functional, just no fiat. Follow-up if it triggers: move the fetch behind a tiny Go `PriceService` binding (CORS-free). Flagged, not blocking.
