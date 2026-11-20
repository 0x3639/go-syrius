<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { SearchIcon } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { Input, TokenIcon } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import BalanceNotice from './BalanceNotice.vue'
import TokenAmount from './TokenAmount.vue'

const balances = useBalancesStore()
const { items } = storeToRefs(balances)

const router = useRouter()
const q = ref('')

// Filter ported verbatim from the Svelte TokensPanel: empty query matches all,
// otherwise match on symbol or zts (case-insensitive substring).
const filtered = computed(() => {
  const s = q.value.trim().toLowerCase()
  return items.value.filter(
    (b) =>
      !s ||
      (b.symbol || '').toLowerCase().includes(s) ||
      (b.zts || '').toLowerCase().includes(s),
  )
})
</script>

<template>
  <div>
    <div class="flex flex-wrap items-center gap-3 px-6 py-4">
      <h2 class="section-title flex-1">In this account</h2>
      <div class="relative w-full max-w-72">
        <SearchIcon :size="15" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
        <Input v-model="q" placeholder="Filter by name or symbol" aria-label="search tokens" class="w-full pl-9" />
      </div>
    </div>
    <div v-if="!balances.available" class="px-6 pb-4"><BalanceNotice /></div>
    <template v-if="balances.available && filtered.length">
      <div v-for="b in filtered" :key="b.zts" class="list-row">
        <TokenIcon :symbol="b.symbol || b.zts" class="shrink-0" />
        <div class="min-w-0 flex-1">
          <div class="truncate font-semibold text-foreground">{{ b.symbol || b.zts }}</div>
          <div class="truncate font-mono text-xs text-muted-foreground">{{ b.zts }}</div>
        </div>
        <div class="pl-4 font-mono tabular-nums text-foreground">
          <TokenAmount :amount="b.amount" :decimals="b.decimals ?? 8" />
        </div>
        <button type="button" class="h-9 flex-none rounded-lg border border-input px-3.5 text-sm font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]" :aria-label="`send ${b.symbol || b.zts}`" @click="router?.push('/transfer')">Send</button>
      </div>
    </template>
    <p v-else-if="balances.available" class="border-t border-border/70 px-6 py-6 text-sm text-muted-foreground">{{ q.trim() ? 'No matching tokens.' : 'No tokens held.' }}</p>
  </div>
</template>
