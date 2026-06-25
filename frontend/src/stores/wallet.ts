import { defineStore } from 'pinia'
import * as W from '../../wailsjs/go/app/WalletService'

export const useWalletStore = defineStore('wallet', {
  state: () => ({ locked: true, wallets: [] as string[], active: '' }),
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
    },
    lock() {
      // Re-lock the keystore in the Go backend, not just the UI — otherwise the
      // wallet shows locked while the backend keystore stays decrypted.
      W.Lock().catch(() => {})
      this.locked = true
      this.active = ''
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
