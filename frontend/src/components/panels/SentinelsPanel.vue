<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'

const sentinelStore = useSentinelStore()
const tx = useTxStore()

const { sentinel, depositedQsr, reward: sentinelReward } = storeToRefs(sentinelStore)

const QSR_REQUIRED = 5000000000000n // 50,000 QSR in base units (1e8)
const ZERO = 0n
const error = ref('')

const active = computed(() => !!sentinel.value && sentinel.value.owner !== '')
const deposited = computed(() => BigInt(depositedQsr.value ?? '0'))
const shortfall = computed(() =>
  QSR_REQUIRED > deposited.value ? QSR_REQUIRED - deposited.value : ZERO,
)
const rewardZero = computed(
  () =>
    !sentinelReward.value ||
    (sentinelReward.value.znn === '0' && sentinelReward.value.qsr === '0'),
)

// NoM-confirm pattern: prepare the call, then hand the preview to the global
// NomConfirm dialog via tx.awaitConfirm. The panel renders no modal itself.
async function depositQsr() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareDepositQsr(shortfall.value.toString()))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function register() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareRegisterSentinel())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function collect() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCollectSentinelReward())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function revoke() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareRevokeSentinel())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function withdrawQsr() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareWithdrawQsr())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(() => sentinelStore.refresh())
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') sentinelStore.refresh()
  },
)
</script>

<template>
  <div class="space-y-4 p-4">
    <section
      v-if="active && sentinel"
      class="space-y-3 rounded-lg border border-border bg-card p-4"
    >
      <h2 class="text-sm font-medium text-foreground">Your Sentinel</h2>
      <p class="text-sm text-muted-foreground">
        Status:
        <span class="text-foreground">{{ sentinel.active ? 'Active' : 'Inactive' }}</span>
      </p>
      <p v-if="sentinelReward" class="text-sm text-muted-foreground">
        Uncollected reward
        <span class="font-mono text-foreground"
          >{{ formatAmount(sentinelReward.znn, 8) }} ZNN ·
          {{ formatAmount(sentinelReward.qsr, 8) }} QSR</span
        >
      </p>
      <div class="flex flex-wrap items-center gap-2">
        <Button :disabled="rewardZero" @click="collect">Collect</Button>
        <Button
          variant="outline"
          :disabled="!sentinel.isRevocable"
          aria-label="revoke sentinel"
          @click="revoke"
          >Revoke<template v-if="!sentinel.isRevocable">
            (cooldown {{ sentinel.revokeCooldown }}s)</template
          ></Button
        >
      </div>
    </section>

    <section v-else class="space-y-3 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Register a Sentinel</h2>
      <p class="text-xs text-muted-foreground">
        Requires 50,000 QSR + 5,000 ZNN collateral (returned on revocation).
      </p>
      <template v-if="deposited < QSR_REQUIRED">
        <p class="text-sm text-muted-foreground">
          Deposited
          <span class="font-mono text-foreground"
            >{{ formatAmount(depositedQsr, 8) }} / 50,000 QSR</span
          >
        </p>
        <Button class="w-full" aria-label="deposit qsr" @click="depositQsr"
          >Deposit {{ formatAmount(shortfall.toString(), 8) }} QSR</Button
        >
      </template>
      <template v-else>
        <p class="text-sm text-muted-foreground">50,000 QSR deposited. Ready to register.</p>
        <Button class="w-full" aria-label="register sentinel" @click="register"
          >Register Sentinel (5,000 ZNN)</Button
        >
      </template>
      <Button
        v-if="deposited > ZERO"
        variant="outline"
        class="w-full"
        aria-label="withdraw qsr"
        @click="withdrawQsr"
        >Withdraw deposited QSR</Button
      >
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">
      Preparing… (PoW if required)
    </p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
