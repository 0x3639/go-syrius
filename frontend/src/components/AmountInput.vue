<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    modelValue?: string
    label?: string
  }>(),
  { modelValue: '', label: 'Amount' },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
void props

function onInput(e: Event) {
  const cleaned = (e.target as HTMLInputElement).value.replace(/[^0-9.]/g, '')
  emit('update:modelValue', cleaned)
}
</script>

<template>
  <label class="block text-sm text-muted-foreground"
    >{{ label }}
    <input
      inputmode="decimal"
      :value="modelValue"
      :aria-label="label"
      class="mt-1 w-full rounded bg-card px-3 py-2 font-mono text-foreground outline-none focus:ring-2 focus:ring-primary"
      @input="onInput"
    />
  </label>
</template>
