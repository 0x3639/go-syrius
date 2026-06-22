import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'

export type StakeEntry = { id: string; amount: string; startTimestamp: number; expirationTimestamp: number; durationMonths: number; isMatured: boolean }
export type StakeInfo = { totalAmount: string; entries: StakeEntry[] }
export type RewardInfo = { znn: string; qsr: string }

export const stakeInfo = writable<StakeInfo | null>(null)
export const reward = writable<RewardInfo | null>(null)

export async function refreshStake(): Promise<void> {
  try {
    stakeInfo.set((await Nom.GetStakeList()) as StakeInfo)
    reward.set((await Nom.GetUncollectedReward()) as RewardInfo)
  } catch { /* not connected / locked — leave as-is */ }
}
