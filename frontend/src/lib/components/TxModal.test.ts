import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/TxService', () => ({ ConfirmPublish: vi.fn(), CancelPending: vi.fn() }))
import TxModal from './TxModal.svelte'
import { tx } from '../stores/tx'

describe('TxModal', () => {
  it('renders the built-block preview incl. hash', async () => {
    tx.set({ status: 'awaiting', hash: '', error: '',
      preview: { toAddress: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg', symbol: 'ZNN', zts: 'zts1znn', amount: '150000000', usedPlasma: 21000, difficulty: 0, hash: 'deadbeef', needsPoW: false } })
    render(TxModal)
    expect(await screen.findByText(/deadbeef/)).toBeTruthy()
    expect(await screen.findByText(/1\.5/)).toBeTruthy()
    expect(await screen.findByText(/ZNN/)).toBeTruthy()
  })
})
