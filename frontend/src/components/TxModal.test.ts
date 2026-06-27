import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock TxService so clicking Confirm exercises tx.confirm() -> ConfirmPublish
// without touching Wails. CancelPending is needed because tx.cancel() calls it.
const ConfirmPublish = vi.hoisted(() => vi.fn().mockResolvedValue('hash123'))
const CancelPending = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/TxService', () => ({
  ConfirmPublish,
  CancelPending,
  PrepareSend: vi.fn(),
}))

// Stub nom-ui Button to a plain button that forwards @click + disabled.
vi.mock('nom-ui', () => ({
  Button: {
    props: ['disabled'],
    emits: ['click'],
    template:
      '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
  },
}))

import TxModal from './TxModal.vue'
import { useTxStore } from '../stores/tx'

beforeEach(() => {
  setActivePinia(createPinia())
  ConfirmPublish.mockClear()
})

describe('TxModal (confirm-what-you-sign)', () => {
  it('renders the EXACT amount from preview (formatAmountExact), not a rounded/comma form', async () => {
    const tx = useTxStore()
    tx.preview = {
      toAddress: 'z1abc',
      amount: '5045401869374',
      zts: 'zts1znn',
      symbol: 'ZNN',
      needsPoW: false,
      difficulty: 0,
      hash: 'h',
      usedPlasma: 0,
    } as any
    tx.status = 'awaiting'

    const w = mount(TxModal)

    // EXACT value being signed — 5045401869374 base units at 8 decimals.
    expect(w.text()).toContain('50454.01869374')
    // NOT the display-rounded / thousands-separated form.
    expect(w.text()).not.toContain('50,454')
  })

  it('renders the amount using the token decimals from preview, not a hardcoded 8', () => {
    // FUNDS-CRITICAL: a custom token with 6 decimals. 1500000 base units == 1.5.
    // With the old hardcoded-8 rendering this would wrongly show 0.015.
    const tx = useTxStore()
    tx.preview = {
      toAddress: 'z1abc',
      amount: '1500000',
      zts: 'zts1custom',
      symbol: 'CUSTOM',
      decimals: 6,
      needsPoW: false,
      difficulty: 0,
      hash: 'h',
      usedPlasma: 0,
    } as any
    tx.status = 'awaiting'

    const w = mount(TxModal)

    // Correct 6-decimal rendering.
    expect(w.text()).toContain('1.5 CUSTOM')
    // The WRONG 8-decimal rendering must NOT appear.
    expect(w.text()).not.toContain('0.015')
  })

  it('shows the FULL recipient address (not truncated) so the user verifies it', () => {
    const tx = useTxStore()
    const full = 'z1qrr0sample00000000000000000000000000pcjmg'
    tx.preview = {
      toAddress: full,
      amount: '100000000',
      zts: 'zts1znn',
      symbol: 'ZNN',
      needsPoW: false,
      difficulty: 0,
      hash: 'h',
      usedPlasma: 0,
    } as any
    tx.status = 'awaiting'

    const w = mount(TxModal)
    expect(w.text()).toContain(full)
  })

  it('Confirm calls ConfirmPublish (publishes the held block)', async () => {
    const tx = useTxStore()
    tx.preview = {
      toAddress: 'z1abc',
      amount: '5045401869374',
      zts: 'zts1znn',
      symbol: 'ZNN',
      needsPoW: false,
      difficulty: 0,
      hash: 'h',
      usedPlasma: 0,
    } as any
    tx.status = 'awaiting'

    const w = mount(TxModal)
    await w.findAll('button')[0].trigger('click')

    expect(ConfirmPublish).toHaveBeenCalled()
  })

  it('shows a progress indicator and no buttons while publishing (cannot abort)', () => {
    const tx = useTxStore()
    tx.preview = {
      toAddress: 'z1abc',
      amount: '150000000',
      zts: 'zts1znn',
      symbol: 'ZNN',
      needsPoW: true,
      difficulty: 0,
      hash: '',
      usedPlasma: 0,
    } as any
    tx.status = 'publishing'

    const w = mount(TxModal)
    // Confirm/Cancel are gone mid-broadcast — replaced by the PoW progress.
    expect(w.findAll('button').length).toBe(0)
    expect(w.text()).toContain('Generating Plasma')
  })
})
