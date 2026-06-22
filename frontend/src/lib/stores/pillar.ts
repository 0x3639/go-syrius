import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const pillars = writable<app.PillarSummary[]>([])
export const delegation = writable<app.DelegationInfo | null>(null)
export const pillarReward = writable<app.RewardInfo | null>(null)

export async function refreshPillars(): Promise<void> {
  try {
    pillars.set(await Nom.GetPillarList())
    delegation.set(await Nom.GetDelegation())
    pillarReward.set(await Nom.GetPillarReward())
  } catch { /* not connected / locked — leave as-is */ }
}
