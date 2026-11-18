// stores/balances.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetBalances } = vi.hoisted(() => ({
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ GetBalances }))

import { useBalancesStore } from './balances'
import { useNodeStore } from './node'

beforeEach(() => {
  setActivePinia(createPinia())
  GetBalances.mockClear()
})

describe('balances store', () => {
  it('distinguishes a successful empty wallet from an unavailable balance', async () => {
    const s = useBalancesStore()
    useNodeStore().connected = true
    expect(s.available).toBe(false)
    GetBalances.mockResolvedValueOnce([])
    await s.load()
    expect(s.available).toBe(true)
    GetBalances.mockRejectedValueOnce(new Error('offline'))
    await s.load()
    expect(s.status).toBe('error')
    expect(s.available).toBe(false)
  })
  it('invalidates a request still in flight when the connection is lost', async () => {
    let resolve!: (items: unknown[]) => void
    GetBalances.mockReturnValueOnce(new Promise(r => { resolve = r }))
    const s = useBalancesStore()
    const pending = s.load()
    s.invalidate()
    resolve([{ symbol: 'ZNN', amount: '100' }])
    await pending
    expect(s.items).toEqual([])
    expect(s.status).toBe('idle')
  })
  it('load sets items from GetBalances', async () => {
    const s = useBalancesStore()
    await s.load()
    expect(GetBalances).toHaveBeenCalled()
    expect(s.items[0].symbol).toBe('ZNN')
  })

  it('load falls back to [] on error', async () => {
    GetBalances.mockRejectedValueOnce(new Error('boom'))
    const s = useBalancesStore()
    await s.load()
    expect(s.items).toEqual([])
  })
})
