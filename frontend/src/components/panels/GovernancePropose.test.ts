import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant', 'disabled'], emits: ['click'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
}))
const { PrepareProposeAction } = vi.hoisted(() => ({ PrepareProposeAction: vi.fn(() => Promise.resolve({ summary: 'p' })) }))
vi.mock('../../../wailsjs/go/app/NomService', () => ({ PrepareProposeAction, GetProposeKinds: vi.fn() }))

import GovernancePropose from './GovernancePropose.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'

function setup() {
  setActivePinia(createPinia())
  const gov = useGovernanceStore()
  gov.proposeKinds = [
    { kind: 'spork.create', label: 'Spork — Create', group: 'Spork', fields: [
      { key: 'name', label: 'Spork name', type: 'text', placeholder: '', required: true, min: 5, max: 40 },
      { key: 'description', label: 'Spork description', type: 'text', placeholder: '', required: true, max: 400 },
    ] },
    { kind: 'custom', label: 'Custom (advanced)', group: 'Custom', fields: [
      { key: 'destination', label: 'Destination', type: 'address', placeholder: '', required: true },
      { key: 'data', label: 'Data', type: 'base64', placeholder: '', required: true },
    ] },
  ] as never
  return { gov }
}

describe('GovernancePropose', () => {
  it('renders the selected kind\'s fields and swaps them when kind changes', async () => {
    setup()
    const w = mount(GovernancePropose)
    // default kind = first (spork.create) → its 2 fields present
    expect(w.find('input[aria-label="field name"]').exists()).toBe(true)
    expect(w.find('input[aria-label="field description"]').exists()).toBe(true)
    expect(w.find('input[aria-label="field destination"]').exists()).toBe(false)
    // switch to custom → its fields appear, spork fields gone
    await w.find('select[aria-label="propose kind"]').setValue('custom')
    expect(w.find('input[aria-label="field destination"]').exists()).toBe(true)
    expect(w.find('input[aria-label="field name"]').exists()).toBe(false)
  })

  it('shows length hints and sets maxlength from the field min/max bounds', () => {
    setup()
    const w = mount(GovernancePropose)
    expect(w.text()).toContain('5–40 characters') // spork name (min 5, max 40)
    expect(w.text()).toContain('up to 400 characters') // spork description (max only)
    expect(w.find('input[aria-label="field name"]').attributes('maxlength')).toBe('40')
  })

  it('submits PrepareProposeAction with (name, description, url, kind, params)', async () => {
    setup()
    const awaitConfirm = vi.spyOn(useTxStore(), 'awaitConfirm').mockImplementation(() => {})
    const w = mount(GovernancePropose)
    await w.find('input[aria-label="action name"]').setValue('Act')
    await w.find('input[aria-label="action description"]').setValue('about')
    await w.find('input[aria-label="action url"]').setValue('https://zenon.org')
    await w.find('input[aria-label="field name"]').setValue('MySpork')
    await w.find('input[aria-label="field description"]').setValue('sdesc')
    await w.find('button[aria-label="submit proposal"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareProposeAction).toHaveBeenCalledWith('Act', 'about', 'https://zenon.org', 'spork.create', { name: 'MySpork', description: 'sdesc' })
    expect(awaitConfirm).toHaveBeenCalled()
  })
})
