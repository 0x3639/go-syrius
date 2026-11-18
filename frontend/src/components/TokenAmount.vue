<script setup lang="ts">
import { computed } from 'vue'
import { formatAmount, formatAmountExact } from '../lib/format'
const props = withDefaults(defineProps<{ amount: string; decimals?: number; symbol?: string }>(), { decimals: 8, symbol: '' })
const exact = computed(() => `${formatAmountExact(props.amount, props.decimals)} ${props.symbol}`.trim())
</script>

<template>
  <span class="font-mono tabular-nums" :title="exact">
    <span aria-hidden="true">{{ `${formatAmount(amount, decimals)} ${symbol}`.trim() }}</span>
    <span class="sr-only">{{ exact }}</span>
  </span>
</template>
