import { defineStore } from 'pinia'
import { formatAmountExact } from '../lib/format'

const PRICE_URL = 'https://api.zenon.info/price'
const POLL_MS = 60_000 // endpoint is rate-limited (observed HTTP 429) — poll gently

type PriceState = {
  znnUsd: number | null
  qsrUsd: number | null
  available: boolean
  updatedAt: number
  _timer: ReturnType<typeof setInterval> | null
  _inFlight: boolean
}

export const usePriceStore = defineStore('price', {
  state: (): PriceState => ({
    znnUsd: null, qsrUsd: null, available: false, updatedAt: 0, _timer: null, _inFlight: false,
  }),
  actions: {
    async refresh() {
      if (this._inFlight) return // single in-flight guard
      this._inFlight = true
      try {
        const res = await fetch(PRICE_URL, { method: 'GET' })
        if (!res.ok) { this.available = false; return } // 429 / 5xx → degrade
        const body = await res.json()
        const znn = body?.data?.znn?.usd
        const qsr = body?.data?.qsr?.usd
        if (typeof znn !== 'number' || typeof qsr !== 'number') { this.available = false; return }
        this.znnUsd = znn
        this.qsrUsd = qsr
        this.available = true
        this.updatedAt = Date.now()
      } catch {
        this.available = false // offline / CORS / parse error → degrade
      } finally {
        this._inFlight = false
      }
    },
    // USD value of a single balance line; null when unpriced (ZTS tokens) or
    // no price feed is available. Fiat is display-only — see lib/fiat.ts.
    usdFor(symbol: string, amount: string, decimals: number): number | null {
      if (!this.available) return null
      const price = symbol === 'ZNN' ? this.znnUsd : symbol === 'QSR' ? this.qsrUsd : null
      if (price == null) return null
      return parseFloat(formatAmountExact(amount, decimals)) * price
    },
    // portfolioUsd sums fiat across balances. Returns null when no price is
    // available so the Dashboard can fall back to a ZNN headline.
    portfolioUsd(balances: { symbol: string; amount: string; decimals: number }[]): number | null {
      if (!this.available) return null
      let total = 0
      for (const b of balances) total += this.usdFor(b.symbol, b.amount, b.decimals) ?? 0
      return total
    },
    start() {
      this.refresh()
      if (this._timer) return
      this._timer = setInterval(() => this.refresh(), POLL_MS)
    },
    stop() {
      if (this._timer) { clearInterval(this._timer); this._timer = null }
    },
  },
})
