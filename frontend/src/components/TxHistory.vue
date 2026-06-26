<script setup lang="ts">
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
    <h2 class="mb-2 text-sm text-muted-foreground">Recent transactions</h2>
    <Table>
      <TableBody>
        <TableEmpty v-if="items.length === 0" :colspan="4">No transactions.</TableEmpty>
        <TableRow v-for="t in items" :key="t.hash">
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
