<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { Card, CardContent, Button } from 'nom-ui'
import { useNodeStore } from '../stores/node'
import { useWalletStore } from '../stores/wallet'
import { formatAmount } from '../lib/format'

const node = useNodeStore()
const wallet = useWalletStore()

const znn = computed(() => node.balances.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => node.balances.find((b) => b.symbol === 'QSR'))

// Format base-unit balances with our display rule (3+ integer digits drop the
// decimals + add thousands commas; smaller values round to 2dp). We format here
// rather than via nom-ui's <Amount>, which shows full precision and rounds
// through Number() (precision loss on large balances).
const znnValue = computed(() => formatAmount(znn.value?.amount ?? '0', znn.value?.decimals ?? 8))
const qsrValue = computed(() => formatAmount(qsr.value?.amount ?? '0', qsr.value?.decimals ?? 8))

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
          <div class="font-mono text-2xl text-foreground">{{ znnValue }} <span class="text-base text-muted-foreground">ZNN</span></div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="p-4">
          <div class="text-xs text-muted-foreground">QSR</div>
          <div class="font-mono text-2xl text-foreground">{{ qsrValue }} <span class="text-base text-muted-foreground">QSR</span></div>
        </CardContent>
      </Card>
    </div>
  </main>
</template>
