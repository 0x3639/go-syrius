<script lang="ts">
  import { onMount } from 'svelte'
  import { unreceived, loadUnreceived } from '../stores/unreceived'
  import * as Tx from '../../../wailsjs/go/app/TxService'
  import { formatAmount, shortAddress } from '../format'
  onMount(loadUnreceived)
  async function receive(hash: string) { await Tx.Receive(hash); await loadUnreceived() }
  async function receiveAll() { for (const u of $unreceived) { await Tx.Receive(u.fromHash) } await loadUnreceived() }
</script>
<div class="rounded bg-surface p-4">
  <div class="mb-2 flex items-center justify-between">
    <h2 class="text-sm text-muted">Unreceived ({$unreceived.length})</h2>
    {#if $unreceived.length}<button class="text-xs text-accent" on:click={receiveAll}>Receive all</button>{/if}
  </div>
  {#each $unreceived as u}
    <div class="flex items-center justify-between py-1 text-sm">
      <span class="font-mono">{shortAddress(u.fromAddress)}</span>
      <span class="font-mono">{formatAmount(u.amount, 8)} {u.token}</span>
      <button class="rounded bg-accent/20 px-2 py-1 text-xs text-accent" on:click={() => receive(u.fromHash)}>Receive</button>
    </div>
  {/each}
</div>
