import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ImportMnemonic: vi.fn().mockResolvedValue({ name: 'i.dat', baseAddress: 'z1q' }),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1q', label: '' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))
import ImportMnemonic from './ImportMnemonic.svelte'

describe('ImportMnemonic', () => {
  it('disables Import until name+password+mnemonic provided', async () => {
    render(ImportMnemonic)
    expect((screen.getByRole('button', { name: /^import$/i }) as HTMLButtonElement).disabled).toBe(true)
  })
})
