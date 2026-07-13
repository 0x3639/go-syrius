import { defineStore } from 'pinia'
import * as N from '../../wailsjs/go/app/NodeService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import * as Tx from '../../wailsjs/go/app/TxService'

export type Unreceived = { fromHash: string; fromAddress: string; token: string; amount: string; decimals: number }

export const useUnreceivedStore = defineStore('unreceived', {
  state: () => ({
    items: [] as Unreceived[],
    busy: {} as Record<string, boolean>,
    busyAll: false,
    error: '',
  }),
  actions: {
    async load() {
      const epoch = currentRequestEpoch()
      try {
        const items = (await N.GetUnreceived()) as unknown as Unreceived[]
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.items = items
      } catch {
        if (epoch !== currentRequestEpoch()) return
        this.items = []
      }
    },
    async receive(hash: string) {
      this.error = ''
      this.busy = { ...this.busy, [hash]: true }
      try {
        await Tx.Receive(hash)
        await this.load()
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      } finally {
        const { [hash]: _, ...rest } = this.busy
        this.busy = rest
      }
    },
    async receiveAll() {
      this.error = ''
      this.busyAll = true
      try {
        for (const u of this.items) await Tx.Receive(u.fromHash)
        await this.load()
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      } finally {
        this.busyAll = false
      }
    },
  },
})
