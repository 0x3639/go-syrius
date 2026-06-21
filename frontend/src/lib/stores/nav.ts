import { writable } from 'svelte/store'

export type View = 'dashboard' | 'send'
export const view = writable<View>('dashboard')
