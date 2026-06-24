import { describe, it, expect } from 'vitest'
import { formatAmount, formatAmountExact, shortAddress } from './format'

describe('formatAmount (display)', () => {
  it('drops decimals + adds commas for 3+ integer digits', () => {
    expect(formatAmount('20000000000', 8)).toBe('200') // 200
    expect(formatAmount('5000000000000', 8)).toBe('50,000') // 50000
    expect(formatAmount('50000000000000', 8)).toBe('500,000') // 500000
    expect(formatAmount('5045401869374', 8)).toBe('50,454') // 50454.018… -> 50,454
  })
  it('rounds to 2 decimals for values under 100', () => {
    expect(formatAmount('2001111100', 8)).toBe('20.01') // 20.011111
    expect(formatAmount('1001321200', 8)).toBe('10.01') // 10.013212
    expect(formatAmount('150000000', 8)).toBe('1.5') // 1.5
    expect(formatAmount('0', 8)).toBe('0')
  })
})

describe('formatAmountExact (confirm)', () => {
  it('keeps full precision', () => {
    expect(formatAmountExact('5045401869374', 8)).toBe('50454.01869374')
    expect(formatAmountExact('150000000', 8)).toBe('1.5')
    expect(formatAmountExact('50000000000000', 8)).toBe('500000')
  })
})

describe('shortAddress', () => {
  it('shortens addresses', () => {
    expect(shortAddress('z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg')).toBe('z1qrr0…pcjmg')
  })
})
