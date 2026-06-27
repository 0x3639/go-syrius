import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  // Render all tab content so we can assert routing without driving tab state.
  Tabs: { template: '<div><slot /></div>' },
  TabsList: { template: '<div><slot /></div>' },
  TabsTrigger: { template: '<button><slot /></button>' },
  TabsContent: { template: '<div><slot /></div>' },
}))
vi.mock('./PillarDelegate.vue', () => ({ default: { name: 'PillarDelegate', template: '<div data-test="delegate" />' } }))
vi.mock('./PillarLaunch.vue', () => ({ default: { name: 'PillarLaunch', template: '<div data-test="launch" />' } }))
vi.mock('./PillarActive.vue', () => ({ default: { name: 'PillarActive', template: '<div data-test="active" />' } }))

import PillarPanel from './PillarPanel.vue'
import { usePillarStore } from '../../stores/pillar'

function setup(myPillar: unknown) {
  setActivePinia(createPinia())
  const s = usePillarStore()
  vi.spyOn(s, 'refreshRegistration').mockResolvedValue()
  s.myPillar = myPillar as never
  return s
}

describe('PillarPanel container', () => {
  it('always renders the delegation sub-view', () => {
    setup(null)
    const w = mount(PillarPanel)
    expect(w.find('[data-test="delegate"]').exists()).toBe(true)
  })

  it('renders the launch wizard when no pillar is owned', () => {
    setup(null)
    const w = mount(PillarPanel)
    expect(w.find('[data-test="launch"]').exists()).toBe(true)
    expect(w.find('[data-test="active"]').exists()).toBe(false)
  })

  it('renders the active view when a pillar is owned', () => {
    setup({ name: 'Pillar-A' })
    const w = mount(PillarPanel)
    expect(w.find('[data-test="active"]').exists()).toBe(true)
    expect(w.find('[data-test="launch"]').exists()).toBe(false)
  })

  it('stops polling on unmount', () => {
    const s = setup(null)
    const stop = vi.spyOn(s, 'stopPolling')
    mount(PillarPanel).unmount()
    expect(stop).toHaveBeenCalled()
  })
})
