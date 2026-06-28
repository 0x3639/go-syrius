import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

const PAGE_SIZE = 20

export const useGovernanceStore = defineStore('governance', {
  state: () => ({
    actions: [] as app.ActionDTO[],
    actionCount: 0,
    actionPage: 0,
    selectedAction: null as app.ActionDTO | null,
    votablePillars: [] as string[],
    numActivePillars: 0,
    proposeKinds: [] as app.ProposeKindDTO[],
    error: '',
  }),
  actions: {
    async loadActions(page = 0) {
      this.error = ''
      try {
        const list = await Nom.GetActions(page, PAGE_SIZE)
        this.actions = list.list ?? []
        this.actionCount = list.count ?? 0
        this.actionPage = page
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      }
    },
    async openAction(id: string) {
      this.error = ''
      try {
        this.selectedAction = await Nom.GetAction(id)
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      }
    },
    async loadVotablePillars() {
      try {
        this.votablePillars = await Nom.GetVotablePillars()
      } catch {
        this.votablePillars = [] // locked / not connected ⇒ no voting
      }
    },
    async loadActivePillarCount() {
      try {
        this.numActivePillars = await Nom.GetActivePillarCount()
      } catch {
        this.numActivePillars = 0
      }
    },
    async loadProposeKinds() {
      try {
        this.proposeKinds = await Nom.GetProposeKinds()
      } catch {
        this.proposeKinds = [] // not connected / error ⇒ no form options
      }
    },
  },
})
