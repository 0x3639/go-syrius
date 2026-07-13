import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

export const useAcceleratorStore = defineStore('accelerator', {
  state: () => ({
    projects: [] as app.ProjectDTO[],
    projectCount: 0,
    projectPage: 0,
    myProjects: [] as app.ProjectDTO[],
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
        this.projectCount = list.count ?? 0
        this.projectPage = page
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
    // The active address's Active (approved, unfinished) projects — the picker
    // for requesting a phase payout. Swallows errors (locked/disconnected → []).
    async loadMyProjects() {
      const epoch = currentRequestEpoch()
      try {
        const myProjects = await Nom.GetMyProjects()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.myProjects = myProjects
      } catch {
        if (epoch !== currentRequestEpoch()) return
        this.myProjects = []
      }
    },
    async loadVotablePillars() {
      const epoch = currentRequestEpoch()
      try {
        const votablePillars = await Nom.GetVotablePillars()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.votablePillars = votablePillars
      } catch {
        if (epoch !== currentRequestEpoch()) return
        this.votablePillars = [] // locked / not connected ⇒ no voting
      }
    },
    // Votable items for the active address's pillars + active pillar count, for
    // the Vote view and the top-bar badge. Swallows errors (badge shows 0).
    async refreshVotable() {
      const epoch = currentRequestEpoch()
      try {
        const votable = await Nom.GetVotableForMyPillars()
        const numActivePillars = await Nom.GetActivePillarCount()
        if (epoch !== currentRequestEpoch()) return // stale: another account's data
        this.votable = votable
        this.numActivePillars = numActivePillars
      } catch {
        if (epoch !== currentRequestEpoch()) return
        this.votable = []
      }
    },
  },
})
