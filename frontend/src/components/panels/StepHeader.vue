<script setup lang="ts">
import { CheckIcon } from '@lucide/vue'
const props = withDefaults(
  defineProps<{
    current: 1 | 2 | 3
    steps?: { n: number; label: string }[]
    ariaLabel?: string
    // When clickable, steps up to and including `maxStep` become buttons that
    // emit `select` — lets a wizard navigate back to an already-completed step.
    clickable?: boolean
    maxStep?: number
  }>(),
  {
    steps: () => [
      { n: 1, label: 'Deposit 50,000 QSR' },
      { n: 2, label: 'Deposit 5,000 ZNN' },
      { n: 3, label: 'Sentinel active' },
    ],
    ariaLabel: 'Sentinel launch progress',
    clickable: false,
    maxStep: 3,
  },
)
const emit = defineEmits<{ (e: 'select', n: number): void }>()
function navigable(n: number): boolean {
  return props.clickable && n <= props.maxStep
}
function onSelect(n: number) {
  if (navigable(n)) emit('select', n)
}
</script>

<template>
  <ol class="flex flex-wrap items-center gap-2" :aria-label="props.ariaLabel">
    <li
      v-for="(s, i) in props.steps"
      :key="s.n"
      class="flex items-center gap-2"
      :data-state="s.n < current ? 'done' : s.n === current ? 'current' : 'todo'"
    >
      <component
        :is="navigable(s.n) ? 'button' : 'div'"
        :type="navigable(s.n) ? 'button' : undefined"
        class="flex items-center gap-2"
        :class="navigable(s.n) ? 'cursor-pointer' : ''"
        :aria-label="navigable(s.n) ? s.label : undefined"
        @click="onSelect(s.n)"
      >
        <span
          class="grid h-6 w-6 shrink-0 place-items-center rounded-full border text-xs font-medium"
          :class="s.n < current
            ? 'border-primary bg-primary text-primary-foreground'
            : s.n === current
              ? 'border-primary text-primary'
              : 'border-border text-muted-foreground'"
        >
          <CheckIcon v-if="s.n < current" :size="12" />
          <template v-else>{{ s.n }}</template>
        </span>
        <span
          class="whitespace-nowrap text-xs"
          :class="s.n === current ? 'font-medium text-foreground' : 'text-muted-foreground'"
        >{{ s.label }}</span>
      </component>
      <span v-if="i < props.steps.length - 1" class="mx-1 hidden h-px w-6 bg-border sm:block" />
    </li>
  </ol>
</template>
