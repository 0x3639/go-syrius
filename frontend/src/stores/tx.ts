import { defineStore } from 'pinia'
import * as Tx from '../../wailsjs/go/app/TxService'
import type { app } from '../../wailsjs/go/models'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; decimals: number; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean; summary?: string }

export const useTxStore = defineStore('tx', {
  // _seq identifies the current transaction: bumped whenever a new block
  // reaches the store (prepare/awaitConfirm), so a stale async continuation
  // (e.g. an old cancel) can detect that it no longer owns the state.
  state: () => ({ status: 'idle' as 'idle'|'preparing'|'awaiting'|'publishing'|'done'|'error', preview: null as SendPreview | null, hash: '', error: '', _seq: 0 }),
  actions: {
    reset() { this.status = 'idle'; this.preview = null; this.hash = ''; this.error = '' },
    async prepare(toAddress: string, zts: string, amount: string) {
      this._seq++
      this.status = 'preparing'; this.preview = null; this.hash = ''; this.error = ''
      try {
        this.preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
        this.status = 'awaiting'
      } catch (e: any) { this.status = 'error'; this.error = e?.message ?? String(e) }
    },
    awaitConfirm(preview: SendPreview | app.CallPreview) { this._seq++; this.preview = preview as SendPreview; this.status = 'awaiting'; this.hash = ''; this.error = '' },
    async confirm() {
      this.status = 'publishing'
      try { this.hash = (await Tx.ConfirmPublish()) as string; this.status = 'done'; this.preview = null }
      catch (e: any) { this.status = 'error'; this.error = e?.message ?? String(e) }
    },
    // Cancel the held (awaiting) block: release it in the backend, then clear
    // the frontend state — unless the state moved on during the CancelPending
    // round-trip: a Confirm click must keep its publishing/done/error outcome,
    // and a NEWER prepared block (same status, higher _seq) must not be wiped
    // by this stale cancel.
    async cancel() {
      const seq = this._seq
      await Tx.CancelPending().catch(() => {})
      if (this.status === 'awaiting' && seq === this._seq) this.reset()
    },
    // Synchronous discard, for gate-style enforcement (e.g. the testnet-only
    // Governance gate): leave 'awaiting' immediately — no window where the
    // confirm dialog is open with a live Confirm button — then release the
    // backend-held block in the background.
    discard() {
      if (this.status !== 'awaiting') return
      this.reset()
      Tx.CancelPending().catch(() => {})
    },
  },
})
