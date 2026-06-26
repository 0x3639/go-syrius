import { describe, it, expect } from 'vitest'
import { plasmaLevel, plasmaColorClass } from './plasma'

describe('plasmaLevel', () => {
  it('maps plasma amounts to levels', () => {
    expect(plasmaLevel(0)).toBe('None')
    expect(plasmaLevel(1)).toBe('Low')
    expect(plasmaLevel(20999)).toBe('Low')
    expect(plasmaLevel(21000)).toBe('Medium')
    expect(plasmaLevel(83999)).toBe('Medium')
    expect(plasmaLevel(84000)).toBe('High')
  })
})

describe('plasmaColorClass', () => {
  it('goes off → red → yellow → green', () => {
    expect(plasmaColorClass('None')).toBe('text-muted-foreground')
    expect(plasmaColorClass('Low')).toBe('text-destructive')
    expect(plasmaColorClass('Medium')).toBe('text-yellow-500')
    expect(plasmaColorClass('High')).toBe('text-primary')
  })
})
