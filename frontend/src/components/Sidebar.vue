<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  LayoutDashboardIcon, SendIcon, DownloadIcon, CoinsIcon, ZapIcon, LayersIcon,
  Building2Icon, ShieldCheckIcon, RocketIcon, GiftIcon, SettingsIcon,
  BookUserIcon, ShieldIcon, LinkIcon, ActivityIcon, LayoutGridIcon, HomeIcon,
} from '@lucide/vue'
import { useNodeStore } from '../stores/node'
import { useUnreceivedStore } from '../stores/unreceived'
import { useUiStore } from '../stores/ui'
import AccountSlotPicker from './AccountSlotPicker.vue'
import SidebarLink from './SidebarLink.vue'
import { GetBuildInfo } from '../../wailsjs/go/app/ConfigService'

const node = useNodeStore()
const unreceived = useUnreceivedStore()
const ui = useUiStore()
const everyday = computed(() => ui.mode === 'everyday')

// Everyday: five destinations, with Send and Receive as buttons above them.
// Operator tools are reachable only through More tools, which names them and
// switches to Advanced.
const everydayNav = [
  { to: '/dashboard', label: 'Home', icon: HomeIcon },
  { to: '/activity', label: 'Activity', icon: ActivityIcon },
  { to: '/earn', label: 'Earn', icon: GiftIcon },
  { to: '/tools', label: 'More tools', icon: LayoutGridIcon },
  { to: '/settings', label: 'Settings', icon: SettingsIcon },
]

const walletNav = computed(() => [
  { to: '/dashboard', label: 'Overview', icon: LayoutDashboardIcon },
  { to: '/transfer', label: 'Send', icon: SendIcon },
  { to: '/receive', label: 'Receive', icon: DownloadIcon, badge: unreceived.items.length },
  { to: '/activity', label: 'Activity', icon: ActivityIcon },
  { to: '/tokens', label: 'Tokens', icon: CoinsIcon },
])
const networkNav = [
  { to: '/network/plasma', label: 'Plasma', icon: ZapIcon },
  { to: '/network/staking', label: 'Staking', icon: LayersIcon },
  { to: '/network/pillars', label: 'Pillars', icon: Building2Icon },
  { to: '/network/sentinels', label: 'Sentinels', icon: ShieldCheckIcon },
  { to: '/network/accelerator', label: 'Accelerator', icon: RocketIcon },
  { to: '/network/rewards', label: 'Rewards', icon: GiftIcon },
]
const bottomNav = [
  { to: '/walletconnect', label: 'WalletConnect', icon: LinkIcon },
  { to: '/address-book', label: 'Address book', icon: BookUserIcon },
  { to: '/settings', label: 'Settings', icon: SettingsIcon },
]

const heightLabel = computed(() => node.height.toLocaleString('en-US'))
const checkingSync = computed(() => node.mode === 'embedded' && !node.sync)
const synced = computed(() => node.connected && !node.syncing && !checkingSync.value)
const statusLabel = computed(() => !node.connected ? 'Disconnected' : checkingSync.value ? 'Checking sync…' : node.sync?.state === 'stalled' ? 'Sync stalled' : node.syncing ? 'Syncing…' : 'Synced')

// Progress bar under the sync label: the real percentage while the embedded
// node reports one, full when synced, empty while disconnected.
const syncPercent = computed(() => {
  if (!node.connected) return 0
  if (synced.value) return 100
  const p = Number(node.sync?.percent)
  return Number.isFinite(p) ? Math.max(0, Math.min(100, p)) : 0
})
const syncPercentLabel = computed(() =>
  node.connected && node.syncing && syncPercent.value > 0 && syncPercent.value < 100 ? ` ${Math.floor(syncPercent.value)}%` : '')

const appVersion = ref('')
onMounted(async () => {
  try {
    appVersion.value = (await GetBuildInfo()).version
  } catch {
    appVersion.value = 'dev'
  }
})
</script>

<template>
  <aside class="relative z-10 flex min-h-0 w-56 flex-none flex-col gap-5 border-r border-sidebar-border bg-sidebar px-3.5 py-5 lg:w-64" aria-label="Main navigation">
    <!-- Wordmark -->
    <div class="flex items-center gap-2.5 px-2">
      <img src="../assets/images/syrius-logo.png" alt="" class="h-7 w-7 rounded-md" />
      <span class="text-base font-bold tracking-tight text-sidebar-foreground">go-syrius</span>
    </div>

    <!-- Active account: opens the account switcher -->
    <AccountSlotPicker variant="sidebar" />

    <!-- Only this region scrolls: the picker's dropdown is wider than the
         sidebar and must not sit inside an overflow container. -->
    <div class="-mx-3.5 flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-3.5">
    <template v-if="everyday">
      <div class="grid grid-cols-2 gap-2">
        <RouterLink to="/transfer" class="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-primary text-sm font-bold text-primary-foreground transition-opacity hover:opacity-90">
          <SendIcon :size="15" /> Send
        </RouterLink>
        <RouterLink to="/receive" class="relative inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-input text-sm font-semibold text-sidebar-foreground transition-colors hover:bg-sidebar-accent/60">
          <DownloadIcon :size="15" /> Receive
          <span v-if="unreceived.items.length" class="absolute -right-1 -top-1 rounded-full bg-primary px-1.5 text-xs font-bold leading-5 text-primary-foreground" :aria-label="`${unreceived.items.length} waiting`">{{ unreceived.items.length }}</span>
        </RouterLink>
      </div>
      <nav class="flex flex-col gap-0.5" aria-label="Wallet">
        <SidebarLink v-for="item in everydayNav" :key="item.to" v-bind="item" />
      </nav>
    </template>

    <template v-else>
    <div>
      <div class="px-2.5 pb-1.5 text-xs font-semibold text-muted-foreground">Wallet</div>
      <nav class="flex flex-col gap-0.5" aria-label="Wallet">
        <SidebarLink v-for="item in walletNav" :key="item.to" v-bind="item" />
      </nav>
    </div>

    <div>
      <div class="px-2.5 pb-1.5 text-xs font-semibold text-muted-foreground">Network of Momentum</div>
      <nav class="flex flex-col gap-0.5" aria-label="Network of Momentum">
        <SidebarLink v-for="item in networkNav" :key="item.to" v-bind="item" />
      </nav>
    </div>
    </template>

    <!-- Bottom: utilities + node-sync card -->
    <div class="mt-auto flex flex-col gap-0.5 pt-2">
      <template v-if="!everyday"><SidebarLink v-for="item in bottomNav" :key="item.to" v-bind="item" /></template>
      <div class="mt-2.5 flex flex-col gap-2 rounded-lg bg-sidebar-accent px-3 py-2.5" role="status">
        <div class="flex items-center justify-between gap-2 text-xs">
          <span class="flex items-center gap-1.5 font-semibold" :class="synced ? 'text-success' : 'text-warning'">
            <ShieldIcon :size="14" />{{ statusLabel }}{{ syncPercentLabel }}
          </span>
          <span v-if="node.connected" class="font-mono text-muted-foreground">#{{ heightLabel }}</span>
        </div>
        <div class="h-1 overflow-hidden rounded-full bg-foreground/10" aria-hidden="true">
          <div class="h-full rounded-full transition-[width]" :class="synced ? 'bg-success' : 'bg-warning'" :style="{ width: `${syncPercent}%` }"></div>
        </div>
      </div>
      <p v-if="appVersion" class="px-3 pt-1.5 text-center font-mono text-[11px] text-muted-foreground">{{ appVersion }}</p>
    </div>
    </div>
  </aside>
</template>
