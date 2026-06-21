<script lang="ts">
  import { balances } from '../stores/balances'
  import { formatAmount } from '../format'
</script>
<div class="rounded bg-surface p-4">
  <h2 class="mb-2 text-sm text-muted">Balances</h2>
  {#each $balances as b}
    <div class="flex justify-between py-1">
      <span>{b.symbol || b.zts}</span>
      <span class="font-mono">{formatAmount(b.amount, b.decimals || 8)}</span>
    </div>
  {/each}
</div>
