<script setup lang="ts">
import { computed } from 'vue'
import { formatAmount } from '../lib/format'

const props = withDefaults(
  defineProps<{
    symbol?: string
    amount?: string
    decimals?: number
    tint?: 'green' | 'blue'
  }>(),
  { symbol: '', amount: '0', decimals: 8, tint: 'green' },
)

// nom-ui theme has no vivid `accent`/`qsr` colors: `--accent` is a subtle
// surface tint and `qsr` is undefined. Map the green tint to the brand
// `primary` and the blue tint to `info` (a vivid blue exposed by nom-ui).
const tints = {
  green: 'border-primary/40 bg-primary/5',
  blue: 'border-info/40 bg-info/5',
}
const nums = { green: 'text-primary', blue: 'text-info' }

const tintClass = computed(() => tints[props.tint])
const numClass = computed(() => nums[props.tint])
</script>

<template>
  <div class="rounded border p-4" :class="tintClass">
    <div class="text-xs font-medium uppercase tracking-wide text-muted-foreground">{{ symbol }}</div>
    <div class="mt-1 font-mono text-3xl tabular-nums" :class="numClass" :aria-label="`${symbol} balance`">
      {{ formatAmount(amount, decimals) }}
    </div>
  </div>
</template>
