import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import { useNodeStore } from './node'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }

export const useBalancesStore = defineStore('balances', {
  state: () => ({
    items: [] as TokenBalance[],
    status: 'idle' as 'idle' | 'loading' | 'ready' | 'error',
    _epoch: -1,
    _request: 0,
  }),
  getters: {
    available: (s): boolean => s.status === 'ready' && useNodeStore().connected,
  },
  actions: {
    invalidate() {
      this._request++
      this.items = []
      this.status = 'idle'
    },
    async load() {
      const epoch = currentRequestEpoch()
      if (this._epoch !== epoch) this.invalidate()
      this._epoch = epoch
      const request = ++this._request
      if (this.status !== 'ready') this.status = 'loading'
      try {
        const items = (await N.GetBalances()) as unknown as TokenBalance[]
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.items = items
        this.status = 'ready'
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.items = []
        this.status = 'error'
      }
    },
  },
})
