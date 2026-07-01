<script setup lang="ts">
import { computed } from 'vue'
import {
  LayoutDashboardIcon, SendIcon, DownloadIcon, CoinsIcon, ZapIcon, LayersIcon,
  Building2Icon, ShieldCheckIcon, RocketIcon, GiftIcon, VoteIcon, SettingsIcon,
  BookUserIcon, ShieldIcon,
} from '@lucide/vue'
import { useNodeStore } from '../stores/node'
import { useUiStore } from '../stores/ui'
import SidebarLink from './SidebarLink.vue'

const node = useNodeStore()
const ui = useUiStore()

const showGovernance = computed(() => ui.showGovernance && node.chainId !== 1)

const topNav = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboardIcon },
  { to: '/transfer', label: 'Transfer', icon: SendIcon },
  { to: '/receive', label: 'Receive', icon: DownloadIcon },
]
const networkNav = computed(() => [
  { to: '/tokens', label: 'Tokens', icon: CoinsIcon },
  { to: '/network/plasma', label: 'Plasma', icon: ZapIcon },
  { to: '/network/staking', label: 'Staking', icon: LayersIcon },
  { to: '/network/pillars', label: 'Pillars', icon: Building2Icon },
  { to: '/network/sentinels', label: 'Sentinels', icon: ShieldCheckIcon },
  { to: '/network/accelerator', label: 'Accelerator', icon: RocketIcon },
  { to: '/network/rewards', label: 'Rewards', icon: GiftIcon },
  ...(showGovernance.value ? [{ to: '/network/governance', label: 'Governance', icon: VoteIcon }] : []),
])
const bottomNav = [
  { to: '/settings', label: 'Settings', icon: SettingsIcon },
  { to: '/address-book', label: 'Address book', icon: BookUserIcon },
]

const heightLabel = computed(() => node.height.toLocaleString('en-US'))
const synced = computed(() => node.connected && !node.syncing)
</script>

<template>
  <aside class="flex w-58 flex-none flex-col border-r border-sidebar-border bg-sidebar px-3.5 py-5">
    <!-- Wordmark -->
    <div class="flex items-center gap-2.5 px-2 pb-5">
      <img src="../assets/images/syrius-logo.png" alt="" class="h-7 w-7 rounded-md" />
      <div class="flex flex-col leading-tight">
        <span class="text-base font-bold tracking-tight text-sidebar-foreground">go-syrius</span>
        <span class="text-ledger text-muted-foreground">Network of Momentum</span>
      </div>
    </div>

    <!-- Primary nav -->
    <nav class="flex flex-col gap-0.5">
      <SidebarLink v-for="item in topNav" :key="item.to" v-bind="item" />
    </nav>

    <!-- Network section -->
    <div class="text-ledger mt-5 px-3 pb-1 text-muted-foreground">Network of Momentum</div>
    <nav class="flex flex-col gap-0.5">
      <SidebarLink v-for="item in networkNav" :key="item.to" v-bind="item" />
    </nav>

    <!-- Bottom: settings, address book, node-sync pill -->
    <div class="mt-auto flex flex-col gap-0.5 pt-4">
      <SidebarLink v-for="item in bottomNav" :key="item.to" v-bind="item" />
      <div class="mt-1.5 flex items-center gap-2 rounded-md bg-sidebar-accent px-3 py-2.5">
        <ShieldIcon :size="16" :class="synced ? 'text-success' : 'text-warning'" />
        <span class="text-xs text-muted-foreground">{{ synced ? 'Node synced' : 'Syncing…' }}</span>
        <span class="ml-auto font-mono text-xs" :class="synced ? 'text-success' : 'text-warning'">#{{ heightLabel }}</span>
      </div>
    </div>
  </aside>
</template>
