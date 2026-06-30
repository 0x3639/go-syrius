import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const ClipboardSetText = vi.hoisted(() => vi.fn().mockResolvedValue(true))
vi.mock('../../wailsjs/runtime/runtime', () => ({ ClipboardSetText }))
import AccountSlotPicker from './AccountSlotPicker.vue'
import { useWalletStore } from '../stores/wallet'

const ADDR0 = 'z1qrxx0000000000000000000000000000000k5qtk6'
const ADDR1 = 'z1qsyy0000000000000000000000000000000pcjmg'

function seed() {
  const wallet = useWalletStore()
  wallet.accounts = [
    { index: 0, address: ADDR0, label: 'Main' },
    { index: 1, address: ADDR1, label: '' },
  ]
  wallet.activeIndex = 0
  return wallet
}

beforeEach(() => {
  setActivePinia(createPinia())
  ClipboardSetText.mockClear()
})

describe('AccountSlotPicker', () => {
  it('shows the active slot label and address', () => {
    seed()
    const w = mount(AccountSlotPicker)
    expect(w.text()).toContain('Main')
    expect(w.text()).toContain('z1q')
  })

  it('expands and selects another slot', async () => {
    const wallet = seed()
    const select = vi.spyOn(wallet, 'select').mockResolvedValue(undefined as never)
    const w = mount(AccountSlotPicker)
    await w.find('button[aria-label="Select account"]').trigger('click')
    expect(w.text()).toContain('Account 1') // default label for the unlabeled slot
    const row = w.findAll('[role="option"]').find((r) => r.text().includes('Account 1'))!
    await row.trigger('click')
    expect(select).toHaveBeenCalledWith(1)
  })

  it('copies the active slot address (full, not shortened)', async () => {
    seed()
    const w = mount(AccountSlotPicker)
    await w.find('button[aria-label="copy address"]').trigger('click')
    expect(ClipboardSetText).toHaveBeenCalledWith(ADDR0)
  })

  it('reveals another account via the Add account footer', async () => {
    const wallet = seed()
    const addAccount = vi.spyOn(wallet, 'addAccount').mockResolvedValue(undefined as never)
    const w = mount(AccountSlotPicker)
    await w.find('button[aria-label="Select account"]').trigger('click')
    await w.find('button[aria-label="add account"]').trigger('click')
    expect(addAccount).toHaveBeenCalled()
  })

  it('renders avatars without a gradient and without arbitrary radii or off-scale text', async () => {
    seed()
    const w = mount(AccountSlotPicker)
    await w.find('button[aria-label="Select account"]').trigger('click')
    const html = w.html()
    expect(html).not.toContain('from-primary')
    expect(html).not.toContain('rounded-[7px]')
    expect(html).not.toContain('rounded-[10px]')
    expect(html).not.toContain('text-[15px]')
  })

  it('renames a slot via setLabel', async () => {
    const wallet = seed()
    const setLabel = vi.spyOn(wallet, 'setLabel').mockResolvedValue(undefined as never)
    const w = mount(AccountSlotPicker)
    await w.find('button[aria-label="Select account"]').trigger('click')
    await w.find('button[aria-label="Rename account 0"]').trigger('click')
    await w.find('input[aria-label="Rename account 0"]').setValue('Savings')
    await w.find('button[aria-label="Save account 0"]').trigger('click')
    expect(setLabel).toHaveBeenCalledWith(0, 'Savings')
  })
})
