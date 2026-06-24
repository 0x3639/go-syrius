// ui/Tabs.test.ts
import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import Tabs from './Tabs.svelte'

describe('Tabs', () => {
  it('marks the active tab and switches on click', async () => {
    render(Tabs, { props: { tabs: ['One', 'Two'], active: 'One' } })
    const two = screen.getByRole('button', { name: 'tab Two' })
    expect(screen.getByRole('button', { name: 'tab One' }).className).toContain('text-accent')
    await fireEvent.click(two)
    expect(two.className).toContain('text-accent')
  })
})
