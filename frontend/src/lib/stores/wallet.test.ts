import { describe, it, expect, vi, beforeEach } from 'vitest'
import { get } from 'svelte/store'

vi.mock('../../../wailsjs/go/app/WalletService', () => ({
  Unlock: vi.fn().mockResolvedValue(undefined),
  Lock: vi.fn().mockResolvedValue(undefined),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qrr0...' }]),
  SelectAccount: vi.fn().mockResolvedValue(undefined),
}))

import { wallet, unlock, lock } from './wallet'

describe('wallet store', () => {
  beforeEach(() => { lock() })
  it('unlock populates accounts and clears locked', async () => {
    await unlock('pillar.json', 'pw')
    const s = get(wallet)
    expect(s.locked).toBe(false)
    expect(s.accounts.length).toBe(1)
    expect(s.walletName).toBe('pillar.json')
  })
})
