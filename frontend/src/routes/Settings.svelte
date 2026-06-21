<script lang="ts">
  import { onMount } from 'svelte'
  import { wallet, changePassword, revealMnemonic } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'
  import { node, sync, getConfig, setMode, setUrl, getEmbeddedInfo, deleteEmbeddedData } from '../lib/stores/node'

  let nodeMode = 'remote'
  let remoteUrl = ''
  let localUrl = ''
  let nodeMsg = ''
  let nodeErr = ''
  let loadedMode = 'remote'
  let loadedRemote = ''
  let loadedLocal = ''
  // Track explicit user edits so the async config load can't clobber them and
  // so picking a mode never makes the (still-pristine) URL fields look "edited".
  let modeDirty = false
  let remoteDirty = false
  let localDirty = false
  let showEmbeddedConfirm = false
  let embeddedSize = 0

  async function refreshEmbedded() {
    try { embeddedSize = (await getEmbeddedInfo()).sizeBytes } catch {}
  }

  onMount(async () => {
    const c = await getConfig()
    loadedMode = c.mode
    loadedRemote = c.remoteUrl
    loadedLocal = c.localUrl
    if (!modeDirty) nodeMode = c.mode
    if (!remoteDirty) remoteUrl = c.remoteUrl
    if (!localDirty) localUrl = c.localUrl
    await refreshEmbedded()
  })

  async function applyNode() {
    nodeMsg = ''; nodeErr = ''
    // Embedded mode is gated behind an explicit warning before we ever start it.
    if (nodeMode === 'embedded' && loadedMode !== 'embedded') { showEmbeddedConfirm = true; return }
    try {
      const remoteEdited = remoteDirty && remoteUrl !== loadedRemote
      const localEdited = localDirty && localUrl !== loadedLocal
      if (remoteEdited) { await setUrl('remote', remoteUrl); loadedRemote = remoteUrl }
      if (localEdited) { await setUrl('local', localUrl); loadedLocal = localUrl }
      if (nodeMode !== loadedMode) { await setMode(nodeMode); loadedMode = nodeMode }
      else if (nodeMode === 'remote' ? remoteEdited : localEdited) { await setMode(nodeMode) }
      nodeMsg = 'Node settings applied'
    } catch (e: any) { nodeErr = e?.message ?? String(e) }
  }
  async function confirmStartEmbedded() {
    nodeMsg = ''; nodeErr = ''
    try { await setMode('embedded'); loadedMode = 'embedded'; modeDirty = false; nodeMsg = 'Node settings applied' }
    catch (e: any) { nodeErr = e?.message ?? String(e) }
    finally { showEmbeddedConfirm = false }
  }
  async function doDeleteEmbedded() {
    nodeErr = ''
    try { await deleteEmbeddedData(); await refreshEmbedded() } catch (e: any) { nodeErr = e?.message ?? String(e) }
  }
  function fmtEta(s: number): string {
    if (s <= 0) return ''
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60)
    return h > 0 ? `${h}h ${m}m` : `${m}m`
  }
  async function retryNode() {
    nodeMsg = ''; nodeErr = ''
    try { await setMode(nodeMode) } catch (e: any) { nodeErr = e?.message ?? String(e) }
  }

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

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Node</h2>
    <label class="flex items-center gap-2"><input type="radio" bind:group={nodeMode} value="remote" on:change={() => modeDirty = true} /> Remote</label>
    <input class="w-full rounded bg-bg px-3 py-2 font-mono text-sm" bind:value={remoteUrl} on:input={() => remoteDirty = true} aria-label="wss endpoint url" />
    <label class="flex items-center gap-2"><input type="radio" bind:group={nodeMode} value="local" on:change={() => modeDirty = true} /> Local</label>
    <input class="w-full rounded bg-bg px-3 py-2 font-mono text-sm" bind:value={localUrl} on:input={() => localDirty = true} aria-label="ws endpoint url" />
    <label class="flex items-center gap-2"><input type="radio" bind:group={nodeMode} value="embedded" on:change={() => modeDirty = true} /> Embedded</label>
    <p class="text-xs text-muted">Runs a full node in-app at ws://127.0.0.1:35998</p>

    {#if $node.mode === 'embedded' && $sync}
      <div class="rounded bg-bg p-3 space-y-1 text-sm">
        {#if $sync.targetHeight === 0}
          <p class="text-muted">connecting to peers…</p>
        {:else}
          <div class="h-2 w-full rounded bg-surface"><div class="h-2 rounded bg-accent" style="width:{$sync.percent}%"></div></div>
          <p>{$sync.state} · {$sync.currentHeight} / {$sync.targetHeight} ({$sync.percent.toFixed(1)}%){#if $sync.etaSeconds > 0} · ETA {fmtEta($sync.etaSeconds)}{/if}</p>
        {/if}
        <p class="text-muted">{$sync.peers} peers · {(embeddedSize / 1e9).toFixed(2)} GB on disk</p>
      </div>
    {/if}

    {#if $node.mode !== 'embedded'}
      <button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={doDeleteEmbedded}>Delete embedded data ({(embeddedSize / 1e9).toFixed(2)} GB)</button>
    {/if}

    {#if showEmbeddedConfirm}
      <div class="rounded border border-warn/40 bg-bg p-3 space-y-2">
        <p class="text-warn text-sm">Embedded mode runs a full Zenon node in-app: it needs several GB of disk and can take hours to fully sync. Continue?</p>
        <div class="flex gap-2">
          <button class="rounded bg-accent px-3 py-1 text-bg" on:click={confirmStartEmbedded}>Start embedded</button>
          <button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={() => (showEmbeddedConfirm = false)}>Cancel</button>
        </div>
      </div>
    {/if}

    <div class="flex items-center gap-3">
      <button class="rounded bg-accent px-3 py-1 text-bg" on:click={applyNode} aria-label="Apply node">Apply node</button>
      {#if !$node.connected}<button class="rounded border border-muted/40 px-3 py-1 text-muted" on:click={retryNode}>Retry</button>{/if}
    </div>
    <p class="text-xs text-muted">{$node.connected ? `Connected (${$node.mode}) · height ${$node.height}` : `Disconnected (${$node.mode})`}</p>
    {#if nodeMsg}<p class="text-success text-sm">{nodeMsg}</p>{/if}
    {#if nodeErr}<p class="text-error text-sm" role="alert">{nodeErr}</p>{/if}
  </section>
</div>
