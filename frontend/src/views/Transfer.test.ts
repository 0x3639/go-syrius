import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Transfer from './Transfer.vue'

describe('Transfer page', () => {
  beforeEach(() => setActivePinia(createPinia()))
  it('renders the send form while idle', () => {
    const w = mount(Transfer, { global: { stubs: { SendForm: true, TxModal: true, TxResult: true } } })
    expect(w.findComponent({ name: 'SendForm' }).exists() || w.find('send-form-stub').exists()).toBe(true)
  })
})
