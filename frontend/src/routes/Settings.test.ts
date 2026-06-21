import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ChangePassword: vi.fn().mockResolvedValue(undefined),
  RevealMnemonic: vi.fn().mockResolvedValue('alpha bravo charlie'),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  GetNodeConfig: vi.fn().mockResolvedValue({ mode: 'remote', remoteUrl: 'wss://r:35998', localUrl: 'ws://127.0.0.1:35998' }),
  SetNodeMode: vi.fn().mockResolvedValue(undefined),
  SetNodeURL: vi.fn().mockResolvedValue(undefined),
}))
import Settings from './Settings.svelte'
import { wallet } from '../lib/stores/wallet'
import * as N from '../../wailsjs/go/app/NodeService'

describe('Settings', () => {
  it('reveals the mnemonic after entering a password', async () => {
    wallet.set({ locked: false, walletName: 'w.dat', accounts: [], active: 0 } as any)
    render(Settings)
    await fireEvent.input(screen.getByLabelText(/reveal password/i), { target: { value: 'pw' } })
    await fireEvent.click(screen.getByRole('button', { name: /reveal/i }))
    expect(await screen.findByText(/alpha bravo charlie/)).toBeTruthy()
  })
})

describe('Settings node section', () => {
  it('switching to Local calls SetNodeMode', async () => {
    render(Settings)
    const localRadio = await screen.findByLabelText(/local/i)
    await fireEvent.click(localRadio)
    await fireEvent.click(screen.getByRole('button', { name: /apply node/i }))
    expect(N.SetNodeMode).toHaveBeenCalledWith('local')
  })
})
