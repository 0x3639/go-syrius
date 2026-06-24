import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
vi.mock('../../stores/balances', () => ({ balances: { subscribe: (f: any) => { f([
  { zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' },
  { zts: 'zts1abc', symbol: 'RETARD', decimals: 8, amount: '80000000' },
]); return () => {} } } }))
import TokensPanel from './TokensPanel.svelte'
describe('TokensPanel', () => {
  it('lists tokens and filters by search', async () => {
    render(TokensPanel)
    expect(screen.getByText('ZNN')).toBeTruthy()
    expect(screen.getByText('RETARD')).toBeTruthy()
    await fireEvent.input(screen.getByLabelText('search tokens'), { target: { value: 'reta' } })
    expect(screen.queryByText('ZNN')).toBeNull()
    expect(screen.getByText('RETARD')).toBeTruthy()
  })
})
