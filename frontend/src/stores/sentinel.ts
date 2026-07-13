import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

// 50,000 QSR in base units (1e8) — the Sentinel QSR collateral.
export const QSR_REQUIRED = 5000000000000n
const POLL_INTERVAL_MS = 3000

export const useSentinelStore = defineStore('sentinel', {
  state: () => ({
    sentinel: null as app.SentinelInfo | null,
    depositedQsr: '0',
    reward: null as app.RewardInfo | null,
    // Transient "clearing" flag: a just-published step we're polling to settle.
    pendingStep: null as 'deposit' | 'register' | null,
    pollCount: 0,
    pollHandle: null as number | null,
  }),
  getters: {
    active(s): boolean {
      return !!s.sentinel && s.sentinel.owner !== ''
    },
    qsrCleared(s): boolean {
      try {
        return BigInt(s.depositedQsr || '0') >= QSR_REQUIRED
      } catch {
        return false
      }
    },
  },
  actions: {
    async refresh() {
      const epoch = currentRequestEpoch()
      try {
        const sentinel = await Nom.GetSentinel()
        const depositedQsr = await Nom.GetDepositedQsr()
        const reward = await Nom.GetSentinelReward()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.sentinel = sentinel
        this.depositedQsr = depositedQsr
        this.reward = reward
      } catch {
        /* not connected / locked — leave as-is */
      }
    },
    // Start polling for a just-published step to settle on-chain, then advance.
    beginPending(step: 'deposit' | 'register') {
      this.stopPolling()
      this.pendingStep = step
      this.pollCount = 0
      this.pollHandle = window.setInterval(async () => {
        this.pollCount++
        await this.refresh()
        this.settleCheck()
      }, POLL_INTERVAL_MS)
    },
    // Clear the pending state once the chain reflects the step.
    settleCheck() {
      if (this.pendingStep === 'deposit' && this.qsrCleared) {
        this.stopPolling()
      } else if (this.pendingStep === 'register' && this.active) {
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
