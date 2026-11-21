import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'
import { useNodeStore } from './node'

// What the active account has locked in the pillar and sentinel contracts and
// still owns (it returns on revoke or withdrawal). Read for the holdings
// table; the pillar and sentinel panels keep their own registration state.
export const useCollateralStore = defineStore('collateral', {
  state: () => ({
    info: null as app.Collateral | null,
    status: 'idle' as 'idle' | 'loading' | 'ready' | 'error',
    // True from the moment the snapshot is known to be out of step with the
    // balances (see resync) until a fresh one lands. Readers that add locked
    // amounts to a spendable balance must skip the snapshot while it is stale.
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
      this.info = null
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
    // A ZNN or QSR balance moved, so locked amounts may have moved the other
    // way. Mark the snapshot stale at once, then reload; only the newest
    // request lands, and landing is what clears the mark.
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
        const info = await Nom.GetCollateral()
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.info = info
        this.status = 'ready'
        this.stale = false
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._request) return
        this.info = null
        this.status = 'error'
        this.stale = false
      } finally {
        if (this._active === request) this._active = 0
      }
    },
  },
})
