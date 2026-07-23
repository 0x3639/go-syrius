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
    // owned by the node:sync event (the status DTO carries no meaningful
    // syncing value) — with one exception: LEAVING embedded mode clears them,
    // because only the embedded node runs the sync poller and stale mid-sync
    // progress would otherwise show "Syncing…" for the rest of the session.
    // (Catching up TO embedded must not clear: a live node:sync push may
    // already have landed before this pull resolved.)
    applyStatus(s: { connected?: boolean; height?: number; mode?: string; chainId?: number } | null | undefined) {
      this.connected = !!s?.connected
      this.height = s?.height ?? this.height
      const mode = s?.mode ?? this.mode
      if (this.mode === 'embedded' && mode !== 'embedded') {
        this.syncing = false
        this.sync = null
      }
      this.mode = mode
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
        // Only the embedded node emits sync progress. Drop stragglers from a
        // dying embedded poller after a mode switch — they would re-stick
        // "Syncing…" right after applyStatus cleared it.
        if (this.mode !== 'embedded') return
        this.sync = s
        this.syncing = s?.state !== 'synced'
      })
      // During embedded bulk sync ticks arrive continuously; refreshing every
      // store on each one keeps seven RPC groups running back-to-back against
      // the very node that is trying to insert blocks. Skip until synced.
      EventsOn('momentum:tick', () => { if (!this.syncing) this._onTick?.() })
    },
    // Detach the tick callback (AppShell unmount / lock): momentum:tick fires
    // whenever the node is connected, independent of wallet state, so without
    // this a locked session would keep refreshing all stores every momentum.
    clearTick() {
      this._onTick = null
    },
  },
})
