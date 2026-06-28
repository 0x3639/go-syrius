import { defineStore } from 'pinia'
import * as Cfg from '../../wailsjs/go/app/ConfigService'

// UI preferences persisted in app settings. Currently just the opt-in for the
// experimental, testnet-only Governance navigation tab (off by default).
export const useUiStore = defineStore('ui', {
  state: () => ({
    showGovernance: false,
  }),
  actions: {
    async init() {
      try {
        this.showGovernance = (await Cfg.GetSettings()).showGovernance ?? false
      } catch {
        /* offline/locked — keep the default */
      }
    },
    async setShowGovernance(v: boolean) {
      this.showGovernance = v
      try {
        const s = await Cfg.GetSettings()
        s.showGovernance = v
        await Cfg.SetSettings(s)
      } catch {
        /* best-effort persist; the in-memory flag still updates the nav */
      }
    },
  },
})
