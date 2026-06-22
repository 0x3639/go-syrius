import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'

export type PillarSummary = { name: string; rank: number; weight: string; delegateRewardPercent: number; producerAddress: string }
export type DelegationInfo = { name: string; status: number; weight: string }
export type RewardInfo = { znn: string; qsr: string }

export const pillars = writable<PillarSummary[]>([])
export const delegation = writable<DelegationInfo | null>(null)
export const pillarReward = writable<RewardInfo | null>(null)

export async function refreshPillars(): Promise<void> {
  try {
    pillars.set((await Nom.GetPillarList()) as PillarSummary[])
    delegation.set((await Nom.GetDelegation()) as DelegationInfo)
    pillarReward.set((await Nom.GetPillarReward()) as RewardInfo)
  } catch { /* not connected / locked — leave as-is */ }
}
