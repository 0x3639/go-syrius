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
  GetEmbeddedInfo: vi.fn().mockResolvedValue({ running: false, dataDir: '/d/embedded', sizeBytes: 0 }),
  DeleteEmbeddedData: vi.fn().mockResolvedValue(undefined),
}))
import Settings from './Settings.svelte'
import { wallet } from '../lib/stores/wallet'
import { node, sync } from '../lib/stores/node'
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

describe('Settings embedded', () => {
  it('does not start embedded until the warning is confirmed', async () => {
    render(Settings)
    const emb = await screen.findByLabelText(/embedded/i)
    await fireEvent.click(emb)
    await fireEvent.click(screen.getByRole('button', { name: /apply node/i }))
    // a confirm dialog appears; SetNodeMode not called yet
    expect(N.SetNodeMode).not.toHaveBeenCalledWith('embedded')
    await fireEvent.click(screen.getByRole('button', { name: /start embedded/i }))
    expect(N.SetNodeMode).toHaveBeenCalledWith('embedded')
  })

  it('surfaces an error and does not report success when embedded start fails', async () => {
    ;(N.SetNodeMode as any).mockRejectedValueOnce(new Error('start failed'))
    render(Settings)
    const emb = await screen.findByLabelText(/embedded/i)
    await fireEvent.click(emb)
    await fireEvent.click(screen.getByRole('button', { name: /apply node/i }))
    await fireEvent.click(screen.getByRole('button', { name: /start embedded/i }))
    expect(await screen.findByText(/start failed/i)).toBeTruthy()
    expect(screen.queryByText(/Node settings applied/i)).toBeNull()
  })

  it('shows connecting-to-peers when target is 0', async () => {
    node.set({ mode: 'embedded', connected: false, syncing: true, height: 0, peers: 0 })
    sync.set({ state: 'starting', currentHeight: 10, targetHeight: 0, percent: 0, etaSeconds: 0, peers: 0 })
    render(Settings)
    expect(await screen.findByText(/connecting to peers/i)).toBeTruthy()
  })
})
