import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Stub the children so the container test asserts routing, not their internals.
vi.mock('./SentinelLaunch.vue', () => ({
  default: { name: 'SentinelLaunch', template: '<div data-test="launch" />' },
}))
vi.mock('./SentinelActive.vue', () => ({
  default: { name: 'SentinelActive', template: '<div data-test="active" />' },
}))

import SentinelsPanel from './SentinelsPanel.vue'
import { useSentinelStore } from '../../stores/sentinel'

function setup(sentinel: unknown) {
  setActivePinia(createPinia())
  const s = useSentinelStore()
  vi.spyOn(s, 'refresh').mockResolvedValue()
  s.sentinel = sentinel as never
  return s
}

describe('SentinelsPanel container', () => {
  it('renders the launch wizard when there is no active sentinel', () => {
    setup(null)
    const w = mount(SentinelsPanel)
    expect(w.find('[data-test="launch"]').exists()).toBe(true)
    expect(w.find('[data-test="active"]').exists()).toBe(false)
  })

  it('renders the active view when a sentinel is owned', () => {
    setup({ owner: 'z1own', active: true, isRevocable: true, revokeCooldown: 0 })
    const w = mount(SentinelsPanel)
    expect(w.find('[data-test="active"]').exists()).toBe(true)
    expect(w.find('[data-test="launch"]').exists()).toBe(false)
  })

  it('stops polling on unmount', () => {
    const s = setup(null)
    const stop = vi.spyOn(s, 'stopPolling')
    mount(SentinelsPanel).unmount()
    expect(stop).toHaveBeenCalled()
  })
})
