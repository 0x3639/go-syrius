<script setup lang="ts">
import { useBalancesStore } from '../stores/balances'
import { useHoldings } from '../lib/useHoldings'
import { formatFiat } from '../lib/fiat'
import TokenAmount from './TokenAmount.vue'

// Every asset the account holds, split into what can be spent now, what is
// waiting to be received, and what is committed: staking, fusion, pillar and
// sentinel collateral, registration deposits. `dense`
// (Advanced) adds the token standard and a fiat column.
defineProps<{ dense?: boolean }>()
const balances = useBalancesStore()
const { rows, waitingUnknown } = useHoldings()
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full border-collapse text-sm" aria-label="Holdings">
      <thead>
        <tr class="text-xs text-muted-foreground">
          <th scope="col" class="border-t border-border px-6 py-2 text-left font-semibold">Asset</th>
          <th scope="col" class="border-t border-border px-3 py-2 text-right font-semibold">Spendable</th>
          <th scope="col" class="border-t border-border px-3 py-2 text-right font-semibold">Waiting</th>
          <th scope="col" class="border-t border-border px-3 py-2 text-right font-semibold" :class="dense ? '' : 'pr-6'">Committed</th>
          <th v-if="dense" scope="col" class="border-t border-border py-2 pl-3 pr-6 text-right font-semibold">Est. value</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.zts" :data-testid="`holding-${r.symbol}`" :data-zts="r.zts">
          <td class="border-t border-border px-6 align-top" :class="dense ? 'py-2.5' : 'py-3.5'">
            <div class="flex items-center gap-3">
              <span class="avatar-dot" :class="r.symbol === 'QSR' ? 'bg-info/15 text-info' : r.protocol ? '' : 'bg-foreground/10 text-muted-foreground'" aria-hidden="true">{{ r.symbol.slice(0, 3) }}</span>
              <span class="min-w-0">
                <span class="block font-semibold text-foreground">{{ r.name }} <span v-if="r.name !== r.symbol" class="font-medium text-muted-foreground">{{ r.symbol }}</span></span>
                <span v-if="dense && r.zts.startsWith('zts1')" class="block font-mono text-xs text-muted-foreground">{{ r.zts }}</span>
                <span v-else-if="!r.protocol" class="block text-xs text-muted-foreground">Custom token</span>
                <span v-if="!dense && balances.available" class="block text-xs text-muted-foreground">{{ r.usd === null ? 'No price' : '≈ ' + formatFiat(r.usd) }}</span>
              </span>
            </div>
          </td>
          <td class="whitespace-nowrap border-t border-border px-3 text-right align-top text-foreground" :class="dense ? 'py-2.5' : 'py-3.5'" :aria-label="`${r.symbol} balance`">
            <TokenAmount v-if="balances.available" :amount="r.spendable" :decimals="r.decimals" />
            <span v-else>—</span>
          </td>
          <td class="whitespace-nowrap border-t border-border px-3 text-right align-top" :class="dense ? 'py-2.5' : 'py-3.5'">
            <span v-if="waitingUnknown" class="font-semibold text-warning" title="Couldn't check for incoming payments" aria-label="Unknown: couldn't check for incoming payments">?</span>
            <span v-else-if="r.waiting !== '0'" class="text-primary">+<TokenAmount :amount="r.waiting" :decimals="r.decimals" /></span>
            <span v-else class="text-muted-foreground">—</span>
          </td>
          <td class="border-t border-border px-3 text-right align-top" :class="dense ? 'py-2.5' : 'py-3.5 pr-6'">
            <template v-if="r.committed.length">
              <span class="whitespace-nowrap text-foreground"><TokenAmount :amount="r.committedTotal" :decimals="r.decimals" /></span>
              <!-- One line per commitment. With several, each shows its own amount
                   so the total above can be checked against its parts. -->
              <span v-for="c in r.committed" :key="c.label" class="block text-xs text-muted-foreground">
                <!-- The explicit space matters: a bare trailing space inside the
                     template is dropped by the compiler, gluing amount to label. -->
                <template v-if="r.committed.length > 1"><TokenAmount :amount="c.amount" :decimals="r.decimals" />{{ ' ' }}</template>{{ c.label }}
              </span>
            </template>
            <span v-else-if="!r.committedUnknown && !r.committedPending" class="text-muted-foreground">—</span>
            <!-- Unknown and still-loading are their own answers, never "—". -->
            <span v-if="r.committedUnknown" class="block text-xs font-semibold text-warning" data-testid="committed-unknown"
              title="Staked or locked amounts could not be read from the node. They are retried automatically.">? couldn't check</span>
            <span v-else-if="r.committedPending" class="block text-xs text-muted-foreground" data-testid="committed-pending">updating…</span>
          </td>
          <td v-if="dense" class="whitespace-nowrap border-t border-border py-2.5 pl-3 pr-6 text-right align-top font-mono tabular-nums">
            <span v-if="!balances.available" class="text-muted-foreground">—</span>
            <span v-else-if="r.usd === null" class="font-sans text-muted-foreground">No price</span>
            <span v-else class="text-foreground">≈ {{ formatFiat(r.usd) }}</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
