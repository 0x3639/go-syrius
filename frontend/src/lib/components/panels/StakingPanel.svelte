<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../../../wailsjs/go/app/NomService'
  import { stakeInfo, reward, refreshStake } from '../../stores/stake'
  import { tx, awaitConfirm } from '../../stores/tx'
  import { formatAmount } from '../../format'
  import Field from '../ui/Field.svelte'
  import Input from '../ui/Input.svelte'
  import Button from '../ui/Button.svelte'

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

<div class="space-y-4 p-4">
  {#if $stakeInfo}
    <p class="text-sm text-muted">Total staked {formatAmount($stakeInfo.totalAmount, 8)} ZNN</p>
  {/if}

  <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
    <div class="flex items-center justify-between">
      <h2 class="text-sm font-medium text-text">Uncollected reward</h2>
      {#if $reward}<span class="text-sm text-muted">{formatAmount($reward.znn, 8)} ZNN · {formatAmount($reward.qsr, 8)} QSR</span>{/if}
    </div>
    <Button variant="outline" class="w-full" disabled={rewardZero} on:click={collect}>Collect Reward</Button>
  </section>

  <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Stake ZNN</h2>
    <Field label="Amount (ZNN)" hint="Available / Minimum">
      <Input bind:value={amount} placeholder="ZNN amount (min 1)" ariaLabel="znn amount" />
    </Field>
    <Field label="Stake Duration">
      <select
        class="w-full rounded border border-border bg-elevated px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent"
        bind:value={months}
        aria-label="duration months"
      >
        {#each Array(12) as _, i}<option value={String(i + 1)}>{i + 1} Month{i ? 's' : ''}</option>{/each}
      </select>
    </Field>
    <p class="text-xs text-muted">Your ZNN will be locked for the selected duration and earn rewards until it matures.</p>
    <Button variant="primary" class="w-full" on:click={stake}>Stake ZNN</Button>
  </section>

  <section class="space-y-2 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Your stakes</h2>
    {#each ($stakeInfo?.entries ?? []) as e}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">{formatAmount(e.amount, 8)} ZNN · {e.durationMonths}mo</span>
        <Button variant="outline" disabled={!e.isMatured} on:click={() => cancel(e.id)} aria-label="cancel stake">Cancel</Button>
      </div>
    {/each}
    {#if !$stakeInfo || $stakeInfo.entries.length === 0}<p class="text-xs text-muted">No active stakes.</p>{/if}
  </section>

  {#if error}<p class="text-sm text-error" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
</div>
