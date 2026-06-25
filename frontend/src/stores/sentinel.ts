import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const useSentinelStore = defineStore('sentinel', {
  state: () => ({
    sentinel: null as app.SentinelInfo | null,
    depositedQsr: '0',
    reward: null as app.RewardInfo | null,
  }),
  actions: {
    async refresh() {
      try {
        this.sentinel = await Nom.GetSentinel()
        this.depositedQsr = await Nom.GetDepositedQsr()
        this.reward = await Nom.GetSentinelReward()
      } catch { /* not connected / locked — leave as-is */ }
    },
  },
})
