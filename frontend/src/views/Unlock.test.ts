import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ id: 'Main.dat', name: 'Main', baseAddress: 'z1qmain' }]),
  Unlock: unlock,
  Lock: vi.fn(),
  RenameWallet: vi.fn().mockResolvedValue(undefined),
  ImportKeystore: vi.fn().mockResolvedValue({ id: 'Main.dat', name: 'Main', baseAddress: 'z1qmain' }),
  PickKeystoreFile: vi.fn().mockResolvedValue(''),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qmain', label: '' }]),
  SelectAccount: vi.fn().mockResolvedValue(undefined),
  SetAccountLabel: vi.fn().mockResolvedValue(undefined),
}))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
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
  it('renders the "Welcome back" heading', async () => {
    const w = mount(Unlock, { global: { stubs: { WalletPicker: true } } })
    await new Promise((r) => setTimeout(r)) // loadWallets
    expect(w.text()).toContain('Welcome back')
  })

  it('unlocks the selected wallet and routes to /dashboard', async () => {
    const w = mount(Unlock, { global: { stubs: { WalletPicker: true } } })
    await new Promise((r) => setTimeout(r)) // loadWallets
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('button[aria-label="Unlock"]').trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(unlock).toHaveBeenCalledWith('Main.dat', 'pw')
    expect(push).toHaveBeenCalledWith('/dashboard')
  })

  it('disables Unlock until a wallet is selected', async () => {
    const w = mount(Unlock, {
      global: {
        stubs: {
          WalletPicker: {
            props: ['modelValue'],
            template:
              '<select aria-label="picker" :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option value="">none</option><option value="Main.dat">Main</option></select>',
          },
        },
      },
    })
    await new Promise((r) => setTimeout(r)) // loadWallets -> auto-selects Main.dat
    const btn = () => w.find('button[aria-label="Unlock"]')
    expect(btn().attributes('disabled')).toBeUndefined()
    await w.find('select[aria-label="picker"]').setValue('') // deselect
    expect(btn().attributes('disabled')).toBeDefined()
  })

  it('warns when an imported keystore has an already-present address', async () => {
    const WS: any = await import('../../wailsjs/go/app/WalletService')
    WS.PickKeystoreFile.mockResolvedValueOnce('/k.dat')
    // Same baseAddress as the existing "Main" wallet, different id.
    WS.ImportKeystore.mockResolvedValueOnce({ id: 'dup.dat', name: 'Copy', baseAddress: 'z1qmain' })
    const w = mount(Unlock, { global: { stubs: { WalletPicker: true } } })
    await new Promise((r) => setTimeout(r)) // loadWallets -> [Main z1qmain]
    const importBtn = w.findAll('button').find((b) => b.text().includes('Import keystore'))!
    await importBtn.trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(w.text()).toContain('same address')
  })
})
