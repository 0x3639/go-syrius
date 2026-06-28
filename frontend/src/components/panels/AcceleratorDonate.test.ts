import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDonate: vi.fn(() => Promise.resolve({ kind: 'donate' })),
}))

import AcceleratorDonate from './AcceleratorDonate.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useTxStore } from '../../stores/tx'

describe('AcceleratorDonate', () => {
  it('forwards the donate call', async () => {
    setActivePinia(createPinia())
    const tx = useTxStore()
    const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
    const w = mount(AcceleratorDonate)
    await w.find('input[aria-label="donate amount"]').setValue('100000000')
    await w.find('button[aria-label="donate"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareDonate).toHaveBeenCalledWith('100000000', 'QSR')
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'donate' })
  })
})
