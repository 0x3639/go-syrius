import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/go/app/TxService', () => ({ PrepareSend: vi.fn(), ConfirmPublish: vi.fn(), CancelPending: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))
vi.mock('../stores/balances', () => ({ balances: { subscribe: (f: any) => { f([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '0' }]); return () => {} } } }))
import SendModal from './SendModal.svelte'
describe('SendModal', () => {
  it('renders the send form when open', () => {
    render(SendModal, { props: { open: true } })
    expect(screen.getByLabelText('recipient')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'close' })).toBeTruthy()
  })
  it('renders nothing when closed', () => {
    render(SendModal, { props: { open: false } })
    expect(screen.queryByLabelText('recipient')).toBeNull()
  })
})
