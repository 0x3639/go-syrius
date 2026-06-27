import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareCollectSentinelReward: vi.fn(() => Promise.resolve({ kind: 'collect' })),
  PrepareRevokeSentinel: vi.fn(() => Promise.resolve({ kind: 'revoke' })),
}))

import SentinelActive from './SentinelActive.vue'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'

function setup(reward: unknown, sentinel: unknown) {
  setActivePinia(createPinia())
  const s = useSentinelStore()
  const tx = useTxStore()
  vi.spyOn(s, 'refresh').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  s.reward = reward as never
  s.sentinel = sentinel as never
  return { s, awaitConfirm }
}

const REVOCABLE = { owner: 'z1own', active: true, isRevocable: true, revokeCooldown: 0 }

describe('SentinelActive', () => {
  it('disables Collect when the reward is zero', () => {
    setup({ znn: '0', qsr: '0' }, REVOCABLE)
    const w = mount(SentinelActive)
    const collect = w.findAll('button').find((b) => b.text() === 'Collect')!
    expect(collect.attributes('disabled')).toBeDefined()
  })

  it('disables Revoke with a cooldown note when not revocable', () => {
    setup({ znn: '0', qsr: '0' }, { owner: 'z1own', active: true, isRevocable: false, revokeCooldown: 42 })
    const w = mount(SentinelActive)
    const revoke = w.find('button[aria-label="revoke sentinel"]')
    expect(revoke.attributes('disabled')).toBeDefined()
    expect(revoke.text()).toContain('42')
  })

  it('forwards the collect call to tx.awaitConfirm', async () => {
    const { awaitConfirm } = setup({ znn: '100', qsr: '0' }, REVOCABLE)
    const w = mount(SentinelActive)
    await w.findAll('button').find((b) => b.text() === 'Collect')!.trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'collect' })
  })
})
