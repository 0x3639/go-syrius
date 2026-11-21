import { computed } from 'vue'
import { useBalancesStore } from '../stores/balances'
import { useUnreceivedStore } from '../stores/unreceived'
import { useStakeStore } from '../stores/stake'
import { usePlasmaStore } from '../stores/plasma'
import { usePriceStore } from '../stores/price'
import { useNodeStore } from '../stores/node'

export type Commitment = { label: string; amount: string }
export type Holding = {
  zts: string
  symbol: string
  name: string
  decimals: number
  /** Protocol asset (ZNN/QSR) as opposed to a custom token. */
  protocol: boolean
  spendable: string
  /** Sent to this account but not yet received. */
  waiting: string
  /** Owned but not spendable right now: staked ZNN, fused QSR. */
  committed: Commitment[]
  committedTotal: string
  /** Fiat estimate of spendable + committed; null when the asset has no price. */
  usd: number | null
}

const NAMES: Record<string, string> = { ZNN: 'Zenon', QSR: 'Quasar' }

export function sumBase(values: (string | undefined | null)[]): string {
  let total = 0n
  for (const v of values) { try { total += BigInt(v || '0') } catch { /* ignore malformed */ } }
  return total.toString()
}

// One row per asset the account has anything to do with: spendable balance,
// payments waiting to be received, and amounts committed to staking or fusion.
// ZNN and QSR always appear, including at zero. Pillar and sentinel collateral
// is not included: the stores do not expose it per account yet.
export function useHoldings() {
  const balances = useBalancesStore()
  const unreceived = useUnreceivedStore()
  const stake = useStakeStore()
  const plasma = usePlasmaStore()
  const price = usePriceStore()
  const node = useNodeStore()

  const rows = computed<Holding[]>(() => {
    const bySymbol = new Map<string, Holding>()
    const row = (symbol: string, decimals = 8, zts = ''): Holding => {
      let r = bySymbol.get(symbol)
      if (!r) {
        r = { zts, symbol, name: NAMES[symbol] ?? symbol, decimals, protocol: symbol in NAMES, spendable: '0', waiting: '0', committed: [], committedTotal: '0', usd: null }
        bySymbol.set(symbol, r)
      }
      return r
    }
    row('ZNN'); row('QSR')
    for (const b of balances.items) {
      const r = row(b.symbol, b.decimals, b.zts)
      r.zts = b.zts; r.decimals = b.decimals; r.spendable = b.amount
    }
    if (node.connected) {
      for (const u of unreceived.items) {
        const r = row(u.token, u.decimals ?? 8)
        r.waiting = sumBase([r.waiting, u.amount])
      }
      const staked = stake.available ? stake.stakeInfo?.totalAmount : undefined
      if (staked && staked !== '0') row('ZNN').committed.push({ label: 'staked', amount: staked })
      const fused = plasma.info?.qsrFused
      if (fused && fused !== '0') row('QSR').committed.push({ label: 'fused for plasma', amount: fused })
    }
    for (const r of bySymbol.values()) {
      r.committedTotal = sumBase(r.committed.map((c) => c.amount))
      r.usd = price.usdFor(r.symbol, sumBase([r.spendable, r.committedTotal]), r.decimals)
    }
    return [...bySymbol.values()]
  })

  /** Fiat total, or null without a price feed or loaded balances. */
  const totalUsd = computed(() => {
    if (!balances.available || !price.available) return null
    return rows.value.reduce((t, r) => t + (r.usd ?? 0), 0)
  })
  /** Assets the estimate leaves out because they have no price. */
  const unpriced = computed(() => rows.value.filter((r) => r.usd === null && (r.spendable !== '0' || r.committedTotal !== '0')).map((r) => r.symbol))
  const waitingCount = computed(() => (node.connected ? unreceived.items.length : 0))

  return { rows, totalUsd, unpriced, waitingCount }
}
