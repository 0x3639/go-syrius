<script lang="ts">
  export let wallets: { name: string; baseAddress: string }[] = []
  export let selected = ''
</script>
<ul class="space-y-1">
  {#each wallets as w}
    <li>
      <button class="w-full rounded px-3 py-2 text-left {selected === w.name ? 'bg-accent/20' : 'bg-surface'}"
        aria-pressed={selected === w.name}
        on:click={() => (selected = w.name)}>
        <div class="text-text">{w.name}</div>
        <div class="font-mono text-xs text-muted">{w.baseAddress}</div>
      </button>
    </li>
  {/each}
</ul>
