<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../../../wailsjs/go/app/NomService'
  import { tx, awaitConfirm } from '../../stores/tx'
  import { formatAmount } from '../../format'
  import Button from '../ui/Button.svelte'

  type Reward = { znn: string; qsr: string }
  const ZERO: Reward = { znn: '0', qsr: '0' }

  type Source = {
    label: string
    reward: Reward
    read: () => Promise<Reward>
    collect: () => Promise<any>
  }

  let error = ''
  let sources: Source[] = [
    { label: 'Delegation', reward: ZERO, read: () => Nom.GetPillarReward() as any, collect: () => Nom.PrepareCollectPillarReward() as any },
    { label: 'Staking', reward: ZERO, read: () => Nom.GetUncollectedReward() as any, collect: () => Nom.PrepareCollectReward() as any },
    { label: 'Sentinel', reward: ZERO, read: () => Nom.GetSentinelReward() as any, collect: () => Nom.PrepareCollectSentinelReward() as any },
  ]

  function hasReward(r: Reward): boolean {
    return r.znn !== '0' || r.qsr !== '0'
  }

  function fail(e: any) { error = e?.message ?? String(e) }

  async function load() {
    error = ''
    try {
      sources = await Promise.all(
        sources.map(async (s) => ({ ...s, reward: (await s.read()) ?? ZERO })),
      )
    } catch (e) { fail(e) }
  }

  async function collect(s: Source) {
    error = ''
    try { awaitConfirm((await s.collect()) as any) } catch (e) { fail(e) }
  }

  onMount(load)
  $: if ($tx.status === 'done') load()
</script>

<div class="space-y-4 p-4">
  <section class="space-y-2 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Uncollected rewards</h2>
    {#each sources as s}
      <div class="flex items-center justify-between gap-4 border-b border-border/60 py-2 last:border-b-0">
        <div class="text-sm">
          <p class="font-medium text-text">{s.label}</p>
          <p class="font-mono text-xs text-muted">{formatAmount(s.reward.znn, 8)} ZNN / {formatAmount(s.reward.qsr, 8)} QSR</p>
        </div>
        <Button
          variant="primary"
          disabled={!hasReward(s.reward)}
          on:click={() => collect(s)}
          aria-label={`collect ${s.label}`}
        >Collect</Button>
      </div>
    {/each}
  </section>

  {#if error}<p class="text-sm text-error" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
</div>
