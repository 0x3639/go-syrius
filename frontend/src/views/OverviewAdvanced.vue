<script setup lang="ts">
import { computed, ref, watch, type Component } from 'vue'
import { useRouter } from 'vue-router'
import {
  ZapIcon, LayersIcon, Building2Icon, ShieldCheckIcon, RocketIcon, CoinsIcon,
  LinkIcon, ServerIcon, PinIcon,
} from '@lucide/vue'
import { useNodeStore } from '../stores/node'
import { useWalletStore } from '../stores/wallet'
import { useBalancesStore } from '../stores/balances'
import { usePlasmaStore } from '../stores/plasma'
import { useStakeStore } from '../stores/stake'
import { usePillarStore } from '../stores/pillar'
import { useSentinelStore } from '../stores/sentinel'
import { useAcceleratorStore } from '../stores/accelerator'
import { useTokenStore } from '../stores/token'
import { useWalletConnectStore } from '../stores/walletconnect'
import { useUiStore, PINNABLE, MAX_PINS, type PinId } from '../stores/ui'
import { useHoldings } from '../lib/useHoldings'
import { useAttention } from '../lib/useAttention'
import { useNodeRefresh } from '../lib/useNodeRefresh'
import { formatAmount } from '../lib/format'
import { formatFiat } from '../lib/fiat'
import { plasmaLevel } from '../lib/plasma'
import TxHistory from '../components/TxHistory.vue'
import BalanceNotice from '../components/BalanceNotice.vue'
import HoldingsTable from '../components/HoldingsTable.vue'
import AttentionList from '../components/AttentionList.vue'

// Operator overview: what this account must act on first, then shortcuts the
// operator chose, then holdings, then a tile per position or service. Tiles
// the account actually uses come first; the rest collapse into one quiet row.
const router = useRouter()
const node = useNodeStore()
const wallet = useWalletStore()
const balances = useBalancesStore()
const plasma = usePlasmaStore()
const stake = useStakeStore()
const pillar = usePillarStore()
const sentinel = useSentinelStore()
const accelerator = useAcceleratorStore()
const token = useTokenStore()
const walletConnect = useWalletConnectStore()
const ui = useUiStore()
const { totalUsd, unpriced, waitingUnknown, committedUnknown } = useHoldings()
const { items } = useAttention()

useNodeRefresh(() => token.refresh())
// Issued tokens belong to an address; follow account switches. The store's
// epoch check drops a response that arrives for the previous account.
watch(() => wallet.activeIndex, () => { if (node.connected) token.refresh() })

const editingPins = ref(false)
const pinIds = Object.keys(PINNABLE) as PinId[]

type Tile = { key: string; to: string; icon: Component; title: string; status: string; tone: 'ok' | 'warn' | 'bad' | 'info'; figure: string; detail: string; used: boolean }
const TONES = { ok: 'bg-success/15 text-success', warn: 'bg-warning/15 text-warning', bad: 'bg-destructive/15 text-destructive', info: 'bg-info/15 text-info' } as const

const tiles = computed<Tile[]>(() => {
  const level = plasma.info ? plasmaLevel(plasma.info.currentPlasma ?? 0) : null
  const stakes = stake.available ? stake.stakeInfo?.entries ?? [] : []
  const votes = pillar.ownsPillar ? accelerator.needsVoteCount : 0
  const delegated = pillar.delegation?.name || ''
  const sessions = walletConnect.sessions
  const stalled = node.sync?.state === 'stalled'
  return [
    { key: 'plasma', to: '/network/plasma', icon: ZapIcon, title: 'Plasma', status: level ?? '', tone: level === 'High' || level === 'Medium' ? 'ok' : 'warn', figure: plasma.info ? `${formatAmount(plasma.info.qsrFused, 8)} QSR fused` : '', detail: `${plasma.fusionEntries.length} fusion ${plasma.fusionEntries.length === 1 ? 'entry' : 'entries'}`, used: !!plasma.info },
    { key: 'staking', to: '/network/staking', icon: LayersIcon, title: 'Staking', status: stakes.some((e) => e.isMatured) ? 'Withdrawable' : 'Active', tone: 'ok', figure: `${formatAmount(stake.stakeInfo?.totalAmount ?? '0', 8)} ZNN`, detail: `${stakes.length} ${stakes.length === 1 ? 'entry' : 'entries'}`, used: stakes.length > 0 },
    { key: 'pillar', to: '/network/pillars', icon: Building2Icon, title: 'Pillar', status: pillar.ownsPillar ? 'Owner' : 'Delegating', tone: 'ok', figure: pillar.ownsPillar ? pillar.myPillar?.name ?? '' : delegated, detail: pillar.ownsPillar ? `Shares ${pillar.myPillar?.giveDelegateRewardPct ?? 0}% of delegate rewards` : 'Delegated ZNN stays spendable', used: pillar.ownsPillar || !!delegated },
    { key: 'accelerator', to: '/network/accelerator?sub=Vote', icon: RocketIcon, title: 'Accelerator-Z', status: votes ? `${votes} to vote` : 'Up to date', tone: votes ? 'info' : 'ok', figure: votes ? `${votes} ${votes === 1 ? 'vote' : 'votes'} waiting` : 'No votes due', detail: `As pillar ${pillar.myPillar?.name ?? ''}`, used: pillar.ownsPillar },
    { key: 'sentinel', to: '/network/sentinels', icon: ShieldCheckIcon, title: 'Sentinel', status: 'Active', tone: 'ok', figure: 'Registered', detail: 'Earning ZNN and QSR', used: sentinel.active },
    { key: 'tokens', to: '/tokens', icon: CoinsIcon, title: 'Tokens issued', status: 'Owner', tone: 'ok', figure: `${token.myTokens.length} ${token.myTokens.length === 1 ? 'token' : 'tokens'}`, detail: 'Mint, burn and transfer ownership', used: token.myTokens.length > 0 },
    { key: 'walletconnect', to: '/walletconnect', icon: LinkIcon, title: 'WalletConnect', status: 'Connected', tone: 'ok', figure: sessions[0]?.name ?? '', detail: sessions.length > 1 ? `and ${sessions.length - 1} more` : 'Disconnect from the WalletConnect screen', used: sessions.length > 0 },
    { key: 'node', to: '/settings', icon: ServerIcon, title: 'Node', status: !node.connected ? 'Disconnected' : stalled ? 'Stalled' : node.syncing ? 'Syncing' : 'Synced', tone: !node.connected || stalled ? 'bad' : node.syncing ? 'warn' : 'ok', figure: node.connected ? `#${node.height.toLocaleString('en-US')}` : '—', detail: node.mode === 'embedded' ? `Embedded${node.sync ? ` · ${node.sync.peers} peers` : ''}` : 'Remote node', used: true },
  ]
})
const used = computed(() => tiles.value.filter((t) => t.used))
const unused = computed(() => tiles.value.filter((t) => !t.used))
</script>

<template>
  <div class="page-content">
    <BalanceNotice />

    <section class="panel-list" aria-labelledby="ov-action">
      <h2 id="ov-action" class="section-title px-6 py-4">Needs action <span class="font-normal text-muted-foreground">· {{ items.length }}</span></h2>
      <AttentionList dense />
    </section>

    <section class="rounded-2xl border border-border bg-card px-6 py-3.5" aria-label="Pinned actions">
      <div class="flex flex-wrap items-center gap-2">
        <span class="mr-1 inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground"><PinIcon :size="14" />Pinned</span>
        <button v-for="id in ui.pins" :key="id" type="button" :data-testid="`pin-${id}`"
          class="inline-flex h-8.5 items-center rounded-lg border border-input px-3 text-sm font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]"
          @click="router.push(PINNABLE[id].to)">{{ PINNABLE[id].label }}</button>
        <span v-if="!ui.pins.length" class="hint">Nothing pinned yet.</span>
        <button type="button" class="ml-auto text-sm font-semibold text-primary hover:underline" :aria-expanded="editingPins" @click="editingPins = !editingPins">
          {{ editingPins ? 'Done' : 'Edit pins' }}
        </button>
      </div>
      <fieldset v-if="editingPins" class="mt-3 grid grid-cols-1 gap-x-6 gap-y-1.5 border-t border-border pt-3 @xl:grid-cols-2 @3xl:grid-cols-3">
        <legend class="sr-only">Choose pinned actions</legend>
        <label v-for="id in pinIds" :key="id" class="flex items-center gap-2 text-sm text-foreground">
          <input type="checkbox" class="accent-primary" :checked="ui.pins.includes(id)"
            :disabled="!ui.pins.includes(id) && ui.pins.length >= MAX_PINS"
            :aria-label="`Pin ${PINNABLE[id].label}`" @change="ui.togglePin(id)" />
          {{ PINNABLE[id].label }}
        </label>
        <p class="hint col-span-full">Up to {{ MAX_PINS }} shortcuts.</p>
      </fieldset>
    </section>

    <section class="panel-list" aria-labelledby="ov-holdings">
      <h2 id="ov-holdings" class="section-title px-6 py-4">Holdings</h2>
      <HoldingsTable dense />
      <p v-if="balances.available && totalUsd !== null" class="hint border-t border-border px-6 py-3" data-testid="estimate-scope">
        <!-- Only claim staked and locked amounts are covered when they were read. -->
        <template v-if="committedUnknown">Estimate {{ formatFiat(totalUsd) }} covers ZNN and QSR, spendable plus the committed amounts shown. Staked or locked amounts could not all be checked, so it may be low.</template>
        <template v-else>Estimate {{ formatFiat(totalUsd) }} covers ZNN and QSR, spendable and committed (staked, fused, collateral and deposits).</template>
        <template v-if="unpriced.length">{{ unpriced.join(', ') }} unpriced.</template> Waiting amounts are excluded<template v-if="waitingUnknown"> and could not be checked</template>.
      </p>
    </section>

    <section aria-labelledby="ov-positions">
      <h2 id="ov-positions" class="sr-only">Positions and services</h2>
      <div class="grid grid-cols-1 gap-3 @xl:grid-cols-2 @4xl:grid-cols-4">
        <button v-for="t in used" :key="t.key" type="button" :data-testid="`tile-${t.key}`"
          class="flex min-w-0 flex-col gap-1 rounded-xl border border-border bg-card px-4 py-3 text-left transition-colors hover:bg-foreground/[0.04]"
          @click="router.push(t.to)">
          <span class="flex items-center gap-2 text-xs font-semibold text-muted-foreground">
            <component :is="t.icon" :size="14" />{{ t.title }}
            <span v-if="t.status" class="status-pill ml-auto" :class="TONES[t.tone]">{{ t.status }}</span>
          </span>
          <span class="truncate font-mono text-sm font-bold tabular-nums text-foreground">{{ t.figure || '—' }}</span>
          <span class="truncate text-xs text-muted-foreground">{{ t.detail }}</span>
        </button>
      </div>
      <div v-if="unused.length" class="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <span>Not used by this account:</span>
        <button v-for="t in unused" :key="t.key" type="button" :data-testid="`unused-${t.key}`"
          class="rounded-full border border-dashed border-border px-3 py-1 transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
          @click="router.push(t.to)">{{ t.title }}</button>
      </div>
    </section>

    <section class="panel-list">
      <h2 class="section-title px-6 pb-1 pt-5">Recent activity</h2>
      <div class="px-3 pb-3">
        <TxHistory />
      </div>
    </section>
  </div>
</template>
