<script setup lang="ts">
import { ref, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, TokenIcon } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import { formatAmount } from '../lib/format'

const { items } = storeToRefs(useBalancesStore())

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
  <div class="space-y-3">
    <Input v-model="q" placeholder="Filter held tokens…" aria-label="search tokens" class="w-full" />
    <template v-if="filtered.length">
      <div
        v-for="b in filtered"
        :key="b.zts"
        class="flex items-center justify-between rounded border border-border bg-card px-4 py-3"
      >
        <div class="flex min-w-0 items-center gap-3">
          <TokenIcon :symbol="b.symbol || b.zts" class="shrink-0" />
          <div class="min-w-0">
            <div class="truncate font-medium text-foreground">{{ b.symbol || b.zts }}</div>
            <div class="truncate font-mono text-xs text-muted-foreground">{{ b.zts }}</div>
          </div>
        </div>
        <div class="pl-4 font-mono tabular-nums text-foreground">
          {{ formatAmount(b.amount, b.decimals ?? 8) }}
        </div>
      </div>
    </template>
    <p v-else class="text-sm text-muted-foreground">No tokens.</p>
  </div>
</template>
