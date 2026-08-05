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
      } catch {
        if (epoch !== currentRequestEpoch()) return
        // Clear rather than leave as-is (mirrors balances): after an account
        // switch a failed refresh would otherwise keep showing the PREVIOUS
        // account's token list as if it were this one's (CWE-200).
        this.myTokens = []
      }
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
