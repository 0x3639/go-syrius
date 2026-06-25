import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const usePlasmaStore = defineStore('plasma', {
  state: () => ({
    info: null as app.PlasmaInfo | null,
  }),
  actions: {
    async refresh() {
      try {
        this.info = await Nom.GetPlasmaInfo()
      } catch { /* not connected / locked — leave as-is */ }
    },
  },
})
