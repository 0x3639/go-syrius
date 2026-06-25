// stores/balances.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetBalances } = vi.hoisted(() => ({
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ GetBalances }))

import { useBalancesStore } from './balances'

beforeEach(() => {
  setActivePinia(createPinia())
  GetBalances.mockClear()
})

describe('balances store', () => {
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
