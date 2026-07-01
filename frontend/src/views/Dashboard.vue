<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { SendIcon, DownloadIcon } from '@lucide/vue'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'
import { formatAmount } from '../lib/format'
import { formatFiat } from '../lib/fiat'
import TxHistory from '../components/TxHistory.vue'

const router = useRouter()
const balances = useBalancesStore()
const price = usePriceStore()

const znn = computed(() => balances.items.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => balances.items.find((b) => b.symbol === 'QSR'))

const totalUsd = computed(() => price.portfolioUsd(balances.items))

const cards = computed(() =>
  [
    { name: 'Zenon', symbol: 'ZNN', amount: znn.value?.amount ?? '0', decimals: znn.value?.decimals ?? 8 },
    { name: 'Quasar', symbol: 'QSR', amount: qsr.value?.amount ?? '0', decimals: qsr.value?.decimals ?? 8 },
  ].map((c) => {
    const usd = price.usdFor(c.symbol, c.amount, c.decimals)
    return { ...c, fiat: usd == null ? null : '≈ ' + formatFiat(usd) }
  }),
)
</script>

<template>
  <div class="flex flex-col gap-5">
    <!-- Plasma hero -->
    <div class="overflow-hidden rounded-xl bg-plasma p-7 text-[#0c1f12] shadow-md">
      <p class="text-ledger opacity-70">TOTAL PORTFOLIO VALUE</p>
      <div class="mb-4 mt-2 font-mono text-5xl font-bold tabular-nums tracking-tight">
        <template v-if="totalUsd !== null">{{ formatFiat(totalUsd) }}</template>
        <template v-else>{{ formatAmount(znn?.amount ?? '0', znn?.decimals ?? 8) }} <span class="text-2xl">ZNN</span></template>
      </div>
      <div class="flex gap-2.5">
        <button class="inline-flex h-9 items-center gap-1.5 rounded-md bg-[rgba(8,24,14,0.9)] px-4 text-sm font-semibold text-[#eafff1]" @click="router.push('/transfer')">
          <SendIcon :size="15" /> Send
        </button>
        <button class="inline-flex h-9 items-center gap-1.5 rounded-md bg-[rgba(8,24,14,0.9)] px-4 text-sm font-semibold text-[#eafff1]" @click="router.push('/receive')">
          <DownloadIcon :size="15" /> Receive
        </button>
      </div>
    </div>

    <!-- Token balances -->
    <div class="grid grid-cols-2 gap-4">
      <div v-for="c in cards" :key="c.symbol" class="flex items-center gap-3.5 rounded-xl border border-border bg-card p-5">
        <div>
          <div class="text-base font-semibold text-foreground">{{ c.name }}</div>
          <div class="font-mono text-xs text-muted-foreground">{{ c.symbol }}</div>
        </div>
        <div class="ml-auto text-right">
          <div class="font-mono text-xl tabular-nums text-foreground" :aria-label="`${c.symbol} balance`">
            {{ formatAmount(c.amount, c.decimals) }}
          </div>
          <div v-if="c.fiat" class="font-mono text-xs text-muted-foreground">
            {{ c.fiat }}
          </div>
        </div>
      </div>
    </div>

    <!-- Recent activity -->
    <div class="rounded-xl border border-border bg-card">
      <div class="flex items-center px-5 pb-1.5 pt-4">
        <span class="text-base font-semibold text-foreground">Recent activity</span>
        <button class="ml-auto text-sm text-muted-foreground transition-colors hover:text-foreground" @click="router.push('/tokens')">View all</button>
      </div>
      <div class="px-3 pb-3">
        <TxHistory />
      </div>
    </div>
  </div>
</template>
