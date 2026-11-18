import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'
import { useNodeStore } from './node'

export const useStakeStore = defineStore('stake', {
  state: () => ({
    stakeInfo: null as app.StakeInfo | null,
    reward: null as app.RewardInfo | null,
    status: 'idle' as 'idle' | 'loading' | 'ready' | 'error',
    _epoch: -1,
    _request: 0,
  }),
  getters: {
    available: (s) => s.status === 'ready' && useNodeStore().connected,
  },
  actions: {
    invalidate() {
      this._epoch = currentRequestEpoch()
      this._request++
      this.stakeInfo = null
      this.reward = null
      this.status = 'idle'
    },
    async refresh() {
      const epoch = currentRequestEpoch()
      if (this._epoch !== epoch) this.invalidate()
      const request = ++this._request
      if (this.status !== 'ready') this.status = 'loading'
      try {
        const [stakeInfo, reward] = await Promise.all([Nom.GetStakeList(), Nom.GetUncollectedReward()])
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.stakeInfo = stakeInfo
        this.reward = reward
        this.status = 'ready'
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.stakeInfo = null
        this.reward = null
        this.status = 'error'
      }
    },
  },
})
