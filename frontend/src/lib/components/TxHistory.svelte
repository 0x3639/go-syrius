<script lang="ts">
  import { txs } from '../stores/txs'
  import { formatAmount, shortAddress } from '../format'
</script>
<div class="rounded border border-border bg-surface px-4 py-3">
  <h2 class="mb-2 text-sm text-muted">Recent transactions</h2>
  {#if $txs.length === 0}<p class="text-sm text-muted">No transactions.</p>{/if}
  {#each $txs as t}
    <div class="flex items-center justify-between border-b border-border/60 py-1.5 text-sm last:border-b-0">
      <span class="{t.direction === 'send' ? 'text-error' : 'text-success'}">{t.direction}</span>
      <span class="font-mono text-muted">{shortAddress(t.counterparty)}</span>
      <span class="font-mono text-text">{formatAmount(t.amount, 8)} {t.token}</span>
    </div>
  {/each}
</div>
