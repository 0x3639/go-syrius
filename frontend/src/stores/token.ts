import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const useTokenStore = defineStore('token', {
  state: () => ({
    myTokens: [] as app.TokenInfo[],
    lookedUp: null as app.TokenInfo | null,
  }),
  actions: {
    async refresh() {
      try {
        this.myTokens = await Nom.GetMyTokens()
      } catch { /* not connected / locked — leave as-is */ }
    },
    async lookup(zts: string) {
      const t = await Nom.GetTokenByZts(zts)
      this.lookedUp = t && t.tokenStandard !== '' ? t : null
    },
  },
})
