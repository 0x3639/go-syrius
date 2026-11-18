import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

export const usePlasmaStore = defineStore('plasma', {
  state: () => ({
    info: null as app.PlasmaInfo | null,
    fusionEntries: [] as app.FusionEntry[],
    _request: 0,
  }),
  actions: {
    invalidate() {
      this._request++
      this.info = null
      this.fusionEntries = []
    },
    async refresh() {
      const epoch = currentRequestEpoch()
      const request = ++this._request
      try {
        const info = await Nom.GetPlasmaInfo()
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        const entries = await Nom.GetFusionEntries()
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.info = info
        this.fusionEntries = entries
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.info = null
        this.fusionEntries = []
      }
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
