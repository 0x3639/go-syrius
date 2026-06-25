import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const useStakeStore = defineStore('stake', {
  state: () => ({
    stakeInfo: null as app.StakeInfo | null,
    reward: null as app.RewardInfo | null,
  }),
  actions: {
    async refresh() {
      try {
        this.stakeInfo = await Nom.GetStakeList()
        this.reward = await Nom.GetUncollectedReward()
      } catch { /* not connected / locked — leave as-is */ }
    },
  },
})
