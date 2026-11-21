import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import * as Tx from '../../wailsjs/go/app/TxService'
import { useNodeStore } from './node'

export type Unreceived = { fromHash: string; fromAddress: string; token: string; zts?: string; amount: string; decimals: number }

export const useUnreceivedStore = defineStore('unreceived', {
  state: () => ({
    items: [] as Unreceived[],
    busy: {} as Record<string, boolean>,
    busyAll: false,
    error: '',
    // Outcome of the latest lookup. An empty `items` only means "nothing is
    // waiting" when this is 'ok'; 'failed' means the node did not answer and
    // payments may still be waiting. 'idle' is before the first answer.
    lookup: 'idle' as 'idle' | 'ok' | 'failed',
    // When a lookup last succeeded for the current account and connection
    // (ms since epoch, 0 = never).
    lastCheckedAt: 0,
    _request: 0,
    _scope: 0,
  }),
  getters: {
    // "Couldn't check": connected, but the lookup itself failed. A lost
    // connection is a different state, owned by the balance notice.
    failed: (s): boolean => s.lookup === 'failed' && useNodeStore().connected,
  },
  actions: {
    invalidate() {
      this._request++
      this._scope++
      this.items = []
      this.lookup = 'idle'
      this.lastCheckedAt = 0
      this.error = ''
      this.busy = {}
      this.busyAll = false
    },
    async load() {
      const epoch = currentRequestEpoch()
      const request = ++this._request
      try {
        const items = (await N.GetUnreceived()) as unknown as Unreceived[]
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.items = items
        this.lookup = 'ok'
        this.lastCheckedAt = Date.now()
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        // Do not keep showing rows we can no longer vouch for, and do not let
        // the empty list read as "nothing waiting": record the failure.
        this.items = []
        this.lookup = 'failed'
      }
    },
    async receive(hash: string) {
      const scope = this._scope
      const epoch = currentRequestEpoch()
      const current = () => scope === this._scope && epoch === currentRequestEpoch()
      this.error = ''
      this.busy = { ...this.busy, [hash]: true }
      try {
        await Tx.Receive(hash)
        if (current()) await this.load()
      } catch (e: any) {
        if (current()) this.error = e?.message ?? String(e)
      } finally {
        if (scope === this._scope) {
          const { [hash]: _, ...rest } = this.busy
          this.busy = rest
        }
      }
    },
    async receiveAll() {
      const scope = this._scope
      const epoch = currentRequestEpoch()
      const current = () => scope === this._scope && epoch === currentRequestEpoch()
      this.error = ''
      this.busyAll = true
      try {
        for (const u of [...this.items]) {
          if (!current()) return
          await Tx.Receive(u.fromHash)
        }
        if (current()) await this.load()
      } catch (e: any) {
        if (current()) this.error = e?.message ?? String(e)
      } finally {
        if (scope === this._scope) this.busyAll = false
      }
    },
  },
})
