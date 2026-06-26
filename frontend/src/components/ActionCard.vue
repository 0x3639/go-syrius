<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    label?: string
    direction?: 'send' | 'receive'
    badge?: number
    receiving?: boolean
  }>(),
  { label: '', direction: 'send', badge: 0, receiving: false },
)

defineEmits<{ click: [] }>()
void props
</script>

<template>
  <button
    :aria-label="label"
    class="group relative flex flex-col items-center justify-center gap-2 rounded border bg-card p-4 text-foreground transition hover:border-primary hover:bg-accent"
    :class="badge > 0 ? 'border-primary' : 'border-border'"
    @click="$emit('click')"
  >
    <span
      v-if="badge > 0"
      class="absolute right-2 top-2 flex h-5 min-w-[1.25rem] items-center justify-center rounded-full bg-primary px-1 text-xs font-semibold text-primary-foreground"
      :aria-label="`${badge} pending`"
      >{{ badge }}</span
    >
    <span class="text-primary" aria-hidden="true">
      <!-- Receiving: a spinner (the wallet is doing PoW / generating plasma to claim the block). -->
      <svg
        v-if="receiving"
        class="animate-spin"
        width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"
      >
        <path d="M21 12a9 9 0 1 1-6.219-8.56" />
      </svg>
      <svg
        v-else
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
        <template v-if="direction === 'send'">
          <path d="M12 16V8" />
          <path d="M8.5 11.5 12 8l3.5 3.5" />
        </template>
        <template v-else>
          <path d="M12 8v8" />
          <path d="M8.5 12.5 12 16l3.5-3.5" />
        </template>
      </svg>
    </span>
    <span class="text-sm font-medium">{{ receiving ? 'Receiving…' : label }}</span>
  </button>
</template>
