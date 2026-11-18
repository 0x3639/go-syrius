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
    _request: 0,
    _preferenceRequest: 0,
    _targetAccount: -1,
    _controlTail: null as Promise<void> | null,
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
          // Error text can embed node-supplied content (rendered as text by the
          // toast — Vue escapes — but still untrusted): bound its size so a
          // malicious node cannot shove megabytes into UI state.
          const msg = (typeof payload === 'string' ? payload : payload?.error)?.slice(0, 300)
          this.lastError = msg && msg.length > 0 ? msg : 'Auto-receive failed'
          this.errorCount++
        })
      } catch {
        /* runtime unavailable (tests/offline) */
      }
    },
    // Serialize backend control calls. A newer intent supersedes queued work
    // and stale completions, while an already-dispatched start finishes before
    // the following stop/restart. Preferences persist in the same order.
    async updateWorker(activeIndex: number | null, persist?: boolean) {
      const request = ++this._request
      this.account = -1
      const run = (this._controlTail ?? Promise.resolve()).catch(() => {}).then(async () => {
        if (persist !== undefined) await Cfg.SetAutoReceive(persist)
        if (request !== this._request) return
        await N.StopAutoReceive()
        // The user may turn it off or switch accounts while stopping.
        if (request !== this._request || activeIndex === null || !this.enabled) return
        await N.StartAutoReceive()
        if (request === this._request && this.enabled) this.account = activeIndex
      })
      this._controlTail = run
      try {
        await run
      } finally {
        if (this._controlTail === run) this._controlTail = null
      }
    },
    // Start always stops first: the backend otherwise keeps the old account's
    // worker. Clear the recorded account immediately so a switch back to it
    // cannot skip a restart while another account's start is pending.
    start(activeIndex: number) {
      this._targetAccount = activeIndex
      return this.updateWorker(activeIndex)
    },
    stop() {
      return this.updateWorker(null)
    },
    // Load the persisted flag + resume (the subscription doesn't survive a
    // restart). Re-arm on every unlock even for the same account: the node
    // connection can recover while AppShell is unmounted and stop the old worker.
    async init(activeIndex: number) {
      this.wireEvents()
      const preferenceRequest = ++this._preferenceRequest
      this._targetAccount = activeIndex
      try {
        const settings = await Cfg.GetSettings()
        if (preferenceRequest !== this._preferenceRequest) return
        this.enabled = settings.autoReceive
        if (this.enabled) await this.start(this._targetAccount)
      } catch {
        /* offline/locked — leave as-is */
      }
    },
    async toggle(activeIndex: number) {
      this._preferenceRequest++
      this._targetAccount = activeIndex
      this.enabled = !this.enabled
      try {
        await this.updateWorker(this.enabled ? activeIndex : null, this.enabled)
      } catch {
        /* ignore */
      }
    },
    // Follow account switches: re-sweep + re-subscribe for the new account.
    async followAccount(activeIndex: number) {
      this._targetAccount = activeIndex
      if (this.enabled && activeIndex !== this.account) await this.start(activeIndex)
    },
  },
})
