<script lang="ts">
  import { onMount } from 'svelte'
  import { wallet, lock } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'
  import { initNodeEvents } from '../lib/stores/node'
  import { loadBalances } from '../lib/stores/balances'
  import { loadTxs } from '../lib/stores/txs'
  import { loadUnreceived } from '../lib/stores/unreceived'
  import * as Cfg from '../../wailsjs/go/app/ConfigService'
  import * as N from '../../wailsjs/go/app/NodeService'
  import AddressDisplay from '../lib/components/AddressDisplay.svelte'
  import BalanceList from '../lib/components/BalanceList.svelte'
  import TxHistory from '../lib/components/TxHistory.svelte'
  import StatusBar from '../lib/components/StatusBar.svelte'
  import AccountSwitcher from '../lib/components/AccountSwitcher.svelte'
  import UnreceivedPanel from '../lib/components/UnreceivedPanel.svelte'

  let autoReceive = false
  $: active = $wallet.accounts.find((a) => a.index === $wallet.active)
  async function refresh() { await Promise.all([loadBalances(), loadTxs(), loadUnreceived()]) }
  onMount(async () => {
    initNodeEvents(refresh)
    refresh()
    try { autoReceive = (await Cfg.GetSettings()).autoReceive } catch {}
  })
  $: if ($wallet.active >= 0) refresh()

  async function toggleAutoReceive() {
    try {
      const s = await Cfg.GetSettings()
      s.autoReceive = autoReceive
      await Cfg.SetSettings(s)
      if (autoReceive) await N.StartAutoReceive()
      else await N.StopAutoReceive()
    } catch {}
  }
</script>

<div class="mx-auto mt-8 w-[44rem] space-y-4">
  <div class="flex items-center justify-between">
    <StatusBar />
    <div class="flex items-center gap-2">
      <label class="flex items-center gap-1 text-xs text-muted">
        <input type="checkbox" bind:checked={autoReceive} on:change={toggleAutoReceive} />
        Auto-receive
      </label>
      <AccountSwitcher />
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-accent" on:click={() => view.set('send')}>Send</button>
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('plasma')}>Plasma</button>
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('stake')}>Staking</button>
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('settings')}>Settings</button>
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={lock}>Lock</button>
    </div>
  </div>
  {#if active}<AddressDisplay address={active.address} />{/if}
  <BalanceList />
  <UnreceivedPanel />
  <TxHistory />
</div>
