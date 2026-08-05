// stores/txs.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetTransactions } = vi.hoisted(() => ({
  GetTransactions: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ GetTransactions }))

import { useTxsStore, type TxRecord } from './txs'

const pairRecord = (i: number): TxRecord => ({
  hash: `h${i}`,
  direction: 'pair', // filtered out by the Transfers view
  method: '',
  counterparty: 'z1q...',
  token: 'ZNN',
  amount: '0',
  decimals: 8,
  momentumHeight: i,
  confirmed: true,
  timestamp: 0,
})

beforeEach(() => {
  setActivePinia(createPinia())
  GetTransactions.mockReset()
})

describe('txs store fetch caps', () => {
  // A malicious node can claim hasMore forever while returning only records
  // the Transfers filter hides — the page never fills, so without a hard cap
  // load()/ensure() loop and buffer unbounded (CWE-835).
  it('load stops at the chunk cap against an endless filtered-out history', async () => {
    let calls = 0
    GetTransactions.mockImplementation(async (idx: number) => {
      calls++
      return { records: [pairRecord(idx)], hasMore: true }
    })
    const s = useTxsStore()
    await s.load()
    expect(calls).toBe(50) // MAX_CHUNKS
    expect(s.hasMoreBlocks).toBe(false) // capped: no further fetching
    expect(s.buffer.length).toBe(50)
  })

  it('ensure stops at the chunk cap too', async () => {
    let calls = 0
    GetTransactions.mockImplementation(async (idx: number) => {
      calls++
      return { records: [pairRecord(idx)], hasMore: true }
    })
    const s = useTxsStore()
    await s.ensure()
    expect(calls).toBe(50)
    expect(s.hasMoreBlocks).toBe(false)
  })

  it('load fills a page normally below the cap', async () => {
    GetTransactions.mockImplementation(async (idx: number) => ({
      records: Array.from({ length: 20 }, (_, k) => ({ ...pairRecord(idx * 20 + k), direction: 'in', amount: '1' })),
      hasMore: true,
    }))
    const s = useTxsStore()
    await s.load()
    // One 20-row chunk fills page 0 (10 rows) plus the next-page probe.
    expect(GetTransactions).toHaveBeenCalledTimes(1)
    expect(s.hasMoreBlocks).toBe(true) // genuinely more, not capped
    expect(s.pageItems.length).toBe(10)
  })
})
