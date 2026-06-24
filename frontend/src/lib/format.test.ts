import { describe, it, expect } from 'vitest'
import { formatAmount, formatAmountExact } from './format'

describe('formatAmount (display)', () => {
  it('drops decimals + adds commas for 3+ integer digits', () => {
    expect(formatAmount('20000000000', 8)).toBe('200') // 200
    expect(formatAmount('50000000000000', 8)).toBe('500,000') // 500000
    expect(formatAmount('5045401869374', 8)).toBe('50,454') // 50454.018… -> 50,454
    expect(formatAmount('49998000000000', 8)).toBe('499,980') // 499980.00000000 -> 499,980
  })
  it('rounds to 2 decimals for values under 100', () => {
    expect(formatAmount('2001111100', 8)).toBe('20.01') // 20.011111
    expect(formatAmount('150000000', 8)).toBe('1.5') // 1.5
    expect(formatAmount('0', 8)).toBe('0')
  })
})

describe('formatAmountExact', () => {
  it('keeps full precision', () => {
    expect(formatAmountExact('5045401869374', 8)).toBe('50454.01869374')
    expect(formatAmountExact('150000000', 8)).toBe('1.5')
  })
})
