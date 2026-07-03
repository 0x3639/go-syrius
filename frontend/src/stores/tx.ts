import { defineStore } from 'pinia'
import * as Tx from '../../wailsjs/go/app/TxService'
import type { app } from '../../wailsjs/go/models'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; decimals: number; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean; summary?: string; holdId?: number }
export type TxStatus = 'idle'|'preparing'|'awaiting'|'publishing'|'done'|'error'

// Identifies the latest prepare() call (the store is a singleton, so a module
// token suffices): a stale continuation must not resurrect a dialog the user
// navigated away from or that a newer prepare superseded.
let prepareToken = 0

export const useTxStore = defineStore('tx', {
  state: () => ({ status: 'idle' as TxStatus, preview: null as SendPreview | null, hash: '', error: '' }),
  actions: {
    reset() { prepareToken++; this.status = 'idle'; this.preview = null; this.hash = ''; this.error = '' },
    async prepare(toAddress: string, zts: string, amount: string) {
      const token = ++prepareToken
      this.status = 'preparing'; this.preview = null; this.hash = ''; this.error = ''
      try {
        const preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
        // Nothing is on-chain yet, so a prepare that lost ownership (navigation
        // reset bumps the token; another flow took the state) is dropped — but
        // its backend hold MUST be released, or the single pending slot would
        // diverge from whatever a dialog is displaying.
        if (token !== prepareToken || this.status !== 'preparing') {
          const orphan = preview?.holdId ?? 0
          if (orphan !== 0) Tx.CancelPending(orphan).catch(() => {})
          return
        }
        this.preview = preview
        this.status = 'awaiting'
      } catch (e: any) {
        if (token !== prepareToken || this.status !== 'preparing') return
        this.status = 'error'; this.error = e?.message ?? String(e)
      }
    },
    // NOTE: the panel flows (tx.awaitConfirm(await Nom.PrepareX())) carry no
    // staleness guard on purpose — if the user navigates mid-prepare, the
    // global dialog follows them (long-standing UX). Funds-safety does not
    // depend on it: ConfirmPublish(holdId) refuses to broadcast anything but
    // the exact block this preview describes.
    awaitConfirm(preview: SendPreview | app.CallPreview) { this.preview = preview as SendPreview; this.status = 'awaiting'; this.hash = ''; this.error = '' },
    async confirm() {
      const holdId = this.preview?.holdId ?? 0
      this.status = 'publishing'
      try {
        // holdId travels to the backend: ConfirmPublish refuses if the held
        // block is no longer the one this dialog displayed (confirm-what-you-
        // sign across the binding boundary).
        const hash = (await Tx.ConfirmPublish(holdId)) as string
        // ASYMMETRIC on purpose: a real broadcast surfaces even if the user
        // closed the dialog or navigated meanwhile (state idle/done/error) —
        // funds moved, and hiding that invites a double-send. The one
        // exception: a NEWER transaction actively in flight keeps its state;
        // wiping a live dialog would strand that block's hold.
        // Snapshot as the full union: TS's flow analysis otherwise still
        // narrows this.status to 'publishing' across the await.
        const cur = this.status as TxStatus
        const ownedByUs = cur === 'publishing' && (this.preview?.holdId ?? 0) === holdId
        const newerActive = !ownedByUs
          && (cur === 'preparing' || cur === 'awaiting' || cur === 'publishing')
        if (newerActive) return // outcome still lands in history/balances
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
