<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import Field from '../Field.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useStakeStore } from '../../stores/stake'
import { useTxStore } from '../../stores/tx'
import { formatAmount, formatAmountExact, toBase } from '../../lib/format'
import { useBalancesStore } from '../../stores/balances'
import TokenAmount from '../TokenAmount.vue'
import AmountInput from '../AmountInput.vue'

const stake = useStakeStore()
const tx = useTxStore()

const { stakeInfo, reward } = storeToRefs(stake)

const amount = ref('')
const months = ref('1')
const error = ref('')
const balances = useBalancesStore()
const available = computed(() => balances.items.find(b => b.symbol === 'ZNN')?.amount ?? '0')
const availableLabel = computed(() => `${balances.available ? formatAmountExact(available.value, 8) + ' ZNN available' : 'Balance unavailable'} · Minimum 1 ZNN`)
const canStake = computed(() => {
  try {
    const base = BigInt(toBase(amount.value, 8))
    return balances.available && base >= 100000000n && base <= BigInt(available.value)
  } catch { return false }
})
function unlockDate(timestamp: number) {
  return new Date(timestamp * 1000).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

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
  if (!canStake.value) return
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
  <div class="space-y-5">
    <p v-if="stakeInfo" class="text-sm text-muted-foreground">
      Total staked {{ formatAmount(stakeInfo.totalAmount, 8) }} ZNN
    </p>

    <section class="space-y-3 panel-card">
      <div class="flex items-center justify-between">
        <h2 class="text-sm font-medium text-foreground">Uncollected reward</h2>
        <span v-if="reward" class="font-mono text-sm text-muted-foreground">
          <TokenAmount :amount="reward.qsr" symbol="QSR" />
        </span>
      </div>
      <Button variant="outline" class="w-full" :disabled="rewardZero" @click="collect"
        >Collect reward</Button
      >
    </section>

    <section class="space-y-3 panel-card">
      <h2 class="text-sm font-medium text-foreground">Stake ZNN</h2>
      <div class="space-y-1">
        <AmountInput v-model="amount" label="Amount (ZNN)" aria-label="znn amount" :max="balances.available ? formatAmountExact(available, 8) : ''" />
        <p class="text-xs text-muted-foreground">{{ availableLabel }}</p>
      </div>
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
      <Button class="w-full" :disabled="!canStake" @click="doStake">Stake ZNN</Button>
    </section>

    <section class="space-y-2 panel-card">
      <h2 class="text-sm font-medium text-foreground">Your stakes</h2>
      <div v-for="e in entries" :key="e.id" class="flex items-center justify-between text-sm">
        <div><TokenAmount :amount="e.amount" symbol="ZNN" /> · {{ e.durationMonths }} month{{ e.durationMonths === 1 ? '' : 's' }}
          <p class="text-xs text-muted-foreground">{{ e.isMatured ? 'Ready to withdraw' : `Unlocks ${unlockDate(e.expirationTimestamp)}` }}</p>
        </div>
        <Button variant="outline" :disabled="!e.isMatured" aria-label="withdraw stake" @click="cancel(e.id)"
          >Withdraw</Button
        >
      </div>
      <p v-if="entries.length === 0" class="text-xs text-muted-foreground">No active stakes.</p>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing transaction…</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
