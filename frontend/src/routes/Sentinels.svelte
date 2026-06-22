<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { sentinel, depositedQsr, sentinelReward, refreshSentinel } from '../lib/stores/sentinel'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

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

<div class="mx-auto mt-8 w-[40rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Sentinels</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  {#if active && $sentinel}
    <section class="rounded bg-surface p-4 space-y-2">
      <h2 class="text-sm text-muted">Your sentinel</h2>
      <p class="text-sm">Status: {$sentinel.active ? 'Active' : 'Inactive'}</p>
      {#if $sentinelReward}<p class="text-sm">Uncollected reward {formatAmount($sentinelReward.znn, 8)} ZNN · {formatAmount($sentinelReward.qsr, 8)} QSR</p>{/if}
      <div class="flex gap-2">
        <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-40" disabled={rewardZero} on:click={collect}>Collect</button>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs disabled:opacity-40" disabled={!$sentinel.isRevocable} on:click={revoke} aria-label="revoke sentinel">Revoke{#if !$sentinel.isRevocable} (cooldown {$sentinel.revokeCooldown}s){/if}</button>
      </div>
    </section>
  {:else}
    <section class="rounded bg-surface p-4 space-y-2">
      <h2 class="text-sm text-muted">Register a Sentinel</h2>
      <p class="text-xs text-muted">Requires 50,000 QSR + 5,000 ZNN collateral (returned on revocation).</p>
      {#if deposited < QSR_REQUIRED}
        <p class="text-sm">Deposited {formatAmount($depositedQsr, 8)} / 50,000 QSR</p>
        <button class="rounded bg-accent px-3 py-1 text-bg" on:click={depositQsr} aria-label="deposit qsr">Deposit {formatAmount(shortfall.toString(), 8)} QSR</button>
      {:else}
        <p class="text-sm">50,000 QSR deposited. Ready to register.</p>
        <button class="rounded bg-accent px-3 py-1 text-bg" on:click={register} aria-label="register sentinel">Register Sentinel (5,000 ZNN)</button>
      {/if}
      {#if deposited > ZERO}
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={withdrawQsr} aria-label="withdraw qsr">Withdraw deposited QSR</button>
      {/if}
    </section>
  {/if}

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
