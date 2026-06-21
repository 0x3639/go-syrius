import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/WalletService', () => ({
  SelectAccount: vi.fn().mockResolvedValue(undefined),
  SetAccountLabel: vi.fn().mockResolvedValue(undefined),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1q', label: 'Savings' }]),
}))
import AccountSwitcher from './AccountSwitcher.svelte'
import { wallet } from '../stores/wallet'

describe('AccountSwitcher', () => {
  it('shows the account label', async () => {
    wallet.set({ locked: false, walletName: 'w.dat', active: 0,
      accounts: [{ index: 0, address: 'z1q', label: 'Savings' }] } as any)
    render(AccountSwitcher)
    expect(await screen.findByText(/Savings/)).toBeTruthy()
  })
})
