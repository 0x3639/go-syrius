import { defineStore } from 'pinia'
import * as Tx from '../../wailsjs/go/app/TxService'
import type { app } from '../../wailsjs/go/models'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean; summary?: string }

export const useTxStore = defineStore('tx', {
  state: () => ({ status: 'idle' as 'idle'|'preparing'|'awaiting'|'publishing'|'done'|'error', preview: null as SendPreview | null, hash: '', error: '' }),
  actions: {
    reset() { this.status = 'idle'; this.preview = null; this.hash = ''; this.error = '' },
    async prepare(toAddress: string, zts: string, amount: string) {
      this.status = 'preparing'; this.preview = null; this.hash = ''; this.error = ''
      try {
        this.preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
        this.status = 'awaiting'
      } catch (e: any) { this.status = 'error'; this.error = e?.message ?? String(e) }
    },
    awaitConfirm(preview: SendPreview | app.CallPreview) { this.preview = preview as SendPreview; this.status = 'awaiting'; this.hash = ''; this.error = '' },
    async confirm() {
      this.status = 'publishing'
      try { this.hash = (await Tx.ConfirmPublish()) as string; this.status = 'done'; this.preview = null }
      catch (e: any) { this.status = 'error'; this.error = e?.message ?? String(e) }
    },
    async cancel() { await Tx.CancelPending().catch(() => {}); this.reset() },
  },
})
