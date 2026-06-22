import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const plasmaInfo = writable<app.PlasmaInfo | null>(null)
export const fusionEntries = writable<app.FusionEntry[]>([])

export async function refreshPlasma(): Promise<void> {
  try {
    plasmaInfo.set(await Nom.GetPlasmaInfo())
    fusionEntries.set(await Nom.GetFusionEntries())
  } catch { /* not connected / locked — leave as-is */ }
}
export async function estimatePlasma(qsr: string): Promise<number> {
  try { return await Nom.EstimatePlasma(qsr) } catch { return 0 }
}
