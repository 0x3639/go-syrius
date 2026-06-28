import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Tabs: { props: ['modelValue'], template: '<div><slot /></div>' },
  TabsList: { template: '<div><slot /></div>' },
  TabsTrigger: { props: ['value'], template: '<button :aria-label="`sub ${value}`"><slot /></button>' },
  TabsContent: { props: ['value'], template: '<div><slot /></div>' },
  Button: { props: ['variant', 'disabled'], emits: ['click'], template: '<button><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  GetActions: vi.fn(() => Promise.resolve({ count: 0, list: [] })),
  GetVotablePillars: vi.fn(() => Promise.resolve([])),
  GetActivePillarCount: vi.fn(() => Promise.resolve(0)),
  GetProposeKinds: vi.fn(() => Promise.resolve([])),
  PrepareGovernanceVote: vi.fn(),
  PrepareExecuteAction: vi.fn(),
}))

import GovernancePanel from './GovernancePanel.vue'
import { useGovernanceStore } from '../../stores/governance'

describe('GovernancePanel', () => {
  it('loads governance data on mount and renders both sub-tabs', async () => {
    setActivePinia(createPinia())
    const gov = useGovernanceStore()
    const loadActions = vi.spyOn(gov, 'loadActions')
    const w = mount(GovernancePanel)
    await new Promise((r) => setTimeout(r))
    expect(loadActions).toHaveBeenCalled()
    expect(w.find('button[aria-label="sub Vote"]').exists()).toBe(true)
    expect(w.find('button[aria-label="sub Actions"]').exists()).toBe(true)
  })

  it('renders the Propose sub-tab and loads kinds on mount', async () => {
    setActivePinia(createPinia())
    const gov = useGovernanceStore()
    const loadProposeKinds = vi.spyOn(gov, 'loadProposeKinds')
    const w = mount(GovernancePanel)
    await new Promise((r) => setTimeout(r))
    expect(loadProposeKinds).toHaveBeenCalled()
    expect(w.find('button[aria-label="sub Propose"]').exists()).toBe(true)
  })
})
