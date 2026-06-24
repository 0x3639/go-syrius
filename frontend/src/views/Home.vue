<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { Card, CardContent, Button, Amount } from 'nom-ui'
import { useNodeStore } from '../stores/node'
import { useWalletStore } from '../stores/wallet'

const node = useNodeStore()
const wallet = useWalletStore()

const znn = computed(() => node.balances.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => node.balances.find((b) => b.symbol === 'QSR'))

// nom-ui's <Amount> takes `value` as a *display* number, not base units — it
// formats with `decimals` but does NOT divide. Our store carries base-unit
// strings (e.g. "150000000" = 1.5 ZNN at 8 decimals), so scale here. Done with
// strings to avoid Number precision loss on large balances.
function toDisplay(amount: string | undefined, decimals: number): string {
  const raw = amount ?? '0'
  if (decimals <= 0) return raw
  const neg = raw.startsWith('-')
  const digits = (neg ? raw.slice(1) : raw).padStart(decimals + 1, '0')
  const whole = digits.slice(0, digits.length - decimals)
  const frac = digits.slice(digits.length - decimals)
  return `${neg ? '-' : ''}${whole}.${frac}`
}

const znnValue = computed(() => toDisplay(znn.value?.amount, znn.value?.decimals ?? 8))
const qsrValue = computed(() => toDisplay(qsr.value?.amount, qsr.value?.decimals ?? 8))

onMounted(async () => {
  await node.connect()
  await node.loadBalances()
})
</script>

<template>
  <main class="min-h-screen space-y-4 bg-background p-8">
    <div class="flex items-center justify-between">
      <span class="text-foreground">{{ wallet.active }}</span>
      <Button variant="ghost" @click="wallet.lock()">Lock</Button>
    </div>
    <div class="grid grid-cols-2 gap-3">
      <Card>
        <CardContent class="p-4">
          <div class="text-xs text-muted-foreground">ZNN</div>
          <Amount :value="znnValue" :decimals="znn?.decimals ?? 8" symbol="ZNN" />
        </CardContent>
      </Card>
      <Card>
        <CardContent class="p-4">
          <div class="text-xs text-muted-foreground">QSR</div>
          <Amount :value="qsrValue" :decimals="qsr?.decimals ?? 8" symbol="QSR" />
        </CardContent>
      </Card>
    </div>
  </main>
</template>
