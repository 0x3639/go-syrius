import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

export const usePlasmaStore = defineStore('plasma', {
  state: () => ({
    info: null as app.PlasmaInfo | null,
    fusionEntries: [] as app.FusionEntry[],
  }),
  actions: {
    async refresh() {
      const epoch = currentRequestEpoch()
      try {
        const info = await Nom.GetPlasmaInfo()
        const entries = await Nom.GetFusionEntries()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.info = info
        this.fusionEntries = entries
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
