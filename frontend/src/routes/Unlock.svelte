<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte'
  import * as W from '../../wailsjs/go/app/WalletService'
  import { unlock } from '../lib/stores/wallet'
  import PasswordInput from '../lib/components/PasswordInput.svelte'
  import WalletPicker from '../lib/components/WalletPicker.svelte'

  const dispatch = createEventDispatcher()
  let wallets: { name: string; baseAddress: string }[] = []
  let selected = ''
  let password = ''
  let error = ''
  let busy = false

  async function refresh() {
    wallets = ((await W.ListWallets()) ?? []) as { name: string; baseAddress: string }[]
    if (!selected && wallets[0]) selected = wallets[0].name
  }
  onMount(refresh)

  async function doUnlock() {
    error = ''; busy = true
    try { await unlock(selected, password); dispatch('unlocked') }
    catch (e: any) { error = e?.message ?? String(e) }
    finally { busy = false; password = '' }
  }

  async function doImport() {
    error = ''
    try {
      const path = await W.PickKeystoreFile()
      if (!path) return            // cancelled
      await W.ImportKeystore(path)
      await refresh()
    } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-16 w-[28rem] space-y-4">
  <h1 class="text-xl">Unlock wallet</h1>
  {#if wallets.length === 0}
    <p class="text-muted">No wallets yet. Import a keystore to begin.</p>
  {:else}
    <WalletPicker {wallets} bind:selected />
    <PasswordInput bind:value={password} />
    <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50"
      disabled={busy || !selected} on:click={doUnlock} aria-label="Unlock">Unlock</button>
  {/if}
  <button class="w-full rounded border border-muted/40 py-2 text-muted" on:click={doImport}>Import keystore…</button>
  {#if error}<p class="text-error" role="alert">{error}</p>{/if}
</div>
