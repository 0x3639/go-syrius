<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'
import { isOpen, isActionApproved } from '../../lib/governance'
import type { app } from '../../../wailsjs/go/models'

const gov = useGovernanceStore()
const tx = useTxStore()
const { actions, votablePillars, numActivePillars } = storeToRefs(gov)
const error = ref('')

const ownsPillar = computed(() => votablePillars.value.length > 0)
const selectedPillar = ref('')
watch(
  votablePillars,
  (list) => {
    // Reset when the current selection is gone (wallet/pillar list changed),
    // not only when empty — a stale name would silently mis-scope every vote.
    if (!list.includes(selectedPillar.value)) selectedPillar.value = list[0] ?? ''
  },
  { immediate: true },
)

// Phase 1: list ALL open actions (no per-pillar "needs my vote" yet — that
// arrives in Phase 2 with the governance getPillarVotes read).
const openActions = computed(() => (actions.value ?? []).filter(isOpen))

function passing(a: app.ActionDTO): boolean {
  return isActionApproved(a.votes, a, numActivePillars.value)
}

async function vote(id: string, choice: number) {
  error.value = ''
  if (!selectedPillar.value) {
    error.value = 'Select a pillar to vote as.'
    return
  }
  try {
    tx.awaitConfirm(await Nom.PrepareGovernanceVote(id, selectedPillar.value, choice))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="space-y-3 p-4">
    <p v-if="!ownsPillar" class="text-sm text-muted-foreground">
      Voting on governance actions is for pillar operators. Register or run a pillar to vote.
    </p>
    <template v-else>
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        Vote as
        <!-- An address can own multiple pillars, so offer a picker only when
             there's a genuine choice; otherwise the pillar is fixed by the
             active wallet account (switch accounts to vote as another). -->
        <select
          v-if="votablePillars.length > 1"
          v-model="selectedPillar"
          aria-label="vote pillar"
          class="rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary"
        >
          <option v-for="n in votablePillars" :key="n" :value="n">{{ n }}</option>
        </select>
        <span v-else aria-label="vote pillar" class="font-medium text-foreground">{{ selectedPillar }}</span>
      </div>

      <p v-if="openActions.length === 0" class="text-sm text-muted-foreground">
        No governance actions are open for voting right now.
      </p>

      <div
        v-for="a in openActions"
        :key="a.id"
        class="space-y-2 rounded-lg border border-border bg-card p-3"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-sm font-medium text-foreground">{{ a.name }}</span>
          <span class="text-xs text-muted-foreground">round {{ a.round + 1 }}</span>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ a.votes.yes }} yes · {{ a.votes.no }} no · {{ a.votes.total }} votes
          ({{ a.activePillarThreshold }}% quorum / {{ a.directionalThreshold }}% directional)
          <span v-if="passing(a)" class="text-primary"> · passing</span>
        </p>
        <div class="flex flex-wrap gap-2">
          <Button :aria-label="`vote yes ${a.id}`" @click="vote(a.id, 0)">Yes</Button>
          <Button variant="outline" :aria-label="`vote no ${a.id}`" @click="vote(a.id, 1)">No</Button>
          <Button variant="outline" :aria-label="`vote abstain ${a.id}`" @click="vote(a.id, 2)">Abstain</Button>
        </div>
      </div>
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
