import { writable } from 'svelte/store'
import * as Tx from '../../../wailsjs/go/app/TxService'

export type SendPreview = { toAddress: string; symbol: string; zts: string; amount: string; usedPlasma: number; difficulty: number; hash: string; needsPoW: boolean }
export type TxState = { status: 'idle' | 'preparing' | 'awaiting' | 'publishing' | 'done' | 'error'; preview: SendPreview | null; hash: string; error: string }

export const tx = writable<TxState>({ status: 'idle', preview: null, hash: '', error: '' })

export async function prepare(toAddress: string, zts: string, amount: string): Promise<void> {
  tx.set({ status: 'preparing', preview: null, hash: '', error: '' })
  try {
    const preview = (await Tx.PrepareSend({ toAddress, zts, amount } as any)) as unknown as SendPreview
    tx.set({ status: 'awaiting', preview, hash: '', error: '' })
  } catch (e: any) {
    tx.set({ status: 'error', preview: null, hash: '', error: e?.message ?? String(e) })
  }
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
