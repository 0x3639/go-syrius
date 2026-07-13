import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const GetBalances = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/NodeService', () => ({ GetBalances }))

import { useBalancesStore } from './balances'
import { bumpRequestEpoch } from '../lib/requestEpoch'

describe('request epoch staleness guard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    GetBalances.mockReset()
  })

  it('discards a slow response that resolves after an account switch', async () => {
    const store = useBalancesStore()
    // Account A's request hangs in flight…
    let resolveSlow!: (v: unknown) => void
    GetBalances.mockReturnValueOnce(new Promise((r) => { resolveSlow = r }))
    const slow = store.load()

    // …the user switches to account B, whose request resolves first…
    bumpRequestEpoch()
    GetBalances.mockResolvedValueOnce([{ zts: 'b', symbol: 'B', decimals: 8, amount: '2' }])
    await store.load()

    // …then A's stale response finally lands. It must NOT overwrite B's data.
    resolveSlow([{ zts: 'a', symbol: 'A', decimals: 8, amount: '1' }])
    await slow

    expect(store.items).toEqual([{ zts: 'b', symbol: 'B', decimals: 8, amount: '2' }])
  })

  it('commits normally when no session change happened', async () => {
    const store = useBalancesStore()
    GetBalances.mockResolvedValueOnce([{ zts: 'a', symbol: 'A', decimals: 8, amount: '1' }])
    await store.load()
    expect(store.items).toHaveLength(1)
  })

  it('a stale error must not blank fresher data', async () => {
    const store = useBalancesStore()
    let rejectSlow!: (e: unknown) => void
    GetBalances.mockReturnValueOnce(new Promise((_r, rej) => { rejectSlow = rej }))
    const slow = store.load()

    bumpRequestEpoch()
    GetBalances.mockResolvedValueOnce([{ zts: 'b', symbol: 'B', decimals: 8, amount: '2' }])
    await store.load()

    rejectSlow(new Error('node dropped'))
    await slow

    expect(store.items).toEqual([{ zts: 'b', symbol: 'B', decimals: 8, amount: '2' }])
  })
})
