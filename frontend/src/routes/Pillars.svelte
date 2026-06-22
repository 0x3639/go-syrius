<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { pillars, delegation, pillarReward, refreshPillars } from '../lib/stores/pillar'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let search = ''
  let error = ''

  onMount(refreshPillars)
  $: rewardZero = !$pillarReward || ($pillarReward.znn === '0' && $pillarReward.qsr === '0')
  $: delegated = !!$delegation && $delegation.name !== ''
  $: filtered = ($pillars ?? []).filter((p) => p.name.toLowerCase().includes(search.trim().toLowerCase()))
  $: if ($tx.status === 'done') refreshPillars()

  async function delegate(name: string) {
    error = ''
    try { awaitConfirm((await Nom.PrepareDelegate(name)) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function undelegate() {
    error = ''
    try { awaitConfirm((await Nom.PrepareUndelegate()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function collect() {
    error = ''
    try { awaitConfirm((await Nom.PrepareCollectPillarReward()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Pillars</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Your delegation</h2>
    {#if delegated}
      <p class="text-sm">Delegated to <span class="font-mono">{$delegation.name}</span> · weight {formatAmount($delegation.weight, 8)} ZNN</p>
      <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={undelegate}>Undelegate</button>
    {:else}
      <p class="text-xs text-muted">Not delegated.</p>
    {/if}
    {#if $pillarReward}<p class="text-sm">Uncollected reward {formatAmount($pillarReward.znn, 8)} ZNN · {formatAmount($pillarReward.qsr, 8)} QSR</p>{/if}
    <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-40" disabled={rewardZero} on:click={collect}>Collect</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Pillars</h2>
    <input class="w-full rounded bg-bg px-3 py-2" placeholder="search pillars" bind:value={search} aria-label="search pillars" />
    {#each filtered as p}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">#{p.rank} <span>{p.name}</span> · {formatAmount(p.weight, 8)} ZNN · {p.delegateRewardPercent}%{#if p.name === $delegation?.name} · current{/if}</span>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => delegate(p.name)} aria-label={`delegate to ${p.name}`}>Delegate</button>
      </div>
    {/each}
    {#if filtered.length === 0}<p class="text-xs text-muted">No pillars.</p>{/if}
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
