import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

export const useStakeStore = defineStore('stake', {
  state: () => ({
    stakeInfo: null as app.StakeInfo | null,
    reward: null as app.RewardInfo | null,
  }),
  actions: {
    async refresh() {
      const epoch = currentRequestEpoch()
      try {
        const stakeInfo = await Nom.GetStakeList()
        const reward = await Nom.GetUncollectedReward()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.stakeInfo = stakeInfo
        this.reward = reward
      } catch { /* not connected / locked — leave as-is */ }
    },
  },
})
