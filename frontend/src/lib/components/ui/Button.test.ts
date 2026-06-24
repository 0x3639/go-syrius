// ui/Button.test.ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import Button from './Button.svelte'

describe('Button', () => {
  it('renders a primary (green) button and fires click', async () => {
    const onClick = vi.fn()
    const { component } = render(Button, { props: { variant: 'primary' } })
    component.$on('click', onClick)
    const btn = screen.getByRole('button')
    expect(btn.className).toContain('bg-accent')
    await fireEvent.click(btn)
    expect(onClick).toHaveBeenCalled()
  })
  it('disables', () => {
    render(Button, { props: { disabled: true } })
    expect((screen.getByRole('button') as HTMLButtonElement).disabled).toBe(true)
  })
})
