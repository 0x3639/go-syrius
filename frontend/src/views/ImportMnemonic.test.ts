import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ id: 'abc.dat', name: 'Imp', baseAddress: 'z1' }))
const Unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([]),
  ImportMnemonic,
  Unlock,
  Lock: vi.fn(),
}))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import ImportMnemonic_ from './ImportMnemonic.vue'

beforeEach(() => { setActivePinia(createPinia()); push.mockClear() })

describe('ImportMnemonic.vue', () => {
  it('imports a 12-word mnemonic and routes home', async () => {
    const w = mount(ImportMnemonic_)
    const twelve = 'a b c d e f g h i j k l'
    await w.find('textarea[aria-label="mnemonic"]').setValue(twelve)
    await w.find('input[aria-label="wallet name"]').setValue('Imp')
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('input[aria-label="confirm password"]').setValue('pw')
    await w.find('button[aria-label="Import"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    // Passes the display name without `.dat`; unlocks by the backend-assigned id.
    expect(ImportMnemonic).toHaveBeenCalledWith('Imp', 'pw', twelve)
    expect(Unlock).toHaveBeenCalledWith('abc.dat', 'pw')
    expect(push).toHaveBeenCalledWith('/dashboard')
  })
})
