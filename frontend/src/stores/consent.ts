import { defineStore } from 'pinia'
import { GetSettings } from '../../wailsjs/go/app/ConfigService'
import { useNodeStore } from './node'
import { MAINNET_CHAIN_ID } from '../lib/network'

// The frontend's view of the mainnet opt-in, used only to EXPLAIN why nothing
// can be signed and where to change that. It grants nothing: TxService checks
// the persisted setting itself when a transaction is prepared and again
// immediately before broadcast, whatever this store says.
export const useConsentStore = defineStore('consent', {
  state: () => ({
    // null until the setting has been read.
    mainnetAllowed: null as boolean | null,
    // Where the user was when they went to review the setting, so Settings can
    // offer the way back. A route path; '' when there is nowhere to return to.
    returnTo: '',
    // Bumped on every explicit write, so a read that started earlier cannot
    // replace the newer value when it resolves.
    _writes: 0,
  }),
  getters: {
    // Connected to mainnet with the opt-in known to be off: every send,
    // receive and contract call will be refused by the backend.
    blocked: (s): boolean => {
      const node = useNodeStore()
      return node.connected && node.chainId === MAINNET_CHAIN_ID && s.mainnetAllowed === false
    },
  },
  actions: {
    async load() {
      const writes = this._writes
      try {
        const allowed = !!(await GetSettings())?.allowMainnetSend
        // Settings changed the opt-in while this read was in flight: the read
        // is older than what the backend just accepted, so drop it.
        if (writes === this._writes) this.mainnetAllowed = allowed
      } catch { /* keep the last known value */ }
    },
    // Called by Settings after the backend has accepted a change.
    setAllowed(v: boolean) { this._writes++; this.mainnetAllowed = v },
    leaveFrom(path: string) { this.returnTo = path },
    clearReturn() { this.returnTo = '' },
  },
})
