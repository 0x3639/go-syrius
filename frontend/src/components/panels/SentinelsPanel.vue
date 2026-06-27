<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useSentinelStore } from '../../stores/sentinel'
import SentinelLaunch from './SentinelLaunch.vue'
import SentinelActive from './SentinelActive.vue'

// Container: refresh on mount, then show the active view or the launch wizard.
// The step within the wizard is derived from chain state by the children.
const sentinelStore = useSentinelStore()
const { active } = storeToRefs(sentinelStore)

onMounted(() => sentinelStore.refresh())
onUnmounted(() => sentinelStore.stopPolling())
</script>

<template>
  <div class="space-y-4 p-4">
    <SentinelActive v-if="active" />
    <SentinelLaunch v-else />
  </div>
</template>
