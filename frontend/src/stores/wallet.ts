import { defineStore } from 'pinia'
import * as W from '../../wailsjs/go/app/WalletService'

export type AccountInfo = { index: number; address: string; label: string }

export const useWalletStore = defineStore('wallet', {
  state: () => ({ locked: true, wallets: [] as string[], active: '', accounts: [] as AccountInfo[], activeIndex: 0 }),
  actions: {
    async loadWallets() {
      try {
        const list = (await W.ListWallets()) as unknown as Array<{ name: string }>
        this.wallets = list.map((w) => w.name)
        if (!this.active && this.wallets.length) this.active = this.wallets[0]
      } catch { this.wallets = [] }
    },
    async unlock(name: string, password: string) {
      await W.Unlock(name, password)
      this.active = name
      this.locked = false
      await this.loadAccounts()
      this.activeIndex = 0
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
    // Persist a new keystore from a mnemonic. Does NOT unlock — the caller
    // unlocks afterward (mirrors the Svelte create/import flow). Throws on error.
    async importMnemonic(file: string, password: string, mnemonic: string): Promise<void> {
      await W.ImportMnemonic(file, password, mnemonic)
      await this.loadWallets()
    },
    // Import an existing keystore file; wallet stays locked (user then unlocks).
    async importKeystore(srcPath: string): Promise<void> {
      await W.ImportKeystore(srcPath)
      await this.loadWallets()
    },
    async pickKeystoreFile(): Promise<string> {
      return (await W.PickKeystoreFile()) || ''
    },
  },
})
