<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'
import { usePriceStore } from '../stores/price'

const route = useRoute()
const price = usePriceStore()
const title = computed(() => (route.meta.title as string) ?? '')

onMounted(() => price.start())
onBeforeUnmount(() => price.stop())
</script>

<template>
  <div class="flex h-screen bg-background">
    <Sidebar />
    <div class="flex min-w-0 flex-1 flex-col">
      <TopBar :title="title" />
      <main class="flex-1 overflow-y-auto p-7">
        <router-view />
      </main>
    </div>
  </div>
</template>
