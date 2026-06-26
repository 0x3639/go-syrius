<script setup lang="ts">
import { ref, computed } from 'vue'
import { storeToRefs } from 'pinia'
import {
  Address,
  Table,
  TableBody,
  TableCell,
  TableEmpty,
  TableRow,
  TxDirection,
  TxStatus,
} from 'nom-ui'
import { useTxsStore } from '../stores/txs'
import { formatAmount } from '../lib/format'

const { items } = storeToRefs(useTxsStore())

// Default to real value transfers only — show only In/Out rows that move a
// non-zero amount, hiding the Pair claim blocks and zero-amount action calls
// (CollectReward, plasma fuse, …). Toggle to "All" for every row (like nomscan).
const transfersOnly = ref(true)
function isTransfer(t: { direction: string; amount: string }): boolean {
  if (t.direction === 'pair') return false
  try {
    return BigInt(t.amount || '0') > 0n
  } catch {
    return true
  }
}
const displayed = computed(() => (transfersOnly.value ? items.value.filter(isTransfer) : items.value))

// Our store carries `confirmed: boolean`; nom-ui TxStatus takes a 4-state enum.
// We only distinguish confirmed vs not, so map true -> success, false -> pending.
function status(confirmed: boolean): 'success' | 'pending' {
  return confirmed ? 'success' : 'pending'
}
</script>

<template>
  <div class="rounded border border-border bg-card px-4 py-3">
    <div class="mb-2 flex items-center justify-between">
      <h2 class="text-sm text-muted-foreground">Recent transactions</h2>
      <div class="flex items-center gap-0.5 rounded-md border border-border p-0.5 text-xs">
        <button
          type="button"
          aria-label="show transfers only"
          class="rounded px-2 py-1 transition-colors"
          :class="transfersOnly ? 'bg-foreground/10 font-medium text-foreground' : 'text-muted-foreground hover:text-foreground'"
          @click="transfersOnly = true"
        >Transfers</button>
        <button
          type="button"
          aria-label="show all transactions"
          class="rounded px-2 py-1 transition-colors"
          :class="!transfersOnly ? 'bg-foreground/10 font-medium text-foreground' : 'text-muted-foreground hover:text-foreground'"
          @click="transfersOnly = false"
        >All</button>
      </div>
    </div>
    <Table>
      <TableBody>
        <TableEmpty v-if="displayed.length === 0" :colspan="5">No transactions.</TableEmpty>
        <TableRow v-for="t in displayed" :key="t.hash">
          <TableCell>
            <span v-if="t.direction === 'pair'" class="rounded bg-info/15 px-2 py-0.5 text-xs font-medium text-info">Pair</span>
            <TxDirection v-else :direction="(t.direction as 'in' | 'out')" />
          </TableCell>
          <TableCell>
            <span v-if="t.method" class="rounded bg-foreground/10 px-2 py-0.5 text-xs text-muted-foreground">{{ t.method }}</span>
          </TableCell>
          <TableCell>
            <Address :address="t.counterparty" :copy="false" :tooltip="false" />
          </TableCell>
          <TableCell class="text-right font-mono text-foreground">
            <template v-if="!t.token"><span class="text-muted-foreground">—</span></template>
            <template v-else>{{ formatAmount(t.amount, t.decimals ?? 8) }} {{ t.token }}</template>
          </TableCell>
          <TableCell class="text-right">
            <TxStatus :status="status(t.confirmed)" />
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
