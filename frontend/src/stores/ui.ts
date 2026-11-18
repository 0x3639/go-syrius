import { defineStore } from 'pinia'

// UI preferences persisted locally.
export const useUiStore = defineStore('ui', {
  state: () => ({
    theme: 'dark' as 'dark' | 'light',
    // Whether the logo intro animation plays on launch. On by default; users can
    // turn it off in Settings. Frontend-only preference, persisted to localStorage
    // (App.vue reads the key directly at startup, before any store init runs).
    splashEnabled: true,
  }),
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
    init() {
      this.initTheme()
      try { this.splashEnabled = localStorage.getItem('syrius.splash') !== '0' } catch { /* ignore */ }
    },
  },
})
