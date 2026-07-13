import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'
import { currentRequestEpoch } from '../lib/requestEpoch'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }

export const useBalancesStore = defineStore('balances', {
  state: () => ({ items: [] as TokenBalance[] }),
  actions: {
    async load() {
      const epoch = currentRequestEpoch()
      try {
        const items = (await N.GetBalances()) as unknown as TokenBalance[]
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.items = items
      } catch {
        if (epoch !== currentRequestEpoch()) return
        this.items = []
      }
    },
  },
})
