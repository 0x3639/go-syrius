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
  for (const s of [useStakeStore(), usePillarStore()] as any[]) s.refresh = vi.fn()
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
      const committed = () => w.get('[data-zts="zts1znnxxxxxxxxxxxxx9z4ulx"]').findAll('td')[3].text()
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
      expect(committed()).toBe('—') // not 500 on top of the new balance

      land()
      await new Promise((r) => setTimeout(r))
      await w.vm.$nextTick()
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
