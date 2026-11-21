<script setup lang="ts">
import { computed, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { ZapIcon, BookUserIcon, LinkIcon, Building2Icon, ShieldCheckIcon, CoinsIcon, RocketIcon } from '@lucide/vue'
import { usePlasmaStore } from '../stores/plasma'
import { useWalletConnectStore } from '../stores/walletconnect'
import { formatAmount } from '../lib/format'

// Everyday's index of everything outside its five destinations. Tools a
// holder may use open directly. Operator tools are named so they can be
// discovered, but open in Advanced: the router switches the layout and says
// so (see router/index.ts, meta.advanced).
const router = useRouter()
const plasma = usePlasmaStore()
const walletConnect = useWalletConnectStore()

type Tool = { to: string; icon: Component; name: string; about: string; state?: string }

const everyday = computed<Tool[]>(() => [
  { to: '/network/plasma', icon: ZapIcon, name: 'Plasma', about: 'Fuse QSR so your transactions go out instantly instead of waiting on proof-of-work.', state: plasma.info && plasma.info.qsrFused !== '0' ? `${formatAmount(plasma.info.qsrFused, 8)} QSR fused` : '' },
  { to: '/address-book', icon: BookUserIcon, name: 'Address book', about: 'Names for the addresses you send to.' },
  { to: '/walletconnect', icon: LinkIcon, name: 'WalletConnect', about: 'Connect to a Zenon bridge, see connected apps and disconnect them.', state: walletConnect.sessions.length ? `${walletConnect.sessions.length} connected` : '' },
])
const advanced: Tool[] = [
  { to: '/network/pillars', icon: Building2Icon, name: 'Pillars', about: 'Register and run a pillar, or inspect the pillar list.' },
  { to: '/network/sentinels', icon: ShieldCheckIcon, name: 'Sentinels', about: 'Register and run a sentinel node.' },
  { to: '/tokens', icon: CoinsIcon, name: 'Token issuance', about: 'Issue a token, mint, burn and transfer ownership.' },
  { to: '/network/accelerator', icon: RocketIcon, name: 'Accelerator-Z', about: 'Propose, fund and vote on community projects.' },
]
</script>

<template>
  <div class="page-content space-y-7">
    <section aria-labelledby="tools-everyday">
      <h2 id="tools-everyday" class="section-title mb-3">For everyday use</h2>
      <div class="grid grid-cols-1 gap-3 @xl:grid-cols-2 @4xl:grid-cols-3">
        <button v-for="t in everyday" :key="t.to" type="button" :data-testid="`tool-${t.to}`"
          class="flex gap-3.5 rounded-2xl border border-border bg-card p-4 text-left transition-colors hover:bg-foreground/[0.04]"
          @click="router.push(t.to)">
          <span class="avatar-dot bg-foreground/10 text-foreground" aria-hidden="true"><component :is="t.icon" :size="16" /></span>
          <span class="min-w-0">
            <span class="block font-semibold text-foreground">{{ t.name }}</span>
            <span class="hint block">{{ t.about }}</span>
            <span v-if="t.state" class="mt-1 block text-xs font-semibold text-primary">{{ t.state }}</span>
          </span>
        </button>
      </div>
    </section>

    <section aria-labelledby="tools-advanced">
      <h2 id="tools-advanced" class="section-title">Advanced mode only</h2>
      <p class="hint mb-3 mt-1 max-w-prose">Tools for running network infrastructure, issuing tokens and governing funding. They are kept out of Everyday. Choosing one switches to Advanced, and you can switch back from the top bar.</p>
      <div class="grid grid-cols-1 gap-3 @xl:grid-cols-2 @4xl:grid-cols-3">
        <button v-for="t in advanced" :key="t.to" type="button" :data-testid="`tool-${t.to}`"
          class="flex gap-3.5 rounded-2xl border border-dashed border-border bg-card p-4 text-left transition-colors hover:bg-foreground/[0.04]"
          @click="router.push(t.to)">
          <span class="avatar-dot bg-foreground/10 text-muted-foreground" aria-hidden="true"><component :is="t.icon" :size="16" /></span>
          <span class="min-w-0">
            <span class="block font-semibold text-foreground">{{ t.name }}</span>
            <span class="hint block">{{ t.about }}</span>
            <span class="mt-1 block text-xs font-medium text-muted-foreground">Opens in Advanced mode</span>
          </span>
        </button>
      </div>
    </section>
  </div>
</template>
