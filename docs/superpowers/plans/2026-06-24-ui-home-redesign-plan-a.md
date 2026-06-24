# UI Home Redesign — Plan A (design system + home shell) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. For the visual components, also use **superpowers:frontend-design** — the reference is the nom-ui screenshots described below; refine markup against `wails dev`, the tokens here are the source of truth.

**Goal:** Port the nom-ui (MIT) look-and-feel into our Svelte stack and restructure the unlocked app into a single home page — theme + fonts + base `ui/` components + the `Home` shell (4-card row, status strip, 7-tab bar), the Tokens tab, and the Send/Receive modals. (Plan B fills the other six tab panels.)

**Architecture:** Presentation-only. Redefine the Tailwind theme tokens to nom-ui's dark palette (keeping token *names* so existing components inherit the look), self-host Space Grotesk + JetBrains Mono, build small `ui/` primitives, then compose `Home.svelte` and the Send/Receive dialogs from existing stores (`balances`, `tx`, `node`, `plasma`, `pillar`) — the Go bindings and confirm-what-you-sign path are untouched.

**Tech Stack:** Svelte 3, Tailwind 3.4, Vite 3, Vitest 0.34, `@testing-library/svelte`, `@fontsource-variable/*`, `qrcode` (already installed).

## Reference (nom-ui screenshots, dark theme)

Home layout, top→bottom: a compact top bar (account switcher · auto-receive · Lock); a **row of 4 cards** — ZNN balance (green-tinted, mono number), QSR balance (blue-tinted, mono number), **Send** (outline card, up-arrow icon), **Receive** (outline card, down-arrow icon); a **status strip** ("Account Height: N · Tokens: N · Plasma: ⚡High · Pillar: None"); and a **tab bar** (active tab in green with a green underline) over a card panel. Primary buttons are green-filled with dark text. Inputs are dark with subtle borders. Tighter corners (radius 0.375rem).

## Global Constraints

- **Dark-only.** No light theme. Root carries the `dark` class.
- **Exact nom-ui dark tokens** (from nom-ui `src/style.css` `.dark`): bg `hsl(0 0% 8%)`, card/surface `hsl(0 0% 10%)`, elevated/popover `hsl(0 0% 14%)`, text `hsl(0 0% 98%)`, muted `hsl(0 0% 65%)`, border `hsl(0 0% 20%)`, **primary green `hsl(145 100% 42%)`** (text-on-primary `hsl(0 0% 8%)`), **QSR blue `#0061EB`** (`hsl(217 100% 46%)`), success `hsl(145 63% 45%)`, warning `hsl(38 95% 55%)`, error `hsl(352 86% 58%)`. Radius `0.375rem`.
- **Fonts:** Space Grotesk Variable (UI/sans), JetBrains Mono Variable (numbers/amounts/addresses) — bundled via `@fontsource-variable`, no CDN.
- **Presentation-only:** do not change any `wailsjs` binding, Go file, or the `tx` confirm/publish path. Reuse existing stores and `TxModal`/`TxResult`.
- Frontend commands (no `GOWORK` needed): `cd frontend`; `pnpm install`; `pnpm test` (vitest), `pnpm run check` (svelte-check), `pnpm run build`. Visual: `GOWORK=off wails dev` from repo root.
- Commits are GPG-signed (controller handles signing — implementers STAGE only, per the dispatch).
- Plan B will add the other six panels and delete the old `routes/*.svelte`; **do not delete any existing route file in Plan A** (they stay reachable-by-code until Plan B harvests them).

## File Structure

- `frontend/tailwind.config.js` — nom-ui color tokens (with `<alpha-value>`), radius, font families.
- `frontend/src/app.css` — font imports, base styles, `dark` defaults.
- `frontend/package.json` — add `@fontsource-variable/space-grotesk`, `@fontsource-variable/jetbrains-mono`.
- `NOTICE` (repo root) — nom-ui attribution.
- `frontend/src/lib/components/ui/{Card,Button,Input,Field,Tabs}.svelte` (+ tests) — base primitives.
- `frontend/src/lib/components/{BalanceCard,ActionCard,StatusStrip}.svelte` (+ tests) — home pieces.
- `frontend/src/lib/components/{SendModal,ReceiveModal}.svelte` (+ tests) — action dialogs.
- `frontend/src/lib/components/panels/{TokensPanel,PanelPlaceholder}.svelte` (+ TokensPanel test) — Plan A panels.
- `frontend/src/routes/Home.svelte` (+ test) — the home page.
- `frontend/src/App.svelte` — route unlocked → `Home`.

---

## Task 1: Theme foundation + fonts

**Files:** Modify `frontend/tailwind.config.js`, `frontend/src/app.css`, `frontend/package.json`; create `NOTICE`.

**Interfaces:**
- Produces: Tailwind tokens `bg surface elevated text muted border accent accent-fg qsr success warn error`, `rounded` = 0.375rem, `font-sans` (Space Grotesk), `font-mono` (JetBrains Mono). `accent` is now GREEN (was blue) — existing `bg-accent`/`text-accent` become green automatically.

- [ ] **Step 1: Add the font packages**

Run: `cd frontend && pnpm add @fontsource-variable/space-grotesk @fontsource-variable/jetbrains-mono`
Expected: both added to `dependencies`; `pnpm-lock.yaml` updated.

- [ ] **Step 2: Rewrite `frontend/tailwind.config.js`**

```js
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{svelte,ts}'],
  theme: {
    extend: {
      colors: {
        bg: 'hsl(0 0% 8% / <alpha-value>)',
        surface: 'hsl(0 0% 10% / <alpha-value>)',
        elevated: 'hsl(0 0% 14% / <alpha-value>)',
        text: 'hsl(0 0% 98% / <alpha-value>)',
        muted: 'hsl(0 0% 65% / <alpha-value>)',
        border: 'hsl(0 0% 20% / <alpha-value>)',
        accent: 'hsl(145 100% 42% / <alpha-value>)',     // nom-ui green primary
        'accent-fg': 'hsl(0 0% 8% / <alpha-value>)',      // dark text on green
        qsr: 'hsl(217 100% 46% / <alpha-value>)',         // zenon blue #0061EB
        success: 'hsl(145 63% 45% / <alpha-value>)',
        warn: 'hsl(38 95% 55% / <alpha-value>)',
        error: 'hsl(352 86% 58% / <alpha-value>)',
      },
      borderRadius: { DEFAULT: '0.375rem' },
      fontFamily: {
        sans: ['"Space Grotesk Variable"', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono Variable"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
    },
  },
  plugins: [],
}
```

- [ ] **Step 3: Rewrite `frontend/src/app.css`**

```css
@import '@fontsource-variable/space-grotesk';
@import '@fontsource-variable/jetbrains-mono';

@tailwind base;
@tailwind components;
@tailwind utilities;

:root { color-scheme: dark; }
html, body { @apply bg-bg text-text; font-family: theme('fontFamily.sans'); }
/* The app is dark-only. */
:root { --radius: 0.375rem; }
```

- [ ] **Step 4: Add `NOTICE` (repo root)**

```
This product's UI design tokens and aesthetic are adapted from nom-ui
(https://github.com/digitalSloth/nom-ui), MIT License, Copyright (c) digitalSloth.
The design was re-implemented from scratch in Svelte; no nom-ui source code is included.
```

- [ ] **Step 5: Verify build + types**

Run: `cd frontend && pnpm install && pnpm run check && pnpm run build`
Expected: `svelte-check` 0 errors; Vite build succeeds. (The Space Grotesk / JetBrains Mono `@import`s resolve from `node_modules`.)

- [ ] **Step 6: Visual smoke (controller/implementer)**

Run from repo root: `GOWORK=off wails dev` — confirm the app renders near-black with light text and the new fonts (existing screens now show green where they used blue `accent`). Stop dev after confirming.

- [ ] **Step 7: Stage** (controller commits)

`git add frontend/tailwind.config.js frontend/src/app.css frontend/package.json frontend/pnpm-lock.yaml NOTICE`

---

## Task 2: Base `ui/` primitives

**Files:** Create `frontend/src/lib/components/ui/{Card,Button,Input,Field,Tabs}.svelte`; tests `ui/{Button,Tabs,Field}.test.ts`.

**Interfaces:**
- Produces:
  - `Card` — slot wrapper; props: `class` (extra classes).
  - `Button` — props: `variant: 'primary'|'outline'|'ghost'|'danger'` (default `primary`), `disabled`, `type`; forwards `on:click`; slot = label.
  - `Input` — `bind:value`, props `placeholder`, `type`, `ariaLabel`; forwards `on:input`.
  - `Field` — props `label`, `hint`, `error`; slot = control.
  - `Tabs` — props `tabs: string[]`, `bind:active: string`; renders the tab row, green active state; sets `active` on click.

- [ ] **Step 1: Write `Card.svelte`**

```svelte
<script lang="ts">
  let extra = ''
  export { extra as class }
</script>
<div class="rounded border border-border bg-surface {extra}"><slot /></div>
```

- [ ] **Step 2: Write `Button.svelte`**

```svelte
<script lang="ts">
  export let variant: 'primary' | 'outline' | 'ghost' | 'danger' = 'primary'
  export let disabled = false
  export let type: 'button' | 'submit' = 'button'
  const styles = {
    primary: 'bg-accent text-accent-fg hover:brightness-110',
    outline: 'border border-border text-text hover:bg-elevated',
    ghost: 'text-muted hover:text-text hover:bg-elevated',
    danger: 'bg-error text-text hover:brightness-110',
  }
</script>
<button {type} {disabled} on:click
  class="rounded px-4 py-2 text-sm font-medium transition disabled:opacity-50 disabled:cursor-not-allowed {styles[variant]}">
  <slot />
</button>
```

- [ ] **Step 3: Write `Input.svelte`**

```svelte
<script lang="ts">
  export let value = ''
  export let placeholder = ''
  export let type = 'text'
  export let ariaLabel = ''
</script>
<input {type} {placeholder} aria-label={ariaLabel} bind:value on:input
  class="w-full rounded border border-border bg-elevated px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent" />
```

- [ ] **Step 4: Write `Field.svelte`**

```svelte
<script lang="ts">
  export let label = ''
  export let hint = ''
  export let error = ''
</script>
<label class="block space-y-1">
  {#if label}<span class="text-sm text-text">{label}</span>{/if}
  <slot />
  {#if error}<span class="block text-xs text-error">{error}</span>
  {:else if hint}<span class="block text-xs text-muted">{hint}</span>{/if}
</label>
```

- [ ] **Step 5: Write `Tabs.svelte`**

```svelte
<script lang="ts">
  export let tabs: string[] = []
  export let active = tabs[0] ?? ''
</script>
<div class="flex flex-wrap gap-1 border-b border-border">
  {#each tabs as t}
    <button
      class="px-4 py-2 text-sm transition -mb-px border-b-2 {t === active ? 'border-accent text-accent font-medium' : 'border-transparent text-muted hover:text-text'}"
      aria-label={`tab ${t}`}
      on:click={() => (active = t)}>{t}</button>
  {/each}
</div>
```

- [ ] **Step 6: Write tests `ui/Button.test.ts`, `ui/Tabs.test.ts`, `ui/Field.test.ts`**

```ts
// ui/Button.test.ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import Button from './Button.svelte'

describe('Button', () => {
  it('renders a primary (green) button and fires click', async () => {
    const onClick = vi.fn()
    const { component } = render(Button, { props: { variant: 'primary' } })
    component.$on('click', onClick)
    const btn = screen.getByRole('button')
    expect(btn.className).toContain('bg-accent')
    await fireEvent.click(btn)
    expect(onClick).toHaveBeenCalled()
  })
  it('disables', () => {
    render(Button, { props: { disabled: true } })
    expect((screen.getByRole('button') as HTMLButtonElement).disabled).toBe(true)
  })
})
```

```ts
// ui/Tabs.test.ts
import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import Tabs from './Tabs.svelte'

describe('Tabs', () => {
  it('marks the active tab and switches on click', async () => {
    render(Tabs, { props: { tabs: ['One', 'Two'], active: 'One' } })
    const two = screen.getByRole('button', { name: 'tab Two' })
    expect(screen.getByRole('button', { name: 'tab One' }).className).toContain('text-accent')
    await fireEvent.click(two)
    expect(two.className).toContain('text-accent')
  })
})
```

```ts
// ui/Field.test.ts
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Field from './Field.svelte'

describe('Field', () => {
  it('shows label + hint, and error replaces hint', () => {
    const { rerender } = render(Field, { props: { label: 'Amount', hint: 'min 1' } })
    expect(screen.getByText('Amount')).toBeTruthy()
    expect(screen.getByText('min 1')).toBeTruthy()
    rerender({ label: 'Amount', hint: 'min 1', error: 'too low' })
    expect(screen.getByText('too low')).toBeTruthy()
    expect(screen.queryByText('min 1')).toBeNull()
  })
})
```

- [ ] **Step 7: Run tests + types**

Run: `cd frontend && pnpm test -- src/lib/components/ui && pnpm run check`
Expected: the 3 test files pass; svelte-check 0 errors.

- [ ] **Step 8: Stage**

`git add frontend/src/lib/components/ui`

---

## Task 3: Home pieces — BalanceCard, ActionCard, StatusStrip

**Files:** Create `frontend/src/lib/components/{BalanceCard,ActionCard,StatusStrip}.svelte`; tests `{BalanceCard,ActionCard,StatusStrip}.test.ts`.

**Interfaces:**
- Consumes: `formatAmount` (`lib/format.ts`); stores `balances`, `node`, `plasmaInfo` (`stores/plasma`), `delegation` (`stores/pillar`).
- Produces:
  - `BalanceCard` — props `symbol: string`, `amount: string` (base units), `decimals: number`, `tint: 'green'|'blue'`.
  - `ActionCard` — props `label: string`, `direction: 'send'|'receive'`; forwards `on:click`.
  - `StatusStrip` — no props; reads stores.

- [ ] **Step 1: Write `BalanceCard.svelte`**

```svelte
<script lang="ts">
  import { formatAmount } from '../format'
  export let symbol = ''
  export let amount = '0'
  export let decimals = 8
  export let tint: 'green' | 'blue' = 'green'
  const tints = {
    green: 'border-accent/40 bg-accent/5',
    blue: 'border-qsr/40 bg-qsr/5',
  }
  const nums = { green: 'text-accent', blue: 'text-qsr' }
</script>
<div class="rounded border p-4 {tints[tint]}">
  <div class="text-xs text-muted">{symbol}</div>
  <div class="mt-1 font-mono text-2xl {nums[tint]}" aria-label={`${symbol} balance`}>{formatAmount(amount, decimals)}</div>
</div>
```

- [ ] **Step 2: Write `ActionCard.svelte`**

```svelte
<script lang="ts">
  export let label = ''
  export let direction: 'send' | 'receive' = 'send'
</script>
<button on:click aria-label={label}
  class="flex flex-col items-center justify-center gap-1 rounded border border-border bg-surface p-4 text-text transition hover:bg-elevated hover:border-accent">
  <span class="text-accent" aria-hidden="true">
    {#if direction === 'send'}↑{:else}↓{/if}
  </span>
  <span class="text-sm">{label}</span>
</button>
```

(Implementer note: the screenshots use circled up/down arrows — swap the glyph for a Lucide-style inline SVG if you prefer; the `↑/↓` is a working placeholder, refine via frontend-design.)

- [ ] **Step 3: Write `StatusStrip.svelte`**

```svelte
<script lang="ts">
  import { node } from '../stores/node'
  import { balances } from '../stores/balances'
  import { plasmaInfo } from '../stores/plasma'
  import { delegation } from '../stores/pillar'
  function plasmaLevel(p: number): string {
    if (p >= 84000) return 'High'
    if (p >= 21000) return 'Medium'
    if (p > 0) return 'Low'
    return 'None'
  }
  $: pillarName = $delegation && $delegation.name ? $delegation.name : 'None'
</script>
<div class="flex flex-wrap items-center gap-x-6 gap-y-1 rounded border border-border bg-surface px-4 py-2 text-sm text-muted">
  <span>Account Height: <span class="font-medium text-text">{$node.height}</span></span>
  <span>Tokens: <span class="font-medium text-text">{$balances.length}</span></span>
  <span>Plasma: <span class="font-medium text-accent">⚡ {plasmaLevel($plasmaInfo?.currentPlasma ?? 0)}</span></span>
  <span>Pillar: <span class="font-medium text-text">{pillarName}</span></span>
</div>
```

(Implementer note: the High/Medium/Low thresholds are derived from typical plasma needs; adjust if the wallet already has a canonical mapping — search `currentPlasma`/`plasmaLevel` first.)

- [ ] **Step 4: Write tests**

```ts
// BalanceCard.test.ts
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import BalanceCard from './BalanceCard.svelte'
describe('BalanceCard', () => {
  it('renders symbol + mono formatted amount with tint', () => {
    render(BalanceCard, { props: { symbol: 'ZNN', amount: '150000000', decimals: 8, tint: 'green' } })
    const el = screen.getByLabelText('ZNN balance')
    expect(el.className).toContain('font-mono')
    expect(el.textContent).toContain('1.5')
  })
})
```

```ts
// ActionCard.test.ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import ActionCard from './ActionCard.svelte'
describe('ActionCard', () => {
  it('fires click', async () => {
    const onClick = vi.fn()
    const { component } = render(ActionCard, { props: { label: 'Send', direction: 'send' } })
    component.$on('click', onClick)
    await fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(onClick).toHaveBeenCalled()
  })
})
```

```ts
// StatusStrip.test.ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../stores/node', () => ({ node: { subscribe: (f: any) => { f({ height: 9 }); return () => {} } } }))
vi.mock('../stores/balances', () => ({ balances: { subscribe: (f: any) => { f([{ zts: 'z', symbol: 'ZNN', decimals: 8, amount: '0' }]); return () => {} } } }))
vi.mock('../stores/plasma', () => ({ plasmaInfo: { subscribe: (f: any) => { f({ currentPlasma: 90000 }); return () => {} } } }))
vi.mock('../stores/pillar', () => ({ delegation: { subscribe: (f: any) => { f(null); return () => {} } } }))
import StatusStrip from './StatusStrip.svelte'
describe('StatusStrip', () => {
  it('renders the four stats', () => {
    render(StatusStrip)
    expect(screen.getByText('9')).toBeTruthy()        // height
    expect(screen.getByText('1')).toBeTruthy()        // tokens
    expect(screen.getByText(/High/)).toBeTruthy()     // plasma
    expect(screen.getByText('None')).toBeTruthy()     // pillar
  })
})
```

- [ ] **Step 5: Run tests + types**

Run: `cd frontend && pnpm test -- src/lib/components/BalanceCard src/lib/components/ActionCard src/lib/components/StatusStrip && pnpm run check`
Expected: pass; svelte-check 0.

- [ ] **Step 6: Stage**

`git add frontend/src/lib/components/BalanceCard.svelte frontend/src/lib/components/ActionCard.svelte frontend/src/lib/components/StatusStrip.svelte frontend/src/lib/components/*.test.ts`

---

## Task 4: Send & Receive modals

**Files:** Create `frontend/src/lib/components/{SendModal,ReceiveModal}.svelte`; tests `{SendModal,ReceiveModal}.test.ts`.

**Interfaces:**
- Consumes: `SendForm` (`lib/components/SendForm.svelte`), `tx`/`prepare` (`stores/tx`), `TxModal`, `TxResult`, `balances`, `AddressDisplay`, `Card`, `Button`.
- Produces: `SendModal`/`ReceiveModal` — prop `open: boolean`; forward `on:close`.

A small inline overlay wrapper is fine (no new dependency). Reuse the exact Send logic from `routes/Send.svelte` (the `toBase` helper + `prepare`).

- [ ] **Step 1: Write `SendModal.svelte`** (reuses SendForm + the existing tx flow)

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { balances } from '../stores/balances'
  import { tx, prepare, resetTx } from '../stores/tx'
  import SendForm from './SendForm.svelte'
  import TxModal from './TxModal.svelte'
  import TxResult from './TxResult.svelte'
  export let open = false
  const dispatch = createEventDispatcher()
  function toBase(decimal: string, decimals: number): string {
    const [i, f = ''] = decimal.split('.')
    const frac = (f + '0'.repeat(decimals)).slice(0, decimals)
    return (BigInt(i || '0') * BigInt(10) ** BigInt(decimals) + BigInt(frac || '0')).toString()
  }
  async function onSend(e: CustomEvent) {
    const { recipient, zts, amountDecimal } = e.detail
    const tok = $balances.find((b) => b.zts === zts)
    await prepare(recipient, zts, toBase(amountDecimal, tok?.decimals ?? 8))
  }
  function close() { resetTx(); open = false; dispatch('close') }
</script>
{#if open}
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4" on:click|self={close} role="presentation">
    <div class="w-[28rem] rounded border border-border bg-elevated p-5 space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg">Send</h2>
        <button class="text-muted hover:text-text" aria-label="close" on:click={close}>✕</button>
      </div>
      <SendForm on:send={onSend} />
      {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
      {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
      {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
      {#if $tx.status === 'done'}<TxResult />{/if}
    </div>
  </div>
{/if}
```

- [ ] **Step 2: Write `ReceiveModal.svelte`**

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { wallet } from '../stores/wallet'
  import AddressDisplay from './AddressDisplay.svelte'
  export let open = false
  const dispatch = createEventDispatcher()
  $: address = $wallet.accounts.find((a) => a.index === $wallet.active)?.address ?? ''
  function close() { open = false; dispatch('close') }
</script>
{#if open}
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4" on:click|self={close} role="presentation">
    <div class="w-[28rem] rounded border border-border bg-elevated p-5 space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg">Receive</h2>
        <button class="text-muted hover:text-text" aria-label="close" on:click={close}>✕</button>
      </div>
      <AddressDisplay {address} />
    </div>
  </div>
{/if}
```

- [ ] **Step 3: Write tests**

```ts
// SendModal.test.ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/TxService', () => ({ PrepareSend: vi.fn(), ConfirmPublish: vi.fn(), CancelPending: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))
vi.mock('../stores/balances', () => ({ balances: { subscribe: (f: any) => { f([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '0' }]); return () => {} } } }))
import SendModal from './SendModal.svelte'
describe('SendModal', () => {
  it('renders the send form when open', () => {
    render(SendModal, { props: { open: true } })
    expect(screen.getByLabelText('recipient')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'close' })).toBeTruthy()
  })
  it('renders nothing when closed', () => {
    render(SendModal, { props: { open: false } })
    expect(screen.queryByLabelText('recipient')).toBeNull()
  })
})
```

```ts
// ReceiveModal.test.ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/runtime/runtime', () => ({ ClipboardSetText: vi.fn() }))
vi.mock('../stores/wallet', () => ({ wallet: { subscribe: (f: any) => { f({ accounts: [{ index: 0, address: 'z1qtest' }], active: 0 }); return () => {} } } }))
import ReceiveModal from './ReceiveModal.svelte'
describe('ReceiveModal', () => {
  it('shows the active address when open', async () => {
    render(ReceiveModal, { props: { open: true } })
    expect(await screen.findByText('z1qtest')).toBeTruthy()
  })
})
```

- [ ] **Step 4: Run tests + types**

Run: `cd frontend && pnpm test -- src/lib/components/SendModal src/lib/components/ReceiveModal && pnpm run check`
Expected: pass; svelte-check 0.

- [ ] **Step 5: Stage**

`git add frontend/src/lib/components/SendModal.svelte frontend/src/lib/components/ReceiveModal.svelte frontend/src/lib/components/SendModal.test.ts frontend/src/lib/components/ReceiveModal.test.ts`

---

## Task 5: Tokens panel + placeholder

**Files:** Create `frontend/src/lib/components/panels/{TokensPanel,PanelPlaceholder}.svelte`; test `panels/TokensPanel.test.ts`.

**Interfaces:**
- Consumes: `balances` store, `formatAmount`, `Input`.
- Produces: `TokensPanel` (holdings list + search), `PanelPlaceholder` (prop `name: string`).

The nom-ui Tokens tab is a **holdings list with search** (symbol, name, balance) — NOT token management. (Issue/mint/burn stays on the existing `routes/Tokens.svelte` for now; re-homing it is a follow-up. Do not delete that route.)

- [ ] **Step 1: Write `PanelPlaceholder.svelte`**

```svelte
<script lang="ts">
  export let name = ''
</script>
<div class="p-8 text-center text-muted">
  <p>The {name} view is being restyled (Plan B).</p>
</div>
```

- [ ] **Step 2: Write `TokensPanel.svelte`**

```svelte
<script lang="ts">
  import { balances } from '../../stores/balances'
  import { formatAmount } from '../../format'
  import Input from '../ui/Input.svelte'
  let q = ''
  $: filtered = $balances.filter((b) => {
    const s = q.trim().toLowerCase()
    return !s || (b.symbol || '').toLowerCase().includes(s) || (b.zts || '').toLowerCase().includes(s)
  })
</script>
<div class="space-y-3 p-4">
  <Input bind:value={q} placeholder="Search tokens…" ariaLabel="search tokens" />
  {#each filtered as b}
    <div class="flex items-center justify-between rounded border border-border bg-surface px-4 py-3">
      <div>
        <div class="font-medium">{b.symbol || b.zts}</div>
        <div class="text-xs text-muted font-mono">{b.zts}</div>
      </div>
      <div class="font-mono">{formatAmount(b.amount, b.decimals || 8)}</div>
    </div>
  {:else}
    <p class="text-sm text-muted">No tokens.</p>
  {/each}
</div>
```

- [ ] **Step 3: Write `panels/TokensPanel.test.ts`**

```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
vi.mock('../../stores/balances', () => ({ balances: { subscribe: (f: any) => { f([
  { zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' },
  { zts: 'zts1abc', symbol: 'RETARD', decimals: 8, amount: '80000000' },
]); return () => {} } } }))
import TokensPanel from './TokensPanel.svelte'
describe('TokensPanel', () => {
  it('lists tokens and filters by search', async () => {
    render(TokensPanel)
    expect(screen.getByText('ZNN')).toBeTruthy()
    expect(screen.getByText('RETARD')).toBeTruthy()
    await fireEvent.input(screen.getByLabelText('search tokens'), { target: { value: 'reta' } })
    expect(screen.queryByText('ZNN')).toBeNull()
    expect(screen.getByText('RETARD')).toBeTruthy()
  })
})
```

- [ ] **Step 4: Run tests + types**

Run: `cd frontend && pnpm test -- src/lib/components/panels/TokensPanel && pnpm run check`
Expected: pass; svelte-check 0.

- [ ] **Step 5: Stage**

`git add frontend/src/lib/components/panels`

---

## Task 6: Home shell + routing

**Files:** Create `frontend/src/routes/Home.svelte`, test `routes/Home.test.ts`; modify `frontend/src/App.svelte`.

**Interfaces:**
- Consumes: everything from Tasks 2–5 + `AccountSwitcher`, `wallet`/`lock`, `balances`/`loadBalances`, `node`/`initNodeEvents`, `refreshPlasma`, `refreshPillars`, `loadTxs`/`loadUnreceived` (existing), `ConfigService`.
- Produces: `Home.svelte` — the single unlocked page.

- [ ] **Step 1: Write `Home.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import { wallet, lock } from '../lib/stores/wallet'
  import { balances, loadBalances } from '../lib/stores/balances'
  import { initNodeEvents } from '../lib/stores/node'
  import { refreshPlasma } from '../lib/stores/plasma'
  import { refreshPillars } from '../lib/stores/pillar'
  import * as Cfg from '../../wailsjs/go/app/ConfigService'
  import * as N from '../../wailsjs/go/app/NodeService'
  import AccountSwitcher from '../lib/components/AccountSwitcher.svelte'
  import BalanceCard from '../lib/components/BalanceCard.svelte'
  import ActionCard from '../lib/components/ActionCard.svelte'
  import StatusStrip from '../lib/components/StatusStrip.svelte'
  import Tabs from '../lib/components/ui/Tabs.svelte'
  import Button from '../lib/components/ui/Button.svelte'
  import SendModal from '../lib/components/SendModal.svelte'
  import ReceiveModal from '../lib/components/ReceiveModal.svelte'
  import TokensPanel from '../lib/components/panels/TokensPanel.svelte'
  import PanelPlaceholder from '../lib/components/panels/PanelPlaceholder.svelte'

  const TABS = ['Tokens', 'Rewards', 'Plasma', 'Pillar', 'Staking', 'Sentinels', 'Accelerator']
  let active = 'Tokens'
  let sendOpen = false
  let receiveOpen = false
  let autoReceive = false

  function bal(sym: string) { return $balances.find((b) => b.symbol === sym) }
  async function refresh() { await Promise.all([loadBalances(), refreshPlasma(), refreshPillars()]) }
  onMount(async () => {
    initNodeEvents(refresh)
    refresh()
    try { autoReceive = (await Cfg.GetSettings()).autoReceive } catch {}
  })
  $: if ($wallet.active >= 0) refresh()
  async function toggleAutoReceive() {
    try {
      const s = await Cfg.GetSettings(); s.autoReceive = autoReceive; await Cfg.SetSettings(s)
      if (autoReceive) await N.StartAutoReceive(); else await N.StopAutoReceive()
    } catch {}
  }
</script>

<div class="mx-auto mt-6 w-[56rem] max-w-full space-y-4 px-4">
  <div class="flex items-center justify-between">
    <AccountSwitcher />
    <div class="flex items-center gap-3">
      <label class="flex items-center gap-1 text-xs text-muted">
        <input type="checkbox" bind:checked={autoReceive} on:change={toggleAutoReceive} /> Auto-receive
      </label>
      <Button variant="ghost" on:click={lock}>Lock</Button>
    </div>
  </div>

  <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
    <BalanceCard symbol="ZNN" amount={bal('ZNN')?.amount ?? '0'} decimals={bal('ZNN')?.decimals ?? 8} tint="green" />
    <BalanceCard symbol="QSR" amount={bal('QSR')?.amount ?? '0'} decimals={bal('QSR')?.decimals ?? 8} tint="blue" />
    <ActionCard label="Send" direction="send" on:click={() => (sendOpen = true)} />
    <ActionCard label="Receive" direction="receive" on:click={() => (receiveOpen = true)} />
  </div>

  <StatusStrip />

  <div class="rounded border border-border bg-surface">
    <Tabs tabs={TABS} bind:active />
    {#if active === 'Tokens'}<TokensPanel />{:else}<PanelPlaceholder name={active} />{/if}
  </div>
</div>

<SendModal bind:open={sendOpen} />
<ReceiveModal bind:open={receiveOpen} />
```

- [ ] **Step 2: Update `frontend/src/App.svelte`** — route unlocked → `Home` (keep locked screens; drop the per-feature `$view` branches; leave the route files in place for Plan B)

```svelte
<script lang="ts">
  import './app.css'
  import { onMount } from 'svelte'
  import { wallet } from './lib/stores/wallet'
  import { view } from './lib/stores/nav'
  import * as N from '../wailsjs/go/app/NodeService'
  import Unlock from './routes/Unlock.svelte'
  import Create from './routes/Create.svelte'
  import ImportMnemonic from './routes/ImportMnemonic.svelte'
  import Home from './routes/Home.svelte'
  import Settings from './routes/Settings.svelte'
  onMount(async () => { try { await N.Connect() } catch {} })
</script>
{#if $wallet.locked && $view === 'create'}
  <Create />
{:else if $wallet.locked && $view === 'import'}
  <ImportMnemonic />
{:else if $wallet.locked}
  <Unlock />
{:else if $view === 'settings'}
  <Settings />
{:else}
  <Home />
{/if}
```

(Settings stays reachable via `view.set('settings')`; a Settings entry point lives in the top bar in the follow-up. For Plan A, Settings remains routable but is not yet linked from Home — acceptable, it is restyled in the follow-up.)

- [ ] **Step 3: Write `routes/Home.test.ts`**

```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings: vi.fn().mockResolvedValue({ autoReceive: false }), SetSettings: vi.fn() }))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ StartAutoReceive: vi.fn(), StopAutoReceive: vi.fn() }))
vi.mock('../../wailsjs/go/app/TxService', () => ({ PrepareSend: vi.fn(), ConfirmPublish: vi.fn(), CancelPending: vi.fn() }))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))
vi.mock('../lib/stores/node', () => ({ node: { subscribe: (f: any) => { f({ height: 9 }); return () => {} } }, initNodeEvents: vi.fn() }))
vi.mock('../lib/stores/plasma', () => ({ plasmaInfo: { subscribe: (f: any) => { f({ currentPlasma: 0 }); return () => {} }, set: vi.fn() }, refreshPlasma: vi.fn() }))
vi.mock('../lib/stores/pillar', () => ({ delegation: { subscribe: (f: any) => { f(null); return () => {} } }, refreshPillars: vi.fn() }))
vi.mock('../lib/stores/balances', () => ({ balances: { subscribe: (f: any) => { f([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }, { zts: 'zts1qsr', symbol: 'QSR', decimals: 8, amount: '0' }]); return () => {} } }, loadBalances: vi.fn() }))
vi.mock('../lib/stores/wallet', () => ({ wallet: { subscribe: (f: any) => { f({ accounts: [{ index: 0, address: 'z1qtest' }], active: 0, locked: false }); return () => {} } }, lock: vi.fn(), select: vi.fn(), setLabel: vi.fn() }))
import Home from './Home.svelte'
describe('Home', () => {
  it('renders balance cards, status strip, tabs; Tokens active by default; Send opens modal', async () => {
    render(Home)
    expect(screen.getByLabelText('ZNN balance').textContent).toContain('1.5')
    expect(screen.getByLabelText('QSR balance')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'tab Tokens' }).className).toContain('text-accent')
    expect(screen.getByLabelText('search tokens')).toBeTruthy()       // Tokens panel mounted
    await fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(screen.getByLabelText('recipient')).toBeTruthy()           // SendModal opened
  })
  it('switches to a placeholder tab', async () => {
    render(Home)
    await fireEvent.click(screen.getByRole('button', { name: 'tab Plasma' }))
    expect(screen.getByText(/being restyled/)).toBeTruthy()
  })
})
```

- [ ] **Step 4: Run the full frontend suite + types + build**

Run: `cd frontend && pnpm test && pnpm run check && pnpm run build`
Expected: all tests pass (new + existing that still apply); svelte-check 0; build succeeds. (Existing route tests for `Dashboard`/`Send` still pass — those files are untouched. If `Dashboard.test.ts` referenced removed nav wiring, it is unaffected since `Dashboard.svelte` still exists.)

- [ ] **Step 5: Visual verification**

Run from repo root: `GOWORK=off wails dev` against the testnet node. Confirm against the reference: 4-card row (green ZNN / blue QSR / Send / Receive), status strip, 7 green-accented tabs with Tokens showing the holdings + search, Send/Receive modals open and the send flow still confirms+publishes. Refine spacing/typography with frontend-design as needed.

- [ ] **Step 6: Stage**

`git add frontend/src/routes/Home.svelte frontend/src/routes/Home.test.ts frontend/src/App.svelte`

---

## Self-Review / Verification (end-to-end)

- `cd frontend && pnpm test` — all green (ui primitives, home pieces, modals, TokensPanel, Home).
- `pnpm run check` — 0 errors; `pnpm run build` — succeeds.
- Visual (`wails dev`): the home matches the nom-ui references; Send/Receive work; the funds path is unchanged (Send still routes through `TxModal`/`ConfirmPublish`).
- No Go/binding files changed; no existing `routes/*.svelte` deleted (Plan B owns that).

## Hand-off to Plan B

Plan B implements `RewardsPanel` (new aggregation) + `Plasma/Pillar/Staking/Sentinels/Accelerator` panels (adapted from the current route bodies), swaps them into `Home`'s `{#if active === …}`, deletes the now-dead `routes/{Plasma,Stake,Pillars,Sentinels,Accelerator,Send,Dashboard}.svelte` + `StatusBar.svelte`, and re-homes token management. The follow-up sub-project restyles Unlock/Create/Import + Settings.
