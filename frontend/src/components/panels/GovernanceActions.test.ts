import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], emits: ['click'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
const { PrepareExecuteAction } = vi.hoisted(() => ({ PrepareExecuteAction: vi.fn(() => Promise.resolve({ summary: 'x' })) }))
vi.mock('../../../wailsjs/go/app/NomService', () => ({ PrepareExecuteAction, GetActions: vi.fn() }))

import GovernanceActions from './GovernanceActions.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'

function setup() {
  setActivePinia(createPinia())
  const gov = useGovernanceStore()
  gov.numActivePillars = 100
  gov.actions = [
    { id: '0xv', name: 'OpenAct', destination: 'z1dest', data: 'AAEC', type: 2, round: 0, status: 0,
      executed: false, expired: false, activePillarThreshold: 50, directionalThreshold: 50,
      votes: { yes: 60, no: 10, total: 70 } },
    { id: '0xa', name: 'ApprovedAct', destination: 'z1dest', data: '', type: 2, round: 0, status: 1,
      executed: false, expired: false, activePillarThreshold: 50, directionalThreshold: 50,
      votes: { yes: 0, no: 0, total: 0 } },
    { id: '0xe', name: 'ExecutedAct', destination: 'z1dest', data: '', type: 2, round: 0, status: 1,
      executed: true, expired: false, activePillarThreshold: 50, directionalThreshold: 50,
      votes: { yes: 0, no: 0, total: 0 } },
  ] as never
  return { gov }
}

describe('GovernanceActions', () => {
  it('shows all actions by default', () => {
    setup()
    const w = mount(GovernanceActions)
    expect(w.text()).toContain('OpenAct')
    expect(w.text()).toContain('ApprovedAct')
  })

  it('Approved filter shows only status=1 actions', async () => {
    setup()
    const w = mount(GovernanceActions)
    await w.find('button[aria-label="filter Approved"]').trigger('click')
    expect(w.text()).toContain('ApprovedAct')
    expect(w.text()).not.toContain('OpenAct')
  })

  it('Execute button shows only for Approved && !executed and dispatches', async () => {
    const { gov } = setup()
    const awaitConfirm = vi.spyOn(useTxStore(), 'awaitConfirm').mockImplementation(() => {})
    const w = mount(GovernanceActions)
    // expand ApprovedAct
    await w.find('button[aria-label="details 0xa"]').trigger('click')
    const execBtn = w.find('button[aria-label="execute 0xa"]')
    expect(execBtn.exists()).toBe(true)
    await execBtn.trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareExecuteAction).toHaveBeenCalledWith('0xa')
    expect(awaitConfirm).toHaveBeenCalled()
    // ExecutedAct must NOT offer execute
    await w.find('button[aria-label="details 0xe"]').trigger('click')
    expect(w.find('button[aria-label="execute 0xe"]').exists()).toBe(false)
  })
})
