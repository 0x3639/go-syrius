import { writable } from 'svelte/store'

export type View = 'dashboard' | 'send' | 'create' | 'import' | 'unlock' | 'settings' | 'plasma'
export const view = writable<View>('dashboard')
