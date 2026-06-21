<script lang="ts">
  import { wallet, select } from '../stores/wallet'
  async function onChange(e: Event) { await select(Number((e.target as HTMLSelectElement).value)) }
</script>
<select class="rounded bg-surface px-2 py-1 text-sm" on:change={onChange} value={$wallet.active}>
  {#each $wallet.accounts as a}<option value={a.index}>Account {a.index}</option>{/each}
</select>
