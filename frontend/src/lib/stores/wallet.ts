import { writable } from 'svelte/store'
import * as W from '../../../wailsjs/go/app/WalletService'

export type Account = { index: number; address: string }
export type WalletState = { locked: boolean; walletName: string; accounts: Account[]; active: number }

export const wallet = writable<WalletState>({ locked: true, walletName: '', accounts: [], active: 0 })

export async function unlock(name: string, password: string): Promise<void> {
  await W.Unlock(name, password)
  const accounts = (await W.CurrentAccounts()) as unknown as Account[]
  wallet.set({ locked: false, walletName: name, accounts, active: 0 })
}

export async function generateMnemonic(): Promise<string> {
  return (await W.GenerateMnemonic()) as string
}

export async function importMnemonic(name: string, password: string, mnemonic: string): Promise<void> {
  await W.ImportMnemonic(name, password, mnemonic)
}

export function lock(): void {
  W.Lock().catch(() => {})
  wallet.set({ locked: true, walletName: '', accounts: [], active: 0 })
}

export async function select(index: number): Promise<void> {
  await W.SelectAccount(index)
  wallet.update((s) => ({ ...s, active: index }))
}

export async function refreshAccounts(): Promise<void> {
  const accounts = (await W.CurrentAccounts()) as unknown as Account[]
  wallet.update((s) => ({ ...s, accounts }))
}

export async function changePassword(name: string, oldPassword: string, newPassword: string): Promise<void> {
  await W.ChangePassword(name, oldPassword, newPassword)
}

export async function revealMnemonic(password: string): Promise<string> {
  return (await W.RevealMnemonic(password)) as string
}
