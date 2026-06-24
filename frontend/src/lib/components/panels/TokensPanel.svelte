<script lang="ts">
  import { balances } from '../../stores/balances'
  import { formatAmount } from '../../format'
  import Input from '../ui/Input.svelte'
  let q = ''
  $: filtered = $balances.filter((b) => {
    const s = q.trim().toLowerCase()
    return !s || (b.symbol || '').toLowerCase().includes(s) || (b.zts || '').toLowerCase().includes(s)
  })
</script>
<div class="space-y-3 p-4">
  <Input bind:value={q} placeholder="Search tokens…" ariaLabel="search tokens" />
  {#each filtered as b}
    <div class="flex items-center justify-between rounded border border-border bg-surface px-4 py-3">
      <div class="min-w-0">
        <div class="font-medium truncate">{b.symbol || b.zts}</div>
        <div class="text-xs text-muted font-mono truncate">{b.zts}</div>
      </div>
      <div class="font-mono tabular-nums pl-4">{formatAmount(b.amount, b.decimals || 8)}</div>
    </div>
  {:else}
    <p class="text-sm text-muted">No tokens.</p>
  {/each}
</div>
