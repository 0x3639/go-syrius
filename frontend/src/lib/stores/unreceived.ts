import { writable } from 'svelte/store'
import * as N from '../../../wailsjs/go/app/NodeService'

export type Unreceived = { fromHash: string; fromAddress: string; token: string; amount: string }
export const unreceived = writable<Unreceived[]>([])

export async function loadUnreceived(): Promise<void> {
  try { unreceived.set((await N.GetUnreceived()) as unknown as Unreceived[]) } catch { unreceived.set([]) }
}
