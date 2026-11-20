<script setup lang="ts">
import { useNodeRefresh } from '../../lib/useNodeRefresh'
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useStakeStore } from '../../stores/stake'
import { usePillarStore } from '../../stores/pillar'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import type { app } from '../../../wailsjs/go/models'
import TokenAmount from '../TokenAmount.vue'

type Reward = { znn: string; qsr: string }

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
  reward: Reward | null
  qsrOnly?: boolean
  collect: () => Promise<app.CallPreview>
}

// Mirrors the Svelte panel's source list (Delegation / Staking / Sentinel),
// with each reward sourced from its Pinia store and each collect mapped to the
// matching Nom.PrepareCollect* preparer.
const sources = computed<Source[]>(() => [
  {
    label: 'Delegation',
    reward: pillarReward.value,
    collect: () => Nom.PrepareCollectPillarReward(),
  },
  {
    label: 'Staking',
    reward: stakeReward.value,
    qsrOnly: true,
    collect: () => Nom.PrepareCollectReward(),
  },
  {
    label: 'Sentinel',
    reward: sentinelReward.value,
    collect: () => Nom.PrepareCollectSentinelReward(),
  },
])

function hasReward(source: Source): boolean {
  if (!source.reward) return false
  return source.qsrOnly
    ? source.reward.qsr !== '0'
    : source.reward.znn !== '0' || source.reward.qsr !== '0'
}

// Headline: everything collectable right now, summed across the sources.
function total(pick: (s: Source) => string | undefined): string {
  let t = 0n
  for (const s of sources.value) { try { t += BigInt(pick(s) || '0') } catch { /* ignore malformed */ } }
  return t.toString()
}
const totalZnn = computed(() => total((s) => (s.qsrOnly ? '0' : s.reward?.znn)))
const totalQsr = computed(() => total((s) => s.reward?.qsr))
const readyCount = computed(() => sources.value.filter(hasReward).length)

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
    await tx.prepareCall(() => s.collect())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

useNodeRefresh(refreshAll)
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') refreshAll()
  },
)
</script>

<template>
  <div class="page-narrow space-y-5">
    <section class="panel-accent">
      <p class="text-sm font-medium text-muted-foreground">Ready to collect</p>
      <p class="mt-1 font-mono text-3xl font-bold text-foreground">
        <TokenAmount :amount="totalZnn" symbol="ZNN" /> <span class="text-muted-foreground">+</span> <TokenAmount :amount="totalQsr" symbol="QSR" />
      </p>
      <p class="hint mt-1">{{ readyCount === 0 ? 'Nothing to collect yet.' : `From ${readyCount} ${readyCount === 1 ? 'source' : 'sources'}. Collect each one below.` }}</p>
    </section>

    <section class="panel-list">
      <h2 class="section-title px-6 py-4">By source</h2>
      <div v-for="s in sources" :key="s.label" class="list-row justify-between">
        <div class="text-sm" :data-testid="`reward-${s.label.toLowerCase()}`">
          <p class="font-semibold text-foreground">{{ s.label }}</p>
          <p class="font-mono text-xs" :class="hasReward(s) ? 'text-primary' : 'text-muted-foreground'">
            <template v-if="!s.reward">—</template>
            <template v-else>
              <template v-if="!s.qsrOnly"><TokenAmount :amount="s.reward.znn" symbol="ZNN" /> · </template>
              <TokenAmount :amount="s.reward.qsr" symbol="QSR" />
            </template>
          </p>
        </div>
        <Button
          :variant="hasReward(s) ? 'default' : 'outline'"
          :disabled="!hasReward(s)"
          :aria-label="`collect ${s.label}`"
          @click="collect(s)"
        >Collect</Button>
      </div>
    </section>

    <p class="hint">Rewards build up over time and wait here until you collect them. Collecting sends them to this account.</p>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing transaction…</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
