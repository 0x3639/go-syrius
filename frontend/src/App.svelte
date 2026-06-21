<script lang="ts">
  import './app.css'
  import { wallet } from './lib/stores/wallet'
  import Unlock from './routes/Unlock.svelte'
  import Dashboard from './routes/Dashboard.svelte'
</script>
{#if $wallet.locked}
  <Unlock />
{:else}
  <Dashboard />
{/if}
