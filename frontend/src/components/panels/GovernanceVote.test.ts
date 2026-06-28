import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], emits: ['click'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
const { PrepareGovernanceVote } = vi.hoisted(() => ({ PrepareGovernanceVote: vi.fn(() => Promise.resolve({ summary: 'v' })) }))
vi.mock('../../../wailsjs/go/app/NomService', () => ({ PrepareGovernanceVote }))

import GovernanceVote from './GovernanceVote.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'

function setup(opts: { pillars?: string[] } = {}) {
  setActivePinia(createPinia())
  const gov = useGovernanceStore()
  gov.numActivePillars = 100
  gov.votablePillars = opts.pillars ?? ['P1']
  gov.actions = [
    { id: '0xopen', name: 'OpenAct', type: 2, round: 0, status: 0, executed: false, expired: false,
      activePillarThreshold: 50, directionalThreshold: 50, votes: { yes: 1, no: 0, total: 1 } },
    { id: '0xclosed', name: 'ClosedAct', type: 2, round: 0, status: 1, executed: true, expired: false,
      activePillarThreshold: 50, directionalThreshold: 50, votes: { yes: 0, no: 0, total: 0 } },
    { id: '0xexpired', name: 'ExpiredAct', type: 2, round: 0, status: 0, executed: false, expired: true,
      activePillarThreshold: 50, directionalThreshold: 50, votes: { yes: 0, no: 0, total: 0 } },
  ] as never
  return { gov }
}

describe('GovernanceVote', () => {
  it('shows a pillar-operator note when no pillar is owned', () => {
    setup({ pillars: [] })
    const w = mount(GovernanceVote)
    expect(w.text().toLowerCase()).toContain('pillar operators')
  })

  it('lists only open actions (Voting && !expired)', () => {
    setup()
    const w = mount(GovernanceVote)
    expect(w.text()).toContain('OpenAct')
    expect(w.text()).not.toContain('ClosedAct')
    expect(w.text()).not.toContain('ExpiredAct')
  })

  it('shows the pillar as static text (no dropdown) when the account owns exactly one pillar', () => {
    setup({ pillars: ['Solo-Pillar'] })
    const w = mount(GovernanceVote)
    expect(w.find('select[aria-label="vote pillar"]').exists()).toBe(false)
    expect(w.text()).toContain('Solo-Pillar')
  })

  it('shows a dropdown only when the account owns multiple pillars', () => {
    setup({ pillars: ['P1', 'P2'] })
    const w = mount(GovernanceVote)
    expect(w.find('select[aria-label="vote pillar"]').exists()).toBe(true)
  })

  it('forwards a Yes vote with (id, selectedPillar, 0)', async () => {
    setup({ pillars: ['P1', 'P2'] })
    // Spy AFTER setup() installs the fresh pinia, so the spy targets the same
    // tx store instance the component will use (mirrors AcceleratorVote.test).
    const awaitConfirm = vi.spyOn(useTxStore(), 'awaitConfirm').mockImplementation(() => {})
    const w = mount(GovernanceVote)
    await w.find('select[aria-label="vote pillar"]').setValue('P2')
    await w.find('button[aria-label="vote yes 0xopen"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareGovernanceVote).toHaveBeenCalledWith('0xopen', 'P2', 0)
    expect(awaitConfirm).toHaveBeenCalled()
  })
})
