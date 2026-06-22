import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetStakeList: vi.fn().mockResolvedValue({ totalAmount: '500000000', entries: [
    { id: 'abc', amount: '500000000', startTimestamp: 1, expirationTimestamp: 2, durationMonths: 3, isMatured: false },
  ] }),
  GetUncollectedReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  PrepareStake: vi.fn(), PrepareCancelStake: vi.fn(), PrepareCollectReward: vi.fn(),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Stake from './Stake.svelte'

describe('Stake', () => {
  it('disables Cancel for a non-matured stake', async () => {
    render(Stake)
    const btn = await screen.findByRole('button', { name: /cancel stake/i })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })
})
