<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import Field from '../Field.vue'

const pillarStore = usePillarStore()
const tx = useTxStore()
const { myPillar, reward } = storeToRefs(pillarStore)
const error = ref('')

const rewardZero = computed(
  () => !reward.value || (reward.value.znn === '0' && reward.value.qsr === '0'),
)

// Edit (UpdatePillar) form — producer address, reward address, and the two
// reward percentages. Pre-filled from the current pillar; the name is the
// identifier and cannot change.
const editing = ref(false)
const editProducer = ref('')
const editReward = ref('')
const editMomentum = ref('')
const editDelegate = ref('')

function startEdit() {
  if (!myPillar.value) return
  editProducer.value = myPillar.value.producerAddress
  editReward.value = myPillar.value.rewardAddress
  editMomentum.value = String(myPillar.value.giveMomentumRewardPct)
  editDelegate.value = String(myPillar.value.giveDelegateRewardPct)
  error.value = ''
  editing.value = true
}
function cancelEdit() {
  editing.value = false
}

const pctValid = computed(() => {
  const m = Number(editMomentum.value)
  const d = Number(editDelegate.value)
  return (
    editMomentum.value.trim() !== '' &&
    editDelegate.value.trim() !== '' &&
    Number.isInteger(m) && m >= 0 && m <= 100 &&
    Number.isInteger(d) && d >= 0 && d <= 100
  )
})
const canSave = computed(
  () => editProducer.value.trim() !== '' && editReward.value.trim() !== '' && pctValid.value,
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
async function saveEdit() {
  error.value = ''
  try {
    tx.awaitConfirm(
      await Nom.PrepareUpdatePillar(
        myPillar.value?.name ?? '',
        editProducer.value.trim(),
        editReward.value.trim(),
        Number(editMomentum.value),
        Number(editDelegate.value),
      ),
    )
    editing.value = false
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// Refresh after a collect/revoke/update settles (reward updates; revoke clears
// ownership; update changes producer/reward/percentages).
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

    <!-- Read view -->
    <template v-if="!editing">
      <dl class="space-y-2 text-sm text-muted-foreground">
        <div>
          <dt>Producer</dt>
          <dd class="break-all font-mono text-foreground">{{ myPillar.producerAddress }}</dd>
        </div>
        <div>
          <dt>Reward address</dt>
          <dd class="break-all font-mono text-foreground">{{ myPillar.rewardAddress }}</dd>
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
        <Button variant="outline" aria-label="edit pillar" @click="startEdit">Edit configuration</Button>
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
    </template>

    <!-- Edit view (UpdatePillar) -->
    <template v-else>
      <p class="text-xs text-muted-foreground">
        Update your pillar's producer address, reward address, and reward percentages. The pillar
        name cannot be changed.
      </p>
      <Field label="Producer address" hint="Your pillar node's block-producing address.">
        <Input v-model="editProducer" placeholder="z1…" aria-label="edit producer address" />
      </Field>
      <Field label="Reward address" hint="Where pillar rewards are collected.">
        <Input v-model="editReward" placeholder="z1…" aria-label="edit reward address" />
      </Field>
      <Field label="Momentum reward % (to delegators)">
        <Input v-model="editMomentum" placeholder="0–100" aria-label="edit momentum percent" />
      </Field>
      <Field label="Delegate reward % (to delegators)">
        <Input v-model="editDelegate" placeholder="0–100" aria-label="edit delegate percent" />
      </Field>
      <div class="flex flex-wrap items-center gap-2">
        <Button :disabled="!canSave" aria-label="save pillar" @click="saveEdit">Save changes</Button>
        <Button variant="outline" aria-label="cancel edit" @click="cancelEdit">Cancel</Button>
      </div>
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
  </section>
</template>
