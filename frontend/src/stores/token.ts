import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const useTokenStore = defineStore('token', {
  state: () => ({
    myTokens: [] as app.TokenInfo[],
    searchResults: [] as app.TokenInfo[],
  }),
  actions: {
    async refresh() {
      try {
        this.myTokens = await Nom.GetMyTokens()
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
