<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import { CircleCheckIcon } from '@lucide/vue'

const sentinelStore = useSentinelStore()
const tx = useTxStore()
const { sentinel, reward } = storeToRefs(sentinelStore)
const error = ref('')

const rewardZero = computed(
  () => !reward.value || (reward.value.znn === '0' && reward.value.qsr === '0'),
)

async function collect() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCollectSentinelReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function revoke() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareRevokeSentinel())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// Refresh after a collect/revoke settles (reward updates; revoke flips active).
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') sentinelStore.refresh()
  },
)
</script>

<template>
  <section v-if="sentinel" class="space-y-3 rounded-lg border border-border bg-card p-4">
    <div class="flex items-center gap-2">
      <CircleCheckIcon class="text-primary" :size="18" />
      <h2 class="text-sm font-medium text-foreground">Your Sentinel</h2>
      <span class="rounded-full bg-primary/15 px-2 py-0.5 text-xs font-medium text-primary">
        {{ sentinel.active ? 'Active' : 'Inactive' }}
      </span>
    </div>
    <p class="text-sm text-muted-foreground">
      Your Sentinel is {{ sentinel.active ? 'active and earning rewards.' : 'registered.' }}
    </p>
    <p v-if="reward" class="text-sm text-muted-foreground">
      Uncollected reward
      <span class="font-mono text-foreground"
        >{{ formatAmount(reward.znn, 8) }} ZNN · {{ formatAmount(reward.qsr, 8) }} QSR</span
      >
    </p>
    <div class="flex flex-wrap items-center gap-2">
      <Button :disabled="rewardZero" @click="collect">Collect</Button>
      <Button
        variant="outline"
        :disabled="!sentinel.isRevocable"
        aria-label="revoke sentinel"
        @click="revoke"
        >Revoke<template v-if="!sentinel.isRevocable">
          (cooldown {{ sentinel.revokeCooldown }}s)</template
        ></Button
      >
    </div>
    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
  </section>
</template>
