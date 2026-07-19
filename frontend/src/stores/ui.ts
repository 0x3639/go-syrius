import { defineStore } from 'pinia'
import * as Cfg from '../../wailsjs/go/app/ConfigService'
import { useNodeStore } from './node'

// UI preferences persisted in app settings. Currently just the opt-in for the
// experimental, testnet-only Governance navigation tab (off by default).
export const useUiStore = defineStore('ui', {
  state: () => ({
    showGovernance: false,
    // TEMPORARY kill switch mirror (ConfigService.IsGovernanceFeatureEnabled):
    // governance is fully disabled pending an SDK update. Fails CLOSED.
    governanceFeatureEnabled: false,
    theme: 'dark' as 'dark' | 'light',
    // Whether the logo intro animation plays on launch. On by default; users can
    // turn it off in Settings. Frontend-only preference, persisted to localStorage
    // (App.vue reads the key directly at startup, before any store init runs).
    splashEnabled: true,
  }),
  getters: {
    // SINGLE source of truth for the TESTNET-ONLY Governance gate, consumed by
    // both the Sidebar (tab) and NetworkPage (panel) so the two can never
    // drift. Fails CLOSED: chainId 0 = not known yet (pre-connect), 1 =
    // mainnet; only a confirmed testnet (> 1) allows Governance.
    governanceAllowed(): boolean {
      return this.governanceFeatureEnabled && this.showGovernance && useNodeStore().chainId > 1
    },
  },
  actions: {
    // NOTE: this store owns the theme PREFERENCE (persisted as syrius.theme).
    // The DOM (dark class) is applied in exactly one place: App.vue watches
    // ui.theme and forwards it to nom-ui's setTheme — no second toggler here.
    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark'
      try { localStorage.setItem('syrius.theme', this.theme) } catch { /* ignore */ }
    },
    setSplashEnabled(v: boolean) {
      this.splashEnabled = v
      try { localStorage.setItem('syrius.splash', v ? '1' : '0') } catch { /* ignore */ }
    },
    // Restore the persisted theme. Sync and store-only, so main.ts can call it
    // before mount; App.vue's immediate theme watch then applies it during App
    // setup — still ahead of the first paint, including the locked Unlock/
    // Create/Import screens, which never mount AppShell.
    initTheme() {
      try { const t = localStorage.getItem('syrius.theme'); if (t === 'light' || t === 'dark') this.theme = t } catch { /* ignore */ }
    },
    async init() {
      this.initTheme()
      try { this.splashEnabled = localStorage.getItem('syrius.splash') !== '0' } catch { /* ignore */ }
      try {
        this.showGovernance = (await Cfg.GetSettings()).showGovernance ?? false
      } catch {
        /* offline/locked — keep the default */
      }
      try {
        this.governanceFeatureEnabled = (await Cfg.IsGovernanceFeatureEnabled()) === true
      } catch {
        /* keep false — fail closed */
      }
    },
    async setShowGovernance(v: boolean) {
      this.showGovernance = v
      try {
        await Cfg.SetShowGovernance(v)
      } catch {
        /* best-effort persist; the in-memory flag still updates the nav */
      }
    },
  },
})
