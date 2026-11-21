import { computed, type Component } from 'vue'
import { AlertTriangleIcon, ArrowDownIcon, GiftIcon, LayersIcon, RocketIcon, ZapIcon } from '@lucide/vue'
import { useNodeStore } from '../stores/node'
import { useUnreceivedStore } from '../stores/unreceived'
import { useStakeStore } from '../stores/stake'
import { usePillarStore } from '../stores/pillar'
import { useSentinelStore } from '../stores/sentinel'
import { usePlasmaStore } from '../stores/plasma'
import { useAcceleratorStore } from '../stores/accelerator'
import { useUiStore } from '../stores/ui'
import { formatAmount } from './format'
import { plasmaLevel } from './plasma'
import { sumBase } from './useHoldings'

export type AttentionItem = {
  key: string
  tone: 'primary' | 'info' | 'warning' | 'danger'
  icon: Component
  title: string
  detail: string
  action: string
  to: string
}

// The single list behind "is there anything I need to do?". Both layouts read
// it; only the wording of a few entries and their destination differ. Every
// entry is a read of data the shell already keeps fresh and only navigates.
export function useAttention() {
  const node = useNodeStore()
  const unreceived = useUnreceivedStore()
  const stake = useStakeStore()
  const pillar = usePillarStore()
  const sentinel = useSentinelStore()
  const plasma = usePlasmaStore()
  const accelerator = useAcceleratorStore()
  const ui = useUiStore()

  const rewardZnn = computed(() => sumBase([pillar.reward?.znn, sentinel.reward?.znn]))
  const rewardQsr = computed(() => sumBase([pillar.reward?.qsr, stake.reward?.qsr, sentinel.reward?.qsr]))
  const hasRewards = computed(() => rewardZnn.value !== '0' || rewardQsr.value !== '0')

  const items = computed<AttentionItem[]>(() => {
    const everyday = ui.mode === 'everyday'
    const list: AttentionItem[] = []
    if (!node.connected) return list // BalanceNotice owns the disconnected state

    if (node.sync?.state === 'stalled') {
      list.push({ key: 'sync', tone: 'danger', icon: AlertTriangleIcon, title: everyday ? 'Your node has stopped syncing' : 'Node sync stalled', detail: 'Balances and incoming payments may be out of date.', action: 'Node settings', to: '/settings' })
    }
    const waiting = unreceived.items
    if (unreceived.failed) {
      list.push({ key: 'incoming-unknown', tone: 'warning', icon: AlertTriangleIcon, title: "Couldn't check for incoming payments", detail: 'The node did not answer. Payments may still be waiting.', action: 'Open Receive', to: '/receive' })
    } else if (waiting.length) {
      const first = waiting.slice(0, 3).map((u) => `${formatAmount(u.amount, u.decimals ?? 8)} ${u.token}`).join(' · ')
      list.push({ key: 'incoming', tone: 'primary', icon: ArrowDownIcon, title: `${waiting.length} ${waiting.length === 1 ? 'payment' : 'payments'} waiting to be received`, detail: waiting.length > 3 ? `${first} and ${waiting.length - 3} more` : first, action: 'Open payments', to: '/receive' })
    }
    const votes = pillar.ownsPillar ? accelerator.needsVoteCount : 0
    if (votes > 0) {
      list.push({ key: 'vote', tone: 'info', icon: RocketIcon, title: `${pillar.myPillar?.name || 'Your pillar'} has ${votes} ${votes === 1 ? 'vote' : 'votes'} waiting`, detail: 'Accelerator-Z projects are waiting on your pillar.', action: everyday ? 'Open in Advanced' : 'Open votes', to: '/network/accelerator?sub=Vote' })
    }
    const matured = stake.available ? (stake.stakeInfo?.entries ?? []).filter((e) => e.isMatured).length : 0
    if (matured > 0) {
      list.push({ key: 'matured', tone: 'primary', icon: LayersIcon, title: `${matured} ${matured === 1 ? 'stake has' : 'stakes have'} finished`, detail: 'The ZNN can be withdrawn back to your balance.', action: 'Withdraw', to: everyday ? '/earn' : '/network/staking' })
    }
    if (hasRewards.value) {
      list.push({ key: 'rewards', tone: 'primary', icon: GiftIcon, title: 'Rewards ready to collect', detail: `${formatAmount(rewardZnn.value, 8)} ZNN · ${formatAmount(rewardQsr.value, 8)} QSR`, action: 'Collect', to: everyday ? '/earn' : '/network/rewards' })
    }
    // Operators act on plasma directly; holders are told about proof-of-work
    // when they review a transaction, so Everyday does not nag about it.
    if (!everyday && plasma.info) {
      const level = plasmaLevel(plasma.info.currentPlasma ?? 0)
      if (level === 'None' || level === 'Low') {
        list.push({ key: 'plasma', tone: 'warning', icon: ZapIcon, title: `Plasma is ${level === 'None' ? 'empty' : 'low'}`, detail: 'The next transaction needs a short proof-of-work first.', action: 'Fuse QSR', to: '/network/plasma' })
      }
    }
    return list
  })

  return { items, rewardZnn, rewardQsr, hasRewards }
}
