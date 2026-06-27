import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDepositQsr: vi.fn(() => Promise.resolve({ kind: 'deposit' })),
  PrepareRegisterSentinel: vi.fn(() => Promise.resolve({ kind: 'register' })),
  PrepareWithdrawQsr: vi.fn(() => Promise.resolve({ kind: 'withdraw' })),
}))

import SentinelLaunch from './SentinelLaunch.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'

const CLEARED = '5000000000000' // 50,000 QSR

function setup(depositedQsr = '0', pendingStep: 'deposit' | 'register' | null = null) {
  setActivePinia(createPinia())
  const s = useSentinelStore()
  const tx = useTxStore()
  vi.spyOn(s, 'refresh').mockResolvedValue()
  const begin = vi.spyOn(s, 'beginPending').mockImplementation(() => {})
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.depositedQsr = depositedQsr
  s.pendingStep = pendingStep
  return { s, tx, begin, awaitConfirm }
}

describe('SentinelLaunch', () => {
  it('step 1: shows the deposit action when no QSR is deposited', () => {
    setup('0')
    const w = mount(SentinelLaunch)
    expect(w.find('button[aria-label="deposit qsr"]').exists()).toBe(true)
    expect(w.find('button[aria-label="register sentinel"]').exists()).toBe(false)
    expect(w.find('[data-state="current"]').text()).toContain('Deposit 50,000 QSR')
  })

  it('step 2: shows Register + the withdraw escape hatch once QSR clears', () => {
    setup(CLEARED)
    const w = mount(SentinelLaunch)
    expect(w.find('button[aria-label="register sentinel"]').exists()).toBe(true)
    expect(w.find('button[aria-label="withdraw qsr"]').exists()).toBe(true)
    expect(w.find('button[aria-label="deposit qsr"]').exists()).toBe(false)
  })

  it('clearing: shows the waiting message and hides actions while a step is pending', () => {
    setup('0', 'deposit')
    const w = mount(SentinelLaunch)
    expect(w.text()).toContain('Waiting for the Sentinel contract to credit it')
    expect(w.find('button[aria-label="deposit qsr"]').exists()).toBe(false)
  })

  it('forwards the deposit call and begins polling when it completes', async () => {
    const { tx, begin, awaitConfirm } = setup('0')
    const w = mount(SentinelLaunch)
    await w.find('button[aria-label="deposit qsr"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareDepositQsr).toHaveBeenCalledWith('5000000000000')
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'deposit' })
    tx.status = 'done'
    await w.vm.$nextTick()
    expect(begin).toHaveBeenCalledWith('deposit')
  })

  it('slow clearing: offers a Stop waiting escape that calls stopPolling', async () => {
    const { s } = setup('0', 'register')
    s.pollCount = 6
    const stop = vi.spyOn(s, 'stopPolling').mockImplementation(() => {})
    const w = mount(SentinelLaunch)
    const btn = w.find('button[aria-label="stop waiting"]')
    expect(btn.exists()).toBe(true)
    await btn.trigger('click')
    expect(stop).toHaveBeenCalled()
  })
})
