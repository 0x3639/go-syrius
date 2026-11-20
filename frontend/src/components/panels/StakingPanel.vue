<script setup lang="ts">
import { useNodeRefresh } from '../../lib/useNodeRefresh'
import { computed, ref, watch } from 'vue'
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
const rewardZero = computed(() => !stake.available || !reward.value || reward.value.qsr === '0')
// Progress of a stake through its lock period (0–100), for the row meter.
function progress(e: { startTimestamp: number; expirationTimestamp: number; isMatured: boolean }) {
  if (e.isMatured) return 100
  const span = e.expirationTimestamp - e.startTimestamp
  if (span <= 0) return 0
  return Math.max(0, Math.min(100, ((Date.now() / 1000 - e.startTimestamp) / span) * 100))
}
const nextUnlock = computed(() => {
  const pending = entries.value.filter((e) => !e.isMatured).map((e) => e.expirationTimestamp)
  if (entries.value.some((e) => e.isMatured)) return 'Ready now'
  return pending.length ? unlockDate(Math.min(...pending)) : '—'
})
const QUICK_MONTHS = ['1', '3', '6', '12']
const entries = computed(() => stake.available ? stakeInfo.value?.entries ?? [] : [])

// NoM-confirm pattern: prepare the call, then hand the preview to the global
// NomConfirm dialog via tx.awaitConfirm. The panel renders no modal itself.
// ZNN has 8 decimals; amount is converted to base units, duration is a plain
// month string — matching the Svelte original's PrepareStake(toBase(amount), months).
async function doStake() {
  if (!canStake.value) return
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareStake(toBase(amount.value, 8), months.value))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function cancel(id: string) {
  if (!stake.available || !entries.value.some(e => e.id === id && e.isMatured)) return
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareCancelStake(id))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function collect() {
  if (rewardZero.value) return
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareCollectReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

useNodeRefresh(() => stake.refresh())
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') stake.refresh()
  },
)
</script>

<template>
  <div class="space-y-5">
    <p v-if="stake.status === 'error'" class="text-sm text-destructive" role="alert">
      Staking data could not be loaded. <button class="underline" @click="stake.refresh()">Retry</button>
    </p>
    <p v-else-if="!stake.available" class="text-sm text-muted-foreground">Loading staking data…</p>

    <!-- Summary strip. The reward block stays the first <section> on the page. -->
    <div class="panel-list grid grid-cols-1 @2xl:grid-cols-3">
      <section class="order-last flex flex-wrap items-center gap-3 border-t border-border px-6 py-5 @2xl:border-l @2xl:border-t-0">
        <div class="min-w-0 flex-1">
          <h2 class="hint">Uncollected reward</h2>
          <span v-if="reward" class="font-mono text-2xl font-bold" :class="rewardZero ? 'text-foreground' : 'text-primary'">
            <TokenAmount :amount="reward.qsr" symbol="QSR" />
          </span>
          <span v-else class="font-mono text-2xl font-bold text-foreground">—</span>
        </div>
        <Button :disabled="rewardZero" @click="collect"
          >Collect reward</Button
        >
      </section>
      <div class="px-6 py-5">
        <div class="hint">Total staked</div>
        <div class="font-mono text-2xl font-bold text-foreground">{{ stakeInfo ? formatAmount(stakeInfo.totalAmount, 8) : '—' }} ZNN</div>
      </div>
      <div class="border-t border-border px-6 py-5 @2xl:border-l @2xl:border-t-0">
        <div class="hint">Next unlock</div>
        <div class="text-xl font-semibold leading-8 text-foreground">{{ nextUnlock }}</div>
      </div>
    </div>

    <div class="page-halves">
      <section class="panel-card space-y-5">
        <h2 class="section-title">New stake</h2>
        <div class="space-y-1.5">
          <AmountInput v-model="amount" label="Amount (ZNN)" symbol="ZNN" :max="balances.available ? formatAmountExact(available, 8) : ''" />
          <p class="hint">{{ availableLabel }}</p>
        </div>
        <div class="space-y-2">
          <span class="field-label block">For how long?</span>
          <div class="grid grid-cols-4 gap-2" role="group" aria-label="Common durations">
            <button
              v-for="m in QUICK_MONTHS"
              :key="m"
              type="button"
              class="choice flex flex-col items-center py-2"
              :class="months === m ? 'choice-on' : ''"
              :aria-pressed="months === m"
              @click="months = m"
            >
              <span class="text-base font-bold">{{ m }}</span>
              <span class="text-xs text-muted-foreground">{{ m === '1' ? 'month' : 'months' }}</span>
            </button>
          </div>
          <Field label="Or pick any length">
            <select
              v-model="months"
              class="w-full rounded-lg border border-input bg-background px-3 py-2 text-foreground outline-none focus:ring-2 focus:ring-ring"
              aria-label="duration months"
            >
              <option v-for="i in 12" :key="i" :value="String(i)">
                {{ i }} Month{{ i > 1 ? 's' : '' }}
              </option>
            </select>
          </Field>
        </div>
        <p class="hint">
          Your ZNN is locked for the selected duration. It unlocks at maturity and keeps earning rewards until you withdraw it.
        </p>
        <Button class="h-12 w-full text-base font-bold" :disabled="!canStake" @click="doStake">Stake ZNN</Button>
      </section>

      <section class="panel-list">
        <h2 class="section-title px-6 py-4">Your stakes</h2>
        <div v-for="e in entries" :key="e.id" class="space-y-2.5 border-t border-border/70 px-6 py-4 text-sm">
          <div class="flex items-center justify-between gap-3">
            <div><span class="text-lg"><TokenAmount :amount="e.amount" symbol="ZNN" /></span> · {{ e.durationMonths }} month{{ e.durationMonths === 1 ? '' : 's' }}
              <p class="text-xs" :class="e.isMatured ? 'font-semibold text-success' : 'text-muted-foreground'">{{ e.isMatured ? 'Ready to withdraw' : `Unlocks ${unlockDate(e.expirationTimestamp)}` }}</p>
            </div>
            <Button :variant="e.isMatured ? 'default' : 'outline'" :disabled="!e.isMatured" aria-label="withdraw stake" @click="cancel(e.id)"
              >Withdraw</Button
            >
          </div>
          <div class="h-1.5 overflow-hidden rounded-full bg-foreground/10" aria-hidden="true">
            <div class="h-full rounded-full bg-primary" :style="{ width: `${progress(e)}%` }"></div>
          </div>
        </div>
        <p v-if="stake.available && entries.length === 0" class="border-t border-border/70 px-6 py-6 text-sm text-muted-foreground">No active stakes.</p>
      </section>
    </div>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing transaction…</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
