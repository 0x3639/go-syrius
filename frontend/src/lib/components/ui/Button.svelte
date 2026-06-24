<script lang="ts">
  export let variant: 'primary' | 'outline' | 'ghost' | 'danger' = 'primary'
  export let disabled = false
  export let type: 'button' | 'submit' = 'button'
  const styles = {
    primary: 'bg-accent text-accent-fg hover:brightness-110',
    outline: 'border border-border text-text hover:bg-elevated',
    ghost: 'text-muted hover:text-text hover:bg-elevated',
    danger: 'bg-error text-text hover:brightness-110',
  }
</script>
<button {type} {disabled} {...$$restProps} on:click
  class="rounded px-4 py-2 text-sm font-medium transition disabled:opacity-50 disabled:cursor-not-allowed {styles[variant]}">
  <slot />
</button>
