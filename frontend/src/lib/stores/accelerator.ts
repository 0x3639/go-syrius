import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const projects = writable<app.ProjectDTO[]>([])
export const selectedProject = writable<app.ProjectDTO | null>(null)
export const votablePillars = writable<string[]>([])
export const accError = writable('')

export async function loadProjects(page = 0): Promise<void> {
  accError.set('')
  try {
    const list = await Nom.GetProjects(page, 20)
    projects.set(list.list ?? [])
  } catch (e: any) {
    accError.set(e?.message ?? String(e))
  }
}

export async function openProject(id: string): Promise<void> {
  accError.set('')
  try {
    selectedProject.set(await Nom.GetProject(id))
  } catch (e: any) {
    accError.set(e?.message ?? String(e))
  }
}

export async function loadVotablePillars(): Promise<void> {
  try {
    votablePillars.set(await Nom.GetVotablePillars())
  } catch {
    votablePillars.set([]) // locked / not connected ⇒ no voting
  }
}
