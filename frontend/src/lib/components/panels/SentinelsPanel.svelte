<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../../../wailsjs/go/app/NomService'
  import { sentinel, depositedQsr, sentinelReward, refreshSentinel } from '../../stores/sentinel'
  import { tx, awaitConfirm } from '../../stores/tx'
  import { formatAmount } from '../../format'
  import Button from '../ui/Button.svelte'

  const QSR_REQUIRED = BigInt('5000000000000') // 50,000 QSR in base units (1e8)
  const ZERO = BigInt('0')
  let error = ''

  onMount(refreshSentinel)
  $: active = !!$sentinel && $sentinel.owner !== ''
  $: deposited = BigInt($depositedQsr ?? '0')
  $: shortfall = QSR_REQUIRED > deposited ? QSR_REQUIRED - deposited : ZERO
  $: rewardZero = !$sentinelReward || ($sentinelReward.znn === '0' && $sentinelReward.qsr === '0')
  $: if ($tx.status === 'done') refreshSentinel()

  async function depositQsr() {
    error = ''
    try { awaitConfirm((await Nom.PrepareDepositQsr(shortfall.toString())) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function register() {
    error = ''
    try { awaitConfirm((await Nom.PrepareRegisterSentinel()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function collect() {
    error = ''
    try { awaitConfirm((await Nom.PrepareCollectSentinelReward()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function revoke() {
    error = ''
    try { awaitConfirm((await Nom.PrepareRevokeSentinel()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
  async function withdrawQsr() {
    error = ''
    try { awaitConfirm((await Nom.PrepareWithdrawQsr()) as any) } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="space-y-4 p-4">
  {#if active && $sentinel}
    <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
      <h2 class="text-sm font-medium text-text">Your Sentinel</h2>
      <p class="text-sm text-muted">Status: <span class="text-text">{$sentinel.active ? 'Active' : 'Inactive'}</span></p>
      {#if $sentinelReward}
        <p class="text-sm text-muted">Uncollected reward <span class="font-mono text-text">{formatAmount($sentinelReward.znn, 8)} ZNN · {formatAmount($sentinelReward.qsr, 8)} QSR</span></p>
      {/if}
      <div class="flex flex-wrap items-center gap-2">
        <Button variant="primary" disabled={rewardZero} on:click={collect}>Collect</Button>
        <Button variant="outline" disabled={!$sentinel.isRevocable} on:click={revoke} aria-label="revoke sentinel">Revoke{#if !$sentinel.isRevocable} (cooldown {$sentinel.revokeCooldown}s){/if}</Button>
      </div>
    </section>
  {:else}
    <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
      <h2 class="text-sm font-medium text-text">Register a Sentinel</h2>
      <p class="text-xs text-muted">Requires 50,000 QSR + 5,000 ZNN collateral (returned on revocation).</p>
      {#if deposited < QSR_REQUIRED}
        <p class="text-sm text-muted">Deposited <span class="font-mono text-text">{formatAmount($depositedQsr, 8)} / 50,000 QSR</span></p>
        <Button variant="primary" class="w-full" on:click={depositQsr} aria-label="deposit qsr">Deposit {formatAmount(shortfall.toString(), 8)} QSR</Button>
      {:else}
        <p class="text-sm text-muted">50,000 QSR deposited. Ready to register.</p>
        <Button variant="primary" class="w-full" on:click={register} aria-label="register sentinel">Register Sentinel (5,000 ZNN)</Button>
      {/if}
      {#if deposited > ZERO}
        <Button variant="outline" class="w-full" on:click={withdrawQsr} aria-label="withdraw qsr">Withdraw deposited QSR</Button>
      {/if}
    </section>
  {/if}

  {#if error}<p class="text-sm text-error" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
</div>
