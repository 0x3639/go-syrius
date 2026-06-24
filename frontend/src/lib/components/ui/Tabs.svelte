<script lang="ts">
  export let tabs: string[] = []
  export let active = tabs[0] ?? ''
</script>
<div class="flex flex-wrap gap-1 border-b border-border">
  {#each tabs as t}
    <button
      class="px-4 py-2 text-sm transition -mb-px border-b-2 {t === active ? 'border-accent text-accent font-medium' : 'border-transparent text-muted hover:text-text'}"
      aria-label={`tab ${t}`}
      on:click={() => (active = t)}>{t}</button>
  {/each}
</div>
