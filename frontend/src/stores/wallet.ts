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
    lock() { this.locked = true },
  },
})
