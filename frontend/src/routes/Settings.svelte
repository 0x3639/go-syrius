<script lang="ts">
  import { wallet, changePassword, revealMnemonic } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'

  let oldP = '', newP = '', confirmP = '', cpMsg = '', cpErr = ''
  $: canChange = oldP.length > 0 && newP.length > 0 && newP === confirmP
  async function doChange() {
    cpMsg = ''; cpErr = ''
    try { await changePassword($wallet.walletName, oldP, newP); cpMsg = 'Password changed'; oldP = newP = confirmP = '' }
    catch (e: any) { cpErr = e?.message ?? String(e) }
  }

  let revealP = '', revealed = '', revErr = ''
  async function doReveal() {
    revErr = ''; revealed = ''
    try { revealed = await revealMnemonic(revealP) } catch (e: any) { revErr = e?.message ?? String(e) }
    revealP = ''
  }
  function hide() { revealed = '' }
</script>

<div class="mx-auto mt-8 w-[32rem] space-y-6">
  <div class="flex items-center justify-between"><h1 class="text-xl">Settings</h1>
    <button class="text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button></div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Change password</h2>
    <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="Current password" bind:value={oldP} aria-label="current password" />
    <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="New password" bind:value={newP} aria-label="new password" />
    <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="Confirm new password" bind:value={confirmP} aria-label="confirm new password" />
    <button class="rounded bg-accent px-3 py-1 text-bg disabled:opacity-50" disabled={!canChange} on:click={doChange}>Change</button>
    {#if cpMsg}<p class="text-success text-sm">{cpMsg}</p>{/if}
    {#if cpErr}<p class="text-error text-sm" role="alert">{cpErr}</p>{/if}
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Reveal mnemonic</h2>
    <p class="text-warn text-xs">Anyone who sees these words controls your funds. Reveal only in private.</p>
    {#if revealed}
      <div class="rounded bg-bg p-3 font-mono text-sm break-words">{revealed}</div>
      <button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={hide}>Hide</button>
    {:else}
      <input type="password" class="w-full rounded bg-bg px-3 py-2" placeholder="Password" bind:value={revealP} aria-label="reveal password" />
      <button class="rounded bg-accent px-3 py-1 text-bg" on:click={doReveal}>Reveal</button>
    {/if}
    {#if revErr}<p class="text-error text-sm" role="alert">{revErr}</p>{/if}
  </section>
</div>
