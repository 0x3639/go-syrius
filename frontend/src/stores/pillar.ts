import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const usePillarStore = defineStore('pillar', {
  state: () => ({
    delegation: null as app.DelegationInfo | null,
    pillars: [] as app.PillarSummary[],
    reward: null as app.RewardInfo | null,
  }),
  actions: {
    async refreshDelegation() {
      try {
        this.delegation = await Nom.GetDelegation()
      } catch { /* not connected / locked — leave as-is */ }
    },
    async refresh() {
      try {
        this.pillars = await Nom.GetPillarList()
        this.delegation = await Nom.GetDelegation()
        this.reward = await Nom.GetPillarReward()
      } catch { /* not connected / locked — leave as-is */ }
    },
  },
})
