import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import { withTimeout } from '../lib/withTimeout'
import type { app } from '../../wailsjs/go/models'
import { useNodeStore } from './node'

export const useStakeStore = defineStore('stake', {
  state: () => ({
    stakeInfo: null as app.StakeInfo | null,
    reward: null as app.RewardInfo | null,
    status: 'idle' as 'idle' | 'loading' | 'ready' | 'error',
    // True from the moment the snapshot is known to be out of step with the
    // balance (see resync) until a fresh one lands. Readers that add staked
    // ZNN to the spendable balance must skip the snapshot while it is stale.
    stale: false,
    _epoch: -1,
    _request: 0,
    // Id of the request in flight that can still land (0 = none).
    _active: 0,
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
      this.stale = false
      this._active = 0 // an in-flight request can no longer land
    },
    // Periodic reload. Starting a new request supersedes the one in flight, so
    // on a node slower than the tick interval every response would be
    // discarded and no snapshot would ever land. A periodic caller therefore
    // lets the current request finish. resync() and refresh() still supersede:
    // they are asked for when newer data is known to exist.
    refreshIfIdle() {
      if (this._active) return Promise.resolve()
      return this.refresh()
    },
    // The ZNN balance moved, so the held snapshot may describe ZNN that is
    // now spendable (or the reverse). Mark it stale at once, then reload; only
    // the newest request lands, and landing is what clears the mark.
    resync() {
      this.stale = true
      return this.refresh()
    },
    async refresh() {
      const epoch = currentRequestEpoch()
      if (this._epoch !== epoch) this.invalidate()
      const request = ++this._request
      if (this.status !== 'ready') this.status = 'loading'
      this._active = request
      try {
        // Bounded: an unanswered read must release `_active` (so periodic
        // reloads resume) and surface as unknown, not hold the rows forever.
        const [stakeInfo, reward] = await withTimeout(Promise.all([Nom.GetStakeList(), Nom.GetUncollectedReward()]))
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.stakeInfo = stakeInfo
        this.reward = reward
        this.status = 'ready'
        this.stale = false
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.stakeInfo = null
        this.reward = null
        this.status = 'error'
        this.stale = false
      } finally {
        if (this._active === request) this._active = 0
      }
    },
  },
})
