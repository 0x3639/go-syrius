import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const GetSettings = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings }))

import { useConsentStore } from './consent'
import { useNodeStore } from './node'

describe('consent store', () => {
  beforeEach(() => { setActivePinia(createPinia()); GetSettings.mockReset() })

  it('is blocked only on mainnet, connected, with the opt-in known to be off', async () => {
    const s = useConsentStore()
    const node = useNodeStore()
    node.connected = true; node.chainId = 1
    expect(s.blocked).toBe(false) // unknown is not "off": never warn on a guess
    GetSettings.mockResolvedValueOnce({ allowMainnetSend: false })
    await s.load()
    expect(s.blocked).toBe(true)
    node.chainId = 73404
    expect(s.blocked).toBe(false)
    node.chainId = 1; node.connected = false
    expect(s.blocked).toBe(false)
    node.connected = true
    s.setAllowed(true)
    expect(s.blocked).toBe(false)
  })

  it('keeps the last known value when the setting cannot be read', async () => {
    const s = useConsentStore()
    s.setAllowed(true)
    GetSettings.mockRejectedValueOnce(new Error('io'))
    await s.load()
    expect(s.mainnetAllowed).toBe(true)
  })

  it('remembers where to return to', () => {
    const s = useConsentStore()
    s.leaveFrom('/transfer?to=z1abc')
    expect(s.returnTo).toBe('/transfer?to=z1abc')
    s.clearReturn()
    expect(s.returnTo).toBe('')
  })
})
