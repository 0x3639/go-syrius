import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

// A pillar Register block costs ~105,000 plasma (2 * EmbeddedSimple). We gate on
// this and recommend fusing 500 QSR (~1,050,000 plasma) for a comfortable buffer.
export const PILLAR_PLASMA_REQUIRED = 105000n
export const FUSE_RECOMMENDED_QSR = '500'
const POLL_INTERVAL_MS = 3000

export const usePillarStore = defineStore('pillar', {
  state: () => ({
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
    async refreshDelegation() {
      const epoch = currentRequestEpoch()
      try {
        const delegation = await Nom.GetDelegation()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.delegation = delegation
      } catch { /* not connected / locked — leave as-is */ }
    },
    async refresh() {
      const epoch = currentRequestEpoch()
      try {
        const pillars = await Nom.GetPillarList()
        const delegation = await Nom.GetDelegation()
        const reward = await Nom.GetPillarReward()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.pillars = pillars
        this.delegation = delegation
        this.reward = reward
      } catch { /* not connected / locked — leave as-is */ }
    },
    // Refresh the registration view's chain state (owned pillar, deposit, cost,
    // plasma, reward).
    async refreshRegistration() {
      const epoch = currentRequestEpoch()
      try {
        const myPillar = await Nom.GetMyPillar()
        const depositedQsr = await Nom.GetPillarDepositedQsr()
        const qsrCost = await Nom.GetPillarQsrCost()
        const plasma = await Nom.GetPlasmaInfo()
        const reward = await Nom.GetPillarReward()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.myPillar = myPillar
        this.depositedQsr = depositedQsr
        this.qsrCost = qsrCost
        this.plasma = plasma
        this.reward = reward
      } catch { /* not connected / locked — leave as-is */ }
    },
    // Lightweight owned-pillar refresh for the always-visible status strip — just
    // the owned pillar (1 RPC), independent of which tab is open. The pillar is
    // owned per-address, so this must follow account switches.
    async refreshMyPillar() {
      const epoch = currentRequestEpoch()
      try {
        const myPillar = await Nom.GetMyPillar()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.myPillar = myPillar
      } catch { /* not connected / locked — leave as-is */ }
    },
    // Start polling for a just-published step to settle on-chain, then advance.
    beginPending(step: 'plasma' | 'deposit' | 'register') {
      this.stopPolling()
      this.pendingStep = step
      this.pollCount = 0
      this.pollHandle = window.setInterval(async () => {
        this.pollCount++
        await this.refreshRegistration()
        this.settleCheck()
      }, POLL_INTERVAL_MS)
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
      if (this.pollHandle !== null) {
        clearInterval(this.pollHandle)
        this.pollHandle = null
      }
      this.pendingStep = null
      this.pollCount = 0
    },
  },
})
