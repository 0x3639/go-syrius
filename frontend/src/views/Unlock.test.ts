import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]),
  Unlock: unlock,
  Lock: vi.fn(),
  ImportKeystore: vi.fn().mockResolvedValue({ name: 'Main' }),
  PickKeystoreFile: vi.fn().mockResolvedValue(''),
}))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { template: '<button @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import Unlock from './Unlock.vue'

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
})

describe('Unlock.vue', () => {
  it('unlocks the selected wallet and routes home', async () => {
    const w = mount(Unlock)
    await new Promise((r) => setTimeout(r)) // loadWallets
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('button[aria-label="Unlock"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(unlock).toHaveBeenCalledWith('Main', 'pw')
    expect(push).toHaveBeenCalledWith('/home')
  })
})
