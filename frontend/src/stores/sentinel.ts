import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import { useNodeStore } from './node'
import type { app } from '../../wailsjs/go/models'

// 50,000 QSR in base units (1e8) — the Sentinel QSR collateral.
export const QSR_REQUIRED = 5000000000000n
const POLL_INTERVAL_MS = 3000

export const useSentinelStore = defineStore('sentinel', {
  state: () => ({
    _epoch: -1,
    _request: 0,
    sentinel: null as app.SentinelInfo | null,
    depositedQsr: '0',
    reward: null as app.RewardInfo | null,
    // Transient "clearing" flag: a just-published step we're polling to settle.
    pendingStep: null as 'deposit' | 'register' | null,
    pollCount: 0,
    _pollRequest: 0,
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
    invalidate() {
      this._epoch = currentRequestEpoch()
      this._request++
      this.sentinel = null
      this.depositedQsr = '0'
      this.reward = null
    },
    async refresh() {
      const epoch = currentRequestEpoch()
      if (this._epoch !== epoch) this.invalidate()
      const request = ++this._request
      try {
        const [sentinel, depositedQsr, reward] = await Promise.all([
          Nom.GetSentinel(), Nom.GetDepositedQsr(), Nom.GetSentinelReward(),
        ])
        if (epoch !== currentRequestEpoch() || request !== this._request) return
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
      const request = this._pollRequest
      let pollingEpoch: number | null = null
      const handle = window.setInterval(async () => {
        const epoch = currentRequestEpoch()
        if (!useNodeStore().connected || pollingEpoch === epoch) return
        pollingEpoch = epoch
        try {
          await this.refresh()
          if (!useNodeStore().connected || epoch !== currentRequestEpoch() || this._pollRequest !== request) return
          this.pollCount++
          this.settleCheck()
        } finally { if (pollingEpoch === epoch) pollingEpoch = null }
      }, POLL_INTERVAL_MS)
      this.pollHandle = handle
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
