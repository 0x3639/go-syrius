<script lang="ts">
  export let value = ''
  export let label = 'Amount'
  function onInput(e: Event) { value = (e.target as HTMLInputElement).value.replace(/[^0-9.]/g, '') }
</script>
<label class="block text-sm text-muted">{label}
  <input inputmode="decimal" {value} on:input={onInput} aria-label={label}
    class="mt-1 w-full rounded bg-surface px-3 py-2 font-mono text-text outline-none focus:ring-2 focus:ring-accent" />
</label>
