import { describe, it, expect, vi } from 'vitest'
import { render, waitFor } from '@testing-library/svelte'

vi.mock('qrcode', () => ({
  default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,AAAA') },
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ ClipboardSetText: vi.fn() }))

import QRCode from 'qrcode'
import AddressDisplay from './AddressDisplay.svelte'

const toDataURL = QRCode.toDataURL as unknown as ReturnType<typeof vi.fn>

const A = 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg'
const B = 'z1qzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz0xqv8'

describe('AddressDisplay', () => {
  it('regenerates the QR when the address prop changes', async () => {
    toDataURL.mockClear()
    const { rerender } = render(AddressDisplay, { props: { address: A } })
    await waitFor(() => expect(toDataURL).toHaveBeenCalledWith(A, expect.anything()))

    await rerender({ address: B })
    await waitFor(() => expect(toDataURL).toHaveBeenCalledWith(B, expect.anything()))

    const addrs = toDataURL.mock.calls.map((c: unknown[]) => c[0])
    expect(addrs).toContain(A)
    expect(addrs).toContain(B)
  })
})
