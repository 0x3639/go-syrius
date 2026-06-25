import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }

export const useBalancesStore = defineStore('balances', {
  state: () => ({ items: [] as TokenBalance[] }),
  actions: {
    async load() {
      try { this.items = (await N.GetBalances()) as unknown as TokenBalance[] } catch { this.items = [] }
    },
  },
})
