import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

export const useTokenStore = defineStore('token', {
  state: () => ({
    myTokens: [] as app.TokenInfo[],
    searchResults: [] as app.TokenInfo[],
  }),
  actions: {
    async refresh() {
      const epoch = currentRequestEpoch()
      try {
        const myTokens = await Nom.GetMyTokens()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.myTokens = myTokens
      } catch { /* not connected / locked — leave as-is */ }
    },
    // Search by ZTS id, name, or symbol (backend decides which).
    async search(query: string) {
      this.searchResults = (await Nom.SearchTokens(query)) ?? []
    },
    clearSearch() {
      this.searchResults = []
    },
  },
})
