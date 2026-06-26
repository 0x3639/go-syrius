import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'

export type TxRecord = {
  hash: string; direction: string; method: string; counterparty: string; token: string
  amount: string; decimals: number; momentumHeight: number; confirmed: boolean; timestamp: number
}

const PAGE_SIZE = 10

export const useTxsStore = defineStore('txs', {
  state: () => ({ items: [] as TxRecord[], page: 0, hasMore: false }),
  actions: {
    async load() {
      try {
        const r = (await N.GetTransactions(this.page, PAGE_SIZE)) as unknown as { records: TxRecord[]; hasMore: boolean }
        this.items = r.records ?? []
        this.hasMore = !!r.hasMore
      } catch {
        this.items = []
        this.hasMore = false
      }
    },
    async goto(page: number) {
      this.page = Math.max(0, page)
      await this.load()
    },
    resetPage() {
      this.page = 0
    },
  },
})
