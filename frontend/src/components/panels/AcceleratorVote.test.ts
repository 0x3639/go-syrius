import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareVote: vi.fn(() => Promise.resolve({ kind: 'vote' })),
}))

import AcceleratorVote from './AcceleratorVote.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'

function setup(opts: { ownsPillar?: boolean } = {}) {
  setActivePinia(createPinia())
  const acc = useAcceleratorStore()
  const pillar = usePillarStore()
  const tx = useTxStore()
  vi.spyOn(acc, 'refreshVotable').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  acc.votablePillars = ['MyPillar']
  acc.numActivePillars = 10
  acc.votable = [
    { kind: 'project', id: '0xp1', projectId: '0xp1', projectName: 'AZ-One', name: 'AZ-One',
      znnFundsNeeded: '100000000', qsrFundsNeeded: '0',
      votes: { yes: 1, no: 0, total: 1 }, myVotes: [{ pillar: 'MyPillar', vote: -1 }], needsMyVote: true },
    { kind: 'phase', id: '0xph', projectId: '0xp2', projectName: 'AZ-Two', name: 'Phase-1',
      znnFundsNeeded: '0', qsrFundsNeeded: '0',
      votes: { yes: 0, no: 0, total: 0 }, myVotes: [{ pillar: 'MyPillar', vote: 0 }], needsMyVote: false },
  ] as never
  pillar.myPillar = (opts.ownsPillar === false ? null : { name: 'MyPillar' }) as never
  return { acc, tx, awaitConfirm }
}

describe('AcceleratorVote', () => {
  it('shows a pillar-operator note when no pillar is owned', () => {
    setup({ ownsPillar: false })
    const w = mount(AcceleratorVote)
    expect(w.text().toLowerCase()).toContain('pillar operators')
  })

  it('lists only items the selected pillar has not voted on (default)', () => {
    setup()
    const w = mount(AcceleratorVote)
    expect(w.text()).toContain('AZ-One')   // not voted → shown
    expect(w.text()).not.toContain('Phase-1') // already voted → hidden by default
  })

  it('scopes the list to the selected pillar, not any owned pillar', async () => {
    setActivePinia(createPinia())
    const acc = useAcceleratorStore()
    const pillar = usePillarStore()
    vi.spyOn(acc, 'refreshVotable').mockResolvedValue()
    acc.votablePillars = ['PillarA', 'PillarB']
    acc.numActivePillars = 10
    // PillarA already voted; PillarB has not → needsMyVote is globally true.
    acc.votable = [
      { kind: 'project', id: '0xp1', projectId: '0xp1', projectName: 'AZ-One', name: 'AZ-One',
        znnFundsNeeded: '0', qsrFundsNeeded: '0', votes: { yes: 1, no: 0, total: 1 },
        myVotes: [{ pillar: 'PillarA', vote: 0 }, { pillar: 'PillarB', vote: -1 }], needsMyVote: true },
    ] as never
    pillar.myPillar = { name: 'PillarA' } as never
    const w = mount(AcceleratorVote)
    // Default pillar (PillarA) voted → hidden despite needsMyVote: true
    expect(w.text()).not.toContain('AZ-One')
    // Switch to PillarB (unvoted) → now shown
    await w.find('select[aria-label="vote pillar"]').setValue('PillarB')
    expect(w.text()).toContain('AZ-One')
  })

  it('forwards a Yes vote with (id, pillar, 0)', async () => {
    const { awaitConfirm } = setup()
    const w = mount(AcceleratorVote)
    await w.find('button[aria-label="vote yes 0xp1"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareVote).toHaveBeenCalledWith('0xp1', 'MyPillar', 0)
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'vote' })
  })
})
