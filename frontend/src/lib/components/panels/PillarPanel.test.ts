import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'

vi.mock('../../../../wailsjs/go/app/NomService', () => ({
  GetPillarList: vi.fn().mockResolvedValue([
    { name: 'Alpha', rank: 1, weight: '100000000000', delegateRewardPercent: 90, producerAddress: 'z1a' },
    { name: 'Beta', rank: 2, weight: '50000000000', delegateRewardPercent: 80, producerAddress: 'z1b' },
  ]),
  GetDelegation: vi.fn().mockResolvedValue({ name: '', status: 0, weight: '0' }),
  GetPillarReward: vi.fn().mockResolvedValue({ znn: '0', qsr: '0' }),
  PrepareDelegate: vi.fn(), PrepareUndelegate: vi.fn(), PrepareCollectPillarReward: vi.fn(),
}))
vi.mock('../../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import PillarPanel from './PillarPanel.svelte'

describe('PillarPanel', () => {
  it('filters the pillar list by search text', async () => {
    render(PillarPanel)
    expect(await screen.findByText('Alpha')).toBeTruthy()
    expect(screen.getByText('Beta')).toBeTruthy()
    const search = screen.getByLabelText('search pillars') as HTMLInputElement
    await fireEvent.input(search, { target: { value: 'alph' } })
    expect(screen.getByText('Alpha')).toBeTruthy()
    expect(screen.queryByText('Beta')).toBeNull()
  })

  it('renders a Delegate button for each pillar', async () => {
    render(PillarPanel)
    await screen.findByText('Alpha')
    expect(screen.getByRole('button', { name: /delegate to Alpha/i })).toBeTruthy()
    expect(screen.getByRole('button', { name: /delegate to Beta/i })).toBeTruthy()
  })

  it('hides Undelegate when not delegated', async () => {
    render(PillarPanel)
    await screen.findByText('Alpha')
    expect(screen.queryByRole('button', { name: /undelegate/i })).toBeNull()
  })
})
