import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'

export type TxRecord = {
  hash: string; direction: string; method: string; counterparty: string; token: string
  amount: string; decimals: number; momentumHeight: number; confirmed: boolean; timestamp: number
}

const PAGE_SIZE = 10 // displayed rows per page
const BLOCK_FETCH = 20 // account blocks fetched per backend call

function isTransfer(t: { direction: string; amount: string }): boolean {
  if (t.direction === 'pair') return false
  try {
    return BigInt(t.amount || '0') > 0n
  } catch {
    return true
  }
}

// History pagination over DISPLAYED rows (not raw blocks): each account block
// expands to 1–2 rows and the Transfers filter hides some, so the store buffers
// expanded records by fetching block chunks until it can fill a 10-row page and
// tell whether a next page exists.
export const useTxsStore = defineStore('txs', {
  state: () => ({
    buffer: [] as TxRecord[], // every expanded record fetched so far (unfiltered)
    chunkIndex: 0, // next backend page index (BLOCK_FETCH blocks each)
    hasMoreBlocks: true,
    page: 0, // current displayed page
    transfersOnly: true,
  }),
  getters: {
    filtered(s): TxRecord[] {
      return s.transfersOnly ? s.buffer.filter(isTransfer) : s.buffer
    },
    pageItems(): TxRecord[] {
      return this.filtered.slice(this.page * PAGE_SIZE, this.page * PAGE_SIZE + PAGE_SIZE)
    },
    hasNextPage(): boolean {
      return this.filtered.length > (this.page + 1) * PAGE_SIZE
    },
  },
  actions: {
    async fetchChunk(): Promise<boolean> {
      if (!this.hasMoreBlocks) return false
      try {
        const r = (await N.GetTransactions(this.chunkIndex, BLOCK_FETCH)) as unknown as { records: TxRecord[]; hasMore: boolean }
        this.buffer.push(...(r.records ?? []))
        this.hasMoreBlocks = !!r.hasMore
        this.chunkIndex++
        return true
      } catch {
        this.hasMoreBlocks = false
        return false
      }
    },
    // Fetch chunks until the current page is full and we can tell whether a next
    // page exists, or there are no more blocks.
    async ensure() {
      while (this.filtered.length <= (this.page + 1) * PAGE_SIZE && this.hasMoreBlocks) {
        if (!(await this.fetchChunk())) break
      }
    },
    // Reload from scratch (mount / account switch / new-momentum refresh). Builds
    // a fresh buffer locally and swaps it in once, so the live view never flashes
    // empty (this runs on every momentum tick).
    async load() {
      const fresh: TxRecord[] = []
      let idx = 0
      let more = true
      const filteredLen = () => (this.transfersOnly ? fresh.filter(isTransfer) : fresh).length
      while (filteredLen() <= (this.page + 1) * PAGE_SIZE && more) {
        try {
          const r = (await N.GetTransactions(idx, BLOCK_FETCH)) as unknown as { records: TxRecord[]; hasMore: boolean }
          fresh.push(...(r.records ?? []))
          more = !!r.hasMore
          idx++
        } catch {
          more = false
          break
        }
      }
      this.buffer = fresh
      this.chunkIndex = idx
      this.hasMoreBlocks = more
    },
    async goto(page: number) {
      this.page = Math.max(0, page)
      await this.ensure()
    },
    async setTransfersOnly(v: boolean) {
      this.transfersOnly = v
      this.page = 0
      await this.ensure()
    },
    resetPage() {
      this.page = 0
    },
  },
})
