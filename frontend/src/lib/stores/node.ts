import { writable } from 'svelte/store'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import * as N from '../../../wailsjs/go/app/NodeService'

export type NodeStatus = { mode: string; connected: boolean; syncing: boolean; height: number; peers: number }
export const node = writable<NodeStatus>({ mode: 'remote', connected: false, syncing: false, height: 0, peers: 0 })

export type NodeConfig = { mode: string; remoteUrl: string; localUrl: string }

export async function getConfig(): Promise<NodeConfig> {
  return (await N.GetNodeConfig()) as NodeConfig
}
export async function setMode(mode: string): Promise<void> {
  try { await N.SetNodeMode(mode) } catch { /* status event reflects disconnected */ }
}
export async function setUrl(mode: string, url: string): Promise<void> {
  await N.SetNodeURL(mode, url)
}

// initNodeEvents wires backend push events into the store. Returns nothing;
// onTick is invoked on each momentum so callers can refresh pulled data.
export function initNodeEvents(onTick: () => void): void {
  EventsOn('node:status', (s: NodeStatus) => node.set(s))
  EventsOn('momentum:tick', () => onTick())
}
