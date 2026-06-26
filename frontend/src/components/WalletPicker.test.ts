import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Mock the wallet store binding so rename() resolves without a real backend.
const RenameWallet = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([]),
  RenameWallet,
  Unlock: vi.fn(),
  Lock: vi.fn(),
}))

import WalletPicker from './WalletPicker.vue'
import { useWalletStore } from '../stores/wallet'

const wallets = [
  { id: 'pillar.dat', name: 'Pillar wallet', baseAddress: 'z1qz372aaaaaaaaaaaaaaau3d6g' },
  { id: 'savings.dat', name: 'Savings', baseAddress: 'z1qrr0aaaaaaaaaaaaaaaapcjmg' },
  { id: 'testnet.dat', name: 'Testnet wallet', baseAddress: 'z1qp8kaaaaaaaaaaaaaaaa7m2qd' },
]

beforeEach(() => {
  setActivePinia(createPinia())
  RenameWallet.mockClear()
})

describe('WalletPicker.vue', () => {
  it('collapsed shows only the selected wallet name', () => {
    const w = mount(WalletPicker, { props: { modelValue: 'savings.dat', wallets } })
    const trigger = w.find('button[aria-label="Select wallet"]')
    expect(trigger.text()).toContain('Savings')
    expect(trigger.text()).not.toContain('Pillar wallet')
    // Panel not rendered while collapsed.
    expect(w.find('[role="listbox"]').exists()).toBe(false)
  })

  it('expanding the trigger shows all wallet names', async () => {
    const w = mount(WalletPicker, { props: { modelValue: 'pillar.dat', wallets } })
    await w.find('button[aria-label="Select wallet"]').trigger('click')
    const panel = w.find('[role="listbox"]')
    expect(panel.exists()).toBe(true)
    for (const wm of wallets) expect(panel.text()).toContain(wm.name)
  })

  it('clicking a row emits update:modelValue with that id', async () => {
    const w = mount(WalletPicker, { props: { modelValue: 'pillar.dat', wallets } })
    await w.find('button[aria-label="Select wallet"]').trigger('click')
    const rows = w.findAll('[role="option"]')
    await rows[1].trigger('click') // Savings
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['savings.dat'])
  })

  it('rename: pencil + new value + ✓ calls store.rename(id, value)', async () => {
    const w = mount(WalletPicker, { props: { modelValue: 'pillar.dat', wallets } })
    const store = useWalletStore()
    const renameSpy = vi.spyOn(store, 'rename').mockResolvedValue(undefined)

    await w.find('button[aria-label="Select wallet"]').trigger('click')
    // Enter rename mode on the first row (Pillar wallet).
    await w.find('button[aria-label="Rename Pillar wallet"]').trigger('click')
    const input = w.find('input[aria-label="Rename Pillar wallet"]')
    expect((input.element as HTMLInputElement).value).toBe('Pillar wallet')
    await input.setValue('Renamed')
    await w.find('button[aria-label="Save Pillar wallet"]').trigger('click')

    expect(renameSpy).toHaveBeenCalledWith('pillar.dat', 'Renamed')
  })
})
