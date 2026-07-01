<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{ to: string; label: string; icon: Component }>()
</script>

<template>
  <RouterLink :to="to" v-slot="{ isActive }" custom>
    <a
      :href="to"
      class="group flex items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors"
      :class="isActive
        ? 'bg-sidebar-accent font-semibold text-sidebar-accent-foreground'
        : 'font-medium text-muted-foreground hover:bg-sidebar-accent/60'"
      @click.prevent="$router.push(to)"
    >
      <component :is="icon" :size="18" :class="isActive ? 'text-primary' : ''" />
      {{ label }}
    </a>
  </RouterLink>
</template>
