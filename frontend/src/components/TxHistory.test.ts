import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import TxHistory from './TxHistory.vue'
import { useTxsStore, type TxRecord } from '../stores/txs'

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
  direction: 'receive',
  counterparty: 'z1qrr0...',
  token: 'ZNN',
  amount: '150000000',
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

  it('shows the empty state when there are no txs', () => {
    const w = mount(TxHistory)
    expect(w.text()).toContain('No transactions.')
  })
})
