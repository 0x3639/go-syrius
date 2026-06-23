<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { myTokens, lookedUpToken, refreshTokens, lookupToken } from '../lib/stores/token'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { wallet } from '../lib/stores/wallet'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

  let error = ''
  let lookupZts = ''
  // issue form
  let iName = '', iSymbol = '', iDomain = '', iTotal = '', iMax = ''
  let iDecimals = 8, iMintable = true, iBurnable = true, iUtility = false
  // mint form (per token, keyed by zts)
  let mintZts = '', mintAmount = '', mintReceiver = ''
  // burn form
  let burnAmount = ''
  // update form
  let updZts = '', updOwner = '', updDisableMint = false, updDisableBurn = false

  $: activeAddress = $wallet.accounts.find((a) => a.index === $wallet.active)?.address ?? ''

  onMount(refreshTokens)
  $: if ($tx.status === 'done') refreshTokens()

  function fail(e: any) { error = e?.message ?? String(e) }

  async function issue() {
    error = ''
    try { awaitConfirm((await Nom.PrepareIssueToken(iName, iSymbol, iDomain, iTotal, iMax, iDecimals, iMintable, iBurnable, iUtility)) as any) } catch (e) { fail(e) }
  }
  function startMint(zts: string) { mintZts = zts; mintAmount = ''; mintReceiver = activeAddress }
  async function mint() {
    error = ''
    try { awaitConfirm((await Nom.PrepareMint(mintZts, mintAmount, mintReceiver)) as any) } catch (e) { fail(e) }
  }
  async function doLookup() { error = ''; try { await lookupToken(lookupZts) } catch (e) { fail(e) } }
  async function burn(zts: string) {
    error = ''
    try { awaitConfirm((await Nom.PrepareBurn(zts, burnAmount)) as any) } catch (e) { fail(e) }
  }
  function startUpdate(zts: string, owner: string) { updZts = zts; updOwner = owner; updDisableMint = false; updDisableBurn = false }
  async function update(t: any) {
    error = ''
    try {
      awaitConfirm((await Nom.PrepareUpdateToken(updZts, updOwner, t.isMintable && !updDisableMint, t.isBurnable && !updDisableBurn)) as any)
    } catch (e) { fail(e) }
  }
</script>

<div class="mx-auto mt-8 w-[44rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Tokens</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">My tokens</h2>
    {#each $myTokens as t}
      <div class="border-b border-muted/20 py-2 text-sm space-y-1">
        <p class="font-mono">{t.symbol} · {t.name} · {formatAmount(t.totalSupply, t.decimals)}/{formatAmount(t.maxSupply, t.decimals)} · dec {t.decimals}{#if !t.isMintable} · fixed{/if}</p>
        <p class="text-xs text-muted">{t.tokenStandard}</p>
        <div class="flex flex-wrap gap-2 items-center">
          {#if t.isMintable}
            <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => startMint(t.tokenStandard)} aria-label={`mint ${t.symbol}`}>Mint</button>
          {/if}
          <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => startUpdate(t.tokenStandard, t.owner)} aria-label={`update ${t.symbol}`}>Update</button>
        </div>
        {#if mintZts === t.tokenStandard}
          <div class="flex flex-wrap gap-2 items-center pt-1">
            <input class="rounded bg-bg px-2 py-1 text-xs" placeholder="amount (base units)" bind:value={mintAmount} aria-label="mint amount" />
            <input class="rounded bg-bg px-2 py-1 text-xs w-72" placeholder="receiver" bind:value={mintReceiver} aria-label="mint receiver" />
            <button class="rounded bg-accent px-3 py-1 text-bg text-xs" on:click={mint}>Confirm mint</button>
          </div>
        {/if}
        {#if updZts === t.tokenStandard}
          <div class="flex flex-wrap gap-2 items-center pt-1">
            <input class="rounded bg-bg px-2 py-1 text-xs w-72" placeholder="new owner" bind:value={updOwner} aria-label="update owner" />
            {#if t.isMintable}<label class="text-xs"><input type="checkbox" bind:checked={updDisableMint} /> disable minting</label>{/if}
            {#if t.isBurnable}<label class="text-xs"><input type="checkbox" bind:checked={updDisableBurn} /> disable burning</label>{/if}
            <button class="rounded bg-accent px-3 py-1 text-bg text-xs" on:click={() => update(t)}>Confirm update</button>
          </div>
        {/if}
      </div>
    {/each}
    {#if $myTokens.length === 0}<p class="text-xs text-muted">No tokens owned.</p>{/if}
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Look up / burn</h2>
    <div class="flex gap-2">
      <input class="flex-1 rounded bg-bg px-3 py-2" placeholder="zts1…" bind:value={lookupZts} aria-label="lookup zts" />
      <button class="rounded border border-muted/40 px-3 py-1 text-xs" on:click={doLookup}>Look up</button>
    </div>
    {#if $lookedUpToken}
      <p class="text-sm font-mono">{$lookedUpToken.symbol} · {$lookedUpToken.name} · {$lookedUpToken.tokenStandard}</p>
      {#if $lookedUpToken.isBurnable}
        <div class="flex gap-2 items-center">
          <input class="rounded bg-bg px-2 py-1 text-xs" placeholder="amount (base units)" bind:value={burnAmount} aria-label="burn amount" />
          <button class="rounded bg-accent px-3 py-1 text-bg text-xs" on:click={() => burn($lookedUpToken.tokenStandard)}>Burn</button>
        </div>
      {:else}
        <p class="text-xs text-muted">Token is not burnable.</p>
      {/if}
    {/if}
  </section>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Issue a token (1 ZNN fee)</h2>
    <div class="grid grid-cols-2 gap-2">
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="name" bind:value={iName} aria-label="issue name" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="symbol (A-Z0-9)" bind:value={iSymbol} aria-label="issue symbol" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="domain (optional)" bind:value={iDomain} aria-label="issue domain" />
      <input class="rounded bg-bg px-2 py-1 text-sm" type="number" min="0" max="18" placeholder="decimals" bind:value={iDecimals} aria-label="issue decimals" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="total supply (base units)" bind:value={iTotal} aria-label="issue total" />
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="max supply (base units)" bind:value={iMax} aria-label="issue max" />
    </div>
    <div class="flex gap-4 text-xs">
      <label><input type="checkbox" bind:checked={iMintable} /> mintable</label>
      <label><input type="checkbox" bind:checked={iBurnable} /> burnable</label>
      <label><input type="checkbox" bind:checked={iUtility} /> utility</label>
    </div>
    <button class="rounded bg-accent px-3 py-1 text-bg" on:click={issue} aria-label="issue token">Issue token</button>
  </section>

  {#if error}<p class="text-error text-sm" role="alert">{error}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
