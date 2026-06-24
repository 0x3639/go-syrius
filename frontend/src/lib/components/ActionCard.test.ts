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
  it('shows a count badge when badge > 0, and none when 0', async () => {
    const { rerender } = render(ActionCard, { props: { label: 'Receive', direction: 'receive', badge: 0 } })
    expect(screen.queryByLabelText(/pending/)).toBeNull()
    await rerender({ label: 'Receive', direction: 'receive', badge: 3 })
    const b = screen.getByLabelText('3 pending')
    expect(b.textContent).toContain('3')
  })
})
