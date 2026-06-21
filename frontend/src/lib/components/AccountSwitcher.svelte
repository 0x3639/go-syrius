<script lang="ts">
  import { wallet, select, setLabel } from '../stores/wallet'
  let editing = false
  let draft = ''
  function labelFor(a: { index: number; label?: string }) { return a.label && a.label.trim() ? a.label : `Account ${a.index}` }
  async function onChange(e: Event) { await select(Number((e.target as HTMLSelectElement).value)) }
  function startEdit() { draft = $wallet.accounts.find((a) => a.index === $wallet.active)?.label ?? ''; editing = true }
  async function saveEdit() { await setLabel($wallet.active, draft.trim()); editing = false }
</script>
<div class="flex items-center gap-2">
  <select class="rounded bg-surface px-2 py-1 text-sm" on:change={onChange} value={$wallet.active}>
    {#each $wallet.accounts as a}<option value={a.index}>{labelFor(a)}</option>{/each}
  </select>
  {#if editing}
    <input class="rounded bg-surface px-2 py-1 text-sm" bind:value={draft} aria-label="account label" />
    <button class="text-xs text-accent" on:click={saveEdit}>Save</button>
  {:else}
    <button class="text-xs text-muted" on:click={startEdit} aria-label="edit label">✎</button>
  {/if}
</div>
