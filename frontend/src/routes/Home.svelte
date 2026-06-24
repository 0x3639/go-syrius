<script lang="ts">
  import { onMount } from 'svelte'
  import { wallet, lock } from '../lib/stores/wallet'
  import { balances, loadBalances } from '../lib/stores/balances'
  import { initNodeEvents } from '../lib/stores/node'
  import { view } from '../lib/stores/nav'
  import { tx, resetTx } from '../lib/stores/tx'
  import { refreshPlasma } from '../lib/stores/plasma'
  import { refreshPillars } from '../lib/stores/pillar'
  import * as Cfg from '../../wailsjs/go/app/ConfigService'
  import * as N from '../../wailsjs/go/app/NodeService'
  import AccountSwitcher from '../lib/components/AccountSwitcher.svelte'
  import BalanceCard from '../lib/components/BalanceCard.svelte'
  import ActionCard from '../lib/components/ActionCard.svelte'
  import StatusStrip from '../lib/components/StatusStrip.svelte'
  import Tabs from '../lib/components/ui/Tabs.svelte'
  import Button from '../lib/components/ui/Button.svelte'
  import SendModal from '../lib/components/SendModal.svelte'
  import ReceiveModal from '../lib/components/ReceiveModal.svelte'
  import TokensPanel from '../lib/components/panels/TokensPanel.svelte'
  import RewardsPanel from '../lib/components/panels/RewardsPanel.svelte'
  import PlasmaPanel from '../lib/components/panels/PlasmaPanel.svelte'
  import PillarPanel from '../lib/components/panels/PillarPanel.svelte'
  import StakingPanel from '../lib/components/panels/StakingPanel.svelte'
  import SentinelsPanel from '../lib/components/panels/SentinelsPanel.svelte'
  import AcceleratorPanel from '../lib/components/panels/AcceleratorPanel.svelte'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  const TABS = ['Tokens', 'Rewards', 'Plasma', 'Pillar', 'Staking', 'Sentinels', 'Accelerator']
  let active = 'Tokens'
  let sendOpen = false
  let receiveOpen = false
  let autoReceive = false
  let prevTab = active

  $: if (active !== prevTab) { prevTab = active; resetTx() }

  function bal(sym: string) { return $balances.find((b) => b.symbol === sym) }
  async function refresh() { await Promise.all([loadBalances(), refreshPlasma(), refreshPillars()]) }
  onMount(async () => {
    initNodeEvents(refresh)
    refresh()
    try { autoReceive = (await Cfg.GetSettings()).autoReceive } catch {}
  })
  $: if ($wallet.active >= 0) refresh()
  async function toggleAutoReceive() {
    try {
      const s = await Cfg.GetSettings(); s.autoReceive = autoReceive; await Cfg.SetSettings(s)
      if (autoReceive) await N.StartAutoReceive(); else await N.StopAutoReceive()
    } catch {}
  }
</script>

<div class="mx-auto mt-6 w-[56rem] max-w-full space-y-4 px-4">
  <div class="flex items-center justify-between">
    <AccountSwitcher />
    <div class="flex items-center gap-3">
      <label class="flex items-center gap-1 text-xs text-muted">
        <input type="checkbox" bind:checked={autoReceive} on:change={toggleAutoReceive} /> Auto-receive
      </label>
      <Button variant="ghost" aria-label="Settings" on:click={() => view.set('settings')}>Settings</Button>
      <Button variant="ghost" on:click={lock}>Lock</Button>
    </div>
  </div>

  <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
    <BalanceCard symbol="ZNN" amount={bal('ZNN')?.amount ?? '0'} decimals={bal('ZNN')?.decimals ?? 8} tint="green" />
    <BalanceCard symbol="QSR" amount={bal('QSR')?.amount ?? '0'} decimals={bal('QSR')?.decimals ?? 8} tint="blue" />
    <ActionCard label="Send" direction="send" on:click={() => (sendOpen = true)} />
    <ActionCard label="Receive" direction="receive" on:click={() => (receiveOpen = true)} />
  </div>

  <StatusStrip />

  <div class="rounded border border-border bg-surface">
    <Tabs tabs={TABS} bind:active />
    {#if active === 'Tokens'}<TokensPanel />
    {:else if active === 'Rewards'}<RewardsPanel />
    {:else if active === 'Plasma'}<PlasmaPanel />
    {:else if active === 'Pillar'}<PillarPanel />
    {:else if active === 'Staking'}<StakingPanel />
    {:else if active === 'Sentinels'}<SentinelsPanel />
    {:else if active === 'Accelerator'}<AcceleratorPanel />{/if}
  </div>
</div>

<SendModal bind:open={sendOpen} />
<ReceiveModal bind:open={receiveOpen} />

<!-- SendModal renders its own TxModal/TxResult inside its overlay; suppress the
     global one while it's open so the Send confirm isn't double-rendered. -->
{#if $tx.status === 'awaiting' && $tx.preview && !sendOpen}<TxModal />{/if}
{#if $tx.status === 'done' && !sendOpen}<TxResult />{/if}
