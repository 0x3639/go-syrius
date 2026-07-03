import { defineStore } from 'pinia'
import * as Tx from '../../wailsjs/go/app/TxService'
import type { app } from '../../wailsjs/go/models'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; decimals: number; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean; summary?: string; holdId?: number }

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
      try {
        const hash = (await Tx.ConfirmPublish()) as string
        // Ownership guard: if the state moved on while publishing (e.g. the
        // router's afterEach reset on navigation), the outcome is unowned —
        // don't pop a dialog out of nowhere on an unrelated screen.
        if (this.status !== 'publishing') return
        this.hash = hash; this.status = 'done'; this.preview = null
      } catch (e: any) {
        if (this.status !== 'publishing') return
        this.status = 'error'; this.error = e?.message ?? String(e)
      }
    },
    // THE single cancel path (dialog close, Cancel button, gate enforcement).
    // Reset-first: 'awaiting' is left synchronously, so there is never a frame
    // where a confirm dialog shows a live Confirm for a cancelled block, and a
    // stale continuation can't wipe newer state (there is none to wipe). The
    // backend hold is released in the background, identity-checked by holdId
    // so it can only ever release THIS block — never a newer one that won a
    // race against the RPC.
    discard() {
      if (this.status !== 'awaiting') return
      const holdId = this.preview?.holdId ?? 0
      this.reset()
      Tx.CancelPending(holdId).catch(() => {})
    },
  },
})
