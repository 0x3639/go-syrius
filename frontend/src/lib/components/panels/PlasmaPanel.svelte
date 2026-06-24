<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../../../wailsjs/go/app/NomService'
  import { plasmaInfo, fusionEntries, refreshPlasma, estimatePlasma } from '../../stores/plasma'
  import { tx, awaitConfirm } from '../../stores/tx'
  import { formatAmount } from '../../format'
  import Field from '../ui/Field.svelte'
  import Input from '../ui/Input.svelte'
  import Button from '../ui/Button.svelte'

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

<div class="space-y-4 p-4">
  {#if $plasmaInfo}
    <p class="text-sm text-muted">Current plasma {$plasmaInfo.currentPlasma} / {$plasmaInfo.maxPlasma} · QSR fused {formatAmount($plasmaInfo.qsrFused, 8)} QSR</p>
  {/if}

  <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Fuse Plasma</h2>
    <Field label="Beneficiary Address">
      <Input bind:value={beneficiary} placeholder="z1…" ariaLabel="beneficiary" />
    </Field>
    <Field label="Amount (QSR)" hint={estimate > 0 ? `≈ ${estimate} plasma` : 'Available / Minimum'}>
      <Input bind:value={amount} placeholder="QSR amount" ariaLabel="qsr amount" />
    </Field>
    <Button variant="primary" class="w-full" on:click={fuse}>Fuse Plasma</Button>
  </section>

  <section class="space-y-2 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Fusion entries</h2>
    {#each $fusionEntries as e}
      <div class="flex items-center justify-between text-sm">
        <span class="font-mono">{formatAmount(e.qsrAmount, 8)} QSR → {e.beneficiary.slice(0, 10)}…</span>
        <Button variant="outline" disabled={!e.isRevocable} on:click={() => cancel(e.id)} aria-label="cancel fusion">Cancel</Button>
      </div>
    {/each}
    {#if $fusionEntries.length === 0}<p class="text-xs text-muted">No fusion entries.</p>{/if}
  </section>

  {#if error}<p class="text-sm text-error" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
</div>
