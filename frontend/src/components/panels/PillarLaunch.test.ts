import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareFuse: vi.fn(() => Promise.resolve({ kind: 'fuse' })),
  PreparePillarDepositQsr: vi.fn(() => Promise.resolve({ kind: 'deposit' })),
  PreparePillarWithdrawQsr: vi.fn(() => Promise.resolve({ kind: 'withdraw' })),
  PrepareRegisterPillar: vi.fn(() => Promise.resolve({ kind: 'register' })),
  CheckPillarName: vi.fn(() => Promise.resolve(true)),
}))

import PillarLaunch from './PillarLaunch.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { useWalletStore } from '../../stores/wallet'

const COST = '15000000000000' // arbitrary QSR cost for tests
const ENOUGH_PLASMA = 105000

function setup(opts: { plasma?: number; deposited?: string; cost?: string; pendingStep?: 'plasma' | 'deposit' | 'register' | null } = {}) {
  setActivePinia(createPinia())
  const s = usePillarStore()
  const tx = useTxStore()
  const wallet = useWalletStore()
  wallet.accounts = [{ index: 0, address: 'z1qtest', label: '' }] as never
  wallet.activeIndex = 0
  vi.spyOn(s, 'refreshRegistration').mockResolvedValue()
  const begin = vi.spyOn(s, 'beginPending').mockImplementation(() => {})
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.plasma = { currentPlasma: opts.plasma ?? 0, maxPlasma: 0, qsrFused: '0' } as never
  s.depositedQsr = opts.deposited ?? '0'
  s.qsrCost = opts.cost ?? COST
  s.pendingStep = opts.pendingStep ?? null
  return { s, tx, begin, awaitConfirm }
}

describe('PillarLaunch wizard', () => {
  it('step 1: shows the fuse action when plasma is short', () => {
    setup({ plasma: 0 })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="fuse plasma"]').exists()).toBe(true)
    expect(w.find('[data-state="current"]').text()).toContain('Fuse plasma')
  })

  it('step 2: shows the deposit action + burn warning + withdraw escape once plasma clears', () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: '0', cost: COST })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="deposit pillar qsr"]').exists()).toBe(true)
    expect(w.find('button[aria-label="withdraw pillar qsr"]').exists()).toBe(true)
    expect(w.text().toLowerCase()).toContain('burned')
  })

  it('step 3: shows the register form once QSR clears', () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: COST, cost: COST })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="register pillar"]').exists()).toBe(true)
  })

  it('lets the user click back to the Deposit QSR step to withdraw after it cleared', async () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: COST, cost: COST })
    const w = mount(PillarLaunch)
    // On step 3 the withdraw hatch is not shown.
    expect(w.find('button[aria-label="withdraw pillar qsr"]').exists()).toBe(false)
    // The header's "Deposit QSR" step is clickable (a completed step).
    await w.find('button[aria-label="Deposit QSR"]').trigger('click')
    expect(w.find('button[aria-label="withdraw pillar qsr"]').exists()).toBe(true)
    // It's already cleared, so the deposit button is hidden (nothing to top up).
    expect(w.find('button[aria-label="deposit pillar qsr"]').exists()).toBe(false)
  })

  it('clearing: hides actions and shows the waiting message while pending', () => {
    setup({ plasma: 0, pendingStep: 'plasma' })
    const w = mount(PillarLaunch)
    expect(w.find('button[aria-label="fuse plasma"]').exists()).toBe(false)
    expect(w.text().toLowerCase()).toContain('waiting')
  })

  it('forwards the fuse call and begins polling when it completes', async () => {
    const { tx, begin, awaitConfirm } = setup({ plasma: 0 })
    const w = mount(PillarLaunch)
    await w.find('button[aria-label="fuse plasma"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareFuse).toHaveBeenCalledWith('z1qtest', '50000000000') // 500 QSR in base units
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'fuse' })
    tx.status = 'done'
    await w.vm.$nextTick()
    expect(begin).toHaveBeenCalledWith('plasma')
  })

  it('forwards the deposit call with the QSR shortfall', async () => {
    const { awaitConfirm } = setup({ plasma: ENOUGH_PLASMA, deposited: '0', cost: COST })
    const w = mount(PillarLaunch)
    await w.find('button[aria-label="deposit pillar qsr"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PreparePillarDepositQsr).toHaveBeenCalledWith('15000000000000') // cost − deposited
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'deposit' })
  })

  it('forwards the register call with args in the correct order', async () => {
    const { awaitConfirm } = setup({ plasma: ENOUGH_PLASMA, deposited: COST, cost: COST })
    const w = mount(PillarLaunch)
    await w.find('input[aria-label="pillar name"]').setValue('my-pillar')
    await w.find('input[aria-label="producer address"]').setValue('z1producer')
    await w.find('input[aria-label="reward address"]').setValue('z1reward')
    await w.find('input[aria-label="momentum percent"]').setValue('30')
    await w.find('input[aria-label="delegate percent"]').setValue('70')
    await w.vm.$nextTick() // let the name-availability watcher resolve
    await new Promise((r) => setTimeout(r))
    await w.find('button[aria-label="register pillar"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareRegisterPillar).toHaveBeenCalledWith('my-pillar', 'z1producer', 'z1reward', 30, 70)
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'register' })
  })

  it('disables register when the name is invalid', async () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: COST, cost: COST })
    const w = mount(PillarLaunch)
    await w.find('input[aria-label="pillar name"]').setValue('bad name!')
    expect(w.find('button[aria-label="register pillar"]').attributes('disabled')).toBeDefined()
  })

  it('resets the form defaults to the new address when the account switches', async () => {
    setup({ plasma: ENOUGH_PLASMA, deposited: COST, cost: COST })
    const wallet = useWalletStore()
    wallet.accounts = [
      { index: 0, address: 'z1qtest', label: '' },
      { index: 1, address: 'z1other', label: '' },
    ] as never
    const w = mount(PillarLaunch)
    // Type a custom name + producer on the first account.
    await w.find('input[aria-label="pillar name"]').setValue('keep-me')
    await w.find('input[aria-label="producer address"]').setValue('z1custom')
    // Switch to the second account.
    wallet.activeIndex = 1
    await w.vm.$nextTick()
    expect((w.find('input[aria-label="producer address"]').element as HTMLInputElement).value).toBe('z1other')
    expect((w.find('input[aria-label="reward address"]').element as HTMLInputElement).value).toBe('z1other')
    expect((w.find('input[aria-label="pillar name"]').element as HTMLInputElement).value).toBe('')
  })
})
