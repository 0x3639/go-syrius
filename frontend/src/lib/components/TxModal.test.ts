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

  it('renders the action summary for contract-call previews', async () => {
    tx.set({ status: 'awaiting', hash: '', error: '',
      preview: { toAddress: 'z1qxemdeddedxplasmaxxxxxxxxxxxxxxxxsctrp', symbol: 'QSR', zts: 'zts1qsr', amount: '0', usedPlasma: 0, difficulty: 0, hash: 'cafebabe', needsPoW: false, summary: 'Cancel fusion abc' } })
    render(TxModal)
    expect(await screen.findByText('Cancel fusion abc')).toBeTruthy()
    expect(await screen.findByText(/cafebabe/)).toBeTruthy()
  })

  it('omits the summary for plain Send previews', async () => {
    tx.set({ status: 'awaiting', hash: '', error: '',
      preview: { toAddress: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg', symbol: 'ZNN', zts: 'zts1znn', amount: '150000000', usedPlasma: 21000, difficulty: 0, hash: 'deadbeef', needsPoW: false } })
    render(TxModal)
    expect(screen.queryByText('Cancel fusion abc')).toBeNull()
  })
})
