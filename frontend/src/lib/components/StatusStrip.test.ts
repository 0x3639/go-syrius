import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
vi.mock('../stores/node', () => ({ node: { subscribe: (f: any) => { f({ height: 9 }); return () => {} } } }))
vi.mock('../stores/balances', () => ({ balances: { subscribe: (f: any) => { f([{ zts: 'z', symbol: 'ZNN', decimals: 8, amount: '0' }]); return () => {} } } }))
vi.mock('../stores/plasma', () => ({ plasmaInfo: { subscribe: (f: any) => { f({ currentPlasma: 90000 }); return () => {} } } }))
vi.mock('../stores/pillar', () => ({ delegation: { subscribe: (f: any) => { f(null); return () => {} } } }))
import StatusStrip from './StatusStrip.svelte'
describe('StatusStrip', () => {
  it('renders the four stats', () => {
    render(StatusStrip)
    expect(screen.getByText('9')).toBeTruthy()        // height
    expect(screen.getByText('1')).toBeTruthy()        // tokens
    expect(screen.getByText(/High/)).toBeTruthy()     // plasma
    expect(screen.getByText('None')).toBeTruthy()     // pillar
  })
})
