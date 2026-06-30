import { describe, it, expect } from 'vitest'
import { formatFiat } from './fiat'

describe('formatFiat', () => {
  it('formats thousands with two decimals', () => {
    expect(formatFiat(13639.07)).toBe('$13,639.07')
  })
  it('formats small unit prices to two decimals', () => {
    expect(formatFiat(0.118422)).toBe('$0.12')
  })
  it('formats zero', () => {
    expect(formatFiat(0)).toBe('$0.00')
  })
})
