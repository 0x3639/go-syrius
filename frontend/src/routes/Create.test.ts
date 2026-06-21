import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/WalletService', () => ({
  GenerateMnemonic: vi.fn().mockResolvedValue('alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec romeo sierra tango uniform victor whiskey xray'),
  ImportMnemonic: vi.fn().mockResolvedValue({ name: 'w.dat', baseAddress: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg', label: '' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))

import Create from './Create.svelte'

describe('Create', () => {
  it('shows the generated mnemonic on step 1', async () => {
    render(Create)
    expect(await screen.findByText(/foxtrot/)).toBeTruthy()
  })
})
