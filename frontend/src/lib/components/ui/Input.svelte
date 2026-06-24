<script lang="ts">
  export let value = ''
  export let placeholder = ''
  export let type = 'text'
  export let ariaLabel = ''
  function onInput(e: Event) {
    value = (e.currentTarget as HTMLInputElement).value
  }
</script>
<input {type} {placeholder} aria-label={ariaLabel} {value} on:input={onInput} on:input
  class="w-full rounded border border-border bg-elevated px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent" />
