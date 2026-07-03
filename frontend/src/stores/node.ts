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
    chainId: 0,
    sync: null as SyncStatus | null,
    balances: [] as TokenBalance[],
    _eventsInit: false,
    _onTick: null as (() => void) | null,
    // Bumped on every node:status push; lets the refreshStatus pull detect
    // that a fresher push landed while its RPC was in flight.
    _statusSeq: 0,
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
    // Single field-mapping for a node status payload, shared by the push
    // handler and the pull so the two can never drift. `syncing`/`sync` are
    // NOT set here — they are owned exclusively by the node:sync event (the
    // status DTO does not carry a meaningful syncing value).
    applyStatus(s: { connected?: boolean; height?: number; mode?: string; chainId?: number } | null | undefined) {
      this.connected = !!s?.connected
      this.height = s?.height ?? this.height
      this.mode = s?.mode ?? this.mode
      this.chainId = s?.chainId ?? this.chainId
    },
    // Pull the current node status once. Needed because push events only reach
    // listeners that exist: the connect-time node:status fires while the user
    // is still on the Unlock screen (before AppShell registers initEvents), so
    // without this pull chainId/height stay 0 until the next momentum status.
    async refreshStatus() {
      const seq = this._statusSeq
      try {
        const s = await N.NodeStatus()
        // A push that arrived while the RPC was in flight is fresher than this
        // snapshot — never let the stale pull overwrite it (it could briefly
        // re-open the fail-closed governance gate with an old testnet chainId).
        if (seq !== this._statusSeq) return
        this.applyStatus(s)
      } catch { /* not connected; events will hydrate later */ }
    },
    // initEvents wires backend push events into the store. onTick is invoked on
    // each momentum so callers can refresh pulled data. Listeners register once,
    // but the tick callback is re-pointed on every call so each AppShell mount
    // (one per lock/unlock cycle) drives the refresh — the handler never holds
    // a dead first-mount closure.
    initEvents(onTick: () => void) {
      this._onTick = onTick
      this.refreshStatus() // catch up on any status emitted before we listened
      if (this._eventsInit) return
      this._eventsInit = true
      EventsOn('node:status', (s: any) => {
        this._statusSeq++
        this.applyStatus(s)
      })
      EventsOn('node:sync', (s: any) => {
        this.sync = s
        this.syncing = s?.state !== 'synced'
      })
      EventsOn('momentum:tick', () => this._onTick?.())
    },
    // Detach the tick callback (AppShell unmount / lock): momentum:tick fires
    // whenever the node is connected, independent of wallet state, so without
    // this a locked session would keep refreshing all stores every momentum.
    clearTick() {
      this._onTick = null
    },
  },
})
