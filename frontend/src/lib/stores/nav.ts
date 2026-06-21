import { writable } from 'svelte/store'

export type View = 'dashboard' | 'send' | 'create' | 'import' | 'unlock' | 'settings'
export const view = writable<View>('dashboard')
