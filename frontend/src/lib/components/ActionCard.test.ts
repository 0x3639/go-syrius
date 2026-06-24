import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import ActionCard from './ActionCard.svelte'
describe('ActionCard', () => {
  it('fires click', async () => {
    const onClick = vi.fn()
    const { component } = render(ActionCard, { props: { label: 'Send', direction: 'send' } })
    component.$on('click', onClick)
    await fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(onClick).toHaveBeenCalled()
  })
})
