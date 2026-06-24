import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }

export const useNodeStore = defineStore('node', {
  state: () => ({ connected: false, balances: [] as TokenBalance[] }),
  actions: {
    async connect() {
      try { await N.Connect(); this.connected = true } catch { this.connected = false }
    },
    async loadBalances() {
      try { this.balances = (await N.GetBalances()) as unknown as TokenBalance[] } catch { this.balances = [] }
    },
  },
})
