import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/NodeService', () => ({
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn...', symbol: 'ZNN', decimals: 8, amount: '5000000000000' }]),
  GetTransactions: vi.fn().mockResolvedValue([]),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))

import Dashboard from './Dashboard.svelte'
import { wallet } from '../lib/stores/wallet'

describe('Dashboard', () => {
  it('renders the active address and balances', async () => {
    wallet.set({ locked: false, walletName: 'pillar.json', active: 0,
      accounts: [{ index: 0, address: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }] })
    render(Dashboard)
    expect(await screen.findByText(/ZNN/)).toBeTruthy()
    expect(await screen.findByText(/50000/)).toBeTruthy()
  })
})
