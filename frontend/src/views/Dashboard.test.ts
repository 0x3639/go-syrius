import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Dashboard from './Dashboard.vue'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

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
