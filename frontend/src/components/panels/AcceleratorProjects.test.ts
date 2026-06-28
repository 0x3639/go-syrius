import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
}))

import AcceleratorProjects from './AcceleratorProjects.vue'
import { useAcceleratorStore } from '../../stores/accelerator'

function setup() {
  setActivePinia(createPinia())
  const acc = useAcceleratorStore()
  acc.numActivePillars = 10
  acc.projects = [
    { id: '0xv', name: 'VotingAZ', status: 0, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 0, no: 0, total: 0 }, phases: [] },
    { id: '0xa', name: 'ActiveAZ', status: 1, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 0, no: 0, total: 0 },
      phases: [{ id: '0xph', name: 'Ph', status: 0, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 8, no: 0, total: 8 } }] },
    { id: '0xc', name: 'DoneAZ', status: 4, znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 0, no: 0, total: 0 }, phases: [] },
  ] as never
  return { acc }
}

describe('AcceleratorProjects filters', () => {
  it('shows all by default', () => {
    setup()
    const w = mount(AcceleratorProjects)
    expect(w.text()).toContain('VotingAZ')
    expect(w.text()).toContain('ActiveAZ')
    expect(w.text()).toContain('DoneAZ')
  })

  it('"Active AZs" filter shows only Voting projects', async () => {
    setup()
    const w = mount(AcceleratorProjects)
    await w.find('button[aria-label="filter Voting"]').trigger('click')
    expect(w.text()).toContain('VotingAZ')
    expect(w.text()).not.toContain('DoneAZ')
  })

  it('"Awaiting payout" shows the active project whose phase passes the vote', async () => {
    setup()
    const w = mount(AcceleratorProjects)
    await w.find('button[aria-label="filter Awaiting payout"]').trigger('click')
    // ActiveAZ's phase: 8 yes of 8 total, 10 pillars → 800>330 ✓, passing
    expect(w.text()).toContain('ActiveAZ')
    expect(w.text()).not.toContain('VotingAZ')
  })
})
