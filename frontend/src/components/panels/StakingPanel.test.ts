import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import StakingPanel from './StakingPanel.vue'
import { useStakeStore } from '../../stores/stake'
import { useTxStore } from '../../stores/tx'

// Stub nom-ui Button/Input to plain elements mirroring disabled/click/v-model,
// so we exercise the panel's bindings, not nom-ui internals.
vi.mock('nom-ui', () => ({
  Button: {
    props: ['disabled'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
  },
  Input: {
    props: ['modelValue'],
    template:
      '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

// Mock the NomService preparers — each returns a distinct preview so we can
// assert the right preview is forwarded to tx.awaitConfirm.
const stakePreview = { kind: 'stake' }
const collectPreview = { kind: 'collect' }
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareStake: vi.fn(() => Promise.resolve(stakePreview)),
  PrepareCancelStake: vi.fn(() => Promise.resolve({ kind: 'cancel' })),
  PrepareCollectReward: vi.fn(() => Promise.resolve(collectPreview)),
}))

import * as Nom from '../../../wailsjs/go/app/NomService'

const REWARD = { znn: '0', qsr: '16000000000000' }
const STAKE_INFO = { totalAmount: '0', entries: [] }

function setup() {
  setActivePinia(createPinia())
  const stake = useStakeStore()
  const tx = useTxStore()
  // Don't hit the (mocked-absent) backend on mount.
  vi.spyOn(stake, 'refresh').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  stake.reward = { ...REWARD } as never
  stake.stakeInfo = { ...STAKE_INFO } as never
  return { stake, tx, awaitConfirm }
}

let wrapper: VueWrapper | null = null
function render() {
  wrapper = mount(StakingPanel)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('StakingPanel', () => {
  it('shows only the QSR staking reward and enables collection', async () => {
    setup()
    const w = render()
    await w.vm.$nextTick()

    const reward = w.get('section:nth-of-type(1)')
    expect(reward.text()).toContain('160,000 QSR')
    expect(reward.text()).not.toContain('ZNN')
    expect(reward.get('button').attributes('disabled')).toBeUndefined()
  })

  it('entering an amount + duration and clicking Stake calls PrepareStake(<base-units>, <duration>) then tx.awaitConfirm', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    await w.get('[aria-label="znn amount"]').setValue('1.5')
    await w.get('[aria-label="duration months"]').setValue('3')

    await w.get('section:nth-of-type(2) button').trigger('click') // Stake ZNN button
    await w.vm.$nextTick()

    // 1.5 ZNN at 8 decimals = 150000000 base units; duration is the plain "3" month string.
    expect(Nom.PrepareStake).toHaveBeenCalledWith('150000000', '3')
    expect(awaitConfirm).toHaveBeenCalledWith(stakePreview)
  })

  it('clicking Collect Reward calls PrepareCollectReward then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    await w.get('section:nth-of-type(1) button').trigger('click') // Collect Reward button
    await w.vm.$nextTick()

    expect(Nom.PrepareCollectReward).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith(collectPreview)
  })
})
