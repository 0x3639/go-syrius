import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

vi.mock('../../../../wailsjs/go/app/NomService', () => ({
  GetStakeList: vi.fn().mockResolvedValue({ totalAmount: '500000000', entries: [
    { id: 'abc', amount: '500000000', startTimestamp: 1, expirationTimestamp: 2, durationMonths: 3, isMatured: false },
  ] }),
  GetUncollectedReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  PrepareStake: vi.fn(), PrepareCancelStake: vi.fn(), PrepareCollectReward: vi.fn(),
}))
vi.mock('../../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import StakingPanel from './StakingPanel.svelte'

describe('StakingPanel', () => {
  it('renders amount field, duration select and the Stake ZNN button', () => {
    render(StakingPanel)
    expect(screen.getByLabelText('znn amount')).toBeTruthy()
    expect(screen.getByLabelText('duration months')).toBeTruthy()
    expect(screen.getByRole('button', { name: /stake znn/i })).toBeTruthy()
  })

  it('disables Cancel for a non-matured stake', async () => {
    render(StakingPanel)
    const btn = await screen.findByRole('button', { name: /cancel stake/i })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })

  it('disables Collect Reward when there is no uncollected reward', async () => {
    render(StakingPanel)
    const btn = await screen.findByRole('button', { name: /collect reward/i })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })
})
