<script lang="ts">
  import { txs } from '../stores/txs'
  import { formatAmount, shortAddress } from '../format'
</script>
<div class="rounded bg-surface p-4">
  <h2 class="mb-2 text-sm text-muted">Recent transactions</h2>
  {#if $txs.length === 0}<p class="text-muted">No transactions.</p>{/if}
  {#each $txs as t}
    <div class="flex justify-between border-b border-bg/40 py-1 text-sm">
      <span class="{t.direction === 'send' ? 'text-error' : 'text-success'}">{t.direction}</span>
      <span class="font-mono">{shortAddress(t.counterparty)}</span>
      <span class="font-mono">{formatAmount(t.amount, 8)} {t.token}</span>
    </div>
  {/each}
</div>
