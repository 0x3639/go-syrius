import { defineStore } from 'pinia'
import * as Cfg from '../../wailsjs/go/app/ConfigService'
import * as N from '../../wailsjs/go/app/NodeService'

// Auto-receive engine. The backend subscribes + sweeps for ONE active account at
// a time. Kept in a store (not in Home) so the top-bar toggle works from any
// screen, while Home drives init + account-following.
export const useAutoReceiveStore = defineStore('autoReceive', {
  state: () => ({ enabled: false, account: -1 }),
  actions: {
    // start() always stops first so it re-points at the CURRENT active account —
    // StartAutoReceive alone early-returns "already running" on the old one.
    async start(activeIndex: number) {
      await N.StopAutoReceive()
      await N.StartAutoReceive()
      this.account = activeIndex
    },
    async stop() {
      await N.StopAutoReceive()
      this.account = -1
    },
    // Load the persisted flag + resume (the subscription doesn't survive a
    // restart). Skips a redundant restart when already on this account.
    async init(activeIndex: number) {
      try {
        this.enabled = (await Cfg.GetSettings()).autoReceive
        if (this.enabled && this.account !== activeIndex) await this.start(activeIndex)
      } catch {
        /* offline/locked — leave as-is */
      }
    },
    async toggle(activeIndex: number) {
      this.enabled = !this.enabled
      try {
        const s = await Cfg.GetSettings()
        s.autoReceive = this.enabled
        await Cfg.SetSettings(s)
        if (this.enabled) await this.start(activeIndex)
        else await this.stop()
      } catch {
        /* ignore */
      }
    },
    // Follow account switches: re-sweep + re-subscribe for the new account.
    async followAccount(activeIndex: number) {
      if (this.enabled && activeIndex !== this.account) await this.start(activeIndex)
    },
  },
})
