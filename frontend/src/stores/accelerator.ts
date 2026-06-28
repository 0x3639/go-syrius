import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'

export const useAcceleratorStore = defineStore('accelerator', {
  state: () => ({
    projects: [] as app.ProjectDTO[],
    selectedProject: null as app.ProjectDTO | null,
    votablePillars: [] as string[],
    votable: [] as app.VotableItem[],
    numActivePillars: 0,
    error: '',
  }),
  getters: {
    needsVoteCount(state): number {
      return state.votable.filter((v) => v.needsMyVote).length
    },
  },
  actions: {
    async loadProjects(page = 0) {
      this.error = ''
      try {
        const list = await Nom.GetProjects(page, 20)
        this.projects = list.list ?? []
      } catch (e: any) {
        this.error = e?.message ?? String(e)
      }
    },
    async openProject(id: string) {
      this.error = ''
      try {
        this.selectedProject = await Nom.GetProject(id)
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
    // Votable items for the active address's pillars + active pillar count, for
    // the Vote view and the top-bar badge. Swallows errors (badge shows 0).
    async refreshVotable() {
      try {
        this.votable = await Nom.GetVotableForMyPillars()
        this.numActivePillars = await Nom.GetActivePillarCount()
      } catch {
        this.votable = []
      }
    },
  },
})
