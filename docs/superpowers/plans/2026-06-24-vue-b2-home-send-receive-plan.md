# Vue B2 — Home + Send/Receive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Build the real Home (account bar, balances, status strip, nom-ui Tabs with the Tokens panel, tx history) plus the Send and Receive funds flows, porting the merged Svelte behavior verbatim onto Vue + nom-ui.

**Architecture:** Pinia stores port the Svelte stores 1:1 (balances/tx/txs/unreceived/token/plasma + node events + wallet accounts). The `home` route becomes the real `Home.vue`. Send/Receive are nom-ui `Dialog`s; the `tx` store keeps the confirm-what-you-sign contract; `TxModal` renders the **built block** (not form inputs) via `formatAmountExact`. nom-ui `Tabs/Address/TxStatus/TxDirection/TokenIcon/CopyButton/Table/sonner` provide presentation; our `formatAmount`/`formatAmountExact` handle all amounts.

**Tech Stack:** Vue 3.4 + vue-router + Pinia, Tailwind 4 + nom-ui, Vitest + @vue/test-utils, `qrcode`.

## Global Constraints

- **Branch `frontend-vue-migration`** (after B1 `bc734db`); not merged until B4.
- **Frontend-only:** NO `app/*.go`/`internal/*` changes; bindings consumed as-is.
- **Faithful port** of the merged Svelte components (`main:frontend/src/...`): same validation, copy, thresholds, behavior. nom-ui replaces presentation only.
- **Funds-safety (non-negotiable):** frontend sends intent only; `TxModal` renders the `SendPreview` from the **built block** returned by `PrepareSend`, never the form inputs; confirm modal amounts use `formatAmountExact`; `confirm()`→`ConfirmPublish()`, `cancel()`→`CancelPending()`. No key material in the frontend.
- **Amounts:** always `src/lib/format.ts` (`formatAmount` display / `formatAmountExact` exact). Never nom-ui `Amount` (Number() precision loss).
- **z1 address regex (verbatim from Svelte):** `/^z1[0-9a-z]{38}$/`. **decimal→base** conversion (verbatim): `(BigInt(i||'0') * 10n**BigInt(decimals) + BigInt(frac||'0')).toString()` where `[i,f='']=decimal.split('.')`, `frac=(f+'0'.repeat(decimals)).slice(0,decimals)`.
- **Plasma thresholds (verbatim):** `>=84000 High`, `>=21000 Medium`, `>0 Low`, else `None`.
- **nom-ui** (`github:digitalSloth/nom-ui#63f755a…`, installed): verify each component's export/props against `node_modules/nom-ui/src` before relying on it; adapt or fall back + note it (A/B1 discipline).
- Commands in `frontend/`: `pnpm test`, `pnpm run typecheck`, `pnpm run build`. wails=`~/go/bin/wails`. Commits GPG-signed: **implementers STAGE only**; keep `go.mod` 2.12.0 churn out.

## File Structure

- `src/stores/`: extend `wallet.ts` (accounts); new `balances.ts`, `tx.ts`, `txs.ts`, `unreceived.ts`, `token.ts`, `plasma.ts`, `nodeEvents.ts` (or extend `node.ts`).
- `src/components/`: `BalanceCard.vue`, `ActionCard.vue`, `StatusStrip.vue`, `AccountSwitcher.vue`, `AmountInput.vue`, `TxHistory.vue`, `TokensPanel.vue`, `SendForm.vue`, `SendModal.vue`, `TxModal.vue`, `TxResult.vue`, `ReceiveModal.vue`, `UnreceivedPanel.vue`, `AddressDisplay.vue`, `Toaster` mount in `App.vue`.
- `src/views/Home.vue` (real), `src/views/Settings.vue` (placeholder), `src/router/index.ts` (+settings route).
- Tests colocated `*.test.ts`.

---

## Task 1: Extend the wallet store with accounts (CurrentAccounts/select/setLabel)

**Files:** Modify `src/stores/wallet.ts`, `src/stores/wallet.test.ts`.

**Interfaces:**
- Consumes: `WalletService.CurrentAccounts()` → `AccountInfo[]` (`{ index: number; address: string; label: string }` — confirm shape in `wailsjs/go/models.ts`), `SelectAccount(index)`, `SetAccountLabel(index, label)`.
- Produces (added to `useWalletStore`): state `accounts: AccountInfo[]`, `activeIndex: number`; actions `loadAccounts()`, `select(index)`, `setLabel(index, label)`. `unlock()` and `importMnemonic→unlock` now also call `loadAccounts()` and set `activeIndex=0`. `lock()` clears `accounts=[]`, `activeIndex=0`.

- [ ] **Step 1: Add accounts state + actions to `src/stores/wallet.ts`**

Add `AccountInfo` type + state fields, and these actions (keep existing):

```ts
export type AccountInfo = { index: number; address: string; label: string }
// state: () => ({ locked: true, wallets: [], active: '', accounts: [] as AccountInfo[], activeIndex: 0 }),
    async loadAccounts() {
      try { this.accounts = (await W.CurrentAccounts()) as unknown as AccountInfo[] } catch { this.accounts = [] }
    },
    async select(index: number) {
      await W.SelectAccount(index)
      this.activeIndex = index
    },
    async setLabel(index: number, label: string) {
      await W.SetAccountLabel(index, label)
      await this.loadAccounts()
    },
    activeAddress(): string {
      return this.accounts.find((a) => a.index === this.activeIndex)?.address ?? ''
    },
```

In the existing `unlock(name, password)` action, after setting `locked=false`/`active=name`, add `await this.loadAccounts(); this.activeIndex = 0`. In `lock()`, add `this.accounts = []; this.activeIndex = 0`.

- [ ] **Step 2: Update `src/stores/wallet.test.ts`** — add to the `vi.mock` factory: `CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qxxx', label: '' }]), SelectAccount: vi.fn().mockResolvedValue(undefined), SetAccountLabel: vi.fn().mockResolvedValue(undefined),`. Add a test:

```ts
  it('loads accounts on unlock and selects by index', async () => {
    const s = useWalletStore()
    await s.unlock('Main', 'pw')
    expect(s.accounts).toEqual([{ index: 0, address: 'z1qxxx', label: '' }])
    expect(s.activeAddress()).toBe('z1qxxx')
    await s.select(0)
    expect(s.activeIndex).toBe(0)
  })
```

- [ ] **Step 3: Run** `cd frontend && pnpm test -- src/stores/wallet && pnpm run typecheck` → pass + clean.
- [ ] **Step 4: Stage** `git add frontend/src/stores/wallet.ts frontend/src/stores/wallet.test.ts`. No commit.

---

## Task 2: Data stores (balances, txs, unreceived, token, plasma, pillar-delegation) + node events

**Files:** Create `src/stores/balances.ts`, `txs.ts`, `unreceived.ts`, `token.ts`, `plasma.ts`; extend `src/stores/node.ts`; tests `src/stores/{balances,tx-data}.test.ts`.

**Interfaces:**
- Produces Pinia stores (ports of the Svelte writables):
  - `useBalancesStore`: `items: TokenBalance[]`; `load()` → `NodeService.GetBalances()`. `TokenBalance={zts,symbol,decimals,amount}`.
  - `useTxsStore`: `items: TxRecord[]`; `load(page=0,count=25)` → `NodeService.GetTransactions`. `TxRecord={hash,direction,counterparty,token,amount,momentumHeight,confirmed,timestamp}`.
  - `useUnreceivedStore`: `items: Unreceived[]`; `load()` → `NodeService.GetUnreceived`; `receive(hash)`/`receiveAll()` → `TxService.Receive` with per-hash `busy` map + `busyAll` + `error` (port `UnreceivedPanel`'s logic into the store). `Unreceived={fromHash,fromAddress,token,amount}`.
  - `useTokenStore`: `myTokens`, `lookedUp`; `refresh()`→`NomService.GetMyTokens`, `lookup(zts)`→`NomService.GetTokenByZts`.
  - `usePlasmaStore`: `info`; `refresh()`→`NomService.GetPlasmaInfo`. (Fusion entries deferred to B3 plasma panel.)
  - `useNodeStore` (extend): `height`, `status`, `syncing`; `initEvents(onTick)` registers `EventsOn('node:status'|'node:sync'|'momentum:tick')` ONCE (guard with an `eventsInit` flag) from `../../wailsjs/runtime/runtime`. Keep A's `connect`/`loadBalances` or delegate to balances store.
  - `usePillarStore`: `delegation`; `refreshDelegation()`→`NomService.GetDelegation` (minimal — full pillar panel is B3).

- [ ] **Step 1: Write the stores** — each is a thin `defineStore` over the named binding, mirroring the Svelte writables above (try/catch → empty on failure, EXCEPT `unreceived.receive`/`receiveAll` which surface errors). For `node.initEvents`, guard double-registration:

```ts
// node.ts (additions)
import { EventsOn } from '../../wailsjs/runtime/runtime'
// state adds: height: 0, syncing: false ; (keep connected/balances or move balances out)
    initEvents(onTick: () => void) {
      if (this._eventsInit) return
      this._eventsInit = true
      EventsOn('node:status', (s: any) => { this.connected = !!s?.connected; this.height = s?.height ?? this.height })
      EventsOn('node:sync', (s: any) => { this.syncing = s?.state !== 'synced' })
      EventsOn('momentum:tick', () => onTick())
    },
```
(`_eventsInit` is a non-reactive flag on state initialised `false`.)

- [ ] **Step 2: Tests** — `balances.test.ts` (load sets items from mocked GetBalances); `tx-data.test.ts` covering txs.load, unreceived.load + receive (mock `TxService.Receive`, assert busy + error surfacing). Mock the bindings via `vi.hoisted`+`vi.mock` as in B1.

```ts
// example assertion for unreceived.receive
import { useUnreceivedStore } from './unreceived'
it('receive calls TxService.Receive and surfaces errors', async () => {
  const s = useUnreceivedStore()
  await s.receive('h1'); expect(Receive).toHaveBeenCalledWith('h1')
  Receive.mockRejectedValueOnce(new Error('boom'))
  await s.receive('h2'); expect(s.error).toBe('boom')
})
```

- [ ] **Step 3: Run** `pnpm test -- src/stores && pnpm run typecheck` → pass + clean.
- [ ] **Step 4: Stage** the new/modified store files + tests. No commit.

---

## Task 3: `tx` store (confirm-what-you-sign) + Toaster

**Files:** Create `src/stores/tx.ts`, `src/stores/tx.test.ts`; modify `src/App.vue` (mount `<Toaster/>`).

**Interfaces:**
- Produces `useTxStore`: state `{ status: 'idle'|'preparing'|'awaiting'|'publishing'|'done'|'error', preview: SendPreview|null, hash: string, error: string }`; actions `prepare(toAddress, zts, amount)`, `awaitConfirm(preview)`, `confirm()`, `cancel()`, `reset()`. `SendPreview={toAddress,symbol,zts,amount,usedPlasma,difficulty,hash,needsPoW,summary?}`. Identical to the Svelte `tx.ts`.

- [ ] **Step 1: Write `src/stores/tx.ts`** — port `main:frontend/src/lib/stores/tx.ts` to Pinia verbatim (same state machine + `TxService.PrepareSend`/`ConfirmPublish`/`CancelPending`). Omit the Svelte `view.subscribe` reset; the route reset is wired in Home (Task 10) via a router watch.

```ts
import { defineStore } from 'pinia'
import * as Tx from '../../wailsjs/go/app/TxService'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean; summary?: string }

export const useTxStore = defineStore('tx', {
  state: () => ({ status: 'idle' as 'idle'|'preparing'|'awaiting'|'publishing'|'done'|'error', preview: null as SendPreview | null, hash: '', error: '' }),
  actions: {
    reset() { this.status = 'idle'; this.preview = null; this.hash = ''; this.error = '' },
    async prepare(toAddress: string, zts: string, amount: string) {
      this.status = 'preparing'; this.preview = null; this.hash = ''; this.error = ''
      try {
        this.preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
        this.status = 'awaiting'
      } catch (e: any) { this.status = 'error'; this.error = e?.message ?? String(e) }
    },
    awaitConfirm(preview: SendPreview) { this.preview = preview; this.status = 'awaiting'; this.hash = ''; this.error = '' },
    async confirm() {
      this.status = 'publishing'
      try { this.hash = (await Tx.ConfirmPublish()) as string; this.status = 'done'; this.preview = null }
      catch (e: any) { this.status = 'error'; this.error = e?.message ?? String(e) }
    },
    async cancel() { await Tx.CancelPending().catch(() => {}); this.reset() },
  },
})
```

- [ ] **Step 2: Mount `<Toaster/>` in `src/App.vue`** — add nom-ui `Toaster` (verify export name — likely `Toaster` from nom-ui's sonner) so `useToast` works app-wide:

```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { useTheme, Toaster } from 'nom-ui'
import * as N from '../wailsjs/go/app/NodeService'
const { setTheme } = useTheme()
onMounted(async () => { setTheme?.('dark'); try { await N.Connect() } catch {} })
</script>
<template>
  <RouterView />
  <Toaster />
</template>
```
(Verify nom-ui exports `Toaster` + a `useToast`/`toast`; if the name differs, adapt. If sonner isn't usable, fall back to no toast — note it; toasts are an enhancement, not funds-critical.)

- [ ] **Step 3: Write `src/stores/tx.test.ts`** (contract):

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
const PrepareSend = vi.hoisted(() => vi.fn())
const ConfirmPublish = vi.hoisted(() => vi.fn())
const CancelPending = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/TxService', () => ({ PrepareSend, ConfirmPublish, CancelPending }))
import { useTxStore } from './tx'
beforeEach(() => { setActivePinia(createPinia()); PrepareSend.mockReset(); ConfirmPublish.mockReset() })
describe('tx store (confirm-what-you-sign)', () => {
  it('prepare seats the built-block preview', async () => {
    PrepareSend.mockResolvedValue({ toAddress: 'z1', amount: '150000000', zts: 'zts1znn', needsPoW: true, difficulty: 1, hash: 'h' })
    const s = useTxStore(); await s.prepare('z1', 'zts1znn', '150000000')
    expect(s.status).toBe('awaiting'); expect(s.preview?.amount).toBe('150000000')
  })
  it('confirm publishes the held block', async () => {
    ConfirmPublish.mockResolvedValue('hash123')
    const s = useTxStore(); await s.confirm()
    expect(ConfirmPublish).toHaveBeenCalled(); expect(s.status).toBe('done'); expect(s.hash).toBe('hash123')
  })
  it('prepare error sets error state', async () => {
    PrepareSend.mockRejectedValue(new Error('bad addr'))
    const s = useTxStore(); await s.prepare('x', 'z', '1'); expect(s.status).toBe('error'); expect(s.error).toBe('bad addr')
  })
})
```

- [ ] **Step 4: Run** `pnpm test -- src/stores/tx && pnpm run typecheck` → pass + clean.
- [ ] **Step 5: Stage** `src/stores/tx.ts`, `src/stores/tx.test.ts`, `src/App.vue`. No commit.

---

## Task 4: Display components — BalanceCard, ActionCard, StatusStrip, AccountSwitcher, AmountInput

**Files:** Create those 5 `.vue` + a test for ActionCard (badge) and StatusStrip (plasma level).

**Interfaces:** Consumes `formatAmount`, the stores (balances/node/plasma/pillar/wallet). Produces presentational components used by Home.

- [ ] **Step 1: `BalanceCard.vue`** — props `symbol`, `amount`, `decimals`, `tint:'green'|'blue'`; render `formatAmount(amount, decimals)` in a tinted card (port `main:.../BalanceCard.svelte`; theme classes `text-accent`/`text-qsr` exist in our Tailwind theme — verify, else use `text-foreground`).
- [ ] **Step 2: `ActionCard.vue`** — props `label`, `direction:'send'|'receive'`, `badge=0`; emits `click`; renders the badge (when `badge>0`) + the up/down SVG (port `main:.../ActionCard.svelte` verbatim, `on:click`→`@click`/`$emit('click')`).
- [ ] **Step 3: `StatusStrip.vue`** — reads `useNodeStore().height`, `useBalancesStore().items.length`, `usePlasmaStore().info?.currentPlasma`, `usePillarStore().delegation?.name`; `plasmaLevel()` with the verbatim thresholds; renders the strip (port `main:.../StatusStrip.svelte`).
- [ ] **Step 4: `AccountSwitcher.vue`** — reads `useWalletStore()` (`accounts`, `activeIndex`); `<select>` over accounts (`labelFor` = `label || 'Account '+index`) → `wallet.select(index)`; edit-label affordance → `wallet.setLabel(activeIndex, draft)` (port `main:.../AccountSwitcher.svelte`).
- [ ] **Step 5: `AmountInput.vue`** — props `modelValue`, `label='Amount'`; one-way `:value` + `@input` stripping `[^0-9.]` then `$emit('update:modelValue', cleaned)` (Svelte-3 bind:value analogue; the input sanitises like the Svelte `AmountInput`).
- [ ] **Step 6: Tests** — `ActionCard.test.ts` (badge renders when `badge>0`, hidden at 0; click emits); `StatusStrip.test.ts` (plasmaLevel: 84000→High, 21000→Medium, 1→Low, 0→None) by mounting with mocked stores.

```ts
// ActionCard.test.ts
import { mount } from '@vue/test-utils'; import { describe, it, expect } from 'vitest'
import ActionCard from './ActionCard.vue'
describe('ActionCard', () => {
  it('shows a badge when pending and emits click', async () => {
    const w = mount(ActionCard, { props: { label: 'Receive', direction: 'receive', badge: 3 } })
    expect(w.text()).toContain('3')
    await w.find('button').trigger('click'); expect(w.emitted('click')).toBeTruthy()
  })
  it('hides the badge at zero', () => {
    const w = mount(ActionCard, { props: { label: 'Send', direction: 'send', badge: 0 } })
    expect(w.find('[aria-label$="pending"]').exists()).toBe(false)
  })
})
```

- [ ] **Step 7: Run** `pnpm test -- src/components && pnpm run typecheck` → pass + clean. **Stage** the 5 components + 2 tests. No commit.

---

## Task 5: `TxHistory.vue` (nom-ui Table + TxDirection/TxStatus/Address)

**Files:** Create `src/components/TxHistory.vue`, `src/components/TxHistory.test.ts`.

**Interfaces:** Consumes `useTxsStore().items`, `formatAmount`, nom-ui `Address`/`TxDirection`/`TxStatus` (verify each export+props vs `node_modules/nom-ui/src`).

- [ ] **Step 1: Write `TxHistory.vue`** — "Recent transactions" heading; empty state "No transactions."; for each `t` in `txs.items` a row using nom-ui `TxDirection` (from `t.direction`), `Address` (`t.counterparty`), `formatAmount(t.amount, 8)` + `t.token`, and `TxStatus` (from `t.confirmed`). VERIFY the nom-ui component props; if `TxDirection`/`TxStatus` props don't map cleanly to our `direction:string`/`confirmed:boolean`, fall back to the Svelte text rendering (`direction` colored span) and note it. Use nom-ui `Table` for layout if its API fits; otherwise a simple div grid (cosmetic).
- [ ] **Step 2: Test** — mounting with a mocked txs store (stub nom-ui Address/TxDirection/TxStatus) renders a row with the formatted amount and the empty state when no txs.
- [ ] **Step 3: Run** `pnpm test -- src/components/TxHistory && pnpm run typecheck`. **Stage.** No commit.

---

## Task 6: `TokensPanel.vue` (+ TokenIcon)

**Files:** Create `src/components/TokensPanel.vue`, `src/components/TokensPanel.test.ts`.

**Interfaces:** Consumes `useBalancesStore().items`, `formatAmount`, nom-ui `Input`/`TokenIcon`, `useRouter` (Manage → `/tokens`, a B4 placeholder route — for B2 the button may route to a not-yet-real route; register a placeholder `tokens` route OR disable the Manage button with a note. Simplest: route to `/settings` placeholder is wrong; instead omit Manage's navigation in B2 and add a TODO comment, OR register a `tokens` placeholder route in Task 10 like `settings`). **Decision: register a placeholder `tokens` route in Task 10** so Manage works.

- [ ] **Step 1: Write `TokensPanel.vue`** — search `Input` (filter by symbol/zts, port the Svelte filter), a Manage button (`router.push('/tokens')`), and a list of token rows (`TokenIcon` + symbol + zts + `formatAmount(amount, decimals)`), empty state "No tokens." Verify `TokenIcon` props (likely takes a token symbol/zts); fall back to a text symbol if unclear.
- [ ] **Step 2: Test** — mounting with a mocked balances store renders matching tokens; typing in search filters; empty state shows "No tokens."
- [ ] **Step 3: Run + Stage.** No commit.

---

## Task 7: `SendForm.vue` + decimal→base helper

**Files:** Create `src/components/SendForm.vue`, `src/components/SendForm.test.ts`.

**Interfaces:** Consumes `useBalancesStore().items`, `AmountInput`, nom-ui `Input`/`Button`. Emits `send` with `{ recipient, zts, amountDecimal }`. Produces the `toBase(decimal, decimals)` helper (export from `src/lib/format.ts` so SendModal reuses it).

- [ ] **Step 1: Add `toBase` to `src/lib/format.ts`** (verbatim conversion from the Svelte SendModal):

```ts
// base-unit string from a decimal string at `decimals` precision.
export function toBase(decimal: string, decimals: number): string {
  const [i, f = ''] = decimal.split('.')
  const frac = (f + '0'.repeat(decimals)).slice(0, decimals)
  return (BigInt(i || '0') * 10n ** BigInt(decimals) + BigInt(frac || '0')).toString()
}
```
Add a format.test.ts case: `toBase('1.5', 8) === '150000000'`, `toBase('200', 8) === '20000000000'`.

- [ ] **Step 2: Write `SendForm.vue`** — recipient `Input` (aria-label "recipient"), token `<select>` over `balances.items` (default to first zts), `AmountInput` (v-model amountDecimal); `validAddr=/^z1[0-9a-z]{38}$/.test(recipient)`, `validAmount=amountDecimal!==''&&Number(amountDecimal)>0`, `canSend=validAddr&&validAmount&&!!zts`; "Send" Button (disabled unless canSend) emits `send` `{recipient, zts, amountDecimal}`. Invalid-address hint when `recipient && !validAddr`.
- [ ] **Step 3: Test** — entering a valid z1 address + amount enables Send; clicking emits `send` with the values; an invalid address shows the hint and keeps Send disabled.

```ts
// SendForm.test.ts (essence)
const addr = 'z1' + 'q'.repeat(38)
await w.find('input[aria-label="recipient"]').setValue(addr)
await w.find('input[aria-label="Amount"]').setValue('1.5')
await w.find('button[aria-label="Send"]').trigger('click')
expect(w.emitted('send')![0][0]).toMatchObject({ recipient: addr, amountDecimal: '1.5' })
```

- [ ] **Step 4: Run + Stage.** No commit.

---

## Task 8: `TxModal.vue` + `TxResult.vue` + `SendModal.vue` (Dialog) — confirm-what-you-sign

**Files:** Create `src/components/TxModal.vue`, `TxResult.vue`, `SendModal.vue` + tests.

**Interfaces:** Consumes `useTxStore`, `useBalancesStore`, `formatAmountExact`, `shortAddress`, `toBase`, nom-ui `Dialog`/`Button`, `useToast`.

- [ ] **Step 1: `TxModal.vue`** — reads `tx.preview`; renders "Confirm — you are signing this exact transaction", optional `summary`, To (`shortAddress(preview.toAddress)`), **Amount `formatAmountExact(preview.amount, 8)` + symbol/zts**, Fee (`needsPoW ? 'PoW (difficulty N)' : 'Feeless (plasma)'`), Hash; Confirm button (disabled while `status==='publishing'`) → `tx.confirm()`; Cancel → `tx.cancel()`. **Amount MUST come from `preview`, never a form input.**
- [ ] **Step 2: `TxResult.vue`** — "Transaction published" + `tx.hash` + a Copy button (`ClipboardSetText` from `wailsjs/runtime/runtime`).
- [ ] **Step 3: `SendModal.vue`** — props `open` (v-model); nom-ui `Dialog` titled "Send" containing `SendForm`; on `send` → `tx.prepare(recipient, zts, toBase(amountDecimal, tok.decimals))` (tok from balances). Inside: `preparing` hint, `error` line, `<TxModal>` when `status==='awaiting'`, `<TxResult>` when `status==='done'`. Closing the Dialog → `tx.reset()` + `update:open=false`. On `status==='done'` fire `toast.success('Transaction published')` (if toast available).
- [ ] **Step 4: Tests** — `TxModal.test.ts`: with a seeded `tx.preview` (amount `5045401869374`), the modal shows `50454.01869374` (exact, not `50,454`) and Confirm calls `ConfirmPublish`. `SendModal.test.ts`: emitting `send` from a stubbed SendForm calls `PrepareSend` with the base-unit amount (`toBase`).

```ts
// TxModal.test.ts (essence) — confirm-what-you-sign exactness
const tx = useTxStore(); tx.preview = { toAddress: 'z1abc', amount: '5045401869374', zts: 'zts1znn', symbol: 'ZNN', needsPoW: false, difficulty: 0, hash: 'h', usedPlasma: 0 } as any; tx.status = 'awaiting'
const w = mount(TxModal, /* stub nom-ui */)
expect(w.text()).toContain('50454.01869374')   // exact
expect(w.text()).not.toContain('50,454')
```

- [ ] **Step 5: Run + Stage.** No commit.

---

## Task 9: `ReceiveModal.vue` + `UnreceivedPanel.vue` + `AddressDisplay.vue`

**Files:** Create those 3 `.vue` + `UnreceivedPanel.test.ts`.

**Interfaces:** Consumes `useWalletStore().activeAddress()`, `useUnreceivedStore` (load/receive/receiveAll/busy/busyAll/error), `formatAmount`, `shortAddress`, nom-ui `Dialog`/`Address`/`CopyButton`/`Button`, `qrcode`.

- [ ] **Step 1: `AddressDisplay.vue`** — props `address`; renders a QR (generate with `qrcode` → data URL in `onMounted`/watch) + nom-ui `Address` (or mono text) + nom-ui `CopyButton` (verify exports; fall back to a Copy `Button` using `ClipboardSetText`).
- [ ] **Step 2: `UnreceivedPanel.vue`** — port `main:.../UnreceivedPanel.svelte` to read `useUnreceivedStore()` (the `busy`/`busyAll`/`error`/`receive`/`receiveAll` now live IN the store from Task 2); on mount `unreceived.load()`; render the list with "Receive"/"Receive all" → "Receiving…" disabled states, error line, and the PoW hint. (The store holds the logic; the component is presentation.)
- [ ] **Step 3: `ReceiveModal.vue`** — props `open` (v-model); nom-ui `Dialog` titled "Receive" containing `AddressDisplay(activeAddress)` + `UnreceivedPanel`.
- [ ] **Step 4: Test** — `UnreceivedPanel.test.ts`: with a mocked unreceived store (one item), renders the row + a "Receive" button; clicking calls `unreceived.receive(hash)`; while busy shows "Receiving…".
- [ ] **Step 5: Run + Stage.** No commit.

---

## Task 10: `Home.vue` assembly + `settings`/`tokens` placeholder routes + auto-receive

**Files:** Modify `src/views/Home.vue` (replace A's de-risk Home), `src/router/index.ts` (+`settings`,`tokens` placeholder routes); Create `src/views/Settings.vue` (placeholder); test `src/views/Home.test.ts` (replace A's).

**Interfaces:** Composes everything; consumes all stores; `useRouter`.

- [ ] **Step 1: Register placeholder routes** in `src/router/index.ts` — add `{ path: '/settings', name: 'settings', component: () => import('../views/Settings.vue') }` and `{ path: '/tokens', name: 'tokens', component: () => import('../views/Settings.vue') }` (reuse a trivial placeholder view for both; B4 splits them). Create `src/views/Settings.vue` as a trivial `<main class="p-8 text-foreground">Settings (coming soon)</main>`.
- [ ] **Step 2: Write `Home.vue`** — port `main:frontend/src/routes/Home.svelte`:
  - account bar: `AccountSwitcher`, Auto-receive checkbox (`@change` toggle), Settings `Button`(`router.push('/settings')`), Lock `Button`(`wallet.lock()`).
  - 4-card row: 2× `BalanceCard` (znn/qsr computed from balances store), `ActionCard` Send (`@click` opens SendModal), `ActionCard` Receive (`:badge="unreceived.items.length"`, opens ReceiveModal).
  - `StatusStrip`.
  - nom-ui `Tabs` with `Tokens` → `TokensPanel`; `Rewards/Plasma/Pillar/Staking/Sentinels/Accelerator` → a shared `<PanelPlaceholder name=…/>` (trivial "coming soon" — B3 replaces). Verify nom-ui `Tabs` API (`Tabs`/`TabsList`/`TabsTrigger`/`TabsContent`); adapt the markup to it.
  - `TxHistory`.
  - `<SendModal v-model:open="sendOpen"/>`, `<ReceiveModal v-model:open="receiveOpen"/>`.
  - `refresh()` = Promise.all(balances.load, plasma.refresh, pillar.refreshDelegation, txs.load, unreceived.load); `onMounted`: `node.initEvents(refresh); refresh(); autoReceive=Cfg.GetSettings().autoReceive; if(autoReceive) startAR()`.
  - Auto-receive: `startAR`/`stopAR`/`onActiveChange`/`toggleAutoReceive` ported verbatim (using `N.StartAutoReceive`/`StopAutoReceive`, `Cfg` settings; `arAccount` tracks `wallet.activeIndex`); a `watch(() => wallet.activeIndex, ...)` drives `onActiveChange` (the Svelte `$: if ($wallet.active>=0)`); reset the `tx` store on tab change (`watch(active,…) => tx.reset()`).
- [ ] **Step 3: Replace `Home.test.ts`** — assert: renders ZNN/QSR via `formatAmount` (mocked balances), the Tabs shows the Tokens panel, the Receive ActionCard badge reflects unreceived count, Lock calls `wallet.lock()`. Mock nom-ui (Tabs/Dialog/etc.) + the bindings + a test router.
- [ ] **Step 4: Run** `pnpm test -- src/views/Home && pnpm run typecheck` → pass + clean.
- [ ] **Step 5: Stage** Home.vue, router/index.ts, Settings.vue, Home.test.ts. No commit.

---

## Task 11: Full integration + gate

**Files:** none new — verification + any glue fixes.

- [ ] **Step 1: Full suite + typecheck + build**

Run: `cd frontend && pnpm test && pnpm run typecheck && pnpm run build`
Expected: ALL tests pass (B1 + B2 stores/components/views + A's format/node); vue-tsc clean; vite build OK.

- [ ] **Step 2: Go sanity** — `cd /Users/dfriestedt/Github/go-syrius && GOWORK=off go build ./...` (frontend/dist now built) → compiles.
- [ ] **Step 3: Stage** any glue fixes. No commit.

---

## Self-Review / Verification (B2)

- `pnpm test` green (all B1+B2); `pnpm run typecheck` clean; `pnpm run build` OK.
- **Live `wails dev` gate (controller):** unlock → Home shows balances/status/tx-history; Send a small testnet tx end-to-end (form → **TxModal shows the exact built-block amount** → Confirm → published toast → history updates); Receive a pending block (badge → ReceiveModal → "Receiving…" → cleared); Auto-receive toggle sweeps; account switch re-points auto-receive; Lock → Unlock.
- **Confirm-what-you-sign verified:** TxModal amount derives from `preview` (built block) via `formatAmountExact`; never the form input.
- No `app/*.go`/`internal/*` changes; `go.mod` 2.12.0 churn not committed.

## Hand-off to B3

B3 replaces the 6 placeholder tab panels (Rewards/Plasma/Pillar/Staking/Sentinels/Accelerator) with real panels + their NomService-backed Pinia stores + actions (each NoM call reuses the `tx` store's `awaitConfirm`→`TxModal` confirm path), using the merged Svelte panels as the reference.
