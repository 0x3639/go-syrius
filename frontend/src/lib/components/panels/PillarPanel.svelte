<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../../../wailsjs/go/app/NomService'
  import { pillars, delegation, pillarReward, refreshPillars } from '../../stores/pillar'
  import { tx, awaitConfirm } from '../../stores/tx'
  import { formatAmount } from '../../format'
  import Input from '../ui/Input.svelte'
  import Button from '../ui/Button.svelte'

  let search = ''
  let error = ''

  onMount(refreshPillars)
  $: rewardZero = !$pillarReward || ($pillarReward.znn === '0' && $pillarReward.qsr === '0')
  $: delegated = !!$delegation && $delegation.name !== ''
  $: filtered = ($pillars ?? [])
    .filter((p) => p.name.toLowerCase().includes(search.trim().toLowerCase()))
    .sort((a, b) => a.rank - b.rank)
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

<div class="space-y-4 p-4">
  <section class="space-y-2 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Your delegation</h2>
    {#if delegated}
      <div class="flex items-center justify-between text-sm">
        <p>Delegated to <span class="font-mono text-accent">{$delegation.name}</span> · weight <span class="font-mono">{formatAmount($delegation.weight, 8)} ZNN</span></p>
        <Button variant="outline" on:click={undelegate} aria-label="undelegate">Undelegate</Button>
      </div>
    {:else}
      <p class="text-xs text-muted">Not delegated.</p>
    {/if}
    {#if $pillarReward}
      <div class="flex items-center justify-between text-sm">
        <p class="text-muted">Uncollected reward <span class="font-mono text-text">{formatAmount($pillarReward.znn, 8)} ZNN</span> · <span class="font-mono text-text">{formatAmount($pillarReward.qsr, 8)} QSR</span></p>
        <Button variant="primary" disabled={rewardZero} on:click={collect}>Collect</Button>
      </div>
    {/if}
  </section>

  <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
    <div class="flex items-center justify-between gap-3">
      <h2 class="text-sm font-medium text-text">Pillars</h2>
      <span class="text-xs text-muted">Sorted by Rank</span>
    </div>
    <Input bind:value={search} placeholder="Search pillars" ariaLabel="search pillars" />

    <div class="space-y-1">
      {#each filtered as p}
        <div class="flex items-center justify-between gap-3 rounded-md border border-transparent px-2 py-2 hover:border-border hover:bg-elevated">
          <div class="flex min-w-0 items-baseline gap-3">
            <span class="shrink-0 font-mono text-xs text-muted">#{p.rank}</span>
            <span class="truncate text-sm text-text">{p.name}</span>
            {#if p.name === $delegation?.name}<span class="shrink-0 rounded bg-accent/15 px-1.5 py-0.5 text-[10px] font-medium text-accent">current</span>{/if}
          </div>
          <div class="flex shrink-0 items-center gap-4">
            <span class="text-xs text-accent">{p.delegateRewardPercent}% APR</span>
            <span class="font-mono text-xs text-muted">{formatAmount(p.weight, 8)} ZNN</span>
            <Button variant="primary" on:click={() => delegate(p.name)} aria-label={`delegate to ${p.name}`}>Delegate</Button>
          </div>
        </div>
      {/each}
      {#if filtered.length === 0}<p class="text-xs text-muted">No pillars.</p>{/if}
    </div>
  </section>

  {#if error}<p class="text-sm text-error" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
</div>
