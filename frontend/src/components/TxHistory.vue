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

// Default to real value transfers only — hide the zero-amount "plumbing" blocks
// (the send that claims an incoming tx, or a send to collect rewards/fuse plasma,
// which move no tokens themselves). Toggle to "All" to see every account block.
const transfersOnly = ref(true)
function isTransfer(amount: string): boolean {
  try {
    return BigInt(amount || '0') > 0n
  } catch {
    return true
  }
}
const displayed = computed(() => (transfersOnly.value ? items.value.filter((t) => isTransfer(t.amount)) : items.value))

// Our store carries `direction: 'send' | 'receive'`; nom-ui TxDirection takes
// the chain-neutral 'in' | 'out'. receive -> in (incoming/green), send -> out.
function dir(direction: string): 'in' | 'out' {
  return direction === 'send' ? 'out' : 'in'
}

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
        <TableEmpty v-if="displayed.length === 0" :colspan="4">No transactions.</TableEmpty>
        <TableRow v-for="t in displayed" :key="t.hash">
          <TableCell>
            <TxDirection :direction="dir(t.direction)" />
          </TableCell>
          <TableCell>
            <Address :address="t.counterparty" :copy="false" :tooltip="false" />
          </TableCell>
          <TableCell class="text-right font-mono text-foreground">
            {{ formatAmount(t.amount, t.decimals ?? 8) }} {{ t.token }}
          </TableCell>
          <TableCell class="text-right">
            <TxStatus :status="status(t.confirmed)" />
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
