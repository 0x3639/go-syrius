<script lang="ts">
  import { tx, confirm, cancel } from '../stores/tx'
  import { formatAmount, shortAddress } from '../format'
  $: p = $tx.preview
</script>
{#if p}
<div class="rounded border border-accent/40 bg-surface p-4 space-y-2" role="dialog" aria-label="Confirm transaction">
  <h2 class="text-sm text-muted">Confirm — you are signing this exact transaction</h2>
  <div class="flex justify-between"><span class="text-muted">To</span><span class="font-mono">{shortAddress(p.toAddress)}</span></div>
  <div class="flex justify-between"><span class="text-muted">Amount</span><span class="font-mono">{formatAmount(p.amount, 8)} {p.symbol || p.zts}</span></div>
  <div class="flex justify-between"><span class="text-muted">Fee</span><span>{p.needsPoW ? `PoW (difficulty ${p.difficulty})` : 'Feeless (plasma)'}</span></div>
  <div class="flex justify-between"><span class="text-muted">Hash</span><span class="font-mono text-xs break-all">{p.hash}</span></div>
  <div class="flex gap-2 pt-2">
    <button class="flex-1 rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={$tx.status === 'publishing'} on:click={confirm}>Confirm</button>
    <button class="flex-1 rounded border border-muted/40 py-2 text-muted" on:click={cancel}>Cancel</button>
  </div>
</div>
{/if}
