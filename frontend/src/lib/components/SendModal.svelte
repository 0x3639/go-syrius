<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { balances } from '../stores/balances'
  import { tx, prepare, resetTx } from '../stores/tx'
  import SendForm from './SendForm.svelte'
  import TxModal from './TxModal.svelte'
  import TxResult from './TxResult.svelte'
  export let open = false
  const dispatch = createEventDispatcher()
  function toBase(decimal: string, decimals: number): string {
    const [i, f = ''] = decimal.split('.')
    const frac = (f + '0'.repeat(decimals)).slice(0, decimals)
    return (BigInt(i || '0') * BigInt(10) ** BigInt(decimals) + BigInt(frac || '0')).toString()
  }
  async function onSend(e: CustomEvent) {
    const { recipient, zts, amountDecimal } = e.detail
    const tok = $balances.find((b) => b.zts === zts)
    await prepare(recipient, zts, toBase(amountDecimal, tok?.decimals ?? 8))
  }
  function close() {
    resetTx()
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
        <h2 class="text-lg font-medium text-text">Send</h2>
        <button class="text-muted transition-colors hover:text-text" aria-label="close" on:click={close}>✕</button>
      </div>
      <SendForm on:send={onSend} />
      {#if $tx.status === 'preparing'}<p class="text-sm text-muted">Preparing… (PoW if required)</p>{/if}
      {#if $tx.status === 'error'}<p class="text-sm text-error" role="alert">{$tx.error}</p>{/if}
      {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
      {#if $tx.status === 'done'}<TxResult />{/if}
    </div>
  </div>
{/if}
