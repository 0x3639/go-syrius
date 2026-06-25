import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const usePlasmaStore = defineStore('plasma', {
  state: () => ({
    info: null as app.PlasmaInfo | null,
    fusionEntries: [] as app.FusionEntry[],
  }),
  actions: {
    async refresh() {
      try {
        this.info = await Nom.GetPlasmaInfo()
        this.fusionEntries = await Nom.GetFusionEntries()
      } catch { /* not connected / locked — leave as-is */ }
    },
    async estimate(qsr: string): Promise<number> {
      try {
        return await Nom.EstimatePlasma(qsr)
      } catch {
        return 0
      }
    },
  },
})
