import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }

export type NodeConfig = { mode: string; remoteUrl: string; localUrl: string }
export type EmbeddedInfo = { running: boolean; dataDir: string; sizeBytes: number }
export type SyncStatus = {
  state: string
  currentHeight: number
  targetHeight: number
  percent: number
  etaSeconds: number
  peers: number
}

export const useNodeStore = defineStore('node', {
  // _eventsInit is a non-reactive guard so initEvents registers listeners once.
  state: () => ({
    connected: false,
    height: 0,
    syncing: false,
    mode: 'remote',
    sync: null as SyncStatus | null,
    balances: [] as TokenBalance[],
    _eventsInit: false,
  }),
  actions: {
    async connect() {
      try { await N.Connect(); this.connected = true } catch { this.connected = false }
    },
    async loadBalances() {
      try { this.balances = (await N.GetBalances()) as unknown as TokenBalance[] } catch { this.balances = [] }
    },
    async getConfig() {
      return (await N.GetNodeConfig()) as unknown as NodeConfig
    },
    async setMode(mode: string) {
      await N.SetNodeMode(mode)
    },
    async setUrl(mode: string, url: string) {
      await N.SetNodeURL(mode, url)
    },
    async getEmbeddedInfo() {
      return (await N.GetEmbeddedInfo()) as unknown as EmbeddedInfo
    },
    async deleteEmbeddedData() {
      await N.DeleteEmbeddedData()
    },
    // initEvents wires backend push events into the store. onTick is invoked on
    // each momentum so callers can refresh pulled data. Guarded to register once.
    initEvents(onTick: () => void) {
      if (this._eventsInit) return
      this._eventsInit = true
      EventsOn('node:status', (s: any) => {
        this.connected = !!s?.connected
        this.height = s?.height ?? this.height
        this.mode = s?.mode ?? this.mode
      })
      EventsOn('node:sync', (s: any) => {
        this.sync = s
        this.syncing = s?.state !== 'synced'
      })
      EventsOn('momentum:tick', () => onTick())
    },
  },
})
