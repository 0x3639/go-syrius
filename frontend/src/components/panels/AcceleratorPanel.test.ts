import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Tabs: { props: ['modelValue'], template: '<div><slot /></div>' },
  TabsList: { template: '<div><slot /></div>' },
  TabsTrigger: { props: ['value'], template: '<button><slot /></button>' },
  TabsContent: { props: ['value'], template: '<div><slot /></div>' },
}))
vi.mock('./AcceleratorVote.vue', () => ({ default: { name: 'AcceleratorVote', template: '<div data-test="vote" />' } }))
vi.mock('./AcceleratorProjects.vue', () => ({ default: { name: 'AcceleratorProjects', template: '<div data-test="projects" />' } }))
vi.mock('./AcceleratorCreate.vue', () => ({ default: { name: 'AcceleratorCreate', template: '<div data-test="create" />' } }))
vi.mock('./AcceleratorDonate.vue', () => ({ default: { name: 'AcceleratorDonate', template: '<div data-test="donate" />' } }))

import AcceleratorPanel from './AcceleratorPanel.vue'
import { useAcceleratorStore } from '../../stores/accelerator'

function setup() {
  setActivePinia(createPinia())
  const acc = useAcceleratorStore()
  vi.spyOn(acc, 'refreshVotable').mockResolvedValue()
  vi.spyOn(acc, 'loadProjects').mockResolvedValue()
  vi.spyOn(acc, 'loadVotablePillars').mockResolvedValue()
  return { acc }
}

describe('AcceleratorPanel container', () => {
  it('renders all four sub-views (Tabs stub shows all content)', () => {
    setup()
    const w = mount(AcceleratorPanel)
    expect(w.find('[data-test="vote"]').exists()).toBe(true)
    expect(w.find('[data-test="projects"]').exists()).toBe(true)
    expect(w.find('[data-test="create"]').exists()).toBe(true)
    expect(w.find('[data-test="donate"]').exists()).toBe(true)
  })

  it('refreshes votable + projects on mount', () => {
    const { acc } = setup()
    mount(AcceleratorPanel)
    expect(acc.refreshVotable).toHaveBeenCalled()
    expect(acc.loadProjects).toHaveBeenCalled()
  })
})
