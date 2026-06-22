import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const sentinel = writable<app.SentinelInfo | null>(null)
export const depositedQsr = writable<string>('0')
export const sentinelReward = writable<app.RewardInfo | null>(null)

export async function refreshSentinel(): Promise<void> {
  try {
    sentinel.set(await Nom.GetSentinel())
    depositedQsr.set(await Nom.GetDepositedQsr())
    sentinelReward.set(await Nom.GetSentinelReward())
  } catch { /* not connected / locked — leave as-is */ }
}
