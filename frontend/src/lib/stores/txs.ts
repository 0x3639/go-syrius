import { writable } from 'svelte/store'
import * as N from '../../../wailsjs/go/app/NodeService'

export type TxRecord = {
  hash: string; direction: string; counterparty: string; token: string
  amount: string; momentumHeight: number; confirmed: boolean; timestamp: number
}
export const txs = writable<TxRecord[]>([])

export async function loadTxs(page = 0, count = 25): Promise<void> {
  try { txs.set((await N.GetTransactions(page, count)) as unknown as TxRecord[]) } catch { txs.set([]) }
}
