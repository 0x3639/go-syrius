import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'

export type PlasmaInfo = { qsrFused: string; currentPlasma: number; maxPlasma: number }
export type FusionEntry = { id: string; beneficiary: string; qsrAmount: string; expirationHeight: number; isRevocable: boolean }

export const plasmaInfo = writable<PlasmaInfo | null>(null)
export const fusionEntries = writable<FusionEntry[]>([])

export async function refreshPlasma(): Promise<void> {
  try {
    plasmaInfo.set((await Nom.GetPlasmaInfo()) as PlasmaInfo)
    fusionEntries.set((await Nom.GetFusionEntries()) as FusionEntry[])
  } catch { /* not connected / locked — leave as-is */ }
}
export async function estimatePlasma(qsr: string): Promise<number> {
  try { return (await Nom.EstimatePlasma(qsr)) as number } catch { return 0 }
}
