import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../../../wailsjs/runtime/runtime', () => ({ ClipboardSetText: vi.fn() }))
vi.mock('../stores/wallet', () => ({ wallet: { subscribe: (f: any) => { f({ accounts: [{ index: 0, address: 'z1qtest' }], active: 0 }); return () => {} } } }))
import ReceiveModal from './ReceiveModal.svelte'
describe('ReceiveModal', () => {
  it('shows the active address when open', async () => {
    render(ReceiveModal, { props: { open: true } })
    expect(await screen.findByText('z1qtest')).toBeTruthy()
  })
})
