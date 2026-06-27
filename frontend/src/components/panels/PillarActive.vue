<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { formatAmount, shortAddress } from '../../lib/format'

const pillarStore = usePillarStore()
const tx = useTxStore()
const { myPillar, reward } = storeToRefs(pillarStore)
const error = ref('')

const rewardZero = computed(
  () => !reward.value || (reward.value.znn === '0' && reward.value.qsr === '0'),
)

async function collect() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCollectPillarReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function revoke() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareRevokePillar(myPillar.value?.name ?? ''))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// Refresh after a collect/revoke settles (reward updates; revoke clears ownership).
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') pillarStore.refreshRegistration()
  },
)
</script>

<template>
  <section v-if="myPillar" class="space-y-3 rounded-lg border border-border bg-card p-4">
    <div class="flex items-center gap-2">
      <svg class="text-primary" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></svg>
      <h2 class="text-sm font-medium text-foreground">Your Pillar</h2>
      <span class="rounded-full bg-primary/15 px-2 py-0.5 text-xs font-medium text-primary">{{ myPillar.name }}</span>
    </div>
    <dl class="space-y-1 text-sm text-muted-foreground">
      <div class="flex justify-between">
        <dt>Producer</dt>
        <dd class="font-mono text-foreground">{{ shortAddress(myPillar.producerAddress) }}</dd>
      </div>
      <div class="flex justify-between">
        <dt>Reward address</dt>
        <dd class="font-mono text-foreground">{{ shortAddress(myPillar.rewardAddress) }}</dd>
      </div>
      <div class="flex justify-between">
        <dt>Momentum / Delegate %</dt>
        <dd class="font-mono text-foreground">{{ myPillar.giveMomentumRewardPct }}% / {{ myPillar.giveDelegateRewardPct }}%</dd>
      </div>
    </dl>
    <p v-if="reward" class="text-sm text-muted-foreground">
      Uncollected reward
      <span class="font-mono text-foreground"
        >{{ formatAmount(reward.znn, 8) }} ZNN · {{ formatAmount(reward.qsr, 8) }} QSR</span
      >
    </p>
    <div class="flex flex-wrap items-center gap-2">
      <Button :disabled="rewardZero" aria-label="collect pillar reward" @click="collect">Collect</Button>
      <Button
        variant="outline"
        :disabled="!myPillar.isRevocable"
        aria-label="revoke pillar"
        @click="revoke"
        >Revoke<template v-if="!myPillar.isRevocable">
          (cooldown {{ myPillar.revokeCooldown }}s)</template
        ></Button
      >
    </div>
    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
  </section>
</template>
