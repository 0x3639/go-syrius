import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

const mocks = vi.hoisted(() => ({
  GetProjects: vi.fn(), GetProject: vi.fn(), GetPhase: vi.fn(), GetVotablePillars: vi.fn(),
  PrepareDonate: vi.fn(), PrepareVote: vi.fn(),
  PrepareCreateProject: vi.fn(), PrepareAddPhase: vi.fn(), PrepareUpdatePhase: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NomService', () => mocks)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Accelerator from './Accelerator.svelte'

const PROJ = {
  id: '0xabc', owner: 'z1qme', name: 'My Project', description: 'd', url: 'https://x.org',
  znnFundsNeeded: '100', qsrFundsNeeded: '1000', creationTimestamp: 0, lastUpdateTimestamp: 0,
  status: 0, votes: { total: 3, yes: 2, no: 1 }, phases: [],
}

describe('Accelerator', () => {
  it('lists projects from GetProjects', async () => {
    mocks.GetProjects.mockResolvedValue({ count: 1, list: [PROJ] })
    mocks.GetVotablePillars.mockResolvedValue([])
    render(Accelerator)
    expect(await screen.findByText(/My Project/)).toBeTruthy()
  })

  it('hides the voting section when the address owns no pillars', async () => {
    mocks.GetProjects.mockResolvedValue({ count: 0, list: [] })
    mocks.GetVotablePillars.mockResolvedValue([])
    render(Accelerator)
    await screen.findByText(/Accelerator-Z/)
    expect(screen.queryByLabelText('vote target id')).toBeNull()
  })

  it('shows the voting section when the address owns a pillar', async () => {
    mocks.GetProjects.mockResolvedValue({ count: 0, list: [] })
    mocks.GetVotablePillars.mockResolvedValue(['MyPillar'])
    render(Accelerator)
    expect(await screen.findByLabelText('vote target id')).toBeTruthy()
  })
})
