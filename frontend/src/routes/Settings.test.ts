import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ChangePassword: vi.fn().mockResolvedValue(undefined),
  RevealMnemonic: vi.fn().mockResolvedValue('alpha bravo charlie'),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))
import Settings from './Settings.svelte'
import { wallet } from '../lib/stores/wallet'

describe('Settings', () => {
  it('reveals the mnemonic after entering a password', async () => {
    wallet.set({ locked: false, walletName: 'w.dat', accounts: [], active: 0 } as any)
    render(Settings)
    await fireEvent.input(screen.getByLabelText(/reveal password/i), { target: { value: 'pw' } })
    await fireEvent.click(screen.getByRole('button', { name: /reveal/i }))
    expect(await screen.findByText(/alpha bravo charlie/)).toBeTruthy()
  })
})
