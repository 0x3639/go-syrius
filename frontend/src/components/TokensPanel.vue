<script setup lang="ts">
import { ref, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { Input, TokenIcon } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import { formatAmount } from '../lib/format'

const router = useRouter()
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

// Manage routes to the /tokens placeholder (registered in Task 10).
function manage() {
  router.push('/tokens')
}
</script>

<template>
  <div class="space-y-3 p-4">
    <div class="flex items-center justify-between">
      <Input v-model="q" placeholder="Search tokens…" aria-label="search tokens" />
      <button
        class="ml-2 shrink-0 rounded border border-border px-3 py-2 text-sm text-muted-foreground hover:text-foreground"
        @click="manage"
      >
        Manage
      </button>
    </div>
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
          {{ formatAmount(b.amount, b.decimals || 8) }}
        </div>
      </div>
    </template>
    <p v-else class="text-sm text-muted-foreground">No tokens.</p>
  </div>
</template>
