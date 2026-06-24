import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'

vi.mock('../stores/balances', () => ({
  balances: { subscribe: (f: any) => { f([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '100000000' }]); return () => {} } },
}))

import SendForm from './SendForm.svelte'

describe('SendForm', () => {
  it('disables Send and flags an invalid z1 address', async () => {
    render(SendForm)
    const btn = screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement
    expect(btn.disabled).toBe(true) // nothing entered yet
    await fireEvent.input(screen.getByLabelText('recipient'), { target: { value: 'z1notvalid' } })
    expect(screen.getByText(/Invalid z1 address/)).toBeTruthy()
    expect(btn.disabled).toBe(true)
  })
})
