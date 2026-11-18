import { defineStore } from 'pinia'
import * as Nom from '../../wailsjs/go/app/NomService'
import { currentRequestEpoch } from '../lib/requestEpoch'
import type { app } from '../../wailsjs/go/models'

export const useAcceleratorStore = defineStore('accelerator', {
  state: () => ({
    _epoch: -1,
    _requests: { projects: 0, project: 0, myProjects: 0, votablePillars: 0, votable: 0 },
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
    invalidate() {
      this._epoch = currentRequestEpoch()
      for (const key of Object.keys(this._requests) as (keyof typeof this._requests)[]) this._requests[key]++
      this.projects = []; this.projectCount = 0; this.projectPage = 0
      this.myProjects = []; this.selectedProject = null
      this.votablePillars = []; this.votable = []; this.numActivePillars = 0
      this.error = ''
    },
    ensureScope() {
      if (this._epoch !== currentRequestEpoch()) this.invalidate()
      return currentRequestEpoch()
    },
    async loadProjects(page = 0) {
      const epoch = this.ensureScope()
      const request = ++this._requests.projects
      this.error = ''
      try {
        const list = await Nom.GetProjects(page, 20)
        if (epoch !== currentRequestEpoch() || request !== this._requests.projects) return
        this.projects = list.list ?? []
        this.projectCount = list.count ?? 0
        this.projectPage = page
      } catch (e: any) {
        if (epoch !== currentRequestEpoch() || request !== this._requests.projects) return
        this.error = e?.message ?? String(e)
      }
    },
    async openProject(id: string) {
      const epoch = this.ensureScope()
      const request = ++this._requests.project
      this.error = ''
      try {
        const project = await Nom.GetProject(id)
        if (epoch !== currentRequestEpoch() || request !== this._requests.project) return
        this.selectedProject = project
      } catch (e: any) {
        if (epoch !== currentRequestEpoch() || request !== this._requests.project) return
        this.error = e?.message ?? String(e)
      }
    },
    // The active address's Active (approved, unfinished) projects — the picker
    // for requesting a phase payout. Swallows errors (locked/disconnected → []).
    async loadMyProjects() {
      const epoch = this.ensureScope()
      const request = ++this._requests.myProjects
      try {
        const myProjects = await Nom.GetMyProjects()
        if (epoch !== currentRequestEpoch() || request !== this._requests.myProjects) return
        this.myProjects = myProjects
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._requests.myProjects) return
        this.myProjects = []
      }
    },
    async loadVotablePillars() {
      const epoch = this.ensureScope()
      const request = ++this._requests.votablePillars
      try {
        const votablePillars = await Nom.GetVotablePillars()
        if (epoch !== currentRequestEpoch() || request !== this._requests.votablePillars) return
        this.votablePillars = votablePillars
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._requests.votablePillars) return
        this.votablePillars = [] // locked / not connected ⇒ no voting
      }
    },
    // Votable items for the active address's pillars + active pillar count, for
    // the Vote view and the top-bar badge. Swallows errors (badge shows 0).
    async refreshVotable() {
      const epoch = this.ensureScope()
      const request = ++this._requests.votable
      try {
        const votable = await Nom.GetVotableForMyPillars()
        const numActivePillars = await Nom.GetActivePillarCount()
        if (epoch !== currentRequestEpoch() || request !== this._requests.votable) return
        this.votable = votable
        this.numActivePillars = numActivePillars
      } catch {
        if (epoch !== currentRequestEpoch() || request !== this._requests.votable) return
        this.votable = []
      }
    },
  },
})
