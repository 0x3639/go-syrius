import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import AmountInput from './AmountInput.vue'

describe('AmountInput', () => {
  it('shows Max and emits the max value when clicked', async () => {
    const w = mount(AmountInput, { props: { modelValue: '', max: '42.5' } })
    const maxBtn = w.find('button[aria-label="max amount"]')
    expect(maxBtn.exists()).toBe(true)
    await maxBtn.trigger('click')
    expect(w.emitted('update:modelValue')![0]).toEqual(['42.5'])
  })

  it('hides Max when there is no positive balance', () => {
    const w = mount(AmountInput, { props: { modelValue: '', max: '0' } })
    expect(w.find('button[aria-label="max amount"]').exists()).toBe(false)
  })

  it.each(['1a.2b3', '-50', '-1.5'])('preserves %s for strict validation', async (value) => {
    const w = mount(AmountInput, { props: { modelValue: '' } })
    await w.find('input').setValue(value)
    expect(w.emitted('update:modelValue')![0]).toEqual([value])
  })
})
