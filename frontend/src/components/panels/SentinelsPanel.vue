<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useSentinelStore } from '../../stores/sentinel'
import { useWalletStore } from '../../stores/wallet'
import SentinelLaunch from './SentinelLaunch.vue'
import SentinelActive from './SentinelActive.vue'

// Container: refresh on mount, then show the active view or the launch wizard.
// The step within the wizard is derived from chain state by the children.
const sentinelStore = useSentinelStore()
const wallet = useWalletStore()
const { active } = storeToRefs(sentinelStore)

onMounted(() => sentinelStore.refresh())
onUnmounted(() => sentinelStore.stopPolling())

// A sentinel is owned per-address — re-fetch (and cancel any poll) on account
// switch so the previous slot's sentinel/deposit/reward view doesn't linger.
watch(
  () => wallet.activeIndex,
  () => {
    sentinelStore.stopPolling()
    sentinelStore.refresh()
  },
)
</script>

<template>
  <div class="space-y-4 p-4">
    <SentinelActive v-if="active" />
    <SentinelLaunch v-else />
  </div>
</template>
