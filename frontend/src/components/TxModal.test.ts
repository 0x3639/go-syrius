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

describe('TxModal decoded effect (governance/accelerator)', () => {
  it('renders every decoded effect field with FULL untruncated values', () => {
    const tx = useTxStore()
    const admin = 'z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz'
    const bigAmount = '123456789012345678901234567890'
    const longDesc = 'véry lông déscription — '.repeat(10) + '🚀 end'
    const longUrl = 'https://forum.zenon.org/some/very/long/path?with=query&and=more'
    tx.preview = {
      toAddress: 'z1gov',
      amount: '100000000',
      zts: 'zts1znn',
      symbol: 'ZNN',
      needsPoW: false,
      difficulty: 0,
      hash: '',
      usedPlasma: 0,
      summary: 'Propose "x" (1 ZNN) — Bridge — Change Administrator calls Bridge.ChangeAdministrator',
      effect: {
        contract: 'Bridge',
        method: 'ChangeAdministrator',
        fields: [
          { label: 'Proposal description', value: longDesc },
          { label: 'Proposal URL', value: longUrl },
          { label: 'administrator', value: admin },
          { label: 'minAmount', value: bigAmount },
        ],
      },
    } as any
    tx.status = 'awaiting'

    const w = mount(TxModal)
    const effect = w.find('[data-testid="tx-effect"]')
    expect(effect.exists()).toBe(true)
    // The decoded action is named and every value appears in full.
    expect(effect.text()).toContain('Bridge.ChangeAdministrator')
    expect(effect.text()).toContain(admin)
    expect(effect.text()).toContain(bigAmount)
    expect(effect.text()).toContain(longDesc)
    expect(effect.text()).toContain(longUrl)
    // Values wrap instead of truncating: the value spans carry break-all.
    const valueSpans = effect.findAll('span.break-all')
    expect(valueSpans.length).toBe(4)
  })

  it('shows no effect section for plain sends (no decoded payload)', () => {
    const tx = useTxStore()
    tx.preview = {
      toAddress: 'z1abc',
      amount: '100000000',
      zts: 'zts1znn',
      symbol: 'ZNN',
      needsPoW: false,
      difficulty: 0,
      hash: '',
      usedPlasma: 0,
    } as any
    tx.status = 'awaiting'

    const w = mount(TxModal)
    expect(w.find('[data-testid="tx-effect"]').exists()).toBe(false)
  })

  it('identifies Stake.CollectReward as QSR-only and labels 0 ZNN as the contract-call value', () => {
    const tx = useTxStore()
    tx.preview = {
      toAddress: 'z1qxemdeddedxstakexxxxxxxxxxxxxxxxjv8v62',
      amount: '0',
      zts: 'zts1znnxxxxxxxxxxxxx9z4ulx',
      symbol: 'ZNN',
      decimals: 8,
      needsPoW: false,
      difficulty: 0,
      hash: '',
      usedPlasma: 0,
      summary: 'Collect staking rewards — QSR only',
      effect: { contract: 'Stake', method: 'CollectReward', fields: [] },
    } as any
    tx.status = 'awaiting'

    const w = mount(TxModal)

    expect(w.get('[data-testid="tx-effect"]').text()).toContain('Stake.CollectReward')
    expect(w.get('[data-testid="staking-reward-asset"]').text()).toContain('QSR only')
    expect(w.text()).toContain('Contract call value')
    expect(w.text()).toContain('0 ZNN')
    expect(w.text()).toContain('no tokens sent')
    expect(w.text()).not.toContain('Amount0 ZNN')
  })
})
