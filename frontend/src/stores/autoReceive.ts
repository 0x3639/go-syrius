import { defineStore } from 'pinia'
import * as Cfg from '../../wailsjs/go/app/ConfigService'
import * as N from '../../wailsjs/go/app/NodeService'
import { EventsOn } from '../../wailsjs/runtime/runtime'

// Auto-receive engine. The backend subscribes + sweeps for ONE active account at
// a time. Kept in a store (not in a view) so the top-bar toggle works from any
// screen, while AppShell drives init + account-following. `receiving` reflects
// the backend actively claiming blocks (PoW/plasma generation) for a progress UI.
export const useAutoReceiveStore = defineStore('autoReceive', {
  state: () => ({
    enabled: false,
    account: -1,
    receiving: false,
    wired: false,
    // Last auto-receive failure message + a counter that bumps on each error, so
    // the UI can surface every occurrence (even repeats of the same message).
    lastError: '',
    errorCount: 0,
  }),
  actions: {
    // Wire the backend's auto-receive events (once per store instance).
    wireEvents() {
      if (this.wired) return
      this.wired = true
      try {
        EventsOn('auto-receive:active', (active: boolean) => {
          this.receiving = !!active
        })
        // Auto-receive runs in the background with no Confirm dialog, so a
        // failure is otherwise invisible — record it for the UI to surface. The
        // backend emits { hash, error }; tolerate a bare string too.
        EventsOn('auto-receive:error', (payload: { hash?: string; error?: string } | string) => {
          const msg = typeof payload === 'string' ? payload : payload?.error
          this.lastError = msg && msg.length > 0 ? msg : 'Auto-receive failed'
          this.errorCount++
        })
      } catch {
        /* runtime unavailable (tests/offline) */
      }
    },
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
      this.wireEvents()
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
        await Cfg.SetAutoReceive(this.enabled)
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
