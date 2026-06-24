import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings: vi.fn().mockResolvedValue({ autoReceive: false }), SetSettings: vi.fn() }))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ StartAutoReceive: vi.fn(), StopAutoReceive: vi.fn() }))
vi.mock('../../wailsjs/go/app/TxService', () => ({ PrepareSend: vi.fn(), ConfirmPublish: vi.fn(), CancelPending: vi.fn() }))
vi.mock('../../wailsjs/go/app/NomService', () => ({}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))
vi.mock('../lib/stores/node', () => ({ node: { subscribe: (f: any) => { f({ height: 9 }); return () => {} } }, initNodeEvents: vi.fn() }))
vi.mock('../lib/stores/plasma', () => ({ plasmaInfo: { subscribe: (f: any) => { f({ currentPlasma: 0, maxPlasma: 0, qsrFused: '0' }); return () => {} }, set: vi.fn() }, fusionEntries: { subscribe: (f: any) => { f([]); return () => {} } }, refreshPlasma: vi.fn(), estimatePlasma: vi.fn() }))
vi.mock('../lib/stores/pillar', () => ({ delegation: { subscribe: (f: any) => { f(null); return () => {} } }, refreshPillars: vi.fn() }))
vi.mock('../lib/stores/balances', () => ({ balances: { subscribe: (f: any) => { f([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }, { zts: 'zts1qsr', symbol: 'QSR', decimals: 8, amount: '0' }]); return () => {} } }, loadBalances: vi.fn() }))
vi.mock('../lib/stores/wallet', () => ({ wallet: { subscribe: (f: any) => { f({ accounts: [{ index: 0, address: 'z1qtest' }], active: 0, locked: false }); return () => {} } }, lock: vi.fn(), select: vi.fn(), setLabel: vi.fn() }))
import Home from './Home.svelte'
describe('Home', () => {
  it('renders balance cards, status strip, tabs; Tokens active by default; Send opens modal', async () => {
    render(Home)
    expect(screen.getByLabelText('ZNN balance').textContent).toContain('1.5')
    expect(screen.getByLabelText('QSR balance')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'tab Tokens' }).className).toContain('text-accent')
    expect(screen.getByLabelText('search tokens')).toBeTruthy()       // Tokens panel mounted
    await fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(screen.getByLabelText('recipient')).toBeTruthy()           // SendModal opened
  })
  it('exposes a Settings entry point', () => {
    render(Home)
    expect(screen.getByRole('button', { name: 'Settings' })).toBeTruthy()
  })
  it('switches to another tab and mounts its panel', async () => {
    render(Home)
    const plasmaTab = screen.getByRole('button', { name: 'tab Plasma' })
    await fireEvent.click(plasmaTab)
    expect(plasmaTab.className).toContain('text-accent')   // Plasma tab now active
    expect(screen.queryByLabelText('search tokens')).toBeNull() // Tokens panel unmounted
  })
})
