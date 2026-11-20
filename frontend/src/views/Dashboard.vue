<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { SendIcon, DownloadIcon, ZapIcon } from '@lucide/vue'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'
import { useNodeStore } from '../stores/node'
import { useUnreceivedStore } from '../stores/unreceived'
import { usePlasmaStore } from '../stores/plasma'
import { useStakeStore } from '../stores/stake'
import { usePillarStore } from '../stores/pillar'
import { useSentinelStore } from '../stores/sentinel'
import { formatAmount, shortAddress } from '../lib/format'
import { formatFiat } from '../lib/fiat'
import { plasmaLevel } from '../lib/plasma'
import { useNodeRefresh } from '../lib/useNodeRefresh'
import { useWalletStore } from '../stores/wallet'
import TxHistory from '../components/TxHistory.vue'
import BalanceNotice from '../components/BalanceNotice.vue'
import TokenAmount from '../components/TokenAmount.vue'

const router = useRouter()
const balances = useBalancesStore()
const price = usePriceStore()
const node = useNodeStore()
const unreceived = useUnreceivedStore()
const plasma = usePlasmaStore()
const stake = useStakeStore()
const pillar = usePillarStore()
const sentinel = useSentinelStore()

const wallet = useWalletStore()

// The shell's per-momentum refresh covers balances, plasma and delegation but
// not staking or reward snapshots, which only their own panels load. Fetch
// them here — on mount, on reconnect and on account switch — so the Earning
// card is not blank until the user happens to open Staking or Rewards.
function refreshEarning() {
  if (!node.connected) return
  stake.refresh()
  pillar.refresh()
  sentinel.refresh()
}
useNodeRefresh(refreshEarning)
watch(() => wallet.activeIndex, refreshEarning)

const znn = computed(() => balances.items.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => balances.items.find((b) => b.symbol === 'QSR'))

const totalUsd = computed(() => balances.available ? price.portfolioUsd(balances.items) : null)

const cards = computed(() =>
  [
    { name: 'Zenon', symbol: 'ZNN', amount: znn.value?.amount ?? '0', decimals: znn.value?.decimals ?? 8, tint: 'bg-primary/15 text-primary' },
    { name: 'Quasar', symbol: 'QSR', amount: qsr.value?.amount ?? '0', decimals: qsr.value?.decimals ?? 8, tint: 'bg-info/15 text-info' },
  ].map((c) => {
    const usd = price.usdFor(c.symbol, c.amount, c.decimals)
    return { ...c, fiat: usd == null ? null : '≈ ' + formatFiat(usd) }
  }),
)

// "Needs attention" column. Everything here is a read of data the shell
// already keeps fresh; the actions only navigate (or reuse receiveAll).
const waiting = computed(() => (node.connected ? unreceived.items : []))

const plasmaLvl = computed(() => plasmaLevel(plasma.info?.currentPlasma ?? 0))
const plasmaKnown = computed(() => node.connected && plasma.info !== null)
const plasmaPercent = computed(() => {
  const max = plasma.info?.maxPlasma ?? 0
  if (max <= 0) return 0
  return Math.max(0, Math.min(100, ((plasma.info?.currentPlasma ?? 0) / max) * 100))
})
const plasmaTone = computed(() => plasmaLvl.value === 'High' || plasmaLvl.value === 'Medium'
  ? { pill: 'bg-success/15 text-success', bar: 'bg-success' }
  : { pill: 'bg-warning/15 text-warning', bar: 'bg-warning' })
const plasmaHint = computed(() => plasmaLvl.value === 'High' || plasmaLvl.value === 'Medium'
  ? 'Your next transaction will be sent instantly.'
  : 'Your next transaction needs a short proof-of-work first. Fuse QSR to make it instant.')

function sum(values: (string | undefined)[]): string {
  let total = 0n
  for (const v of values) { try { total += BigInt(v || '0') } catch { /* ignore malformed */ } }
  return total.toString()
}
const rewardZnn = computed(() => sum([pillar.reward?.znn, sentinel.reward?.znn]))
const rewardQsr = computed(() => sum([pillar.reward?.qsr, stake.reward?.qsr, sentinel.reward?.qsr]))
const hasRewards = computed(() => rewardZnn.value !== '0' || rewardQsr.value !== '0')
const delegatedTo = computed(() => pillar.delegation?.name || '')
</script>

<template>
  <!-- Two columns on a wide content area: balances + activity on the left,
       things that need the user's attention on the right. -->
  <div class="page-content">
    <BalanceNotice />
    <div class="page-split">
      <div class="min-w-0 space-y-5">
        <!-- Balance -->
        <section class="panel-list">
          <div class="flex flex-wrap items-end gap-5 p-6">
            <div class="min-w-0 flex-1">
              <!-- No price feed → the headline is just the ZNN balance; label it as
                   such rather than claiming a (wrong) portfolio total. -->
              <p class="text-sm font-medium text-muted-foreground">{{ !balances.available ? 'Balance unavailable' : totalUsd !== null ? 'Total portfolio value' : 'ZNN balance' }}</p>
              <div class="mt-1 break-words font-mono text-4xl font-bold tabular-nums tracking-tight text-foreground @3xl:text-5xl">
                <template v-if="!balances.available">—</template>
                <template v-else-if="totalUsd !== null">{{ formatFiat(totalUsd) }}</template>
                <template v-else>{{ formatAmount(znn?.amount ?? '0', znn?.decimals ?? 8) }} <span class="text-2xl text-muted-foreground">ZNN</span></template>
              </div>
            </div>
            <div class="flex gap-2.5">
              <button class="inline-flex h-11 items-center gap-2 rounded-xl bg-primary px-5 text-sm font-bold text-primary-foreground transition-opacity hover:opacity-90" @click="router.push('/transfer')">
                <SendIcon :size="16" /> Send
              </button>
              <button class="inline-flex h-11 items-center gap-2 rounded-xl border border-input px-5 text-sm font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]" @click="router.push('/receive')">
                <DownloadIcon :size="16" /> Receive
              </button>
            </div>
          </div>
          <div class="grid grid-cols-1 border-t border-border @xl:grid-cols-2">
            <div v-for="(c, i) in cards" :key="c.symbol" class="flex items-center gap-3 px-6 py-4" :class="i > 0 ? 'border-t border-border @xl:border-l @xl:border-t-0' : ''">
              <span class="avatar-dot" :class="c.tint">{{ c.symbol }}</span>
              <div class="min-w-0 flex-1">
                <div class="font-semibold text-foreground">{{ c.name }}</div>
                <div v-if="balances.available && c.fiat" class="text-xs text-muted-foreground">{{ c.fiat }}</div>
              </div>
              <div class="font-mono text-lg tabular-nums text-foreground" :aria-label="`${c.symbol} balance`">
                <TokenAmount v-if="balances.available" :amount="c.amount" :decimals="c.decimals" />
                <span v-else>—</span>
              </div>
            </div>
          </div>
        </section>

        <!-- Recent activity -->
        <section class="panel-list">
          <h2 class="section-title px-6 pb-1 pt-5">Recent activity</h2>
          <div class="px-3 pb-3">
            <TxHistory />
          </div>
        </section>
      </div>

      <aside class="min-w-0 space-y-5" aria-label="Needs attention">
        <section v-if="waiting.length" class="panel-accent flex flex-col gap-3">
          <h2 class="section-title">{{ waiting.length }} {{ waiting.length === 1 ? 'payment' : 'payments' }} waiting</h2>
          <div v-for="u in waiting.slice(0, 3)" :key="u.fromHash" class="flex items-center justify-between gap-3 text-sm">
            <span class="truncate font-mono text-xs text-muted-foreground">{{ shortAddress(u.fromAddress) }}</span>
            <span class="whitespace-nowrap font-mono text-primary">+<TokenAmount :amount="u.amount" :decimals="u.decimals ?? 8" :symbol="u.token" /></span>
          </div>
          <p v-if="waiting.length > 3" class="hint">and {{ waiting.length - 3 }} more</p>
          <button class="h-11 rounded-xl bg-primary text-sm font-bold text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-50" :disabled="unreceived.busyAll" @click="unreceived.receiveAll()">
            {{ unreceived.busyAll ? 'Receiving…' : 'Receive all' }}
          </button>
          <p v-if="unreceived.error" class="text-xs text-destructive" role="alert">{{ unreceived.error }}</p>
        </section>

        <section class="panel-card flex flex-col gap-2.5">
          <div class="flex items-center gap-2">
            <h2 class="section-title flex-1">Plasma</h2>
            <span v-if="plasmaKnown" class="status-pill" :class="plasmaTone.pill">{{ plasmaLvl }}</span>
          </div>
          <template v-if="plasmaKnown">
            <div class="h-2 overflow-hidden rounded-full bg-foreground/10" aria-hidden="true">
              <div class="h-full rounded-full" :class="plasmaTone.bar" :style="{ width: `${plasmaPercent}%` }"></div>
            </div>
            <p class="hint">{{ plasmaHint }}</p>
          </template>
          <p v-else class="hint">{{ node.connected ? 'Loading plasma…' : 'Shown once the wallet is connected to a node.' }}</p>
          <button class="inline-flex items-center gap-1.5 self-start text-sm font-semibold text-primary hover:underline" @click="router.push('/network/plasma')">
            <ZapIcon :size="14" /> Fuse plasma
          </button>
        </section>

        <section class="panel-card flex flex-col gap-2.5 text-sm">
          <h2 class="section-title">Earning</h2>
          <div class="flex justify-between gap-3">
            <span class="text-muted-foreground">Staked</span>
            <TokenAmount v-if="stake.available && stake.stakeInfo" :amount="stake.stakeInfo.totalAmount" symbol="ZNN" />
            <span v-else class="text-foreground">—</span>
          </div>
          <div class="flex justify-between gap-3">
            <span class="text-muted-foreground">Delegated to</span>
            <span class="truncate text-foreground">{{ !node.connected ? '—' : delegatedTo || 'Nobody yet' }}</span>
          </div>
          <div class="flex justify-between gap-3 border-t border-border pt-2.5">
            <span class="text-muted-foreground">Rewards to collect</span>
            <span v-if="node.connected" :class="hasRewards ? 'text-primary' : 'text-foreground'">
              <TokenAmount :amount="rewardZnn" symbol="ZNN" /> · <TokenAmount :amount="rewardQsr" symbol="QSR" />
            </span>
            <span v-else class="text-foreground">—</span>
          </div>
          <button class="mt-1 h-10 rounded-xl border border-input text-sm font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]" @click="router.push('/network/rewards')">
            {{ node.connected && hasRewards ? 'Collect rewards' : 'View rewards' }}
          </button>
        </section>
      </aside>
    </div>
  </div>
</template>
