import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePriceStore } from './price'

const OK = {
  data: {
    znn: { usd: 0.118422, timestamp: '2026-06-29T23:46:19Z' },
    qsr: { usd: 0.02343554, timestamp: '2026-06-29T23:46:19Z' },
    btc: { usd: 60172.0 }, eth: { usd: 1609.2 },
  },
}

describe('price store', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => vi.restoreAllMocks())

  it('parses a successful response and becomes available', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => OK }))
    const price = usePriceStore()
    await price.refresh()
    expect(price.available).toBe(true)
    expect(price.znnUsd).toBeCloseTo(0.118422)
    expect(price.qsrUsd).toBeCloseTo(0.02343554)
  })

  it('stays unavailable on HTTP 429', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 429, json: async () => ({}) }))
    const price = usePriceStore()
    await price.refresh()
    expect(price.available).toBe(false)
    expect(price.znnUsd).toBeNull()
  })

  it('stays unavailable when fetch throws', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network')))
    const price = usePriceStore()
    await price.refresh()
    expect(price.available).toBe(false)
  })

  it('computes the portfolio total from BigInt balances at full precision', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => OK }))
    const price = usePriceStore()
    await price.refresh()
    // 100 ZNN (8 decimals) * 0.118422 + 200 QSR * 0.02343554 = 11.8422 + 4.687108
    const total = price.portfolioUsd([
      { symbol: 'ZNN', amount: '10000000000', decimals: 8 },
      { symbol: 'QSR', amount: '20000000000', decimals: 8 },
    ])
    expect(total).toBeCloseTo(11.8422 + 4.687108, 4)
  })

  it('portfolioUsd returns null when unavailable', () => {
    const price = usePriceStore()
    expect(price.portfolioUsd([])).toBeNull()
  })
})
