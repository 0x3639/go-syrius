import { defineStore } from 'pinia'
import * as W from '../../wailsjs/go/app/WalletService'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { bumpRequestEpoch } from '../lib/requestEpoch'

export type AccountInfo = { index: number; address: string; label: string }
// WalletMeta mirrors the Go app.WalletMeta returned by ListWallets. `id` is the
// keystore filename (the stable identifier passed to Unlock/RenameWallet); `name`
// is the user-facing display name; `baseAddress` is the wallet's account-0 address.
export type WalletMeta = { id: string; name: string; baseAddress: string }

// Serializes account selection: the backend applies selections one at a time,
// so the frontend must not fire overlapping calls (their backend order would
// be a mutex race, not user intent). While one select is in flight the latest
// requested index is queued and applied after — last intent wins.
let selecting = false
let queuedIndex: number | null = null

// Identifies the wallet session. Bumped on every unlock/lock; a selection
// response that resolves after the wallet changed belongs to the PREVIOUS
// wallet and must not be committed (nor may its queued follow-ups run).
let walletSession = 0

// wallet:locked listener bookkeeping (module-level, like `selecting` above).
let lockEventInit = false

export const useWalletStore = defineStore('wallet', {
  // `active` holds the active wallet's id (keystore filename), not its name.
  state: () => ({ locked: true, wallets: [] as WalletMeta[], active: '', accounts: [] as AccountInfo[], activeIndex: 0 }),
  actions: {
    async loadWallets() {
      try {
        const list = (await W.ListWallets()) as unknown as WalletMeta[]
        this.wallets = list.map((w) => ({ id: w.id, name: w.name, baseAddress: w.baseAddress }))
        if (!this.active && this.wallets.length) this.active = this.wallets[0].id
      } catch { this.wallets = [] }
    },
    async unlock(id: string, password: string) {
      await W.Unlock(id, password)
      walletSession++ // selections in flight belong to the previous wallet
      queuedIndex = null
      bumpRequestEpoch() // a new session: discard responses from the old one
      this.active = id
      this.locked = false
      await this.loadAccounts()
      this.activeIndex = 0
    },
    // Rename a wallet (no password required) and refresh the list so the new name
    // shows everywhere. `id` is the keystore filename.
    async rename(id: string, name: string): Promise<void> {
      await W.RenameWallet(id, name)
      await this.loadWallets()
    },
    lock() {
      // Re-lock the keystore in the Go backend, not just the UI — otherwise the
      // wallet shows locked while the backend keystore stays decrypted.
      W.Lock().catch(() => {})
      this._applyLocked()
    },
    // Local session teardown shared by manual lock() and the backend-initiated
    // wallet:locked event (auto-lock watchdog).
    _applyLocked() {
      walletSession++
      queuedIndex = null
      bumpRequestEpoch()
      this.locked = true
      this.active = ''
      this.accounts = []
      this.activeIndex = 0
    },
    // Wires the backend-initiated lock (auto-lock watchdog). Registered once.
    // Navigation is owned by App.vue's `wallet.locked` watcher (App.vue:34-39),
    // which pushes /unlock whenever `locked` flips — so this listener's only job
    // is local session teardown for backend-initiated locks. Idempotent: Go's
    // Lock() also emits wallet:locked on manual lock, when the store is already
    // locked — that must be a no-op.
    initLockEvent() {
      if (lockEventInit) return
      lockEventInit = true
      EventsOn('wallet:locked', () => {
        if (this.locked) return
        this._applyLocked()
      })
    },
    async loadAccounts() {
      try { this.accounts = (await W.CurrentAccounts()) as unknown as AccountInfo[] } catch { this.accounts = [] }
    },
    async select(index: number) {
      if (selecting) {
        queuedIndex = index // supersede: only the latest intent is applied next
        return
      }
      selecting = true
      try {
        let next: number | null = index
        while (next !== null) {
          const target = next
          next = null
          const session = walletSession
          // The backend returns the AUTHORITATIVE selection; render that, not
          // the assumption that the requested index won.
          const info = (await W.SelectAccount(target)) as unknown as AccountInfo | undefined
          if (session !== walletSession) return // wallet changed mid-selection: stale
          // The backend just switched accounts: any account-scoped response
          // still in flight belongs to the previous account.
          bumpRequestEpoch()
          this.activeIndex = info?.index ?? target
          if (queuedIndex !== null) {
            next = queuedIndex
            queuedIndex = null
          }
        }
      } finally {
        selecting = false
        queuedIndex = null
      }
    },
    async setLabel(index: number, label: string) {
      await W.SetAccountLabel(index, label)
      await this.loadAccounts()
    },
    // Reveal one more account (derivation index) and refresh the list.
    async addAccount() {
      this.accounts = (await W.AddAccount()) as unknown as AccountInfo[]
    },
    activeAddress(): string {
      return this.accounts.find((a) => a.index === this.activeIndex)?.address ?? ''
    },
    async generateMnemonic(): Promise<string> {
      return await W.GenerateMnemonic()
    },
    // Persist a new keystore from a mnemonic and return the new wallet's meta
    // (the backend assigns the uuid keystore filename as `id`). Does NOT unlock —
    // the caller unlocks afterward by `meta.id` (mirrors the create/import flow).
    // `name` is the user-facing display name (no `.dat`). Throws on error.
    async importMnemonic(name: string, password: string, mnemonic: string): Promise<WalletMeta> {
      const meta = (await W.ImportMnemonic(name, password, mnemonic)) as unknown as WalletMeta
      await this.loadWallets()
      return meta
    },
    // Import an existing keystore file; wallet stays locked (user then unlocks).
    // `name` defaults to '' — the backend derives a name from the file when empty.
    async importKeystore(srcPath: string, name = ''): Promise<WalletMeta> {
      const meta = (await W.ImportKeystore(srcPath, name)) as unknown as WalletMeta
      await this.loadWallets()
      return meta
    },
    async pickKeystoreFile(): Promise<string> {
      return (await W.PickKeystoreFile()) || ''
    },
    async changePassword(oldPw: string, newPw: string): Promise<void> {
      await W.ChangePassword(this.active, oldPw, newPw)
    },
    async revealMnemonic(password: string): Promise<string> {
      return await W.RevealMnemonic(password)
    },
  },
})
