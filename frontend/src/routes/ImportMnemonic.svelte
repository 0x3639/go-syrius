<script lang="ts">
  import { importMnemonic, unlock } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'
  let mnemonic = ''
  let name = ''
  let password = ''
  let confirm = ''
  let error = ''
  $: wordCount = mnemonic.trim().split(/\s+/).filter(Boolean).length
  $: looksValid = wordCount === 12 || wordCount === 24
  $: canImport = looksValid && name.trim() !== '' && password.length > 0 && password === confirm
  async function doImport() {
    error = ''
    const file = name.endsWith('.dat') ? name : name + '.dat'
    try { await importMnemonic(file, password, mnemonic.trim()); await unlock(file, password) }
    catch (e: any) { error = e?.message ?? String(e) }
  }
</script>
<div class="mx-auto mt-12 w-[32rem] space-y-4">
  <h1 class="text-xl">Import from mnemonic</h1>
  <textarea class="w-full rounded bg-surface p-3 font-mono text-sm" rows="3" placeholder="word1 word2 …" bind:value={mnemonic} aria-label="mnemonic"></textarea>
  {#if mnemonic && !looksValid}<p class="text-xs text-error">Expected 12 or 24 words ({wordCount})</p>{/if}
  <label class="block text-sm text-muted">Wallet name<input class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={name} aria-label="wallet name" /></label>
  <label class="block text-sm text-muted">Password<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={password} aria-label="password" /></label>
  <label class="block text-sm text-muted">Confirm<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={confirm} aria-label="confirm password" /></label>
  <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!canImport} on:click={doImport} aria-label="Import">Import</button>
  <button class="text-xs text-muted" on:click={() => view.set('unlock')}>Cancel</button>
  {#if error}<p class="text-error" role="alert">{error}</p>{/if}
</div>
