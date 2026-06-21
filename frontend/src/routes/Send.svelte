<script lang="ts">
  import { balances } from '../lib/stores/balances'
  import { tx, prepare } from '../lib/stores/tx'
  import SendForm from '../lib/components/SendForm.svelte'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  function toBase(decimal: string, decimals: number): string {
    const [i, f = ''] = decimal.split('.')
    const frac = (f + '0'.repeat(decimals)).slice(0, decimals)
    return (BigInt(i || '0') * BigInt(10) ** BigInt(decimals) + BigInt(frac || '0')).toString()
  }

  async function onSend(e: CustomEvent) {
    const { recipient, zts, amountDecimal } = e.detail
    const tok = $balances.find((b) => b.zts === zts)
    const base = toBase(amountDecimal, tok?.decimals ?? 8)
    await prepare(recipient, zts, base)
  }
</script>

<div class="mx-auto mt-8 w-[28rem] space-y-4">
  <h1 class="text-xl">Send</h1>
  <SendForm on:send={onSend} />
  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
