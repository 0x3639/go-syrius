import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import RewardsPanel from './RewardsPanel.vue'
import { useStakeStore } from '../../stores/stake'
import { usePillarStore } from '../../stores/pillar'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'

// Stub nom-ui Button to a plain <button> mirroring disabled + click, so we
// exercise the panel's row composition and Nom/tx bindings, not nom-ui.
vi.mock('nom-ui', () => ({
  Button: {
    props: ['disabled'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
  },
}))

// Mock the NomService collect preparers — each returns a distinct preview so we
// can assert the right preview is forwarded to tx.awaitConfirm.
const pillarPreview = { kind: 'pillar' }
const stakePreview = { kind: 'stake' }
const sentinelPreview = { kind: 'sentinel' }
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareCollectPillarReward: vi.fn(() => Promise.resolve(pillarPreview)),
  PrepareCollectReward: vi.fn(() => Promise.resolve(stakePreview)),
  PrepareCollectSentinelReward: vi.fn(() => Promise.resolve(sentinelPreview)),
}))

import * as Nom from '../../../wailsjs/go/app/NomService'

const DUAL_REWARD = { znn: '100000000', qsr: '200000000' }
const STAKE_REWARD = { znn: '0', qsr: '16000000000000' }

function setup() {
  setActivePinia(createPinia())
  const stake = useStakeStore()
  const pillar = usePillarStore()
  const sentinel = useSentinelStore()
  const tx = useTxStore()
  // Don't hit the (mocked-absent) backend on mount.
  vi.spyOn(stake, 'refresh').mockResolvedValue()
  vi.spyOn(pillar, 'refresh').mockResolvedValue()
  vi.spyOn(sentinel, 'refresh').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  // Seed rewards so every Collect button is enabled.
  stake.reward = { ...STAKE_REWARD } as never
  pillar.reward = { ...DUAL_REWARD } as never
  sentinel.reward = { ...DUAL_REWARD } as never
  return { stake, pillar, sentinel, tx, awaitConfirm }
}

function btnFor(w: VueWrapper, label: string) {
  // Each row's Collect button is uniquely identified by its aria-label.
  return w.get(`[aria-label="collect ${label}"]`)
}

let wrapper: VueWrapper | null = null
function render() {
  wrapper = mount(RewardsPanel)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  // Unmount so a lingering component (bound to a prior pinia) cannot re-fire
  // collects against the next test's mocks.
  wrapper?.unmount()
  wrapper = null
})

describe('RewardsPanel', () => {
  it('shows staking as QSR-only while retaining both assets for other reward sources', async () => {
    setup()
    const w = render()
    await w.vm.$nextTick()

    const staking = w.get('[data-testid="reward-staking"]')
    expect(staking.text()).toContain('160,000 QSR')
    expect(staking.text()).not.toContain('ZNN')

    const delegation = w.get('[data-testid="reward-delegation"]')
    expect(delegation.text()).toContain('1 ZNN')
    expect(delegation.text()).toContain('2 QSR')
  })

  it('collecting Delegation calls PrepareCollectPillarReward then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    await btnFor(w, 'Delegation').trigger('click')
    await w.vm.$nextTick()

    expect(Nom.PrepareCollectPillarReward).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith(pillarPreview)
  })

  it('collecting Staking calls PrepareCollectReward then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    await btnFor(w, 'Staking').trigger('click')
    await w.vm.$nextTick()

    expect(Nom.PrepareCollectReward).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith(stakePreview)
  })

  it('collecting Sentinel calls PrepareCollectSentinelReward then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    await btnFor(w, 'Sentinel').trigger('click')
    await w.vm.$nextTick()

    expect(Nom.PrepareCollectSentinelReward).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith(sentinelPreview)
  })

  it('disables staking collection when QSR is zero even if the shared DTO has ZNN', async () => {
    const { stake } = setup()
    stake.reward = { znn: '100000000', qsr: '0' } as never
    const w = render()
    await w.vm.$nextTick()
    expect(btnFor(w, 'Staking').attributes('disabled')).toBeDefined()
  })
})
