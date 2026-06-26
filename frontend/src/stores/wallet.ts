import { defineStore } from 'pinia'
import * as W from '../../wailsjs/go/app/WalletService'

export type AccountInfo = { index: number; address: string; label: string }
// WalletMeta mirrors the Go app.WalletMeta returned by ListWallets. `id` is the
// keystore filename (the stable identifier passed to Unlock/RenameWallet); `name`
// is the user-facing display name; `baseAddress` is the wallet's account-0 address.
export type WalletMeta = { id: string; name: string; baseAddress: string }

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
      this.locked = true
      this.active = ''
      this.accounts = []
      this.activeIndex = 0
    },
    async loadAccounts() {
      try { this.accounts = (await W.CurrentAccounts()) as unknown as AccountInfo[] } catch { this.accounts = [] }
    },
    async select(index: number) {
      await W.SelectAccount(index)
      this.activeIndex = index
    },
    async setLabel(index: number, label: string) {
      await W.SetAccountLabel(index, label)
      await this.loadAccounts()
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
