<script lang="ts">
  import { onMount } from 'svelte'
  import { wallet, lock } from '../lib/stores/wallet'
  import { initNodeEvents } from '../lib/stores/node'
  import { loadBalances } from '../lib/stores/balances'
  import { loadTxs } from '../lib/stores/txs'
  import AddressDisplay from '../lib/components/AddressDisplay.svelte'
  import BalanceList from '../lib/components/BalanceList.svelte'
  import TxHistory from '../lib/components/TxHistory.svelte'
  import StatusBar from '../lib/components/StatusBar.svelte'
  import AccountSwitcher from '../lib/components/AccountSwitcher.svelte'

  $: active = $wallet.accounts.find((a) => a.index === $wallet.active)
  async function refresh() { await Promise.all([loadBalances(), loadTxs()]) }
  onMount(() => { initNodeEvents(refresh); refresh() })
  $: if ($wallet.active >= 0) refresh()
</script>

<div class="mx-auto mt-8 w-[44rem] space-y-4">
  <div class="flex items-center justify-between">
    <StatusBar />
    <div class="flex items-center gap-2">
      <AccountSwitcher />
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={lock}>Lock</button>
    </div>
  </div>
  {#if active}<AddressDisplay address={active.address} />{/if}
  <BalanceList />
  <TxHistory />
</div>
