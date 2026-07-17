<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useStakeStore } from '../../stores/stake'
import { usePillarStore } from '../../stores/pillar'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import type { app } from '../../../wailsjs/go/models'
import { formatAmount } from '../../lib/format'

type Reward = { znn: string; qsr: string }
const ZERO: Reward = { znn: '0', qsr: '0' }

const stake = useStakeStore()
const pillar = usePillarStore()
const sentinel = useSentinelStore()
const tx = useTxStore()

const { reward: stakeReward } = storeToRefs(stake)
const { reward: pillarReward } = storeToRefs(pillar)
const { reward: sentinelReward } = storeToRefs(sentinel)

const error = ref('')

type Source = {
  label: string
  reward: Reward
  qsrOnly?: boolean
  collect: () => Promise<app.CallPreview>
}

// Mirrors the Svelte panel's source list (Delegation / Staking / Sentinel),
// with each reward sourced from its Pinia store and each collect mapped to the
// matching Nom.PrepareCollect* preparer.
const sources = computed<Source[]>(() => [
  {
    label: 'Delegation',
    reward: pillarReward.value ?? ZERO,
    collect: () => Nom.PrepareCollectPillarReward(),
  },
  {
    label: 'Staking',
    reward: stakeReward.value ?? ZERO,
    qsrOnly: true,
    collect: () => Nom.PrepareCollectReward(),
  },
  {
    label: 'Sentinel',
    reward: sentinelReward.value ?? ZERO,
    collect: () => Nom.PrepareCollectSentinelReward(),
  },
])

function hasReward(source: Source): boolean {
  return source.qsrOnly
    ? source.reward.qsr !== '0'
    : source.reward.znn !== '0' || source.reward.qsr !== '0'
}

function refreshAll() {
  stake.refresh()
  pillar.refresh()
  sentinel.refresh()
}

async function collect(s: Source) {
  // NoM-confirm pattern: prepare the call, then hand the preview to the global
  // NomConfirm dialog via tx.awaitConfirm. The panel renders no modal itself.
  error.value = ''
  try {
    const preview = await s.collect()
    tx.awaitConfirm(preview)
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(refreshAll)
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') refreshAll()
  },
)
</script>

<template>
  <div class="space-y-4 p-4">
    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Uncollected rewards</h2>
      <div
        v-for="s in sources"
        :key="s.label"
        class="flex items-center justify-between gap-4 border-b border-border/60 py-2 last:border-b-0"
      >
        <div class="text-sm" :data-testid="`reward-${s.label.toLowerCase()}`">
          <p class="font-medium text-foreground">{{ s.label }}</p>
          <p class="font-mono text-xs text-muted-foreground">
            <template v-if="s.qsrOnly">{{ formatAmount(s.reward.qsr, 8) }} QSR</template>
            <template v-else>
              {{ formatAmount(s.reward.znn, 8) }} ZNN / {{ formatAmount(s.reward.qsr, 8) }} QSR
            </template>
          </p>
        </div>
        <Button
          :disabled="!hasReward(s)"
          :aria-label="`collect ${s.label}`"
          @click="collect(s)"
        >Collect</Button>
      </div>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing… (PoW if required)</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
