import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareCreateProject: vi.fn(() => Promise.resolve({ kind: 'create' })),
  PrepareAddPhase: vi.fn(() => Promise.resolve({ kind: 'addPhase' })),
  PrepareUpdatePhase: vi.fn(() => Promise.resolve({ kind: 'updatePhase' })),
  // The phase-payout picker loads the active address's approved projects.
  GetMyProjects: vi.fn(() =>
    Promise.resolve([
      { id: '0xabc', name: 'AddProj' },
      { id: '0xPID', name: 'UpdProj' },
    ]),
  ),
}))

import AcceleratorCreate from './AcceleratorCreate.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useTxStore } from '../../stores/tx'

// Mount + let onMounted's loadMyProjects() resolve so the dropdown has options.
async function mountReady() {
  setActivePinia(createPinia())
  const tx = useTxStore()
  vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  const w = mount(AcceleratorCreate)
  await new Promise((r) => setTimeout(r))
  await w.vm.$nextTick()
  return w
}

describe('AcceleratorCreate', () => {
  it('lists the active address approved projects in the phase-payout picker', async () => {
    const w = await mountReady()
    const labels = w.find('select[aria-label="project id"]').findAll('option').map((o) => o.text())
    expect(labels).toContain('AddProj')
    expect(labels).toContain('UpdProj')
  })

  it('forwards create + request-payout (add phase) with the form fields', async () => {
    const w = await mountReady()
    await w.find('input[aria-label="create name"]').setValue('Proj')
    await w.find('input[aria-label="create url"]').setValue('https://x.io')
    await w.find('input[aria-label="create znn"]').setValue('100')
    await w.find('input[aria-label="create qsr"]').setValue('200')
    await w.find('input[aria-label="create description"]').setValue('desc')
    await w.find('button[aria-label="create project"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareCreateProject).toHaveBeenCalledWith('Proj', 'desc', 'https://x.io', '100', '200')

    await w.find('select[aria-label="project id"]').setValue('0xabc')
    await w.find('input[aria-label="phase name"]').setValue('Ph1')
    await w.find('input[aria-label="phase url"]').setValue('https://y.io')
    await w.find('input[aria-label="phase znn"]').setValue('10')
    await w.find('input[aria-label="phase qsr"]').setValue('20')
    await w.find('input[aria-label="phase description"]').setValue('pdesc')
    await w.find('button[aria-label="add phase"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareAddPhase).toHaveBeenCalledWith('0xabc', 'Ph1', 'pdesc', 'https://y.io', '10', '20')
  })

  it('forwards update-phase with (projectId, name, description, url, znn, qsr) in order', async () => {
    const w = await mountReady()
    // distinct values so an arg-order swap fails the assertion
    await w.find('select[aria-label="project id"]').setValue('0xPID')
    await w.find('input[aria-label="phase name"]').setValue('UpName')
    await w.find('input[aria-label="phase url"]').setValue('https://upd.io')
    await w.find('input[aria-label="phase znn"]').setValue('11')
    await w.find('input[aria-label="phase qsr"]').setValue('22')
    await w.find('input[aria-label="phase description"]').setValue('UpDesc')
    await w.find('button[aria-label="update phase"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareUpdatePhase).toHaveBeenCalledWith('0xPID', 'UpName', 'UpDesc', 'https://upd.io', '11', '22')
  })
})
