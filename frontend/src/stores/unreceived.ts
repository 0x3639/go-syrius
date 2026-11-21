import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import * as Tx from '../../wailsjs/go/app/TxService'

export type Unreceived = { fromHash: string; fromAddress: string; token: string; zts?: string; amount: string; decimals: number }

export const useUnreceivedStore = defineStore('unreceived', {
  state: () => ({
    items: [] as Unreceived[],
    busy: {} as Record<string, boolean>,
    busyAll: false,
    error: '',
    _request: 0,
    _scope: 0,
  }),
  actions: {
    invalidate() {
      this._request++
      this._scope++
      this.items = []
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
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.items = []
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
