import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/NodeService', () => ({ GetUnreceived: vi.fn().mockResolvedValue([{ fromHash: 'h1', fromAddress: 'z1abc', token: 'ZNN', amount: '100000000' }]) }))
vi.mock('../../../wailsjs/go/app/TxService', () => ({ Receive: vi.fn().mockResolvedValue('r1') }))
import UnreceivedPanel from './UnreceivedPanel.svelte'

describe('UnreceivedPanel', () => {
  it('lists unreceived blocks', async () => {
    render(UnreceivedPanel)
    expect(await screen.findByText('1 ZNN')).toBeTruthy()
    expect(await screen.findByRole('button', { name: 'Receive' })).toBeTruthy()
    expect(await screen.findByRole('button', { name: 'Receive all' })).toBeTruthy()
  })
})
