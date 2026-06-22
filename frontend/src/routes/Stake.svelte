<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { stakeInfo, reward, refreshStake } from '../lib/stores/stake'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let amount = ''
  let months = '1'
  let error = ''

  onMount(refreshStake)
  $: rewardZero = !$reward || ($reward.znn === '0' && $reward.qsr === '0')

  // ZNN has 8 decimals; convert a decimal string to base units (exact BigInt).
  function toBase(v: string): string {
    const [whole, frac = ''] = v.trim().split('.')
    const f = (frac + '00000000').slice(0, 8)
    try { return (BigInt(whole || '0') * BigInt(100000000) + BigInt(f || '0')).toString() } catch { return '0' }
  }

  async function stake() {
    error = ''
    try {
      const preview = (await Nom.PrepareStake(toBase(amount), months)) as any
      awaitConfirm(preview)
    } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function cancel(id: string) {
    error = ''
    try {
      const preview = (await Nom.PrepareCancelStake(id)) as any
      awaitConfirm(preview)
    } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function collect() {
    error = ''
    try {
      const preview = (await Nom.PrepareCollectReward()) as any
      awaitConfirm(preview)
    } catch (e: any) { error = e?.message ?? String(e) }
  }
  // After a publish completes, refresh the stake snapshot + reward.
  $: if ($tx.status === 'done') refreshStake()
</script>

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Staking</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  {#if $stakeInfo}
    <p class="text-sm text-muted">Total staked {formatAmount($stakeInfo.totalAmount, 8)} ZNN</p>
  {/if}

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Uncollected reward</h2>
    {#if $reward}<p class="text-sm">{formatAmount($reward.znn, 8)} ZNN · {formatAmount($reward.qsr, 8)} QSR</p>{/if}
    <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-40" disabled={rewardZero} on:click={collect}>Collect</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Stake ZNN</h2>
    <input class="w-full rounded bg-bg px-3 py-2" placeholder="ZNN amount (min 1)" bind:value={amount} aria-label="znn amount" />
    <label class="block text-sm text-muted">Duration
      <select class="mt-1 w-full rounded bg-bg px-3 py-2" bind:value={months} aria-label="duration months">
        {#each Array(12) as _, i}<option value={String(i + 1)}>{i + 1} month{i ? 's' : ''}</option>{/each}
      </select>
    </label>
    <button class="rounded bg-accent px-3 py-1 text-bg" on:click={stake}>Stake</button>
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Your stakes</h2>
    {#each ($stakeInfo?.entries ?? []) as e}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">{formatAmount(e.amount, 8)} ZNN · {e.durationMonths}mo</span>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs disabled:opacity-40" disabled={!e.isMatured} on:click={() => cancel(e.id)} aria-label="cancel stake">Cancel</button>
      </div>
    {/each}
    {#if !$stakeInfo || $stakeInfo.entries.length === 0}<p class="text-xs text-muted">No active stakes.</p>{/if}
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
