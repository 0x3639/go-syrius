// stores/settings-stores.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const h = vi.hoisted(() => ({
  SetNodeMode: vi.fn().mockResolvedValue(undefined),
  SetNodeURL: vi.fn().mockResolvedValue(undefined),
  GetNodeConfig: vi.fn().mockResolvedValue({ mode: 'remote', remoteUrl: 'wss://node', localUrl: 'ws://127.0.0.1:35998' }),
  GetEmbeddedInfo: vi.fn().mockResolvedValue({ running: true, dataDir: '/data/znn', sizeBytes: 4096 }),
  DeleteEmbeddedData: vi.fn().mockResolvedValue(undefined),
  ChangePassword: vi.fn().mockResolvedValue(undefined),
  RevealMnemonic: vi.fn().mockResolvedValue('alpha bravo charlie delta'),
  Unlock: vi.fn().mockResolvedValue(undefined),
  CurrentAccounts: vi.fn().mockResolvedValue([]),
  Lock: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn(),
  GetBalances: vi.fn(),
  GetNodeConfig: h.GetNodeConfig,
  SetNodeMode: h.SetNodeMode,
  SetNodeURL: h.SetNodeURL,
  GetEmbeddedInfo: h.GetEmbeddedInfo,
  DeleteEmbeddedData: h.DeleteEmbeddedData,
}))

vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ChangePassword: h.ChangePassword,
  RevealMnemonic: h.RevealMnemonic,
  Unlock: h.Unlock,
  CurrentAccounts: h.CurrentAccounts,
  Lock: h.Lock,
  ListWallets: vi.fn(),
}))

import { useNodeStore } from './node'
import { useWalletStore } from './wallet'

beforeEach(() => setActivePinia(createPinia()))

describe('node store settings extensions', () => {
  it('setMode calls SetNodeMode', async () => {
    const s = useNodeStore()
    await s.setMode('local')
    expect(h.SetNodeMode).toHaveBeenCalledWith('local')
  })
  it('setUrl calls SetNodeURL', async () => {
    const s = useNodeStore()
    await s.setUrl('remote', 'wss://example')
    expect(h.SetNodeURL).toHaveBeenCalledWith('remote', 'wss://example')
  })
  it('getConfig returns mocked config', async () => {
    const s = useNodeStore()
    const cfg = await s.getConfig()
    expect(cfg).toEqual({ mode: 'remote', remoteUrl: 'wss://node', localUrl: 'ws://127.0.0.1:35998' })
  })
  it('getEmbeddedInfo returns mocked info', async () => {
    const s = useNodeStore()
    const info = await s.getEmbeddedInfo()
    expect(info).toEqual({ running: true, dataDir: '/data/znn', sizeBytes: 4096 })
  })
  it('deleteEmbeddedData calls DeleteEmbeddedData', async () => {
    const s = useNodeStore()
    await s.deleteEmbeddedData()
    expect(h.DeleteEmbeddedData).toHaveBeenCalled()
  })
})

describe('wallet store settings extensions', () => {
  it('revealMnemonic returns the mocked mnemonic', async () => {
    const s = useWalletStore()
    const m = await s.revealMnemonic('pw')
    expect(h.RevealMnemonic).toHaveBeenCalledWith('pw')
    expect(m).toBe('alpha bravo charlie delta')
  })
  it('changePassword calls ChangePassword with the active wallet', async () => {
    const s = useWalletStore()
    await s.unlock('mywallet', 'pw')
    await s.changePassword('a', 'b')
    expect(h.ChangePassword).toHaveBeenCalledWith('mywallet', 'a', 'b')
  })
})
