import { computed } from 'vue'
import { useBalancesStore } from '../stores/balances'
import { useUnreceivedStore } from '../stores/unreceived'
import { useStakeStore } from '../stores/stake'
import { usePlasmaStore } from '../stores/plasma'
import { usePriceStore } from '../stores/price'
import { useNodeStore } from '../stores/node'

// Protocol token standards. Identity is always the ZTS, never the symbol:
// symbols are not unique on chain, so a custom token may share one.
export const ZNN_ZTS = 'zts1znnxxxxxxxxxxxxx9z4ulx'
export const QSR_ZTS = 'zts1qsrxxxxxxxxxxxxxmrhjll'
const PROTOCOL: Record<string, { symbol: string; name: string }> = {
  [ZNN_ZTS]: { symbol: 'ZNN', name: 'Zenon' },
  [QSR_ZTS]: { symbol: 'QSR', name: 'Quasar' },
}

export type Commitment = { label: string; amount: string }
export type Holding = {
  /** Token standard: the row's identity. */
  zts: string
  symbol: string
  name: string
  decimals: number
  /** Protocol asset (ZNN/QSR by token standard) as opposed to a custom token. */
  protocol: boolean
  spendable: string
  /** Sent to this account but not yet received. */
  waiting: string
  /** Owned but not spendable right now: staked ZNN, QSR this account fused. */
  committed: Commitment[]
  committedTotal: string
  /** Fiat estimate of spendable + committed; null when the asset has no price. */
  usd: number | null
}

export function sumBase(values: (string | undefined | null)[]): string {
  let total = 0n
  for (const v of values) { try { total += BigInt(v || '0') } catch { /* ignore malformed */ } }
  return total.toString()
}

// One row per token standard the account has anything to do with: spendable
// balance, payments waiting to be received, and amounts committed to staking
// or fusion. ZNN and QSR always appear, including at zero. Only the protocol
// rows are priced and only they carry staked/fused amounts, so a custom token
// that reuses a protocol symbol can never borrow their value. Pillar and
// sentinel collateral is not included: the stores do not expose it yet.
export function useHoldings() {
  const balances = useBalancesStore()
  const unreceived = useUnreceivedStore()
  const stake = useStakeStore()
  const plasma = usePlasmaStore()
  const price = usePriceStore()
  const node = useNodeStore()

  const rows = computed<Holding[]>(() => {
    const byZts = new Map<string, Holding>()
    const row = (zts: string, symbol: string, decimals: number): Holding => {
      let r = byZts.get(zts)
      if (!r) {
        const p = PROTOCOL[zts]
        r = { zts, symbol: p?.symbol ?? symbol, name: p?.name ?? symbol, decimals, protocol: !!p, spendable: '0', waiting: '0', committed: [], committedTotal: '0', usd: null }
        byZts.set(zts, r)
      }
      return r
    }
    row(ZNN_ZTS, 'ZNN', 8); row(QSR_ZTS, 'QSR', 8)
    for (const b of balances.items) {
      const r = row(b.zts, b.symbol, b.decimals)
      r.decimals = b.decimals
      r.spendable = sumBase([r.spendable, b.amount])
    }
    if (node.connected) {
      for (const u of unreceived.items) {
        // Identity is the token standard. `token` is display text that falls
        // back to the standard when the node has no token info; an older
        // backend without `zts` is matched by symbol only when unambiguous.
        let zts = u.zts || (u.token.startsWith('zts1') ? u.token : '')
        if (!zts) {
          const same = [...byZts.values()].filter((r) => r.symbol === u.token)
          zts = same.length === 1 ? same[0].zts : `symbol:${u.token}`
        }
        const r = row(zts, u.token, u.decimals ?? 8)
        r.waiting = sumBase([r.waiting, u.amount])
      }
      // Skipped while stale: a snapshot older than the balance would count
      // the same ZNN as both spendable and staked.
      const staked = stake.available && !stake.stale ? stake.stakeInfo?.totalAmount : undefined
      if (staked && staked !== '0') row(ZNN_ZTS, 'ZNN', 8).committed.push({ label: 'staked', amount: staked })
      // Owner side: QSR this account fused and can cancel, whoever benefits.
      // Not `qsrFused`, which is plasma fused FOR this account by anyone.
      const fused = plasma.info?.qsrFusedOwned
      if (fused && fused !== '0') row(QSR_ZTS, 'QSR', 8).committed.push({ label: 'fused for plasma', amount: fused })
    }
    for (const r of byZts.values()) {
      r.committedTotal = sumBase(r.committed.map((c) => c.amount))
      r.usd = r.protocol ? price.usdFor(r.symbol, sumBase([r.spendable, r.committedTotal]), r.decimals) : null
    }
    return [...byZts.values()]
  })

  /** Fiat total, or null without a price feed or loaded balances. */
  const totalUsd = computed(() => {
    if (!balances.available || !price.available) return null
    return rows.value.reduce((t, r) => t + (r.usd ?? 0), 0)
  })
  /** Assets the estimate leaves out because they have no price. */
  const unpriced = computed(() => [...new Set(rows.value.filter((r) => r.usd === null && (r.spendable !== '0' || r.committedTotal !== '0')).map((r) => r.symbol))])
  const waitingCount = computed(() => (node.connected ? unreceived.items.length : 0))

  return { rows, totalUsd, unpriced, waitingCount }
}
