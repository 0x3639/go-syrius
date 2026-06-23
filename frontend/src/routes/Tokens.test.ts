import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'

const mocks = vi.hoisted(() => ({
  GetMyTokens: vi.fn(),
  GetTokenByZts: vi.fn(),
  PrepareIssueToken: vi.fn(), PrepareMint: vi.fn(), PrepareBurn: vi.fn(), PrepareUpdateToken: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NomService', () => mocks)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))
vi.mock('../lib/stores/wallet', () => ({ wallet: { subscribe: (fn: any) => { fn({ accounts: [{ index: 0, address: 'z1qme' }], active: 0 }); return () => {} } } }))

import Tokens from './Tokens.svelte'

const TOK = { name: 'Alpha', symbol: 'ALPHA', domain: '', tokenStandard: 'zts1alpha', owner: 'z1qme', totalSupply: '100', maxSupply: '200', decimals: 0, isMintable: true, isBurnable: true, isUtility: false }

describe('Tokens', () => {
  it('lists my tokens with a Mint control for mintable tokens', async () => {
    mocks.GetMyTokens.mockResolvedValue([TOK])
    render(Tokens)
    expect(await screen.findByText(/ALPHA/)).toBeTruthy()
    expect(screen.getByRole('button', { name: /mint alpha/i })).toBeTruthy()
  })

  it('shows Burn only after a successful lookup of a burnable token', async () => {
    mocks.GetMyTokens.mockResolvedValue([])
    mocks.GetTokenByZts.mockResolvedValue({ ...TOK, owner: 'z1other' })
    render(Tokens)
    // no burn button before lookup
    expect(screen.queryByRole('button', { name: /^burn$/i })).toBeNull()
    const input = screen.getByLabelText('lookup zts') as HTMLInputElement
    await fireEvent.input(input, { target: { value: 'zts1alpha' } })
    await fireEvent.click(screen.getByRole('button', { name: /look up/i }))
    expect(await screen.findByRole('button', { name: /^burn$/i })).toBeTruthy()
  })
})
