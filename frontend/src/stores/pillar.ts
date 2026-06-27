import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
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
    plasmaCleared(s): boolean {
      try {
        return BigInt(s.plasma?.currentPlasma ?? 0) >= PILLAR_PLASMA_REQUIRED
      } catch {
        return false
      }
    },
  },
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
    // Refresh the registration view's chain state (owned pillar, deposit, cost,
    // plasma, reward).
    async refreshRegistration() {
      try {
        this.myPillar = await Nom.GetMyPillar()
        this.depositedQsr = await Nom.GetPillarDepositedQsr()
        this.qsrCost = await Nom.GetPillarQsrCost()
        this.plasma = await Nom.GetPlasmaInfo()
        this.reward = await Nom.GetPillarReward()
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
