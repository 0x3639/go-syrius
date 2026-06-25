// stores/wallet.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
// vi.hoisted so Lock exists when the hoisted vi.mock factory runs.
const Lock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const GenerateMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('w1 w2 w3'))
const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'New.dat' }))
const ImportKeystore = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'Old.dat' }))
const PickKeystoreFile = vi.hoisted(() => vi.fn().mockResolvedValue('/tmp/k.dat'))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
  Lock,
  GenerateMnemonic,
  ImportMnemonic,
  ImportKeystore,
  PickKeystoreFile,
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qxxx', label: '' }]),
  SelectAccount: vi.fn().mockResolvedValue(undefined),
  SetAccountLabel: vi.fn().mockResolvedValue(undefined),
}))
import { useWalletStore } from './wallet'
beforeEach(() => setActivePinia(createPinia()))
describe('wallet store', () => {
  it('lists wallets and unlocks', async () => {
    const s = useWalletStore()
    await s.loadWallets(); expect(s.wallets).toEqual(['Main']); expect(s.active).toBe('Main')
    await s.unlock('Main', 'pw'); expect(s.locked).toBe(false)
  })
  it('lock() re-locks the backend keystore, not just the UI', async () => {
    const s = useWalletStore()
    await s.unlock('Main', 'pw')
    s.lock()
    expect(Lock).toHaveBeenCalled()
    expect(s.locked).toBe(true)
    expect(s.active).toBe('')
  })
  it('lifecycle actions call the bindings', async () => {
    const s = useWalletStore()
    expect(await s.generateMnemonic()).toBe('w1 w2 w3')
    await s.importMnemonic('New.dat', 'pw', 'w1 w2 w3')
    expect(ImportMnemonic).toHaveBeenCalledWith('New.dat', 'pw', 'w1 w2 w3')
    await s.importKeystore('/tmp/k.dat')
    expect(ImportKeystore).toHaveBeenCalledWith('/tmp/k.dat')
    expect(await s.pickKeystoreFile()).toBe('/tmp/k.dat')
  })
  it('loads accounts on unlock and selects by index', async () => {
    const s = useWalletStore()
    await s.unlock('Main', 'pw')
    expect(s.accounts).toEqual([{ index: 0, address: 'z1qxxx', label: '' }])
    expect(s.activeAddress()).toBe('z1qxxx')
    await s.select(0)
    expect(s.activeIndex).toBe(0)
  })
})
