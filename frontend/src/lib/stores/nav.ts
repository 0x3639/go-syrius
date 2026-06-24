import { writable } from 'svelte/store'

export type View = 'dashboard' | 'create' | 'import' | 'unlock' | 'settings' | 'tokens'
export const view = writable<View>('dashboard')
