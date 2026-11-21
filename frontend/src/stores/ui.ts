import { defineStore } from 'pinia'
import { GetSettings, SetPinnedActions, SetUIMode } from '../../wailsjs/go/app/ConfigService'

// Layout mode. Everyday keeps operator tools (pillars, sentinels, token
// issuance, Accelerator) out of the navigation; Advanced shows everything.
// Layout only: it never changes what the backend permits.
export type UiMode = 'everyday' | 'advanced'
const isMode = (v: unknown): v is UiMode => v === 'everyday' || v === 'advanced'

// Shortcuts that can be pinned on the Advanced overview. The ids are the
// backend allowlist (ConfigService.SetPinnedActions); keep the two in sync.
export const PINNABLE = {
  fuse: { label: 'Fuse QSR', to: '/network/plasma' },
  collect: { label: 'Collect rewards', to: '/network/rewards' },
  receive: { label: 'Incoming payments', to: '/receive' },
  vote: { label: 'Vote on projects', to: '/network/accelerator?sub=Vote' },
  send: { label: 'Send', to: '/transfer' },
  stake: { label: 'Stake ZNN', to: '/network/staking' },
  tokens: { label: 'Manage tokens', to: '/tokens' },
  walletconnect: { label: 'WalletConnect', to: '/walletconnect' },
  node: { label: 'Node settings', to: '/settings' },
} as const
export type PinId = keyof typeof PINNABLE
export const MAX_PINS = 6

// UI preferences persisted locally.
export const useUiStore = defineStore('ui', {
  state: () => ({
    theme: 'dark' as 'dark' | 'light',
    // Whether the logo intro animation plays on launch. On by default; users can
    // turn it off in Settings. Frontend-only preference, persisted to localStorage
    // (App.vue reads the key directly at startup, before any store init runs).
    splashEnabled: true,
    // Until the saved preference loads, keep the full (Advanced) layout: it is
    // what every install had before modes existed. The last known mode is
    // cached in localStorage so later launches paint the right layout at once.
    mode: 'advanced' as UiMode,
    pins: ['fuse', 'collect', 'receive'] as PinId[],
    // One-shot message for the shell to toast after an automatic mode change.
    modeNotice: '',
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
    // Sync restore of the cached mode (see `mode` above).
    initMode() {
      try { const m = localStorage.getItem('syrius.mode'); if (isMode(m)) this.mode = m } catch { /* ignore */ }
    },
    // Authoritative load from settings.json. A failure keeps the current mode.
    async loadPrefs() {
      try {
        const s = await GetSettings()
        if (isMode(s?.uiMode)) {
          this.mode = s.uiMode
          try { localStorage.setItem('syrius.mode', s.uiMode) } catch { /* ignore */ }
        }
        if (Array.isArray(s?.pinnedActions)) this.pins = s.pinnedActions.filter((p): p is PinId => p in PINNABLE)
      } catch { /* keep current */ }
    },
    // The UI switches at once; persistence is best-effort (a failed save only
    // means the previous mode returns on next launch).
    setMode(mode: UiMode, notice = '') {
      if (this.mode === mode) return
      this.mode = mode
      this.modeNotice = notice
      try { localStorage.setItem('syrius.mode', mode) } catch { /* ignore */ }
      try { SetUIMode(mode)?.catch?.(() => {}) } catch { /* ignore */ }
    },
    clearModeNotice() { this.modeNotice = '' },
    togglePin(id: PinId) {
      const next = this.pins.includes(id) ? this.pins.filter((p) => p !== id) : [...this.pins, id]
      if (next.length > MAX_PINS) return
      this.pins = next
      try { SetPinnedActions(next)?.catch?.(() => {}) } catch { /* ignore */ }
    },
    init() {
      this.initTheme()
      this.initMode()
      this.loadPrefs()
      try { this.splashEnabled = localStorage.getItem('syrius.splash') !== '0' } catch { /* ignore */ }
    },
  },
})
