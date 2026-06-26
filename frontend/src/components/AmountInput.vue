<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
    // Plain-decimal maximum (the token balance). When > 0 a "Max" button shows.
    max?: string
  }>(),
  { modelValue: '', label: 'Amount', max: '' },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const showMax = computed(() => !!props.max && Number(props.max) > 0)

function onInput(e: Event) {
  const cleaned = (e.target as HTMLInputElement).value.replace(/[^0-9.]/g, '')
  emit('update:modelValue', cleaned)
}

function setMax() {
  emit('update:modelValue', props.max)
}
</script>

<template>
  <label class="block text-sm text-muted-foreground"
    >{{ label }}
    <div class="relative mt-1">
      <input
        inputmode="decimal"
        :value="modelValue"
        :aria-label="label"
        class="w-full rounded bg-card px-3 py-2 pr-16 font-mono text-foreground outline-none focus:ring-2 focus:ring-primary"
        @input="onInput"
      />
      <button
        v-if="showMax"
        type="button"
        aria-label="max amount"
        class="absolute right-2 top-1/2 -translate-y-1/2 rounded bg-primary/15 px-2 py-1 text-xs font-semibold text-primary transition-colors hover:bg-primary/25"
        @click="setMax"
      >
        Max
      </button>
    </div>
  </label>
</template>
