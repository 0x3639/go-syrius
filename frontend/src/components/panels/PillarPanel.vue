<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'

const pillar = usePillarStore()
const tx = useTxStore()

const { pillars, delegation, reward: pillarReward } = storeToRefs(pillar)

const search = ref('')
const error = ref('')

const rewardZero = computed(
  () => !pillarReward.value || (pillarReward.value.znn === '0' && pillarReward.value.qsr === '0'),
)
const delegated = computed(() => !!delegation.value && delegation.value.name !== '')
const filtered = computed(() =>
  (pillars.value ?? [])
    .filter((p) => p.name.toLowerCase().includes(search.value.trim().toLowerCase()))
    .sort((a, b) => a.rank - b.rank),
)

// NoM-confirm pattern: prepare the call, then hand the preview to the global
// NomConfirm dialog via tx.awaitConfirm. The panel renders no modal itself.
async function delegate(name: string) {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareDelegate(name))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function undelegate() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareUndelegate())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function collect() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCollectPillarReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(() => pillar.refresh())
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') pillar.refresh()
  },
)
</script>

<template>
  <div class="space-y-4 p-4">
    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Your delegation</h2>
      <div v-if="delegated && delegation" class="flex items-center justify-between text-sm">
        <p>
          Delegated to <span class="font-mono text-primary">{{ delegation.name }}</span> · weight
          <span class="font-mono">{{ formatAmount(delegation.weight, 8) }} ZNN</span>
        </p>
        <Button variant="outline" aria-label="undelegate" @click="undelegate">Undelegate</Button>
      </div>
      <p v-else class="text-xs text-muted-foreground">Not delegated.</p>

      <div v-if="pillarReward" class="flex items-center justify-between text-sm">
        <p class="text-muted-foreground">
          Uncollected reward
          <span class="font-mono text-foreground">{{ formatAmount(pillarReward.znn, 8) }} ZNN</span> ·
          <span class="font-mono text-foreground">{{ formatAmount(pillarReward.qsr, 8) }} QSR</span>
        </p>
        <Button :disabled="rewardZero" @click="collect">Collect</Button>
      </div>
    </section>

    <section class="space-y-3 rounded-lg border border-border bg-card p-4">
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-sm font-medium text-foreground">Pillars</h2>
        <span class="text-xs text-muted-foreground">Sorted by Rank</span>
      </div>
      <Input v-model="search" placeholder="Search pillars" aria-label="search pillars" />

      <div class="space-y-1">
        <div
          v-for="p in filtered"
          :key="p.name"
          class="flex items-center justify-between gap-3 rounded-md border border-transparent px-2 py-2 hover:border-border hover:bg-muted"
        >
          <div class="flex min-w-0 items-baseline gap-3">
            <span class="shrink-0 font-mono text-xs text-muted-foreground">#{{ p.rank }}</span>
            <span class="truncate text-sm text-foreground">{{ p.name }}</span>
            <span
              v-if="p.name === delegation?.name"
              class="shrink-0 rounded bg-primary/15 px-1.5 py-0.5 text-[10px] font-medium text-primary"
              >current</span
            >
          </div>
          <div class="flex shrink-0 items-center gap-4">
            <span class="text-xs text-primary">{{ p.delegateRewardPercent }}% APR</span>
            <span class="font-mono text-xs text-muted-foreground"
              >{{ formatAmount(p.weight, 8) }} ZNN</span
            >
            <Button :aria-label="`delegate to ${p.name}`" @click="delegate(p.name)">Delegate</Button>
          </div>
        </div>
        <p v-if="filtered.length === 0" class="text-xs text-muted-foreground">No pillars.</p>
      </div>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing… (PoW if required)</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
