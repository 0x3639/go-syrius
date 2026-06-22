import { writable } from 'svelte/store'

export type View = 'dashboard' | 'send' | 'create' | 'import' | 'unlock' | 'settings' | 'plasma' | 'stake' | 'pillars' | 'sentinels'
export const view = writable<View>('dashboard')
