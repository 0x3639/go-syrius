<script lang="ts">
  import { onMount } from 'svelte'
  import { generateMnemonic, importMnemonic, unlock } from '../lib/stores/wallet'
  import { view } from '../lib/stores/nav'

  let step = 1
  let mnemonic = ''
  let words: string[] = []
  let error = ''

  // backup-verify: 3 deterministic positions spread across the phrase
  let positions: number[] = []
  let answers: Record<number, string> = {}

  let name = ''
  let password = ''
  let confirm = ''

  onMount(async () => {
    try {
      mnemonic = await generateMnemonic()
      words = mnemonic.split(/\s+/)
      const idx = new Set<number>()
      let n = 1
      while (idx.size < 3 && n <= 4) { idx.add(Math.floor((words.length * n) / 4)); n++ }
      positions = [...idx].sort((a, b) => a - b)
    } catch (e: any) { error = e?.message ?? String(e) }
  })

  $: verifyOk = positions.length === 3 && positions.every((p) => (answers[p] ?? '').trim() === words[p])
  $: canCreate = name.trim() !== '' && password.length > 0 && password === confirm

  function fileName(): string {
    return name.endsWith('.dat') ? name : name + '.dat'
  }

  async function finish() {
    error = ''
    try {
      const fn = fileName()
      await importMnemonic(fn, password, mnemonic)
      await unlock(fn, password)
    } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-12 w-[32rem] space-y-4">
  <h1 class="text-xl">Create wallet</h1>

  {#if step === 1}
    <p class="text-warn text-sm">Write these 24 words down and store them safely. Anyone with them controls your funds. They are shown only once.</p>
    <div class="grid grid-cols-3 gap-2 rounded bg-surface p-3 font-mono text-sm">
      {#each words as wd, i}<div><span class="text-muted">{i + 1}.</span> {wd}</div>{/each}
    </div>
    <button class="w-full rounded bg-accent py-2 text-bg" on:click={() => (step = 2)}>I've backed it up</button>
  {:else if step === 2}
    <p class="text-sm text-muted">Confirm your backup — enter these words:</p>
    {#each positions as p}
      <label class="block text-sm text-muted">Word #{p + 1}
        <input class="mt-1 w-full rounded bg-surface px-3 py-2 font-mono" bind:value={answers[p]} aria-label={`word ${p + 1}`} />
      </label>
    {/each}
    <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!verifyOk} on:click={() => (step = 3)}>Continue</button>
  {:else}
    <label class="block text-sm text-muted">Wallet name<input class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={name} aria-label="wallet name" /></label>
    <label class="block text-sm text-muted">Password<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={password} aria-label="password" /></label>
    <label class="block text-sm text-muted">Confirm password<input type="password" class="mt-1 w-full rounded bg-surface px-3 py-2" bind:value={confirm} aria-label="confirm password" /></label>
    <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50" disabled={!canCreate} on:click={finish}>Create wallet</button>
  {/if}

  <button class="text-xs text-muted" on:click={() => view.set('unlock')}>Cancel</button>
  {#if error}<p class="text-error" role="alert">{error}</p>{/if}
</div>
