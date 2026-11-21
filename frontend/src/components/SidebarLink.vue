<script setup lang="ts">
import type { Component } from 'vue'

// badge: a count, or '?' when the count could not be determined.
defineProps<{ to: string; label: string; icon: Component; badge?: number | string }>()
</script>

<template>
  <RouterLink :to="to" v-slot="{ isActive }" custom>
    <a
      :href="to"
      class="group flex h-10 items-center gap-3 rounded-lg px-2.5 text-sm transition-colors"
      :class="isActive
        ? 'bg-sidebar-accent font-semibold text-sidebar-accent-foreground'
        : 'font-medium text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'"
      :aria-current="isActive ? 'page' : undefined"
      @click.prevent="$router.push(to)"
    >
      <component :is="icon" :size="18" :class="isActive ? 'text-primary' : ''" />
      <span class="min-w-0 flex-1 truncate">{{ label }}</span>
      <span
        v-if="badge"
        class="rounded-full px-1.5 text-xs font-bold leading-5"
        :class="badge === '?' ? 'bg-warning/20 text-warning' : 'bg-primary text-primary-foreground'"
        :aria-label="badge === '?' ? `Couldn't check` : `${badge} waiting`"
      >{{ badge }}</span>
    </a>
  </RouterLink>
</template>
