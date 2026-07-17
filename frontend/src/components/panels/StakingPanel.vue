<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import Field from '../Field.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useStakeStore } from '../../stores/stake'
import { useTxStore } from '../../stores/tx'
import { formatAmount, toBase } from '../../lib/format'

const stake = useStakeStore()
const tx = useTxStore()

const { stakeInfo, reward } = storeToRefs(stake)

const amount = ref('')
const months = ref('1')
const error = ref('')

// The Stake contract rewards QSR only. RewardInfo is shared with pillar and
// sentinel APIs (which can return both assets), so this panel intentionally
// ignores its always-zero ZNN field.
const rewardZero = computed(() => !reward.value || reward.value.qsr === '0')
const entries = computed(() => stakeInfo.value?.entries ?? [])

// NoM-confirm pattern: prepare the call, then hand the preview to the global
// NomConfirm dialog via tx.awaitConfirm. The panel renders no modal itself.
// ZNN has 8 decimals; amount is converted to base units, duration is a plain
// month string — matching the Svelte original's PrepareStake(toBase(amount), months).
async function doStake() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareStake(toBase(amount.value, 8), months.value))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function cancel(id: string) {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCancelStake(id))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function collect() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCollectReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(() => stake.refresh())
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') stake.refresh()
  },
)
</script>

<template>
  <div class="space-y-4 p-4">
    <p v-if="stakeInfo" class="text-sm text-muted-foreground">
      Total staked {{ formatAmount(stakeInfo.totalAmount, 8) }} ZNN
    </p>

    <section class="space-y-3 rounded-lg border border-border bg-card p-4">
      <div class="flex items-center justify-between">
        <h2 class="text-sm font-medium text-foreground">Uncollected reward</h2>
        <span v-if="reward" class="font-mono text-sm text-muted-foreground">
          {{ formatAmount(reward.qsr, 8) }} QSR
        </span>
      </div>
      <Button variant="outline" class="w-full" :disabled="rewardZero" @click="collect"
        >Collect reward</Button
      >
    </section>

    <section class="space-y-3 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Stake ZNN</h2>
      <Field label="Amount (ZNN)" hint="Available / Minimum">
        <Input v-model="amount" placeholder="ZNN amount (min 1)" aria-label="znn amount" />
      </Field>
      <Field label="Stake Duration">
        <select
          v-model="months"
          class="w-full rounded border border-border bg-card px-3 py-2 text-foreground outline-none focus:ring-2 focus:ring-ring"
          aria-label="duration months"
        >
          <option v-for="i in 12" :key="i" :value="String(i)">
            {{ i }} Month{{ i > 1 ? 's' : '' }}
          </option>
        </select>
      </Field>
      <p class="text-xs text-muted-foreground">
        Your ZNN will be locked for the selected duration and earn rewards until it matures.
      </p>
      <Button class="w-full" @click="doStake">Stake ZNN</Button>
    </section>

    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Your stakes</h2>
      <div v-for="e in entries" :key="e.id" class="flex items-center justify-between text-sm">
        <span class="font-mono">{{ formatAmount(e.amount, 8) }} ZNN · {{ e.durationMonths }}mo</span>
        <Button variant="outline" :disabled="!e.isMatured" aria-label="cancel stake" @click="cancel(e.id)"
          >Cancel</Button
        >
      </div>
      <p v-if="entries.length === 0" class="text-xs text-muted-foreground">No active stakes.</p>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing… (PoW if required)</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
