import { describe, it, expect } from 'vitest'
import { formatAmount, formatAmountExact, toBase, isValidPillarName } from './format'

describe('toBase (decimal -> base units)', () => {
  it('converts decimal strings to base-unit integers at the given precision', () => {
    expect(toBase('1.5', 8)).toBe('150000000')
    expect(toBase('200', 8)).toBe('20000000000')
    expect(toBase('0.00000001', 8)).toBe('1')
  })
})

describe('formatAmount (display)', () => {
  it('distinguishes tiny nonzero values from zero without floating-point loss', () => {
    expect(formatAmount('1', 8)).toBe('<0.01')
    expect(formatAmount('999999', 8)).toBe('<0.01')
    expect(formatAmount('1000000', 8)).toBe('0.01')
    expect(formatAmount('1', 18)).toBe('<0.01')
    expect(formatAmount('0', 18)).toBe('0')
    expect(formatAmount('1', 0)).toBe('1')
    expect(formatAmount('900719925474099312345678', 8)).toBe('9,007,199,254,740,993.12')
  })
  it('groups large amounts without discarding cents', () => {
    expect(formatAmount('20000000000', 8)).toBe('200') // 200
    expect(formatAmount('50000000000000', 8)).toBe('500,000') // 500000
    expect(formatAmount('5045401869374', 8)).toBe('50,454.02') // 50454.018… -> 50,454
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

describe('isValidPillarName', () => {
  it('accepts alphanumerics with single separators between them', () => {
    for (const n of ['Pillar', 'my-pillar', 'a.b_c', 'P1', 'Node-01.eu', 'a']) {
      expect(isValidPillarName(n)).toBe(true)
    }
  })
  it('rejects empty, edge separators, doubles, spaces, symbols, and >40 chars', () => {
    for (const n of ['', '-x', 'x-', 'a--b', 'has space', 'bad!', 'a'.repeat(41)]) {
      expect(isValidPillarName(n)).toBe(false)
    }
  })
  it('rejects a worst-case non-match in linear time (no catastrophic backtracking)', () => {
    // 39 alphanumerics + a non-matching tail: the old nested-quantifier regex
    // (([a-zA-Z0-9]+[-._]?)*) explored ~2^38 partitions here and froze the UI.
    const pathological = 'a'.repeat(39) + '!'
    const start = performance.now()
    expect(isValidPillarName(pathological)).toBe(false)
    expect(performance.now() - start).toBeLessThan(100)
  })
})

// GS-12: malformed decimal strings must be rejected, not silently normalized
// ('1.2.3' used to become 1.2; '-0.5' used to become positive 0.5).
describe('toBase strictness', () => {
  it('rejects multiple dots', () => {
    expect(() => toBase('1.2.3', 8)).toThrow()
  })
  it('rejects signs', () => {
    expect(() => toBase('-0.5', 8)).toThrow()
    expect(() => toBase('+1', 8)).toThrow()
  })
  it('rejects non-numeric garbage and empty strings', () => {
    expect(() => toBase('abc', 8)).toThrow()
    expect(() => toBase('', 8)).toThrow()
    expect(() => toBase('.', 8)).toThrow()
  })
  it('still accepts well-formed values', () => {
    expect(toBase('1.5', 8)).toBe('150000000')
    expect(toBase('.5', 8)).toBe('50000000')
    expect(toBase('7', 2)).toBe('700')
    expect(toBase(' 1.5 ', 8)).toBe('150000000') // trimmed
  })
})
