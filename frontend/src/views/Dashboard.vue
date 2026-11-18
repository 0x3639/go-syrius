<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { SendIcon, DownloadIcon } from '@lucide/vue'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'
import { formatAmount } from '../lib/format'
import { formatFiat } from '../lib/fiat'
import TxHistory from '../components/TxHistory.vue'
import BalanceNotice from '../components/BalanceNotice.vue'
import TokenAmount from '../components/TokenAmount.vue'

const router = useRouter()
const balances = useBalancesStore()
const price = usePriceStore()

const znn = computed(() => balances.items.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => balances.items.find((b) => b.symbol === 'QSR'))

const totalUsd = computed(() => balances.available ? price.portfolioUsd(balances.items) : null)

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
  <!-- Cap the whole page on ultra-wide windows so the hero, balance cards and
       activity card stay aligned instead of only the table centering. -->
  <div class="page-content">
    <BalanceNotice />
    <!-- Plasma hero -->
    <div class="overflow-hidden rounded-xl bg-plasma p-7 text-[#0c1f12] shadow-md">
      <!-- No price feed → the headline is just the ZNN balance; label it as
           such rather than claiming a (wrong) portfolio total. -->
      <p class="text-ledger opacity-70">{{ !balances.available ? 'BALANCE UNAVAILABLE' : totalUsd !== null ? 'TOTAL PORTFOLIO VALUE' : 'ZNN BALANCE' }}</p>
      <div class="mb-4 mt-2 break-words font-mono text-3xl font-bold tabular-nums tracking-tight lg:text-5xl">
        <template v-if="!balances.available">—</template>
        <template v-else-if="totalUsd !== null">{{ formatFiat(totalUsd) }}</template>
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
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div v-for="c in cards" :key="c.symbol" class="flex items-center gap-3.5 rounded-xl border border-border bg-card p-5">
        <div>
          <div class="text-base font-semibold text-foreground">{{ c.name }}</div>
          <div class="font-mono text-xs text-muted-foreground">{{ c.symbol }}</div>
        </div>
        <div class="ml-auto text-right">
          <div class="font-mono text-xl tabular-nums text-foreground" :aria-label="`${c.symbol} balance`">
            <TokenAmount v-if="balances.available" :amount="c.amount" :decimals="c.decimals" />
            <span v-else>—</span>
          </div>
          <div v-if="balances.available && c.fiat" class="font-mono text-xs text-muted-foreground">
            {{ c.fiat }}
          </div>
        </div>
      </div>
    </div>

    <!-- Recent activity -->
    <div class="rounded-xl border border-border bg-card">
      <div class="flex items-center px-5 pb-1.5 pt-4">
        <span class="text-base font-semibold text-foreground">Recent activity</span>
      </div>
      <div class="px-3 pb-3">
        <TxHistory />
      </div>
    </div>
  </div>
</template>
