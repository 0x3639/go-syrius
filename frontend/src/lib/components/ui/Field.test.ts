// ui/Field.test.ts
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Field from './Field.svelte'

describe('Field', () => {
  it('shows label + hint, and error replaces hint', async () => {
    const { rerender } = render(Field, { props: { label: 'Amount', hint: 'min 1' } })
    expect(screen.getByText('Amount')).toBeTruthy()
    expect(screen.getByText('min 1')).toBeTruthy()
    await rerender({ label: 'Amount', hint: 'min 1', error: 'too low' })
    expect(screen.getByText('too low')).toBeTruthy()
    expect(screen.queryByText('min 1')).toBeNull()
  })
})
