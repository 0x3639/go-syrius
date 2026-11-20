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
    _preferenceTail: null as Promise<void> | null,
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
    // Preferences have their own queue: a slow subscription must never prevent
    // saving an off toggle. Unlock waits for these writes before reading settings.
    async savePreference(enabled: boolean) {
      const run = (this._preferenceTail ?? Promise.resolve()).catch(() => {}).then(() => Cfg.SetAutoReceive(enabled))
      this._preferenceTail = run
      try {
        await run
      } finally {
        if (this._preferenceTail === run) this._preferenceTail = null
      }
    },
    // Interrupt setup immediately, then serialize completion/restart so older
    // starts cannot reclaim the worker or overwrite the latest account choice.
    async updateWorker(activeIndex: number | null) {
      const request = ++this._request
      this.account = -1
      const previous = this._controlTail
      const stopped = N.StopAutoReceive()
      const saved = this._preferenceTail ?? Promise.resolve()
      const run = Promise.allSettled([previous, stopped]).then(async ([, stopResult]) => {
        if (stopResult.status === 'rejected') throw stopResult.reason
        if (request !== this._request) return
        // Stop again after an older binding settles: it may have reached Go
        // after our interrupt. No older stop can land after the next start.
        if (previous) await N.StopAutoReceive()
        if (activeIndex === null) return
        await saved
        if (request !== this._request || !this.enabled) return
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
        await this._preferenceTail
        if (preferenceRequest !== this._preferenceRequest) return
        const settings = await Cfg.GetSettings()
        if (preferenceRequest !== this._preferenceRequest) return
        this.enabled = settings.autoReceive
        if (this.enabled) await this.start(this._targetAccount)
        else await this.stop()
      } catch {
        /* offline/locked — leave as-is */
      }
    },
    async toggle(activeIndex: number) {
      this._preferenceRequest++
      this._targetAccount = activeIndex
      this.enabled = !this.enabled
      const saved = this.savePreference(this.enabled)
      try {
        await Promise.all([saved, this.updateWorker(this.enabled ? activeIndex : null)])
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
