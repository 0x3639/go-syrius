import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import TxHistory from './TxHistory.vue'
import { useTxsStore, type TxRecord } from '../stores/txs'
import { useUnreceivedStore } from '../stores/unreceived'

// Stub nom-ui blockchain primitives to trivial templates so the test exercises
// our row composition (mapping + formatting), not nom-ui internals.
vi.mock('nom-ui', () => {
  const passthrough = (tag: string) => ({
    template: `<${tag}><slot /></${tag}>`,
  })
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

describe('TxHistory', () => {
  it('renders a row with the formatted amount', () => {
    const w = mount(TxHistory)
    useTxsStore().items = [tx]
    return w.vm.$nextTick().then(() => {
      expect(w.text()).toContain('Recent transactions')
      expect(w.text()).toContain('1.5')
      expect(w.text()).toContain('ZNN')
      // receive -> 'in', confirmed -> 'success' (mapping into nom-ui primitives)
      expect(w.text()).toContain('in')
      expect(w.text()).toContain('success')
    })
  })

  it('uses the row decimals for a non-8-decimal token, not a hardcoded 8', () => {
    const w = mount(TxHistory)
    // 1500000 base units at 6 decimals == 1.5, NOT 0.015 (the wrong 8-dec form).
    useTxsStore().items = [
      { ...tx, hash: 'h2', token: 'CUSTOM', amount: '1500000', decimals: 6 },
    ]
    return w.vm.$nextTick().then(() => {
      expect(w.text()).toContain('1.5')
      expect(w.text()).toContain('CUSTOM')
      expect(w.text()).not.toContain('0.015')
    })
  })

  it('hides zero-amount plumbing by default and shows it under All', async () => {
    const w = mount(TxHistory)
    useTxsStore().items = [tx, { ...tx, hash: 'z1', amount: '0' }]
    await w.vm.$nextTick()
    expect(w.findAll('tr').length).toBe(1) // only the real transfer
    await w.find('button[aria-label="show all transactions"]').trigger('click')
    await w.vm.$nextTick()
    expect(w.findAll('tr').length).toBe(2) // both rows now
  })

  it('shows method labels + a Pair chip under All, hidden under Transfers', async () => {
    const w = mount(TxHistory)
    useTxsStore().items = [
      { ...tx, hash: 'm1', direction: 'out', method: 'CollectReward', amount: '0' },
      { ...tx, hash: 'p1', direction: 'pair', method: '', amount: '0', token: '' },
    ]
    await w.vm.$nextTick()
    expect(w.text()).toContain('No transactions.') // default Transfers hides both
    await w.find('button[aria-label="show all transactions"]').trigger('click')
    await w.vm.$nextTick()
    expect(w.text()).toContain('CollectReward')
    expect(w.text()).toContain('Pair')
    expect(w.text()).toContain('—') // pair rows show a dash for amount/token, not the zero ZTS
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

    // While busy, the row shows the pulsing Generating Plasma / Receiving label.
    u.busy = { p1: true }
    await w.vm.$nextTick()
    expect(w.find('.animate-pulse').exists()).toBe(true)
    expect(w.text()).toContain('Generating Plasma')
  })

  it('shows the empty state when there are no txs', () => {
    const w = mount(TxHistory)
    expect(w.text()).toContain('No transactions.')
  })
})
