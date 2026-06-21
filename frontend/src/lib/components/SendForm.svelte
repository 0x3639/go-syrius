<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { balances } from '../stores/balances'
  import AmountInput from './AmountInput.svelte'

  const dispatch = createEventDispatcher()
  export let recipient = ''
  export let zts = ''
  export let amountDecimal = ''

  $: if (!zts && $balances[0]) zts = $balances[0].zts
  // z1 bech32: starts z1, lowercase alnum, length ~40. Backend re-validates authoritatively.
  $: validAddr = /^z1[0-9a-z]{38}$/.test(recipient)
  $: validAmount = amountDecimal !== '' && Number(amountDecimal) > 0
  $: canSend = validAddr && validAmount && !!zts
</script>

<div class="space-y-3">
  <label class="block text-sm text-muted">Recipient
    <input bind:value={recipient} aria-label="recipient" placeholder="z1…"
      class="mt-1 w-full rounded bg-surface px-3 py-2 font-mono text-text outline-none focus:ring-2 focus:ring-accent" />
  </label>
  {#if recipient && !validAddr}<p class="text-xs text-error">Invalid z1 address</p>{/if}

  <label class="block text-sm text-muted">Token
    <select bind:value={zts} class="mt-1 w-full rounded bg-surface px-3 py-2 text-text">
      {#each $balances as b}<option value={b.zts}>{b.symbol || b.zts}</option>{/each}
    </select>
  </label>

  <AmountInput bind:value={amountDecimal} />

  <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!canSend}
    aria-label="Send" on:click={() => dispatch('send', { recipient, zts, amountDecimal })}>Send</button>
</div>
