import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import TokensPanel from './TokensPanel.vue'
import { useBalancesStore, type TokenBalance } from '../stores/balances'

// Stub nom-ui Input/TokenIcon to trivial templates: Input is a controlled
// <input> mirroring v-model; TokenIcon just renders its symbol. This exercises
// our filter + row composition, not nom-ui internals.
vi.mock('nom-ui', () => ({
  Input: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  TokenIcon: { props: ['symbol'], template: '<span>{{ symbol }}</span>' },
}))

beforeEach(() => {
  setActivePinia(createPinia())
})

const znn: TokenBalance = { zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }
const qsr: TokenBalance = { zts: 'zts1qsr', symbol: 'QSR', decimals: 8, amount: '500000000' }

describe('TokensPanel', () => {
  it('renders a row per token with formatted balances', () => {
    const w = mount(TokensPanel)
    useBalancesStore().items = [znn, qsr]
    return w.vm.$nextTick().then(() => {
      expect(w.text()).toContain('ZNN')
      expect(w.text()).toContain('zts1znn')
      expect(w.text()).toContain('1.5') // 150000000 / 1e8
      expect(w.text()).toContain('QSR')
      expect(w.text()).toContain('zts1qsr')
      expect(w.text()).toContain('5') // 500000000 / 1e8
    })
  })

  it('filters by symbol when typing in search', async () => {
    const w = mount(TokensPanel)
    useBalancesStore().items = [znn, qsr]
    await w.vm.$nextTick()
    await w.find('input').setValue('znn')
    expect(w.text()).toContain('ZNN')
    expect(w.text()).not.toContain('QSR')
  })

  it('shows the empty state when there are no tokens', () => {
    const w = mount(TokensPanel)
    expect(w.text()).toContain('No tokens.')
  })
})
