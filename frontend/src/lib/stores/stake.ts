import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const stakeInfo = writable<app.StakeInfo | null>(null)
export const reward = writable<app.RewardInfo | null>(null)

export async function refreshStake(): Promise<void> {
  try {
    stakeInfo.set(await Nom.GetStakeList())
    reward.set(await Nom.GetUncollectedReward())
  } catch { /* not connected / locked — leave as-is */ }
}
