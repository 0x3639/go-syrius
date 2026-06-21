import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/TxService', () => ({
  PrepareSend: vi.fn().mockResolvedValue({ toAddress: 'z1...', symbol: 'ZNN', zts: 'zts1znn...', amount: '100000000', usedPlasma: 21000, difficulty: 0, hash: 'abcd', needsPoW: false }),
  ConfirmPublish: vi.fn().mockResolvedValue('abcd'),
  CancelPending: vi.fn().mockResolvedValue(undefined),
  RequiresPoW: vi.fn().mockResolvedValue(false),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))

import Send from './Send.svelte'

describe('Send', () => {
  it('disables Send for an invalid address', async () => {
    render(Send)
    await fireEvent.input(screen.getByLabelText(/recipient/i), { target: { value: 'nope' } })
    await fireEvent.input(screen.getByLabelText(/amount/i), { target: { value: '1' } })
    expect((screen.getByRole('button', { name: /^send$/i }) as HTMLButtonElement).disabled).toBe(true)
  })
})
