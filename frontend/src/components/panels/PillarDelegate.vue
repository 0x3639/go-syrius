<script setup lang="ts">
import { useNodeRefresh } from '../../lib/useNodeRefresh'
import { computed, ref, watch } from 'vue'
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
// The pillar this wallet slot operates (empty when the slot owns none), so its
// row can be highlighted distinctly from the delegated ("current") pillar.
const mine = computed(() => (pillar.ownsPillar ? pillar.myPillar?.name ?? '' : ''))
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
    await tx.prepareCall(() => Nom.PrepareDelegate(name))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function undelegate() {
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareUndelegate())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function collect() {
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareCollectPillarReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

useNodeRefresh(() => pillar.refresh())
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') pillar.refresh()
  },
)
</script>

<template>
  <div class="space-y-5">
    <section class="flex flex-wrap items-center gap-x-8 gap-y-4" :class="delegated ? 'panel-accent' : 'panel-card'">
      <div class="min-w-0 flex-1 basis-56">
        <h2 class="hint">Your delegation</h2>
        <template v-if="delegated && delegation">
          <p class="truncate text-xl font-semibold text-foreground">Delegated to <span class="text-primary">{{ delegation.name }}</span></p>
          <p class="hint">weight <span class="font-mono text-foreground">{{ formatAmount(delegation.weight, 8) }} ZNN</span></p>
        </template>
        <template v-else>
          <p class="text-xl font-semibold text-foreground">Not delegated.</p>
          <p class="hint">Pick a Pillar below to start earning a share of its rewards.</p>
        </template>
      </div>
      <div v-if="pillarReward" class="basis-48">
        <div class="hint">Uncollected reward</div>
        <div class="font-mono text-base" :class="rewardZero ? 'text-foreground' : 'text-primary'">{{ formatAmount(pillarReward.znn, 8) }} ZNN · {{ formatAmount(pillarReward.qsr, 8) }} QSR</div>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button v-if="pillarReward" :disabled="rewardZero" @click="collect">Collect</Button>
        <Button v-if="delegated && delegation" variant="outline" aria-label="undelegate" @click="undelegate">Undelegate</Button>
      </div>
    </section>

    <section class="panel-list">
      <div class="flex flex-wrap items-center gap-3 px-6 py-4">
        <h2 class="section-title">All Pillars</h2>
        <Input v-model="search" placeholder="Search by name" aria-label="search pillars" class="max-w-72 flex-1" />
        <span class="hint ml-auto">Sorted by rank</span>
      </div>

      <div class="hidden gap-3 border-t border-border/70 px-6 py-2 text-xs font-medium text-muted-foreground @2xl:flex">
        <span class="w-10">Rank</span>
        <span class="flex-1">Pillar</span>
        <span class="w-28 text-right" title="Share of delegation rewards distributed to delegators; this is not an annual yield.">Reward share</span>
        <span class="w-40 text-right">Delegated weight</span>
        <span class="w-32"></span>
      </div>
      <div
        v-for="p in filtered"
        :key="p.name"
        class="flex flex-wrap items-center gap-3 border-t border-border/70 px-6 py-3 transition-colors"
        :class="p.name === mine
          ? 'bg-success/10'
          : p.name === delegation?.name ? 'bg-primary/[0.07]' : 'hover:bg-foreground/[0.03]'"
      >
        <span class="w-10 shrink-0 font-mono text-xs text-muted-foreground">#{{ p.rank }}</span>
        <div class="flex min-w-0 flex-1 basis-40 items-baseline gap-2">
          <span class="truncate text-[15px] font-semibold text-foreground" :title="p.name">{{ p.name }}</span>
          <span
            v-if="p.name === mine"
            class="status-pill shrink-0 bg-success/20 text-success"
            >pillar</span
          >
          <span
            v-if="p.name === delegation?.name"
            class="status-pill shrink-0 bg-primary/15 text-primary"
            >delegated</span
          >
        </div>
        <span class="w-28 text-right font-mono text-sm text-foreground" title="Share of delegation rewards distributed to delegators; this is not an annual yield.">{{ p.delegateRewardPercent }}%<span class="sr-only @2xl:hidden"> reward share</span></span>
        <span class="w-40 text-right font-mono text-sm text-muted-foreground"
          >{{ formatAmount(p.weight, 8) }} ZNN</span
        >
        <Button variant="outline" class="w-32 flex-none" :aria-label="`delegate to ${p.name}`" @click="delegate(p.name)">{{ delegated ? 'Switch' : 'Delegate' }}</Button>
      </div>
      <p v-if="filtered.length === 0" class="border-t border-border/70 px-6 py-6 text-sm text-muted-foreground">No pillars.</p>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing transaction…</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
