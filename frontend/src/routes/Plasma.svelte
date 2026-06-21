<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { plasmaInfo, fusionEntries, refreshPlasma, estimatePlasma } from '../lib/stores/plasma'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'
  import { formatAmount } from '../lib/format'

  let beneficiary = ''
  let amount = ''
  let estimate = 0
  let error = ''

  onMount(refreshPlasma)
  $: if (amount) estimatePlasma((amount.split('.')[0] || '0')).then((p) => (estimate = p)); else estimate = 0

  // QSR has 8 decimals; convert a decimal string to base units (exact BigInt).
  function toBase(v: string): string {
    const [whole, frac = ''] = v.trim().split('.')
    const f = (frac + '00000000').slice(0, 8)
    try { return (BigInt(whole || '0') * BigInt(100000000) + BigInt(f || '0')).toString() } catch { return '0' }
  }

  async function fuse() {
    error = ''
    try {
      const preview = (await Nom.PrepareFuse(beneficiary, toBase(amount))) as any
      awaitConfirm(preview)
    } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function cancel(id: string) {
    error = ''
    try {
      const preview = (await Nom.PrepareCancelFuse(id)) as any
      awaitConfirm(preview)
    } catch (e: any) { error = e?.message ?? String(e) }
  }
  // After a publish completes, refresh the plasma snapshot + fusion list.
  $: if ($tx.status === 'done') refreshPlasma()
</script>

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Plasma</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  {#if $plasmaInfo}
    <p class="text-sm text-muted">Current plasma {$plasmaInfo.currentPlasma} / {$plasmaInfo.maxPlasma} · QSR fused {formatAmount($plasmaInfo.qsrFused, 8)} QSR</p>
  {/if}

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Fuse QSR</h2>
    <input class="w-full rounded bg-bg px-3 py-2 font-mono text-sm" placeholder="beneficiary z1…" bind:value={beneficiary} aria-label="beneficiary" />
    <input class="w-full rounded bg-bg px-3 py-2" placeholder="QSR amount" bind:value={amount} aria-label="qsr amount" />
    {#if estimate > 0}<p class="text-xs text-muted">≈ {estimate} plasma</p>{/if}
    <button class="rounded bg-accent px-3 py-1 text-bg" on:click={fuse}>Fuse</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Fusion entries</h2>
    {#each $fusionEntries as e}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">{formatAmount(e.qsrAmount, 8)} QSR → {e.beneficiary.slice(0, 10)}…</span>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs disabled:opacity-40" disabled={!e.isRevocable} on:click={() => cancel(e.id)} aria-label="cancel fusion">Cancel</button>
      </div>
    {/each}
    {#if $fusionEntries.length === 0}<p class="text-xs text-muted">No fusion entries.</p>{/if}
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
