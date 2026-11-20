<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
    ariaLabel?: string
    // Plain-decimal maximum (the token balance). When > 0 a "Max" button shows.
    max?: string
    // Optional larger question-style heading shown instead of the small label
    // (the input keeps `label` as its accessible name).
    heading?: string
    // Optional unit shown inside the field, e.g. the token symbol.
    symbol?: string
  }>(),
  { modelValue: '', label: 'Amount', max: '', heading: '', symbol: '' },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const showMax = computed(() => !!props.max && Number(props.max) > 0)

function onInput(e: Event) {
  // Preserve the user's value so strict validation can reject it. Stripping
  // characters would turn a pasted negative amount into a positive transfer.
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}

function setMax() {
  emit('update:modelValue', props.max)
}
</script>

<template>
  <label class="block" :class="heading ? 'section-title' : 'field-label'"
    >{{ heading || label }}
    <div class="mt-2 flex h-14 items-center gap-3 rounded-xl border border-input bg-background pl-4 pr-2.5 focus-within:ring-2 focus-within:ring-primary">
      <input
        inputmode="decimal"
        :value="modelValue"
        :aria-label="ariaLabel || label"
        class="min-w-0 flex-1 bg-transparent font-mono text-xl font-bold text-foreground outline-none"
        @input="onInput"
      />
      <span v-if="symbol" class="text-sm font-semibold text-muted-foreground">{{ symbol }}</span>
      <button
        v-if="showMax"
        type="button"
        aria-label="max amount"
        class="h-9 rounded-lg bg-primary/15 px-3.5 text-sm font-bold text-primary transition-colors hover:bg-primary/25"
        @click="setMax"
      >
        Max
      </button>
    </div>
  </label>
</template>
