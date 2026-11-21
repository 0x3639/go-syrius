<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { SendIcon, DownloadIcon, ChevronRightIcon } from '@lucide/vue'
import { useBalancesStore } from '../stores/balances'
import { usePriceStore } from '../stores/price'
import { useNodeStore } from '../stores/node'
import { useStakeStore } from '../stores/stake'
import { usePillarStore } from '../stores/pillar'
import { useHoldings, ZNN_ZTS } from '../lib/useHoldings'
import { useAttention } from '../lib/useAttention'
import { formatFiat } from '../lib/fiat'
import TxHistory from '../components/TxHistory.vue'
import BalanceNotice from '../components/BalanceNotice.vue'
import TokenAmount from '../components/TokenAmount.vue'
import HoldingsTable from '../components/HoldingsTable.vue'
import AttentionList from '../components/AttentionList.vue'

// Everyday Home answers two questions: what do I own, and is there anything I
// need to do. Operator concerns (pillar stats, plasma levels, token issuance)
// stay out; anything that genuinely needs the user still shows in the
// attention card and leads to the right place.
const router = useRouter()
const balances = useBalancesStore()
const price = usePriceStore()
const node = useNodeStore()
const stake = useStakeStore()
const pillar = usePillarStore()
const { rows, totalUsd, unpriced, waitingCount, waitingUnknown } = useHoldings()
const { items } = useAttention()

const znn = computed(() => rows.value.find((r) => r.zts === ZNN_ZTS))
const delegatedTo = computed(() => pillar.delegation?.name || '')

// What the estimate covers, said plainly next to the number.
const scope = computed(() => {
  const parts = ['Covers ZNN and QSR, including staked, fused and locked amounts.']
  const missing: string[] = []
  if (unpriced.value.length) missing.push(`${unpriced.value.join(', ')} (no price available)`)
  if (waitingUnknown.value) missing.push('any payments not yet received')
  else if (waitingCount.value) missing.push(`${waitingCount.value} ${waitingCount.value === 1 ? 'payment' : 'payments'} not yet received`)
  if (missing.length) parts.push(`Not counted: ${missing.join(', and ')}.`)
  return parts.join(' ')
})
</script>

<template>
  <div class="page-content">
    <BalanceNotice />
    <div class="page-split">
      <div class="min-w-0 space-y-5">
        <section class="panel-list" aria-labelledby="home-own">
          <div class="flex flex-wrap items-end gap-5 p-6">
            <div class="min-w-0 flex-1 basis-72">
              <!-- No price feed → the headline is just the ZNN balance; label it as
                   such rather than claiming a (wrong) estimate. -->
              <h2 id="home-own" class="text-sm font-medium text-muted-foreground">{{ !balances.available ? 'Balance unavailable' : totalUsd !== null ? 'Estimated value of what you own' : 'ZNN balance' }}</h2>
              <div class="mt-1 break-words font-mono text-4xl font-bold tabular-nums tracking-tight text-foreground">
                <template v-if="!balances.available">—</template>
                <template v-else-if="totalUsd !== null">{{ formatFiat(totalUsd) }}</template>
                <template v-else><TokenAmount :amount="znn?.spendable ?? '0'" /> <span class="text-2xl text-muted-foreground">ZNN</span></template>
              </div>
              <p v-if="balances.available && totalUsd !== null" class="hint mt-1.5 max-w-prose" data-testid="estimate-scope">{{ scope }}</p>
              <p v-else-if="balances.available && !price.available" class="hint mt-1.5">No price feed right now, so no estimate is shown.</p>
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
          <HoldingsTable />
        </section>

        <section class="panel-list">
          <h2 class="section-title px-6 pb-1 pt-5">Recent activity</h2>
          <div class="px-3 pb-3">
            <TxHistory />
          </div>
        </section>
      </div>

      <aside class="min-w-0 space-y-5" aria-label="Needs your attention">
        <section :class="items.length ? 'panel-accent' : 'panel-card'">
          <h2 class="section-title">{{ items.length ? 'Needs your attention' : 'All caught up' }}</h2>
          <AttentionList class="mt-2" />
        </section>

        <section class="panel-card flex flex-col gap-2.5 text-sm">
          <h2 class="section-title">Earning</h2>
          <div class="flex justify-between gap-3">
            <span class="text-muted-foreground">Staked</span>
            <TokenAmount v-if="stake.available && stake.stakeInfo" :amount="stake.stakeInfo.totalAmount" symbol="ZNN" />
            <span v-else class="text-foreground">—</span>
          </div>
          <div class="flex justify-between gap-3">
            <span class="text-muted-foreground">Delegating to</span>
            <span class="truncate text-foreground">{{ !node.connected ? '—' : delegatedTo || 'Nobody yet' }}</span>
          </div>
          <button class="inline-flex items-center gap-1 self-start text-sm font-semibold text-primary hover:underline" @click="router.push('/earn')">
            Open Earn <ChevronRightIcon :size="14" />
          </button>
        </section>
      </aside>
    </div>
  </div>
</template>
