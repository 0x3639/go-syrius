import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { vi } from 'vitest'
import SendForm from './SendForm.vue'
import { useBalancesStore, type TokenBalance } from '../stores/balances'

// Stub nom-ui Input/Button + AmountInput to trivial controlled elements so the
// test exercises SendForm's validation + emit, not the component internals.
// Each forwards fallthrough attrs (aria-label, placeholder, disabled, @click).
vi.mock('nom-ui', () => ({
  Input: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  Button: {
    emits: ['click'],
    template: '<button @click="$emit(\'click\')"><slot /></button>',
  },
}))

vi.mock('./AmountInput.vue', () => ({
  default: {
    props: ['modelValue', 'label'],
    emits: ['update:modelValue'],
    template:
      '<input :aria-label="label" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

const znn: TokenBalance = { zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('SendForm', () => {
  it('enables Send for a valid address + amount and emits the intent on click', async () => {
    useBalancesStore().items = [znn]
    const w = mount(SendForm)
    await w.vm.$nextTick()

    const addr = 'z1' + 'q'.repeat(38)
    await w.find('input[aria-label="recipient"]').setValue(addr)
    await w.find('input[aria-label="Amount"]').setValue('1.5')

    const send = w.find('button[aria-label="Send"]')
    expect((send.element as HTMLButtonElement).disabled).toBe(false)

    await send.trigger('click')

    expect(w.emitted('send')).toBeTruthy()
    expect(w.emitted('send')![0][0]).toMatchObject({
      recipient: addr,
      zts: 'zts1znn',
      amountDecimal: '1.5',
    })
  })

  it('shows the selected token balance', async () => {
    useBalancesStore().items = [znn]
    const w = mount(SendForm)
    await w.vm.$nextTick()
    expect(w.text()).toContain('Balance:')
    expect(w.text()).toContain('1.5 ZNN')
  })

  it('shows the invalid-address hint and keeps Send disabled for a bad address', async () => {
    useBalancesStore().items = [znn]
    const w = mount(SendForm)
    await w.vm.$nextTick()

    await w.find('input[aria-label="recipient"]').setValue('not-an-address')
    await w.find('input[aria-label="Amount"]').setValue('1.5')

    expect(w.text()).toContain('Invalid z1 address')
    expect((w.find('button[aria-label="Send"]').element as HTMLButtonElement).disabled).toBe(true)
    expect(w.emitted('send')).toBeFalsy()
  })
})
