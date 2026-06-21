import { describe, it, expect } from 'vitest'
import { formatAmount, shortAddress } from './format'

describe('format', () => {
  it('formats base units by decimals', () => {
    expect(formatAmount('5000000000000', 8)).toBe('50000')
    expect(formatAmount('150000000', 8)).toBe('1.5')
    expect(formatAmount('0', 8)).toBe('0')
  })
  it('shortens addresses', () => {
    expect(shortAddress('z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg')).toBe('z1qrr0…pcjmg')
  })
})
