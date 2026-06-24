<script lang="ts">
  import { onMount } from 'svelte'
  import { unreceived, loadUnreceived } from '../stores/unreceived'
  import * as Tx from '../../../wailsjs/go/app/TxService'
  import { formatAmount, shortAddress } from '../format'

  onMount(loadUnreceived)

  let busy: Record<string, boolean> = {}
  let busyAll = false
  let error = ''

  async function receive(hash: string) {
    error = ''
    busy = { ...busy, [hash]: true }
    try {
      await Tx.Receive(hash)
      await loadUnreceived()
    } catch (e: any) {
      error = e?.message ?? String(e)
    } finally {
      const { [hash]: _, ...rest } = busy
      busy = rest
    }
  }

  async function receiveAll() {
    error = ''
    busyAll = true
    try {
      for (const u of $unreceived) await Tx.Receive(u.fromHash)
      await loadUnreceived()
    } catch (e: any) {
      error = e?.message ?? String(e)
    } finally {
      busyAll = false
    }
  }
</script>

<div class="rounded border border-border bg-surface px-4 py-3">
  <div class="mb-2 flex items-center justify-between">
    <h2 class="text-sm text-muted">Unreceived ({$unreceived.length})</h2>
    {#if $unreceived.length}
      <button
        class="text-xs text-accent transition-colors hover:text-success disabled:opacity-50"
        disabled={busyAll}
        on:click={receiveAll}>{busyAll ? 'Receiving…' : 'Receive all'}</button>
    {/if}
  </div>
  {#each $unreceived as u}
    <div class="flex items-center justify-between gap-2 border-b border-border/60 py-1.5 text-sm last:border-b-0">
      <span class="font-mono text-muted">{shortAddress(u.fromAddress)}</span>
      <span class="font-mono text-text">{formatAmount(u.amount, 8)} {u.token}</span>
      <button
        class="rounded bg-accent/20 px-2 py-1 text-xs text-accent transition-colors hover:bg-accent/30 disabled:cursor-not-allowed disabled:opacity-60"
        disabled={busy[u.fromHash] || busyAll}
        on:click={() => receive(u.fromHash)}>{busy[u.fromHash] ? 'Receiving…' : 'Receive'}</button>
    </div>
  {/each}
  {#if error}<p class="mt-2 text-xs text-error" role="alert">{error}</p>{/if}
  {#if (busyAll || Object.values(busy).some(Boolean))}
    <p class="mt-2 text-xs text-muted">Receiving may take a few seconds (proof-of-work) when plasma is low…</p>
  {/if}
</div>
