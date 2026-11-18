import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import { useNodeStore } from './node'
import type { app } from '../../wailsjs/go/models'

// A pillar Register block costs ~105,000 plasma (2 * EmbeddedSimple). We gate on
// this and recommend fusing 500 QSR (~1,050,000 plasma) for a comfortable buffer.
export const PILLAR_PLASMA_REQUIRED = 105000n
export const FUSE_RECOMMENDED_QSR = '500'
const POLL_INTERVAL_MS = 3000

export const usePillarStore = defineStore('pillar', {
  state: () => ({
    _epoch: -1,
    _requests: { pillars: 0, delegation: 0, reward: 0, registration: 0, myPillar: 0 },
    // delegation (existing)
    delegation: null as app.DelegationInfo | null,
    pillars: [] as app.PillarSummary[],
    reward: null as app.RewardInfo | null,
    // registration
    myPillar: null as app.OwnedPillarInfo | null,
    depositedQsr: '0',
    qsrCost: '0',
    plasma: null as app.PlasmaInfo | null,
    pendingStep: null as 'plasma' | 'deposit' | 'register' | null,
    pollCount: 0,
    _pollRequest: 0,
    pollHandle: null as number | null,
  }),
  getters: {
    ownsPillar(s): boolean {
      return !!s.myPillar && s.myPillar.name !== ''
    },
    qsrCleared(s): boolean {
      try {
        const cost = BigInt(s.qsrCost || '0')
        return cost > 0n && BigInt(s.depositedQsr || '0') >= cost
      } catch {
        return false
      }
    },
    // Plasma is sufficient if the address has enough available plasma right now,
    // OR enough fused-plasma capacity (maxPlasma) — current plasma dips after each
    // tx and regenerates, so an already-fused address must not be blocked by a
    // momentary low reading.
    plasmaCleared(s): boolean {
      try {
        const current = BigInt(s.plasma?.currentPlasma ?? 0)
        const max = BigInt(s.plasma?.maxPlasma ?? 0)
        return current >= PILLAR_PLASMA_REQUIRED || max >= PILLAR_PLASMA_REQUIRED
      } catch {
        return false
      }
    },
  },
  actions: {
    invalidate() {
      this._epoch = currentRequestEpoch()
      for (const key of Object.keys(this._requests) as (keyof typeof this._requests)[]) this._requests[key]++
      this.pillars = []
      this.delegation = null
      this.reward = null
      this.myPillar = null
      this.depositedQsr = '0'
      this.qsrCost = '0'
      this.plasma = null
    },
    ensureScope() {
      if (this._epoch !== currentRequestEpoch()) this.invalidate()
      return currentRequestEpoch()
    },
    async refreshDelegation() {
      const epoch = this.ensureScope()
      const request = ++this._requests.delegation
      try {
        const delegation = await Nom.GetDelegation()
        if (epoch === currentRequestEpoch() && request === this._requests.delegation) this.delegation = delegation
      } catch { /* retain only this scope's previous snapshot */ }
    },
    async refresh() {
      const epoch = this.ensureScope()
      const listRequest = ++this._requests.pillars
      const delegationRequest = ++this._requests.delegation
      const rewardRequest = ++this._requests.reward
      try {
        const [pillars, delegation, reward] = await Promise.all([
          Nom.GetPillarList(), Nom.GetDelegation(), Nom.GetPillarReward(),
        ])
        if (epoch !== currentRequestEpoch()) return
        if (listRequest === this._requests.pillars) this.pillars = pillars
        if (delegationRequest === this._requests.delegation) this.delegation = delegation
        if (rewardRequest === this._requests.reward) this.reward = reward
      } catch { /* retain only this scope's previous snapshot */ }
    },
    async refreshRegistration() {
      const epoch = this.ensureScope()
      const request = ++this._requests.registration
      const pillarRequest = ++this._requests.myPillar
      const rewardRequest = ++this._requests.reward
      try {
        const [myPillar, depositedQsr, qsrCost, plasma, reward] = await Promise.all([
          Nom.GetMyPillar(), Nom.GetPillarDepositedQsr(), Nom.GetPillarQsrCost(), Nom.GetPlasmaInfo(), Nom.GetPillarReward(),
        ])
        if (epoch !== currentRequestEpoch()) return
        if (pillarRequest === this._requests.myPillar) this.myPillar = myPillar
        if (rewardRequest === this._requests.reward) this.reward = reward
        if (request === this._requests.registration) {
          this.depositedQsr = depositedQsr
          this.qsrCost = qsrCost
          this.plasma = plasma
        }
      } catch { /* retain only this scope's previous snapshot */ }
    },
    // The always-visible status strip needs only the account's owned pillar.
    async refreshMyPillar() {
      const epoch = this.ensureScope()
      const request = ++this._requests.myPillar
      try {
        const myPillar = await Nom.GetMyPillar()
        if (epoch === currentRequestEpoch() && request === this._requests.myPillar) this.myPillar = myPillar
      } catch { /* retain only this scope's previous snapshot */ }
    },
    // Start polling for a just-published step to settle on-chain, then advance.
    beginPending(step: 'plasma' | 'deposit' | 'register') {
      this.stopPolling()
      this.pendingStep = step
      this.pollCount = 0
      const request = this._pollRequest
      let pollingEpoch: number | null = null
      const handle = window.setInterval(async () => {
        const epoch = currentRequestEpoch()
        if (!useNodeStore().connected || pollingEpoch === epoch) return
        pollingEpoch = epoch
        try {
          await this.refreshRegistration()
          if (!useNodeStore().connected || epoch !== currentRequestEpoch() || this._pollRequest !== request) return
          this.pollCount++
          this.settleCheck()
        } finally { if (pollingEpoch === epoch) pollingEpoch = null }
      }, POLL_INTERVAL_MS)
      this.pollHandle = handle
    },
    // Clear the pending state once the chain reflects the step.
    settleCheck() {
      if (this.pendingStep === 'plasma' && this.plasmaCleared) {
        this.stopPolling()
      } else if (this.pendingStep === 'deposit' && this.qsrCleared) {
        this.stopPolling()
      } else if (this.pendingStep === 'register' && this.ownsPillar) {
        this.stopPolling()
      }
    },
    // Stop polling and clear the pending state (settle, unmount, or cancel).
    stopPolling() {
      this._pollRequest++
      if (this.pollHandle !== null) {
        clearInterval(this.pollHandle)
        this.pollHandle = null
      }
      this.pendingStep = null
      this.pollCount = 0
    },
  },
})
