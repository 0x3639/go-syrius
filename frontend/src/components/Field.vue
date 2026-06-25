<script setup lang="ts">
defineProps<{ label?: string; hint?: string; error?: string }>()
</script>

<template>
  <label class="block space-y-1">
    <span v-if="label" class="text-sm text-foreground">{{ label }}</span>
    <slot />
    <span v-if="error" class="block text-xs text-destructive" role="alert">{{ error }}</span>
    <span v-else-if="hint" class="block text-xs text-muted-foreground">{{ hint }}</span>
  </label>
</template>
