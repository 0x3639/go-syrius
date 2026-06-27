<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    current: 1 | 2 | 3
    steps?: { n: number; label: string }[]
    ariaLabel?: string
  }>(),
  {
    steps: () => [
      { n: 1, label: 'Deposit 50,000 QSR' },
      { n: 2, label: 'Deposit 5,000 ZNN' },
      { n: 3, label: 'Sentinel active' },
    ],
    ariaLabel: 'Sentinel launch progress',
  },
)
</script>

<template>
  <ol class="flex flex-wrap items-center gap-2" :aria-label="props.ariaLabel">
    <li
      v-for="(s, i) in props.steps"
      :key="s.n"
      class="flex items-center gap-2"
      :data-state="s.n < current ? 'done' : s.n === current ? 'current' : 'todo'"
    >
      <span
        class="grid h-6 w-6 shrink-0 place-items-center rounded-full border text-xs font-medium"
        :class="s.n < current
          ? 'border-primary bg-primary text-primary-foreground'
          : s.n === current
            ? 'border-primary text-primary'
            : 'border-border text-muted-foreground'"
      >
        <svg v-if="s.n < current" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
        <template v-else>{{ s.n }}</template>
      </span>
      <span
        class="whitespace-nowrap text-xs"
        :class="s.n === current ? 'font-medium text-foreground' : 'text-muted-foreground'"
      >{{ s.label }}</span>
      <span v-if="i < props.steps.length - 1" class="mx-1 hidden h-px w-6 bg-border sm:block" />
    </li>
  </ol>
</template>
