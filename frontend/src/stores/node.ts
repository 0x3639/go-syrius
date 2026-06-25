import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }

export const useNodeStore = defineStore('node', {
  // _eventsInit is a non-reactive guard so initEvents registers listeners once.
  state: () => ({ connected: false, height: 0, syncing: false, balances: [] as TokenBalance[], _eventsInit: false }),
  actions: {
    async connect() {
      try { await N.Connect(); this.connected = true } catch { this.connected = false }
    },
    async loadBalances() {
      try { this.balances = (await N.GetBalances()) as unknown as TokenBalance[] } catch { this.balances = [] }
    },
    // initEvents wires backend push events into the store. onTick is invoked on
    // each momentum so callers can refresh pulled data. Guarded to register once.
    initEvents(onTick: () => void) {
      if (this._eventsInit) return
      this._eventsInit = true
      EventsOn('node:status', (s: any) => { this.connected = !!s?.connected; this.height = s?.height ?? this.height })
      EventsOn('node:sync', (s: any) => { this.syncing = s?.state !== 'synced' })
      EventsOn('momentum:tick', () => onTick())
    },
  },
})
