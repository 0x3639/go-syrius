import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'

// Mock qrcode so the test never touches a real canvas; the address row + copy
// are independent of the QR render.
vi.mock('qrcode', () => ({ default: { toCanvas: vi.fn().mockResolvedValue(undefined) } }))

import AddressDisplay from './AddressDisplay.vue'

const ADDR = 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg'

describe('AddressDisplay', () => {
  it('shows the full address on one line (no wrap) with a working copy button', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

    const w = mount(AddressDisplay, { props: { address: ADDR } })

    const code = w.find('code')
    expect(code.text()).toBe(ADDR) // full address, untruncated
    expect(code.classes()).toContain('whitespace-nowrap') // single line

    await w.find(`button[aria-label="copy address ${ADDR}"]`).trigger('click')
    expect(writeText).toHaveBeenCalledWith(ADDR)
  })
})
