<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { wallet } from '../stores/wallet'
  import AddressDisplay from './AddressDisplay.svelte'
  import UnreceivedPanel from './UnreceivedPanel.svelte'
  export let open = false
  const dispatch = createEventDispatcher()
  $: address = $wallet.accounts.find((a) => a.index === $wallet.active)?.address ?? ''
  function close() {
    open = false
    dispatch('close')
  }
</script>

{#if open}
  <div
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4"
    on:click|self={close}
    role="presentation"
  >
    <div class="w-[28rem] rounded-lg border border-border bg-elevated p-6 space-y-5 shadow-2xl">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-medium text-text">Receive</h2>
        <button class="text-muted transition-colors hover:text-text" aria-label="close" on:click={close}>✕</button>
      </div>
      <AddressDisplay {address} />
      <UnreceivedPanel />
    </div>
  </div>
{/if}
