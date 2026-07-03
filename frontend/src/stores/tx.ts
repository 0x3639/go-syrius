import { defineStore } from 'pinia'
import * as Tx from '../../wailsjs/go/app/TxService'
import type { app } from '../../wailsjs/go/models'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; decimals: number; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean; summary?: string; holdId?: number }

// Identifies the latest prepare() call (the store is a singleton, so a module
// token suffices): a stale continuation must not resurrect a dialog the user
// navigated away from or that a newer prepare superseded.
let prepareToken = 0

export const useTxStore = defineStore('tx', {
  state: () => ({ status: 'idle' as 'idle'|'preparing'|'awaiting'|'publishing'|'done'|'error', preview: null as SendPreview | null, hash: '', error: '' }),
  actions: {
    reset() { this.status = 'idle'; this.preview = null; this.hash = ''; this.error = '' },
    async prepare(toAddress: string, zts: string, amount: string) {
      const token = ++prepareToken
      this.status = 'preparing'; this.preview = null; this.hash = ''; this.error = ''
      try {
        const preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
        // Nothing is on-chain yet, so a prepare that lost ownership (navigation
        // reset, or a newer prepare) is safe to drop — seating it would pop a
        // live Confirm dialog on a screen the user has moved on from.
        if (token !== prepareToken || this.status !== 'preparing') return
        this.preview = preview
        this.status = 'awaiting'
      } catch (e: any) {
        if (token !== prepareToken || this.status !== 'preparing') return
        this.status = 'error'; this.error = e?.message ?? String(e)
      }
    },
    awaitConfirm(preview: SendPreview | app.CallPreview) { this.preview = preview as SendPreview; this.status = 'awaiting'; this.hash = ''; this.error = '' },
    async confirm() {
      const holdId = this.preview?.holdId ?? 0
      this.status = 'publishing'
      try {
        const hash = (await Tx.ConfirmPublish()) as string
        // ASYMMETRIC on purpose: a real broadcast ALWAYS surfaces, even if the
        // user closed the dialog or navigated meanwhile — funds moved, and
        // silently dropping the outcome invites a double-send. (This also lets
        // a genuine success overwrite a raced double-confirm's error.)
        this.hash = hash; this.status = 'done'; this.preview = null
      } catch (e: any) {
        // Nothing happened on-chain. Surface the failure only if THIS
        // transaction still owns the state (same hold, still publishing) — a
        // stale failure must not pop an orphan error dialog elsewhere.
        if (this.status !== 'publishing' || (this.preview?.holdId ?? 0) !== holdId) return
        this.status = 'error'; this.error = e?.message ?? String(e)
      }
    },
    // THE single cancel path (dialog close, Cancel button, gate enforcement).
    // Reset-first: 'awaiting' is left synchronously, so there is never a frame
    // where a confirm dialog shows a live Confirm for a cancelled block, and a
    // stale continuation can't wipe newer state (there is none to wipe). The
    // backend hold is released in the background, identity-checked by holdId
    // so it can only ever release THIS block — never a newer one that won a
    // race against the RPC. Without a holdId there is no identity to check, so
    // we skip the release entirely (the hold is superseded by the next prepare
    // and session-guarded) rather than risk an unconditional cancel.
    discard() {
      if (this.status !== 'awaiting') return
      const holdId = this.preview?.holdId ?? 0
      this.reset()
      if (holdId !== 0) Tx.CancelPending(holdId).catch(() => {})
    },
  },
})
