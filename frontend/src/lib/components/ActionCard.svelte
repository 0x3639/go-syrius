<script lang="ts">
  export let label = ''
  export let direction: 'send' | 'receive' = 'send'
</script>

<button
  on:click
  aria-label={label}
  class="group flex flex-col items-center justify-center gap-2 rounded border border-border bg-surface p-4 text-text transition hover:border-accent hover:bg-elevated"
>
  <span class="text-accent" aria-hidden="true">
    <svg
      width="28"
      height="28"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <circle cx="12" cy="12" r="9" />
      {#if direction === 'send'}
        <path d="M12 16V8" />
        <path d="M8.5 11.5 12 8l3.5 3.5" />
      {:else}
        <path d="M12 8v8" />
        <path d="M8.5 12.5 12 16l3.5-3.5" />
      {/if}
    </svg>
  </span>
  <span class="text-sm font-medium">{label}</span>
</button>
