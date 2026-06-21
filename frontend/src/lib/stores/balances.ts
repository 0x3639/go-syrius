import { writable } from 'svelte/store'
import * as N from '../../../wailsjs/go/app/NodeService'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }
export const balances = writable<TokenBalance[]>([])

export async function loadBalances(): Promise<void> {
  try { balances.set((await N.GetBalances()) as unknown as TokenBalance[]) } catch { balances.set([]) }
}
