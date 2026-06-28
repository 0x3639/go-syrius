<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import { isPassing, quorumNeeded } from '../../lib/accelerator'
import type { app } from '../../../wailsjs/go/models'

const acc = useAcceleratorStore()
const pillar = usePillarStore()
const tx = useTxStore()
const { votable, votablePillars, numActivePillars } = storeToRefs(acc)
const { ownsPillar } = storeToRefs(pillar)
const error = ref('')

const selectedPillar = ref('')
const showAll = ref(false)
watch(
  votablePillars,
  (list) => {
    // Reset when the current selection is gone (wallet/pillar list changed),
    // not only when empty — a stale name would silently mis-scope every row.
    if (!list.includes(selectedPillar.value)) selectedPillar.value = list[0] ?? ''
  },
  { immediate: true },
)

function myVote(item: app.VotableItem): number {
  const e = item.myVotes?.find((m) => m.pillar === selectedPillar.value)
  return e ? e.vote : -1
}
// Whether the *selected* pillar still owes a vote. item.needsMyVote is true if
// ANY owned pillar is unvoted, which would wrongly list rows the selected
// pillar has already voted on.
function needsVoteForPillar(item: app.VotableItem): boolean {
  return myVote(item) === -1
}
const VOTE_LABELS = ['yes', 'no', 'abstain']
const items = computed(() =>
  showAll.value ? votable.value : votable.value.filter(needsVoteForPillar),
)

async function vote(id: string, choice: number) {
  error.value = ''
  if (!selectedPillar.value) {
    error.value = 'Select a pillar to vote as.'
    return
  }
  try {
    tx.awaitConfirm(await Nom.PrepareVote(id, selectedPillar.value, choice))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

watch(
  () => tx.status,
  (s) => {
    if (s === 'done') acc.refreshVotable()
  },
)
</script>

<template>
  <div class="space-y-3 p-4">
    <p v-if="!ownsPillar" class="text-sm text-muted-foreground">
      Voting on Accelerator-Z proposals is for pillar operators. Register or run a pillar to vote.
    </p>
    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-2">
        <label class="flex items-center gap-2 text-sm text-muted-foreground">
          Vote as
          <select
            v-model="selectedPillar"
            aria-label="vote pillar"
            class="rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary"
          >
            <option v-for="n in votablePillars" :key="n" :value="n">{{ n }}</option>
          </select>
        </label>
        <label class="flex items-center gap-2 text-xs text-muted-foreground">
          <input v-model="showAll" type="checkbox" aria-label="show all votable" />
          Show items I've already voted on
        </label>
      </div>

      <p v-if="items.length === 0" class="text-sm text-muted-foreground">
        Nothing awaiting your vote right now.
      </p>

      <div
        v-for="it in items"
        :key="it.id"
        class="space-y-2 rounded-lg border border-border bg-card p-3"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">{{ it.kind }}</span>
          <span class="text-sm font-medium text-foreground">{{ it.name }}</span>
          <span v-if="it.kind === 'phase'" class="text-xs text-muted-foreground">· {{ it.projectName }}</span>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ formatAmount(it.znnFundsNeeded, 8) }} ZNN / {{ formatAmount(it.qsrFundsNeeded, 8) }} QSR
        </p>
        <p class="text-xs text-muted-foreground">
          {{ it.votes.yes }} yes · {{ it.votes.no }} no · {{ it.votes.total }} votes
          (quorum {{ quorumNeeded(numActivePillars) }})
          <span
            v-if="isPassing(it.votes.yes, it.votes.no, it.votes.total, numActivePillars)"
            class="text-primary"
          > · passing</span>
        </p>
        <p v-if="myVote(it) !== -1" class="text-xs text-primary">
          You voted: {{ VOTE_LABELS[myVote(it)] }} (you can change it)
        </p>
        <div class="flex flex-wrap gap-2">
          <Button :aria-label="`vote yes ${it.id}`" @click="vote(it.id, 0)">Yes</Button>
          <Button variant="outline" :aria-label="`vote no ${it.id}`" @click="vote(it.id, 1)">No</Button>
          <Button variant="outline" :aria-label="`vote abstain ${it.id}`" @click="vote(it.id, 2)">Abstain</Button>
        </div>
      </div>
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
