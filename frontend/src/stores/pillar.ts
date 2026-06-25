import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

// Minimal pillar store: delegation only. The full pillar panel (pillars list,
// rewards) lands in B3.
export const usePillarStore = defineStore('pillar', {
  state: () => ({
    delegation: null as app.DelegationInfo | null,
  }),
  actions: {
    async refreshDelegation() {
      try {
        this.delegation = await Nom.GetDelegation()
      } catch { /* not connected / locked — leave as-is */ }
    },
  },
})
