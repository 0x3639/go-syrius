import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Dashboard from './Dashboard.vue'
import { useNodeStore } from '../stores/node'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'
import { useUiStore } from '../stores/ui'
import { useUnreceivedStore } from '../stores/unreceived'
import { useStakeStore } from '../stores/stake'
import { useCollateralStore } from '../stores/collateral'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'

const push = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
// The dashboard refreshes earning snapshots on mount; without a backend those
// calls would wipe the fixtures set up below.
vi.mock('../../wailsjs/go/app/NomService', () => ({}))

function setup(mode: 'everyday' | 'advanced') {
  useUiStore().mode = mode
  const balances = useBalancesStore()
  balances.items = [
    { zts: 'zts1znnxxxxxxxxxxxxx9z4ulx', symbol: 'ZNN', decimals: 8, amount: '1240850319000' },
    { zts: 'zts1qsrxxxxxxxxxxxxxmrhjll', symbol: 'QSR', decimals: 8, amount: '12408500000000' },
    { zts: 'zts1customxxxxxxxxxxxxxxxx', symbol: 'LUMEN', decimals: 6, amount: '25000000000' },
  ] as any
  usePillarStore().refresh = vi.fn() as any
  // A reload "lands" at once unless a test replaces it: landing is what clears
  // a stale mark, exactly as the real stores do.
  for (const s of [useStakeStore(), useCollateralStore()] as any[]) s.refresh = vi.fn(() => { s.stale = false })
  // Staking and collateral have answered unless a test says otherwise, so an
  // empty committed cell means "nothing committed", not "still loading".
  for (const s of [useStakeStore(), useCollateralStore()]) if (s.status === 'idle') s.status = 'ready'
  return mount(Dashboard, { global: { stubs: { TxHistory: true } } })
}

describe('Dashboard', () => {
  beforeEach(() => {
    setActivePinia(createPinia()); push.mockClear()
    useNodeStore().connected = true; useBalancesStore().status = 'ready'
  })

  describe.each(['everyday', 'advanced'] as const)('%s', (mode) => {
    it('does not display zero funds or a fiat total when disconnected', () => {
      useNodeStore().connected = false
      usePriceStore().available = true
      const w = setup(mode)
      expect(w.text()).toContain('Disconnected')
      expect(w.text()).not.toContain('$0.00')
      expect(w.get('[aria-label="ZNN balance"]').text()).toBe('—')
    })

    it('lists every held asset, including tokens without a price', () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0.1; price.available = true
      const w = setup(mode)
      for (const s of ['ZNN', 'QSR', 'LUMEN']) expect(w.find(`[data-testid="holding-${s}"]`).exists()).toBe(true)
      expect(w.get('[data-testid="holding-LUMEN"]').text()).toContain('No price')
      expect(w.get('[data-testid="holding-ZNN"]').text()).toContain('≈ $')
    })

    it('separates spendable, waiting and committed amounts', async () => {
      useUnreceivedStore().items = [{ fromHash: 'h1', fromAddress: 'z1a', token: 'ZNN', amount: '4250000000', decimals: 8 }] as any
      const stake = useStakeStore(); stake.status = 'ready'
      stake.stakeInfo = { totalAmount: '50000000000', entries: [] } as any
      usePlasmaStore().info = { qsrFused: '15000000000', qsrFusedOwned: '15000000000', currentPlasma: 80000, maxPlasma: 100000 } as any
      const w = setup(mode)
      await w.vm.$nextTick()
      const znn = w.get('[data-testid="holding-ZNN"]').text()
      expect(znn).toContain('12,408.5') // spendable
      expect(znn).toContain('+42.5')    // waiting
      expect(znn).toContain('500')    // committed
      expect(znn).toContain('staked')
      expect(w.get('[data-testid="holding-QSR"]').text()).toContain('fused for plasma')
    })

    it('keeps same-symbol tokens apart and never prices a look-alike', () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0.1; price.available = true
      const w = setup(mode)
      useBalancesStore().items.push(
        { zts: 'zts1fakeznnxxxxxxxxxxxxxxx', symbol: 'ZNN', decimals: 8, amount: '99900000000' },
        { zts: 'zts1customtwoxxxxxxxxxxxxx', symbol: 'LUMEN', decimals: 6, amount: '1000000' },
      )
      return w.vm.$nextTick().then(() => {
        expect(w.findAll('[data-testid="holding-ZNN"]')).toHaveLength(2)
        expect(w.findAll('[data-testid="holding-LUMEN"]')).toHaveLength(2)
        const real = w.get('[data-zts="zts1znnxxxxxxxxxxxxx9z4ulx"]').text()
        const fake = w.get('[data-zts="zts1fakeznnxxxxxxxxxxxxxxx"]').text()
        expect(real).toContain('12,408.5')
        expect(real).toContain('≈ $')
        expect(fake).toContain('999')
        expect(fake).toContain('No price')
      })
    })

    it('matches a waiting payment by token standard, not by its display text', async () => {
      useUnreceivedStore().items = [
        // No token info from the node: `token` falls back to the standard.
        { fromHash: 'h1', fromAddress: 'z1a', token: 'zts1znnxxxxxxxxxxxxx9z4ulx', zts: 'zts1znnxxxxxxxxxxxxx9z4ulx', amount: '4250000000', decimals: 8 },
        { fromHash: 'h2', fromAddress: 'z1a', token: 'LUMEN', zts: 'zts1customxxxxxxxxxxxxxxxx', amount: '7000000', decimals: 6 },
      ] as any
      const w = setup(mode)
      await w.vm.$nextTick()
      expect(w.findAll('tbody tr')).toHaveLength(3)
      expect(w.get('[data-zts="zts1znnxxxxxxxxxxxxx9z4ulx"]').text()).toContain('+42.5')
      expect(w.get('[data-zts="zts1customxxxxxxxxxxxxxxxx"]').text()).toContain('+7')
    })

    it('reloads staking when the ZNN balance moves', async () => {
      const w = setup(mode)
      const refresh = useStakeStore().refresh as any
      refresh.mockClear()
      useBalancesStore().items = [{ zts: 'zts1znnxxxxxxxxxxxxx9z4ulx', symbol: 'ZNN', decimals: 8, amount: '1' }] as any
      await w.vm.$nextTick()
      expect(refresh).toHaveBeenCalled()
    })

    it('reloads staking when a ZNN balance row appears', async () => {
      const w = setup(mode)
      const balances = useBalancesStore()
      balances.items = balances.items.filter((b) => b.symbol !== 'ZNN') // all ZNN staked: no row
      const refresh = useStakeStore().refresh as any
      refresh.mockClear()
      balances.items = [...balances.items, { zts: 'zts1znnxxxxxxxxxxxxx9z4ulx', symbol: 'ZNN', decimals: 8, amount: '50000000000' }] as any
      await w.vm.$nextTick()
      expect(refresh).toHaveBeenCalledTimes(1)
    })

    it('never adds an old staking snapshot to a newer balance', async () => {
      const stake = useStakeStore(); stake.status = 'ready'
      stake.stakeInfo = { totalAmount: '50000000000', entries: [] } as any
      const w = setup(mode)
      await w.vm.$nextTick()
      const cells = () => w.get('[data-zts="zts1znnxxxxxxxxxxxxx9z4ulx"]').findAll('td')
      const committed = () => cells()[3].text()
      expect(committed()).toContain('500')

      // The stake is withdrawn elsewhere: the balance rises first, and the
      // staking reload is still in flight.
      let land!: () => void
      ;(stake.refresh as any).mockImplementation(() => new Promise<void>((r) => { land = r }).then(() => {
        stake.stakeInfo = { totalAmount: '0', entries: [] } as any
        stake.stale = false
      }))
      useBalancesStore().items = [{ zts: 'zts1znnxxxxxxxxxxxxx9z4ulx', symbol: 'ZNN', decimals: 8, amount: '1290850319000' }] as any
      await w.vm.$nextTick()
      // Never 500 on top of the NEW balance. Until the reload lands the row
      // keeps the previous consistent picture (old balance, old stake), marked.
      expect(cells()[1].text()).toContain('12,408.5')
      expect(cells()[1].text()).not.toContain('12,908.5')
      expect(committed()).toContain('500')
      expect(committed()).toContain('updating…')

      land()
      await new Promise((r) => setTimeout(r))
      await w.vm.$nextTick()
      expect(cells()[1].text()).toContain('12,908.5')
      expect(committed()).toBe('—')
      expect(stake.stale).toBe(false)
    })

    it('counts QSR this account fused, not plasma others fused for it', async () => {
      // 900 QSR fused for this account by someone else; it owns a 20 QSR fusion.
      usePlasmaStore().info = { qsrFused: '90000000000', qsrFusedOwned: '2000000000', currentPlasma: 80000, maxPlasma: 100000 } as any
      const w = setup(mode)
      await w.vm.$nextTick()
      const qsr = w.get('[data-zts="zts1qsrxxxxxxxxxxxxxmrhjll"]')
      expect(qsr.text()).toContain('fused for plasma')
      expect(qsr.findAll('td')[3].text()).toContain('20')
      expect(qsr.findAll('td')[3].text()).not.toContain('900')
    })

    it('shows pillar and sentinel collateral and deposits as committed, each with its amount', async () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0; price.available = true
      const c = useCollateralStore(); c.status = 'ready'
      c.info = { pillarZnn: '1500000000000', sentinelZnn: '500000000000', sentinelQsr: '5000000000000', pillarDepositedQsr: '0', sentinelDepositedQsr: '120000000000' } as any
      const w = setup(mode)
      await w.vm.$nextTick()
      // Each commitment is its own line: amount, a space, then the label.
      // (TokenAmount also renders an exact value for screen readers, hence \S*.)
      const cell = (zts: string) => w.get(`[data-zts="${zts}"]`).findAll('td')[3]
      const lines = (zts: string) => cell(zts).findAll('span.block').map((s) => s.text())
      expect(cell('zts1znnxxxxxxxxxxxxx9z4ulx').text()).toContain('20,000') // total
      expect(lines('zts1znnxxxxxxxxxxxxx9z4ulx')).toHaveLength(2)
      expect(lines('zts1znnxxxxxxxxxxxxx9z4ulx')[0]).toMatch(/^15,000\S* pillar collateral$/)
      expect(lines('zts1znnxxxxxxxxxxxxx9z4ulx')[1]).toMatch(/^5,000\S* sentinel collateral$/)
      expect(cell('zts1qsrxxxxxxxxxxxxxmrhjll').text()).toContain('51,200')
      expect(lines('zts1qsrxxxxxxxxxxxxxmrhjll')[0]).toMatch(/^50,000\S* sentinel collateral$/)
      expect(lines('zts1qsrxxxxxxxxxxxxxmrhjll')[1]).toMatch(/^1,200\S* deposited for a sentinel$/)
      expect(cell('zts1qsrxxxxxxxxxxxxxmrhjll').text()).not.toContain('deposited for a pillar') // zero entries are left out
      // 12,408.50319 spendable + 20,000 locked ZNN at $1
      expect(w.text()).toContain('$32,408.50')
    })

    it('never adds an old collateral snapshot to a newer balance', async () => {
      const c = useCollateralStore(); c.status = 'ready'
      c.info = { pillarZnn: '1500000000000', sentinelZnn: '0', sentinelQsr: '0', pillarDepositedQsr: '0', sentinelDepositedQsr: '0' } as any
      const w = setup(mode)
      await w.vm.$nextTick()
      const cells = () => w.get('[data-zts="zts1znnxxxxxxxxxxxxx9z4ulx"]').findAll('td')
      expect(cells()[3].text()).toContain('15,000')
      // The pillar is revoked elsewhere: the refund lands in the balance while
      // the collateral reload is still in flight.
      ;(c.refresh as any).mockImplementation(() => new Promise<void>(() => {}))
      useBalancesStore().items = [{ zts: 'zts1znnxxxxxxxxxxxxx9z4ulx', symbol: 'ZNN', decimals: 8, amount: '2740850319000' }] as any
      await w.vm.$nextTick()
      expect(c.stale).toBe(true)
      // Never 15,000 on top of the new 27,408.5 balance: the previous
      // consistent picture stays, marked as updating.
      expect(cells()[1].text()).toContain('12,408.5')
      expect(cells()[1].text()).not.toContain('27,408.5')
      expect(cells()[3].text()).toContain('updating…')
    })

    it('holds the estimate steady while a snapshot is being reloaded', async () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0; price.available = true
      const c = useCollateralStore(); c.status = 'ready'
      c.info = { pillarZnn: '1500000000000', sentinelZnn: '0', sentinelQsr: '0', pillarDepositedQsr: '0', sentinelDepositedQsr: '0' } as any
      const w = setup(mode)
      await w.vm.$nextTick()
      expect(w.text()).toContain('$27,408.50') // 12,408.50 spendable + 15,000 locked
      // An ordinary payment arrives: +10 ZNN. The reload is in flight.
      let land!: () => void
      ;(c.refresh as any).mockImplementation(() => new Promise<void>((r) => { land = r }).then(() => { c.stale = false }))
      useBalancesStore().items = useBalancesStore().items.map((b) => b.symbol === 'ZNN' ? { ...b, amount: '1241850319000' } : b)
      await w.vm.$nextTick()
      expect(c.stale).toBe(true)
      expect(w.text()).toContain('$27,408.50') // held: no dip to $12,418.50
      expect(w.text()).not.toContain('$12,418.50')
      land()
      await new Promise((r) => setTimeout(r))
      await w.vm.$nextTick()
      expect(w.text()).toContain('$27,418.50')
    })

    it('reloads staking and collateral on every momentum, so a late contract update is picked up', async () => {
      const w = setup(mode)
      const stake = useStakeStore().refresh as any
      const coll = useCollateralStore().refresh as any
      stake.mockClear(); coll.mockClear()
      useNodeStore().height = 8419331
      await w.vm.$nextTick()
      expect(stake).toHaveBeenCalledTimes(1)
      expect(coll).toHaveBeenCalledTimes(1)
      // Plain reloads: nothing is marked stale, so nothing flickers.
      expect(useStakeStore().stale).toBe(false)
      expect(useCollateralStore().stale).toBe(false)
      useNodeStore().connected = false
      stake.mockClear(); coll.mockClear()
      useNodeStore().height = 8419332
      await w.vm.$nextTick()
      expect(coll).not.toHaveBeenCalled()
    })

    it('pauses the per-momentum reload while the node is still syncing, then catches up', async () => {
      const w = setup(mode)
      const node = useNodeStore()
      const coll = useCollateralStore().refresh as any
      node.syncing = true
      coll.mockClear()
      node.height = 100; await w.vm.$nextTick()
      node.height = 101; await w.vm.$nextTick()
      expect(coll).not.toHaveBeenCalled()
      node.syncing = false
      await w.vm.$nextTick()
      expect(coll).toHaveBeenCalledTimes(1) // as soon as sync ends, without waiting for a tick
    })

    it('shows a read that failed during a reload, even on rows it is still holding', async () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0.1; price.available = true
      const stake = useStakeStore(); const c = useCollateralStore()
      const w = setup(mode)
      await w.vm.$nextTick()
      // A balance change marks both stale; staking stays in flight, collateral fails.
      ;(stake.refresh as any).mockImplementation(() => new Promise<void>(() => {}))
      ;(c.refresh as any).mockImplementation(() => { c.status = 'error'; c.stale = false })
      useBalancesStore().items = useBalancesStore().items.map((b) => b.symbol === 'ZNN' ? { ...b, amount: '1' } : b)
      await w.vm.$nextTick()
      expect(stake.stale).toBe(true) // the picture is still held
      for (const zts of ['zts1znnxxxxxxxxxxxxx9z4ulx', 'zts1qsrxxxxxxxxxxxxxmrhjll']) {
        expect(w.get(`[data-zts="${zts}"]`).find('[data-testid="committed-unknown"]').exists()).toBe(true)
      }
      expect(w.get('[data-testid="estimate-scope"]').text()).toMatch(/could not (all )?be checked/)
    })

    it('marks every held protocol row as updating, not only the one whose read is stale', async () => {
      const stake = useStakeStore()
      const w = setup(mode)
      await w.vm.$nextTick()
      ;(stake.refresh as any).mockImplementation(() => new Promise<void>(() => {}))
      useBalancesStore().items = useBalancesStore().items.map((b) => b.symbol === 'ZNN' ? { ...b, amount: '1' } : b)
      await w.vm.$nextTick()
      expect(useCollateralStore().stale).toBe(false)
      for (const zts of ['zts1znnxxxxxxxxxxxxx9z4ulx', 'zts1qsrxxxxxxxxxxxxxmrhjll']) {
        expect(w.get(`[data-zts="${zts}"]`).find('[data-testid="committed-pending"]').exists()).toBe(true)
      }
    })

    it('says it could not check locked amounts instead of showing none', async () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0.1; price.available = true
      useCollateralStore().status = 'error'
      const w = setup(mode)
      await w.vm.$nextTick()
      for (const zts of ['zts1znnxxxxxxxxxxxxx9z4ulx', 'zts1qsrxxxxxxxxxxxxxmrhjll']) {
        const cell = w.get(`[data-zts="${zts}"]`).findAll('td')[3]
        expect(cell.find('[data-testid="committed-unknown"]').exists()).toBe(true)
        expect(cell.text()).not.toBe('—')
      }
      const scope = w.get('[data-testid="estimate-scope"]').text()
      expect(scope).toMatch(/could not (all )?be checked/)
      expect(scope).not.toContain('including staked, fused and locked amounts')
      expect(scope).not.toContain('collateral and deposits')
    })

    it('a failed staking read marks ZNN only, and keeps what is known', async () => {
      useStakeStore().status = 'error'
      const c = useCollateralStore(); c.status = 'ready'
      c.info = { pillarZnn: '1500000000000', sentinelZnn: '0', sentinelQsr: '0', pillarDepositedQsr: '0', sentinelDepositedQsr: '0' } as any
      const w = setup(mode)
      await w.vm.$nextTick()
      const znn = w.get('[data-zts="zts1znnxxxxxxxxxxxxx9z4ulx"]').findAll('td')[3]
      expect(znn.text()).toContain('15,000')
      expect(znn.find('[data-testid="committed-unknown"]').exists()).toBe(true)
      expect(w.get('[data-zts="zts1qsrxxxxxxxxxxxxxmrhjll"]').find('[data-testid="committed-unknown"]').exists()).toBe(false)
    })

    it('says committed amounts are still loading before the first answer', async () => {
      useCollateralStore().status = 'loading'
      const w = setup(mode)
      await w.vm.$nextTick()
      expect(w.get('[data-zts="zts1qsrxxxxxxxxxxxxxmrhjll"]').findAll('td')[3].text()).toBe('updating…')
    })

    it('reloads collateral when the QSR balance moves', async () => {
      const w = setup(mode)
      const refresh = useCollateralStore().refresh as any
      refresh.mockClear()
      const balances = useBalancesStore()
      balances.items = balances.items.map((b) => b.symbol === 'QSR' ? { ...b, amount: '1' } : b)
      await w.vm.$nextTick()
      expect(refresh).toHaveBeenCalledTimes(1)
    })

    it('states what the fiat estimate covers and leaves out', () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0.1; price.available = true
      useUnreceivedStore().items = [{ fromHash: 'h1', fromAddress: 'z1a', token: 'ZNN', amount: '1', decimals: 8 }] as any
      const scope = setup(mode).get('[data-testid="estimate-scope"]').text()
      expect(scope).toContain('ZNN and QSR')
      expect(scope).toContain('LUMEN')
    })

    it('includes committed amounts in the estimate', () => {
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0; price.available = true
      const stake = useStakeStore(); stake.status = 'ready'
      stake.stakeInfo = { totalAmount: '50000000000', entries: [] } as any
      // 12,408.50319 spendable + 500 staked at $1
      expect(setup(mode).text()).toContain('$12,908.50')
    })

    it('surfaces waiting payments and opens them', async () => {
      useUnreceivedStore().items = [{ fromHash: 'h1', fromAddress: 'z1a', token: 'ZNN', amount: '4250000000', decimals: 8 }] as any
      const w = setup(mode)
      expect(w.text()).toContain('1 payment waiting to be received')
      await w.get('[data-testid="attention-incoming"]').trigger('click')
      expect(push).toHaveBeenCalledWith('/receive')
    })

    it('says it could not check incoming payments instead of showing none', async () => {
      const unreceived = useUnreceivedStore(); unreceived.lookup = 'failed'
      const price = usePriceStore(); price.znnUsd = 1; price.qsrUsd = 0.1; price.available = true
      const w = setup(mode)
      expect(w.text()).toContain("Couldn't check for incoming payments")
      expect(w.find('[data-testid="attention-incoming"]').exists()).toBe(false)
      const waiting = w.get('[data-zts="zts1znnxxxxxxxxxxxxx9z4ulx"]').findAll('td')[2]
      expect(waiting.text()).toBe('?')
      expect(w.get('[data-testid="estimate-scope"]').text()).toMatch(/any payments not yet received|could not be checked/)
      await w.get('[data-testid="attention-incoming-unknown"]').trigger('click')
      expect(push).toHaveBeenCalledWith('/receive')
    })

    it('tells a pillar owner about a pending vote', async () => {
      const pillar = usePillarStore(); pillar.myPillar = { name: 'Meridian' } as any
      useAcceleratorStore().votable = [{ needsMyVote: true }] as any
      const w = setup(mode)
      expect(w.text()).toContain('Meridian has 1 vote waiting')
      await w.get('[data-testid="attention-vote"]').trigger('click')
      expect(push).toHaveBeenCalledWith('/network/accelerator?sub=Vote')
    })
  })

  it('Everyday falls back to a ZNN headline when price is unavailable', () => {
    usePriceStore().available = false
    const w = setup('everyday')
    expect(w.text()).toContain('ZNN balance')
    expect(w.text()).not.toContain('Estimated value')
    expect(w.text()).not.toContain('≈ $')
  })

  it('Everyday keeps operator detail off Home', () => {
    const w = setup('everyday')
    expect(w.text()).toContain('All caught up')
    for (const s of ['Pinned', 'Sentinel', 'Tokens issued', 'Plasma']) expect(w.text()).not.toContain(s)
  })

  it('Everyday sends vote alerts to Advanced by name', () => {
    const pillar = usePillarStore(); pillar.myPillar = { name: 'Meridian' } as any
    useAcceleratorStore().votable = [{ needsMyVote: true }] as any
    expect(setup('everyday').get('[data-testid="attention-vote"]').text()).toBe('Open in Advanced')
  })

  it('Advanced leads with tiles the account uses and collapses the rest', () => {
    usePlasmaStore().info = { qsrFused: '15000000000', currentPlasma: 80000, maxPlasma: 100000 } as any
    const w = setup('advanced')
    expect(w.find('[data-testid="tile-plasma"]').exists()).toBe(true)
    expect(w.find('[data-testid="tile-node"]').exists()).toBe(true)
    expect(w.find('[data-testid="tile-sentinel"]').exists()).toBe(false)
    expect(w.find('[data-testid="unused-sentinel"]').exists()).toBe(true)
  })

  it('Advanced pins are editable and open their tool', async () => {
    const ui = useUiStore()
    const w = setup('advanced')
    await w.get('[data-testid="pin-fuse"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/network/plasma')
    await w.get('button[aria-expanded="false"]').trigger('click')
    await w.get('input[aria-label="Pin Vote on projects"]').setValue(true)
    expect(ui.pins).toContain('vote')
    expect(w.find('[data-testid="pin-vote"]').exists()).toBe(true)
  })

  it('has no View all button (history pages in place)', () => {
    expect(setup('everyday').text()).not.toContain('View all')
  })
})
