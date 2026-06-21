<script lang="ts">
  import './app.css'
  import { onMount } from 'svelte'
  import { wallet } from './lib/stores/wallet'
  import * as Cfg from '../wailsjs/go/app/ConfigService'
  import * as N from '../wailsjs/go/app/NodeService'
  import Unlock from './routes/Unlock.svelte'
  import Dashboard from './routes/Dashboard.svelte'
  onMount(async () => {
    try {
      const s = await Cfg.GetSettings()
      if (s.nodeUrl) await N.SetNode(s.nodeUrl)
    } catch {}
  })
</script>
{#if $wallet.locked}
  <Unlock />
{:else}
  <Dashboard />
{/if}
