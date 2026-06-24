// stores/wallet.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
// vi.hoisted so Lock exists when the hoisted vi.mock factory runs.
const Lock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
  Lock,
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
})
