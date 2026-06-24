import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

vi.mock('../../../../wailsjs/go/app/NomService', () => ({
  GetPlasmaInfo: vi.fn().mockResolvedValue({ qsrFused: '0', currentPlasma: 0, maxPlasma: 0 }),
  GetFusionEntries: vi.fn().mockResolvedValue([
    { id: 'abc', beneficiary: 'z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz', qsrAmount: '10000000000', expirationHeight: 100, isRevocable: false },
  ]),
  EstimatePlasma: vi.fn().mockResolvedValue(21000),
  PrepareFuse: vi.fn().mockResolvedValue({ toAddress: 'z1qxemdedded', zts: 'zts1qsr', symbol: 'QSR', amount: '10000000000', hash: 'h', summary: 'Fuse', usedPlasma: 0, difficulty: 0, needsPoW: false }),
  PrepareCancelFuse: vi.fn().mockResolvedValue({ toAddress: 'z1qxemdedded', zts: 'zts1qsr', symbol: 'QSR', amount: '0', hash: 'h', summary: 'Cancel', usedPlasma: 0, difficulty: 0, needsPoW: false }),
}))
vi.mock('../../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import PlasmaPanel from './PlasmaPanel.svelte'

describe('PlasmaPanel', () => {
  it('renders beneficiary + amount fields and the Fuse Plasma button', () => {
    render(PlasmaPanel)
    expect(screen.getByLabelText('beneficiary')).toBeTruthy()
    expect(screen.getByLabelText('qsr amount')).toBeTruthy()
    expect(screen.getByRole('button', { name: /fuse plasma/i })).toBeTruthy()
  })

  it('disables Cancel for a non-revocable entry', async () => {
    render(PlasmaPanel)
    const btn = await screen.findByRole('button', { name: /cancel fusion/i })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
  })

  it('renders the fusion-entry QSR amount in human form', async () => {
    render(PlasmaPanel)
    // mock qsrAmount '10000000000' (base units, 8 decimals) → '100 QSR'
    expect(await screen.findByText(/100 QSR/)).toBeTruthy()
  })
})
