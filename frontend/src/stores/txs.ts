import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'

export type TxRecord = {
  hash: string; direction: string; counterparty: string; token: string
  amount: string; decimals: number; momentumHeight: number; confirmed: boolean; timestamp: number
}

export const useTxsStore = defineStore('txs', {
  state: () => ({ items: [] as TxRecord[] }),
  actions: {
    async load(page = 0, count = 25) {
      try { this.items = (await N.GetTransactions(page, count)) as unknown as TxRecord[] } catch { this.items = [] }
    },
  },
})
