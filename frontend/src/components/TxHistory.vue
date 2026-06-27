<script setup lang="ts">
import { computed, ref } from 'vue'
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
import { useUnreceivedStore } from '../stores/unreceived'
import { usePlasmaStore } from '../stores/plasma'
import { formatAmount } from '../lib/format'
import { plasmaLevel } from '../lib/plasma'

const txs = useTxsStore()
const unreceived = useUnreceivedStore()
const plasma = usePlasmaStore()

// While claiming a pending block: PoW generates plasma when the account has none.
const receivingLabel = computed(() =>
  plasmaLevel(plasma.info?.currentPlasma ?? 0) === 'None' ? 'Generating Plasma…' : 'Receiving…',
)

// Receive one pending block, then refresh history so it flips to Confirmed.
async function doReceive(hash: string) {
  await unreceived.receive(hash)
  await txs.load()
}

// Explain an empty page: in Transfers mode a page can be all contract/pair
// activity that's filtered out — point to All rather than imply no transactions.
const emptyMessage = computed(() =>
  txs.transfersOnly && txs.buffer.length > 0
    ? 'No transfers on this page — switch to All to see contract / pair activity.'
    : 'No transactions.',
)

// Our store carries `confirmed: boolean`; nom-ui TxStatus takes a 4-state enum.
function status(confirmed: boolean): 'success' | 'pending' {
  return confirmed ? 'success' : 'pending'
}

// Truncate a 64-char block hash for the row; the full value is copyable.
function shortHash(h: string): string {
  return h && h.length > 12 ? `${h.slice(0, 6)}…${h.slice(-4)}` : h
}

// Show an amount only when value actually moved. Contract calls like
// Delegate/Undelegate carry a token but amount 0 — render "—" like the Pair
// row instead of a misleading "0 ZNN".
function hasValue(t: { token: string; amount: string }): boolean {
  return !!t.token && BigInt(t.amount || '0') > 0n
}

// Copy the full hash; briefly swap the copy icon for a check on the copied row.
const copied = ref('')
async function copyHash(h: string) {
  try {
    await navigator.clipboard?.writeText(h)
    copied.value = h
    window.setTimeout(() => {
      if (copied.value === h) copied.value = ''
    }, 1200)
  } catch {
    /* clipboard unavailable (e.g. no permission); ignore */
  }
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
          :class="txs.transfersOnly ? 'bg-foreground/10 font-medium text-foreground' : 'text-muted-foreground hover:text-foreground'"
          @click="txs.setTransfersOnly(true)"
        >Transfers</button>
        <button
          type="button"
          aria-label="show all transactions"
          class="rounded px-2 py-1 transition-colors"
          :class="!txs.transfersOnly ? 'bg-foreground/10 font-medium text-foreground' : 'text-muted-foreground hover:text-foreground'"
          @click="txs.setTransfersOnly(false)"
        >All</button>
      </div>
    </div>
    <!-- Fixed layout + explicit column widths so the columns never reflow when
         the visible method badges change (Transfers↔All, paging). Address takes
         the remaining space; the rest are pinned. -->
    <Table class="table-fixed">
      <colgroup>
        <col class="w-20" />
        <col class="w-32" />
        <col />
        <col class="w-36" />
        <col class="w-28" />
        <col class="w-40" />
      </colgroup>
      <TableBody>
        <TableEmpty v-if="txs.pageItems.length === 0 && (txs.page > 0 || unreceived.items.length === 0)" :colspan="6">{{ emptyMessage }}</TableEmpty>

        <!-- Pending inbound blocks (newest page only): click to receive; status
             goes Unreceived → Generating Plasma/Receiving (pulsing) → Confirmed. -->
        <template v-if="txs.page === 0">
        <TableRow v-for="u in unreceived.items" :key="u.fromHash">
          <TableCell><TxDirection direction="in" /></TableCell>
          <TableCell></TableCell>
          <TableCell>
            <Address :address="u.fromAddress" :copy="false" :tooltip="false" />
          </TableCell>
          <TableCell>
            <button
              v-if="u.fromHash"
              type="button"
              :aria-label="`copy hash ${u.fromHash}`"
              title="Copy hash"
              class="inline-flex items-center gap-1 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
              @click="copyHash(u.fromHash)"
            >
              <span>{{ shortHash(u.fromHash) }}</span>
              <svg v-if="copied === u.fromHash" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
              <svg v-else width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="13" height="13" x="9" y="9" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
            </button>
          </TableCell>
          <TableCell class="text-right font-mono text-foreground">
            {{ formatAmount(u.amount, u.decimals ?? 8) }} {{ u.token }}
          </TableCell>
          <TableCell class="whitespace-nowrap text-right">
            <span
              v-if="unreceived.busy[u.fromHash]"
              class="inline-flex animate-pulse items-center rounded-full bg-info/15 px-2 py-0.5 text-xs font-medium text-info"
            >{{ receivingLabel }}</span>
            <button
              v-else
              type="button"
              :aria-label="`receive ${u.fromHash}`"
              title="Receive"
              class="inline-flex items-center gap-1 rounded-full bg-foreground/10 px-2 py-0.5 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
              @click="doReceive(u.fromHash)"
            >
              Unreceived
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 8v8M8.5 12.5 12 16l3.5-3.5"/></svg>
            </button>
          </TableCell>
        </TableRow>
        </template>

        <TableRow v-for="t in txs.pageItems" :key="t.hash">
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
          <TableCell>
            <button
              v-if="t.hash"
              type="button"
              :aria-label="`copy hash ${t.hash}`"
              title="Copy hash"
              class="inline-flex items-center gap-1 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
              @click="copyHash(t.hash)"
            >
              <span>{{ shortHash(t.hash) }}</span>
              <svg v-if="copied === t.hash" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
              <svg v-else width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="13" height="13" x="9" y="9" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
            </button>
          </TableCell>
          <TableCell class="text-right font-mono text-foreground">
            <template v-if="!hasValue(t)"><span class="text-muted-foreground">—</span></template>
            <template v-else>{{ formatAmount(t.amount, t.decimals ?? 8) }} {{ t.token }}</template>
          </TableCell>
          <TableCell class="whitespace-nowrap text-right">
            <TxStatus :status="status(t.confirmed)" />
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>

    <div v-if="txs.page > 0 || txs.hasNextPage" class="mt-2 flex items-center justify-end gap-3 text-xs text-muted-foreground">
      <span>Page {{ txs.page + 1 }}</span>
      <button
        type="button"
        aria-label="previous page"
        :disabled="txs.page === 0"
        class="grid h-7 w-7 place-items-center rounded border border-border transition-colors hover:bg-foreground/[0.06] disabled:opacity-40"
        @click="txs.goto(txs.page - 1)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 18l-6-6 6-6"/></svg>
      </button>
      <button
        type="button"
        aria-label="next page"
        :disabled="!txs.hasNextPage"
        class="grid h-7 w-7 place-items-center rounded border border-border transition-colors hover:bg-foreground/[0.06] disabled:opacity-40"
        @click="txs.goto(txs.page + 1)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18l6-6-6-6"/></svg>
      </button>
    </div>
  </div>
</template>
