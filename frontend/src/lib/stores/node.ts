import { writable } from 'svelte/store'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

export type NodeStatus = { mode: string; connected: boolean; syncing: boolean; height: number; peers: number }
export const node = writable<NodeStatus>({ mode: 'remote', connected: false, syncing: false, height: 0, peers: 0 })

// initNodeEvents wires backend push events into the store. Returns nothing;
// onTick is invoked on each momentum so callers can refresh pulled data.
export function initNodeEvents(onTick: () => void): void {
  EventsOn('node:status', (s: NodeStatus) => node.set(s))
  EventsOn('momentum:tick', () => onTick())
}
