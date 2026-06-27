import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import TxHistory from './TxHistory.vue'
import { useTxsStore, type TxRecord } from '../stores/txs'
import { useUnreceivedStore } from '../stores/unreceived'

// Stub nom-ui blockchain primitives to trivial templates so the test exercises
// our row composition (mapping + formatting), not nom-ui internals.
vi.mock('nom-ui', () => {
  const passthrough = (tag: string) => ({ template: `<${tag}><slot /></${tag}>` })
  return {
    Table: passthrough('table'),
    TableBody: passthrough('tbody'),
    TableRow: passthrough('tr'),
    TableCell: passthrough('td'),
    TableEmpty: { template: '<tr><td colspan="4"><slot /></td></tr>' },
    TxDirection: { props: ['direction'], template: '<span>{{ direction }}</span>' },
    TxStatus: { props: ['status'], template: '<span>{{ status }}</span>' },
    Address: { props: ['address'], template: '<span>{{ address }}</span>' },
  }
})

const flush = () => new Promise((r) => setTimeout(r))
beforeEach(() => setActivePinia(createPinia()))

const tx: TxRecord = {
  hash: 'h1',
  direction: 'in',
  method: '',
  counterparty: 'z1qrr0...',
  token: 'ZNN',
  amount: '150000000',
  decimals: 8,
  momentumHeight: 1,
  confirmed: true,
  timestamp: 0,
}

// Seed the store's buffer directly (no backend fetch) for a deterministic page.
function seed(records: TxRecord[]) {
  const s = useTxsStore()
  s.buffer = records
  s.hasMoreBlocks = false
  s.page = 0
  return s
}

describe('TxHistory', () => {
  it('renders a row with the formatted amount', async () => {
    const w = mount(TxHistory)
    seed([tx])
    await w.vm.$nextTick()
    expect(w.text()).toContain('Recent transactions')
    expect(w.text()).toContain('1.5')
    expect(w.text()).toContain('ZNN')
    expect(w.text()).toContain('in') // receive -> 'in'
    expect(w.text()).toContain('success') // confirmed -> 'success'
  })

  it('uses the row decimals for a non-8-decimal token, not a hardcoded 8', async () => {
    const w = mount(TxHistory)
    seed([{ ...tx, hash: 'h2', token: 'CUSTOM', amount: '1500000', decimals: 6 }])
    await w.vm.$nextTick()
    expect(w.text()).toContain('1.5')
    expect(w.text()).toContain('CUSTOM')
    expect(w.text()).not.toContain('0.015')
  })

  it('hides zero-amount plumbing by default and shows it under All', async () => {
    const w = mount(TxHistory)
    seed([tx, { ...tx, hash: 'z1', amount: '0' }])
    await w.vm.$nextTick()
    expect(w.findAll('tr').length).toBe(1) // only the real transfer
    await w.find('button[aria-label="show all transactions"]').trigger('click')
    await flush()
    expect(w.findAll('tr').length).toBe(2) // both rows now
  })

  it('shows method labels + a Pair chip under All, hidden under Transfers', async () => {
    const w = mount(TxHistory)
    seed([
      { ...tx, hash: 'm1', direction: 'out', method: 'CollectReward', amount: '0' },
      { ...tx, hash: 'p1', direction: 'pair', method: '', amount: '0', token: '' },
    ])
    await w.vm.$nextTick()
    expect(w.text()).toContain('No transfers on this page') // default Transfers hides both
    await w.find('button[aria-label="show all transactions"]').trigger('click')
    await flush()
    expect(w.text()).toContain('CollectReward')
    expect(w.text()).toContain('Pair')
    expect(w.text()).toContain('—')
  })

  it('shows a truncated tx hash that copies the full value', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const w = mount(TxHistory)
    seed([{ ...tx, hash: 'abcdef0123456789ff' }])
    await w.vm.$nextTick()
    expect(w.text()).toContain('abcdef…89ff') // 6…4 truncation
    await w.find('button[aria-label="copy hash abcdef0123456789ff"]').trigger('click')
    expect(writeText).toHaveBeenCalledWith('abcdef0123456789ff')
  })

  it('renders — for zero-amount contract calls (Delegate/Undelegate), like Pair', async () => {
    const w = mount(TxHistory)
    seed([{ ...tx, hash: 'd1', direction: 'out', method: 'Delegate', amount: '0', token: 'ZNN' }])
    await w.vm.$nextTick()
    // Zero-amount rows are plumbing, hidden under Transfers; reveal under All.
    await w.find('button[aria-label="show all transactions"]').trigger('click')
    await flush()
    expect(w.text()).toContain('Delegate')
    expect(w.text()).toContain('—')
    expect(w.text()).not.toContain('0 ZNN')
  })

  it('lists unreceived blocks with a receive action that flips to a pulsing status', async () => {
    const w = mount(TxHistory)
    const u = useUnreceivedStore()
    u.items = [{ fromHash: 'p1', fromAddress: 'z1qsender', token: 'ZNN', amount: '100000000', decimals: 8 }]
    const receive = vi.spyOn(u, 'receive').mockResolvedValue(undefined)
    await w.vm.$nextTick()
    expect(w.text()).toContain('Unreceived')
    await w.find('button[aria-label="receive p1"]').trigger('click')
    expect(receive).toHaveBeenCalledWith('p1')

    u.busy = { p1: true }
    await w.vm.$nextTick()
    expect(w.find('.animate-pulse').exists()).toBe(true)
    expect(w.text()).toContain('Generating Plasma')
  })

  it('shows exactly 10 rows per page and pages with the arrows', async () => {
    const w = mount(TxHistory)
    const store = seed(Array.from({ length: 11 }, (_, i) => ({ ...tx, hash: `h${i}` })))
    await w.vm.$nextTick()
    expect(store.pageItems.length).toBe(10) // page is capped at 10 displayed rows
    const goto = vi.spyOn(store, 'goto').mockResolvedValue()
    expect(w.find('button[aria-label="previous page"]').attributes('disabled')).toBeDefined() // page 0
    expect(w.find('button[aria-label="next page"]').attributes('disabled')).toBeUndefined() // 11 > 10 → next
    await w.find('button[aria-label="next page"]').trigger('click')
    expect(goto).toHaveBeenCalledWith(1)
  })

  it('shows the empty state when there are no txs', () => {
    const w = mount(TxHistory)
    expect(w.text()).toContain('No transactions.')
  })
})
