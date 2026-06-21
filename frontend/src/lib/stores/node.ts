import { writable } from 'svelte/store'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import * as N from '../../../wailsjs/go/app/NodeService'

export type NodeStatus = { mode: string; connected: boolean; syncing: boolean; height: number; peers: number }
export const node = writable<NodeStatus>({ mode: 'remote', connected: false, syncing: false, height: 0, peers: 0 })

export type NodeConfig = { mode: string; remoteUrl: string; localUrl: string }

export type SyncStatus = { state: string; currentHeight: number; targetHeight: number; percent: number; etaSeconds: number; peers: number }
export const sync = writable<SyncStatus | null>(null)

export type EmbeddedInfo = { running: boolean; dataDir: string; sizeBytes: number }

export async function getEmbeddedInfo(): Promise<EmbeddedInfo> {
  return (await N.GetEmbeddedInfo()) as EmbeddedInfo
}
export async function deleteEmbeddedData(): Promise<void> {
  await N.DeleteEmbeddedData()
}

export async function getConfig(): Promise<NodeConfig> {
  return (await N.GetNodeConfig()) as NodeConfig
}
export async function setMode(mode: string): Promise<void> {
  await N.SetNodeMode(mode)
}
export async function setUrl(mode: string, url: string): Promise<void> {
  await N.SetNodeURL(mode, url)
}

// initNodeEvents wires backend push events into the store. Returns nothing;
// onTick is invoked on each momentum so callers can refresh pulled data.
export function initNodeEvents(onTick: () => void): void {
  EventsOn('node:status', (s: NodeStatus) => node.set(s))
  EventsOn('node:sync', (s: SyncStatus) => sync.set(s))
  EventsOn('momentum:tick', () => onTick())
}
