import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import BalanceCard from './BalanceCard.svelte'
describe('BalanceCard', () => {
  it('renders symbol + mono formatted amount with tint', () => {
    render(BalanceCard, { props: { symbol: 'ZNN', amount: '150000000', decimals: 8, tint: 'green' } })
    const el = screen.getByLabelText('ZNN balance')
    expect(el.className).toContain('font-mono')
    expect(el.textContent).toContain('1.5')
  })
})
