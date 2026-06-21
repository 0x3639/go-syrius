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

  it('clears the rendered QR when the address becomes empty', async () => {
    toDataURL.mockReset()
    toDataURL.mockResolvedValue('data:image/png;base64,AAAA')
    const { rerender, queryByAltText } = render(AddressDisplay, { props: { address: A } })
    await waitFor(() => expect(queryByAltText('address QR')).not.toBeNull())

    await rerender({ address: '' })
    await waitFor(() => expect(queryByAltText('address QR')).toBeNull())
  })

  it('ignores a stale QR that resolves after a newer one (race)', async () => {
    toDataURL.mockReset()
    const urlA = 'data:image/png;base64,QR_FOR_A'
    const urlB = 'data:image/png;base64,QR_FOR_B'
    // A's promise resolves LATER than B's: A is deferred, B is immediate.
    let resolveA!: (v: string) => void
    toDataURL.mockImplementation((addr: string) => {
      if (addr === A) return new Promise<string>((res) => { resolveA = res })
      return Promise.resolve(urlB)
    })

    const { rerender, queryByAltText } = render(AddressDisplay, { props: { address: A } })
    await rerender({ address: B })
    // B (immediate) settles first.
    await waitFor(() => {
      const img = queryByAltText('address QR') as HTMLImageElement | null
      expect(img?.getAttribute('src')).toBe(urlB)
    })

    // Now let A's stale promise resolve — it must NOT overwrite B.
    resolveA(urlA)
    await Promise.resolve()
    await Promise.resolve()
    const img = queryByAltText('address QR') as HTMLImageElement | null
    expect(img?.getAttribute('src')).toBe(urlB)
  })
})
