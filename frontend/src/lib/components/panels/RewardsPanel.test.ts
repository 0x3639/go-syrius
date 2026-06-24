import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

vi.mock('../../../../wailsjs/go/app/NomService', () => ({
  GetPillarReward: vi.fn().mockResolvedValue({ znn: '500000000', qsr: '0' }),
  GetUncollectedReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  GetSentinelReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  PrepareCollectPillarReward: vi.fn(),
  PrepareCollectReward: vi.fn(),
  PrepareCollectSentinelReward: vi.fn(),
}))
vi.mock('../../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import RewardsPanel from './RewardsPanel.svelte'

describe('RewardsPanel', () => {
  it('lists the three reward sources', async () => {
    render(RewardsPanel)
    expect(await screen.findByText('Delegation')).toBeTruthy()
    expect(screen.getByText('Staking')).toBeTruthy()
    expect(screen.getByText('Sentinel')).toBeTruthy()
  })

  it('enables Collect only for a source with a non-zero reward', async () => {
    render(RewardsPanel)
    const delegation = await screen.findByLabelText('collect Delegation')
    const staking = await screen.findByLabelText('collect Staking')
    const sentinel = await screen.findByLabelText('collect Sentinel')
    expect((delegation as HTMLButtonElement).disabled).toBe(false)
    expect((staking as HTMLButtonElement).disabled).toBe(true)
    expect((sentinel as HTMLButtonElement).disabled).toBe(true)
  })
})
