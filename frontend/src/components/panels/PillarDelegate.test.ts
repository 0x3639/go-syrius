import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import PillarDelegate from './PillarDelegate.vue'
import { usePillarStore } from '../../stores/pillar'
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
const delegatePreview = { kind: 'delegate' }
const collectPreview = { kind: 'collect' }
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDelegate: vi.fn(() => Promise.resolve(delegatePreview)),
  PrepareUndelegate: vi.fn(() => Promise.resolve({ kind: 'undelegate' })),
  PrepareCollectPillarReward: vi.fn(() => Promise.resolve(collectPreview)),
}))

import * as Nom from '../../../wailsjs/go/app/NomService'

const PILLAR = {
  name: 'Pillar-One',
  rank: 1,
  weight: '100000000',
  delegateRewardPercent: 70,
  producerAddress: 'z1producer',
}
const REWARD = { znn: '100000000', qsr: '0' }
const DELEGATION = { name: 'Pillar-One', status: 1, weight: '100000000' }

function setup() {
  setActivePinia(createPinia())
  const pillar = usePillarStore()
  const tx = useTxStore()
  // Don't hit the (mocked-absent) backend on mount.
  vi.spyOn(pillar, 'refresh').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  // Seed a pillar + delegation + reward.
  pillar.pillars = [{ ...PILLAR }] as never
  pillar.delegation = { ...DELEGATION } as never
  pillar.reward = { ...REWARD } as never
  return { pillar, tx, awaitConfirm }
}

let wrapper: VueWrapper | null = null
function render() {
  wrapper = mount(PillarDelegate)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('PillarDelegate', () => {
  it('clicking Delegate on a pillar calls PrepareDelegate(name) then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    await w.get('[aria-label="delegate to Pillar-One"]').trigger('click')
    await w.vm.$nextTick()

    expect(Nom.PrepareDelegate).toHaveBeenCalledWith('Pillar-One')
    expect(awaitConfirm).toHaveBeenCalledWith(delegatePreview)
  })

  it('clicking Collect calls PrepareCollectPillarReward then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    await w.get('button:not([aria-label])').trigger('click') // the Collect button
    await w.vm.$nextTick()

    expect(Nom.PrepareCollectPillarReward).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith(collectPreview)
  })
})
