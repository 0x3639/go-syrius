import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareCollectPillarReward: vi.fn(() => Promise.resolve({ kind: 'collect' })),
  PrepareRevokePillar: vi.fn(() => Promise.resolve({ kind: 'revoke' })),
}))

import PillarActive from './PillarActive.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'

function setup(opts: { reward?: { znn: string; qsr: string }; isRevocable?: boolean; revokeCooldown?: number } = {}) {
  setActivePinia(createPinia())
  const s = usePillarStore()
  const tx = useTxStore()
  vi.spyOn(s, 'refreshRegistration').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.myPillar = {
    name: 'Pillar-A',
    ownerAddress: 'z1own',
    producerAddress: 'z1prod',
    rewardAddress: 'z1rew',
    giveMomentumRewardPct: 0,
    giveDelegateRewardPct: 100,
    isRevocable: opts.isRevocable ?? false,
    revokeCooldown: opts.revokeCooldown ?? 600,
  } as never
  s.reward = opts.reward ?? { znn: '0', qsr: '0' } as never
  return { s, tx, awaitConfirm }
}

describe('PillarActive', () => {
  it('disables Collect when reward is zero', () => {
    setup({ reward: { znn: '0', qsr: '0' } })
    const w = mount(PillarActive)
    expect(w.find('button[aria-label="collect pillar reward"]').attributes('disabled')).toBeDefined()
  })

  it('disables Revoke with a cooldown note when not revocable', () => {
    setup({ isRevocable: false, revokeCooldown: 600 })
    const w = mount(PillarActive)
    const btn = w.find('button[aria-label="revoke pillar"]')
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.text()).toContain('600')
  })

  it('forwards collect to tx.awaitConfirm', async () => {
    const { awaitConfirm } = setup({ reward: { znn: '100', qsr: '0' } })
    const w = mount(PillarActive)
    await w.find('button[aria-label="collect pillar reward"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareCollectPillarReward).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'collect' })
  })

  it('forwards revoke with the pillar name when revocable', async () => {
    setup({ isRevocable: true })
    const w = mount(PillarActive)
    await w.find('button[aria-label="revoke pillar"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareRevokePillar).toHaveBeenCalledWith('Pillar-A')
  })
})
