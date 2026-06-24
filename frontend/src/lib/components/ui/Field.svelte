<script lang="ts">
  export let label = ''
  export let hint = ''
  export let error = ''
</script>
<label class="block space-y-1">
  {#if label}<span class="text-sm text-text">{label}</span>{/if}
  <slot />
  {#if error}<span class="block text-xs text-error">{error}</span>
  {:else if hint}<span class="block text-xs text-muted">{hint}</span>{/if}
</label>
