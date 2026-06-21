<script lang="ts">
  export let value = ''
  export let label = 'Password'
</script>
<label class="block text-sm text-muted">{label}
  <input type="password" bind:value aria-label={label}
    class="mt-1 w-full rounded bg-surface px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent" />
</label>
