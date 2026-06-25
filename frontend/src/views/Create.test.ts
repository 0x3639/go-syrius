import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const GenerateMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('alpha bravo charlie'))
const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'New.dat' }))
const Unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([]),
  GenerateMnemonic,
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
import Create from './Create.vue'

beforeEach(() => { setActivePinia(createPinia()); push.mockClear() })

describe('Create.vue', () => {
  it('generates a mnemonic and creates the wallet through the 3 steps', async () => {
    const w = mount(Create)
    await new Promise((r) => setTimeout(r)) // generateMnemonic
    expect(GenerateMnemonic).toHaveBeenCalled()
    expect(w.text()).toContain('alpha')

    // Step 1 -> 2
    await w.findAll('button').find((b) => b.text() === "I've backed it up")!.trigger('click')
    // Step 2: answer each prompted word position correctly
    const words = ['alpha', 'bravo', 'charlie']
    for (const input of w.findAll('input')) {
      const label = input.attributes('aria-label') || ''
      const m = label.match(/^word (\d+)$/)
      if (m) await input.setValue(words[Number(m[1]) - 1])
    }
    await w.findAll('button').find((b) => b.text() === 'Continue')!.trigger('click')
    // Step 3: name + matching passwords
    await w.find('input[aria-label="wallet name"]').setValue('New')
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('input[aria-label="confirm password"]').setValue('pw')
    await w.findAll('button').find((b) => b.text() === 'Create wallet')!.trigger('click')
    await new Promise((r) => setTimeout(r))

    expect(ImportMnemonic).toHaveBeenCalledWith('New.dat', 'pw', 'alpha bravo charlie')
    expect(Unlock).toHaveBeenCalledWith('New.dat', 'pw')
    expect(push).toHaveBeenCalledWith('/home')
  })
})
