import { writable } from 'svelte/store'
import * as Nom from '../../../wailsjs/go/app/NomService'
import type { app } from '../../../wailsjs/go/models'

export const myTokens = writable<app.TokenInfo[]>([])
export const lookedUpToken = writable<app.TokenInfo | null>(null)

export async function refreshTokens(): Promise<void> {
  try {
    myTokens.set(await Nom.GetMyTokens())
  } catch { /* not connected / locked — leave as-is */ }
}

export async function lookupToken(zts: string): Promise<void> {
  const t = await Nom.GetTokenByZts(zts)
  lookedUpToken.set(t && t.tokenStandard !== '' ? t : null)
}
