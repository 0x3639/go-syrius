import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen, waitFor } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'pillar.json', baseAddress: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }]),
  ImportKeystore: vi.fn().mockResolvedValue(undefined),
  PickKeystoreFile: vi.fn().mockResolvedValue('/abs/path/pillar.json'),
  Lock: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Unlock from './Unlock.svelte'

describe('Unlock', () => {
  it('lists wallets and shows an unlock control', async () => {
    render(Unlock)
    expect(await screen.findByText(/pillar\.json/)).toBeTruthy()
  })
  it('shows an error on wrong password', async () => {
    const W = await import('../../wailsjs/go/app/WalletService')
    ;(W.Unlock as any).mockRejectedValueOnce(new Error('incorrect password'))
    render(Unlock)
    await screen.findByText(/pillar\.json/)
    await fireEvent.input(screen.getByLabelText(/password/i), { target: { value: 'x' } })
    await fireEvent.click(screen.getByRole('button', { name: /unlock/i }))
    expect(await screen.findByText(/incorrect password/i)).toBeTruthy()
  })
  it('imports a keystore via the native file dialog', async () => {
    const W = await import('../../wailsjs/go/app/WalletService')
    ;(W.ListWallets as any).mockClear()
    render(Unlock)
    await screen.findByText(/pillar\.json/)
    await fireEvent.click(screen.getByRole('button', { name: /import keystore/i }))
    await waitFor(() => expect(W.ImportKeystore).toHaveBeenCalledWith('/abs/path/pillar.json'))
    expect(W.PickKeystoreFile).toHaveBeenCalled()
    await waitFor(() => expect(W.ListWallets).toHaveBeenCalledTimes(2)) // initial onMount + after import
  })
})
