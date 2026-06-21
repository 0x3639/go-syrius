<script lang="ts">
  import './app.css'
  import { onMount } from 'svelte'
  import { wallet } from './lib/stores/wallet'
  import { view } from './lib/stores/nav'
  import * as Cfg from '../wailsjs/go/app/ConfigService'
  import * as N from '../wailsjs/go/app/NodeService'
  import Unlock from './routes/Unlock.svelte'
  import Create from './routes/Create.svelte'
  import ImportMnemonic from './routes/ImportMnemonic.svelte'
  import Dashboard from './routes/Dashboard.svelte'
  import Send from './routes/Send.svelte'
  import Settings from './routes/Settings.svelte'
  onMount(async () => {
    try {
      const s = await Cfg.GetSettings()
      if (s.nodeUrl) await N.SetNode(s.nodeUrl)
    } catch {}
  })
</script>
{#if $wallet.locked && $view === 'create'}
  <Create />
{:else if $wallet.locked && $view === 'import'}
  <ImportMnemonic />
{:else if $wallet.locked}
  <Unlock />
{:else if $view === 'send'}
  <Send />
{:else if $view === 'settings'}
  <Settings />
{:else}
  <Dashboard />
{/if}
