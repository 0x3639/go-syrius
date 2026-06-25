import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import AcceleratorPanel from './AcceleratorPanel.vue'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useTxStore } from '../../stores/tx'

// Stub nom-ui Button/Input to plain elements mirroring click/v-model, so we
// exercise the panel's bindings, not nom-ui internals.
vi.mock('nom-ui', () => ({
  Button: {
    props: ['variant', 'disabled'],
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
const donatePreview = { kind: 'donate' }
const votePreview = { kind: 'vote' }
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDonate: vi.fn(() => Promise.resolve(donatePreview)),
  PrepareVote: vi.fn(() => Promise.resolve(votePreview)),
  PrepareCreateProject: vi.fn(() => Promise.resolve({ kind: 'create' })),
  PrepareAddPhase: vi.fn(() => Promise.resolve({ kind: 'addPhase' })),
  PrepareUpdatePhase: vi.fn(() => Promise.resolve({ kind: 'updatePhase' })),
}))

import * as Nom from '../../../wailsjs/go/app/NomService'

const PROJECT = {
  id: '0xproject',
  name: 'Project-One',
  status: 0,
  znnFundsNeeded: '100000000',
  qsrFundsNeeded: '200000000',
  votes: { yes: 1, no: 0, total: 1 },
  phases: [],
}

function setup() {
  setActivePinia(createPinia())
  const acc = useAcceleratorStore()
  const tx = useTxStore()
  // Don't hit the (mocked-absent) backend on mount.
  vi.spyOn(acc, 'loadProjects').mockResolvedValue()
  vi.spyOn(acc, 'loadVotablePillars').mockResolvedValue()
  vi.spyOn(acc, 'openProject').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  // Seed a project + votable pillars so both the Donate and Vote sections render.
  acc.projects = [{ ...PROJECT }] as never
  acc.votablePillars = ['Pillar-One', 'Pillar-Two']
  return { acc, tx, awaitConfirm }
}

let wrapper: VueWrapper | null = null
function render() {
  wrapper = mount(AcceleratorPanel)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('AcceleratorPanel', () => {
  it('clicking Donate calls PrepareDonate(amount, token) then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    // Type a base-unit amount; the token select defaults to QSR.
    await w.get('[aria-label="donate amount"]').setValue('500000000')
    await w.get('button:not([aria-label])').trigger('click') // the Donate button
    await w.vm.$nextTick()

    expect(Nom.PrepareDonate).toHaveBeenCalledWith('500000000', 'QSR')
    expect(awaitConfirm).toHaveBeenCalledWith(donatePreview)
  })

  it('clicking Vote calls PrepareVote(id, pillar, vote) then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    // votePillar defaults to the first votable pillar; voteChoice defaults to 0.
    await w.get('[aria-label="vote target id"]').setValue('0xphase')
    await w.get('[aria-label="vote pillar"]').get('option[value="Pillar-Two"]')
    const buttons = w.findAll('button:not([aria-label])')
    // buttons[0] = Donate, buttons[1] = Vote (both have no aria-label).
    await buttons[1].trigger('click')
    await w.vm.$nextTick()

    expect(Nom.PrepareVote).toHaveBeenCalledWith('0xphase', 'Pillar-One', 0)
    expect(awaitConfirm).toHaveBeenCalledWith(votePreview)
  })
})
