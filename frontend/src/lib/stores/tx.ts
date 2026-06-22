import { writable } from 'svelte/store'
import * as Tx from '../../../wailsjs/go/app/TxService'
import { view } from './nav'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean; summary?: string }
export type TxState = { status: 'idle' | 'preparing' | 'awaiting' | 'publishing' | 'done' | 'error'; preview: SendPreview | null; hash: string; error: string }

export const tx = writable<TxState>({ status: 'idle', preview: null, hash: '', error: '' })

// resetTx returns the shared tx flow to idle, discarding any finished/errored/
// awaiting state. The Go side holds at most one pending block, which the next
// prepare overwrites and which only ConfirmPublish (reachable solely via the
// modal) can act on — so clearing the frontend state alone is sufficient.
export function resetTx(): void {
  tx.set({ status: 'idle', preview: null, hash: '', error: '' })
}

// The tx store is a singleton shared by every route (Send/Plasma/Stake/Pillars).
// Reset it whenever the user navigates so a stale result/error/modal from one
// route never surfaces on an unrelated screen. The initial subscription value
// is skipped (the store already starts idle).
let navInitialized = false
view.subscribe(() => {
  if (!navInitialized) {
    navInitialized = true
    return
  }
  resetTx()
})

export async function prepare(toAddress: string, zts: string, amount: string): Promise<void> {
  tx.set({ status: 'preparing', preview: null, hash: '', error: '' })
  try {
    const preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
    tx.set({ status: 'awaiting', preview, hash: '', error: '' })
  } catch (e: any) {
    tx.set({ status: 'error', preview: null, hash: '', error: e?.message ?? String(e) })
  }
}

// awaitConfirm seats an already-prepared block's preview into the tx flow. The
// Go side (e.g. NomService.PrepareFuse) has already built+held the pending
// block; this only surfaces the confirm-what-you-sign preview so TxModal can
// drive the shared confirm()/ConfirmPublish() path. CallPreview is a superset
// of SendPreview (it adds `summary`), so it slots in directly.
export function awaitConfirm(preview: SendPreview): void {
  tx.set({ status: 'awaiting', preview, hash: '', error: '' })
}

export async function confirm(): Promise<void> {
  tx.update((s) => ({ ...s, status: 'publishing' }))
  try {
    const hash = (await Tx.ConfirmPublish()) as string
    tx.set({ status: 'done', preview: null, hash, error: '' })
  } catch (e: any) {
    tx.update((s) => ({ ...s, status: 'error', error: e?.message ?? String(e) }))
  }
}

export async function cancel(): Promise<void> {
  await Tx.CancelPending().catch(() => {})
  tx.set({ status: 'idle', preview: null, hash: '', error: '' })
}
