<script lang="ts">
  import { formatAmount } from '../format'
  export let symbol = ''
  export let amount = '0'
  export let decimals = 8
  export let tint: 'green' | 'blue' = 'green'
  const tints = {
    green: 'border-accent/40 bg-accent/5',
    blue: 'border-qsr/40 bg-qsr/5',
  }
  const nums = { green: 'text-accent', blue: 'text-qsr' }
</script>

<div class="rounded border p-4 {tints[tint]}">
  <div class="text-xs font-medium uppercase tracking-wide text-muted">{symbol}</div>
  <div
    class="mt-1 font-mono text-3xl tabular-nums {nums[tint]}"
    aria-label={`${symbol} balance`}
  >
    {formatAmount(amount, decimals)}
  </div>
</div>
