import { defineStore } from 'pinia'
import * as Cfg from '../../wailsjs/go/app/ConfigService'

// UI preferences persisted in app settings. Currently just the opt-in for the
// experimental, testnet-only Governance navigation tab (off by default).
export const useUiStore = defineStore('ui', {
  state: () => ({
    showGovernance: false,
    theme: 'dark' as 'dark' | 'light',
    // Whether the logo intro animation plays on launch. On by default; users can
    // turn it off in Settings. Frontend-only preference, persisted to localStorage
    // (App.vue reads the key directly at startup, before any store init runs).
    splashEnabled: true,
  }),
  actions: {
    applyTheme() {
      document.documentElement.classList.toggle('dark', this.theme === 'dark')
    },
    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark'
      this.applyTheme()
      try { localStorage.setItem('syrius.theme', this.theme) } catch { /* ignore */ }
    },
    setSplashEnabled(v: boolean) {
      this.splashEnabled = v
      try { localStorage.setItem('syrius.splash', v ? '1' : '0') } catch { /* ignore */ }
    },
    async init() {
      try { const t = localStorage.getItem('syrius.theme'); if (t === 'light' || t === 'dark') this.theme = t } catch { /* ignore */ }
      try { this.splashEnabled = localStorage.getItem('syrius.splash') !== '0' } catch { /* ignore */ }
      this.applyTheme()
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
