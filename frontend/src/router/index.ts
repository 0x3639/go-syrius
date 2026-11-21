import { createRouter, createMemoryHistory, type RouteRecordRaw } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { useTxStore } from '../stores/tx'
import { useUiStore } from '../stores/ui'
import AppShell from '../components/AppShell.vue'

// Public routes are reachable while locked. Everything else is gated and lives
// under the AppShell (sidebar + topbar). Lazy-loaded so each screen code-splits.
export const PUBLIC_ROUTES = ['unlock', 'create', 'import']

const routes: RouteRecordRaw[] = [
  { path: '/unlock', name: 'unlock', component: () => import('../views/Unlock.vue') },
  { path: '/create', name: 'create', component: () => import('../views/Create.vue') },
  { path: '/import', name: 'import', component: () => import('../views/ImportMnemonic.vue') },
  {
    path: '/',
    component: AppShell,
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', meta: { title: 'Dashboard', subtitle: 'Here is what is happening with your wallet.' }, component: () => import('../views/Dashboard.vue') },
      { path: 'transfer', name: 'transfer', meta: { title: 'Send', subtitle: 'Transfers on Zenon are feeless.' }, component: () => import('../views/Transfer.vue') },
      { path: 'receive', name: 'receive', meta: { title: 'Receive', subtitle: 'Share your address, then accept what arrives.' }, component: () => import('../views/Receive.vue') },
      { path: 'activity', name: 'activity', meta: { title: 'Activity', subtitle: 'Everything this account has sent, received and called.' }, component: () => import('../views/Activity.vue') },
      { path: 'earn', name: 'earn', meta: { title: 'Earn', subtitle: 'Rewards, staking and delegation in one place.', everyday: true }, component: () => import('../views/Earn.vue') },
      { path: 'tools', name: 'tools', meta: { title: 'More tools', subtitle: 'Everything else go-syrius can do.', everyday: true }, component: () => import('../views/MoreTools.vue') },
      { path: 'tokens', name: 'tokens', meta: { advanced: true, title: 'Tokens', subtitle: 'Everything this account holds, and the tokens you issued.' }, component: () => import('../views/Tokens.vue') },
      { path: 'network/plasma', name: 'net-plasma', meta: { title: 'Plasma', subtitle: 'Plasma makes your transactions instant. You get it by locking QSR, and you can unlock it later.', panel: 'plasma' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/staking', name: 'net-staking', meta: { title: 'Staking', subtitle: 'Lock ZNN for a set time and earn QSR until you withdraw it.', panel: 'staking', advanced: true }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/pillars', name: 'net-pillars', meta: { title: 'Pillars', subtitle: 'Back a Pillar with your ZNN and share its rewards. Your ZNN never leaves your wallet.', panel: 'pillars', advanced: true }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/sentinels', name: 'net-sentinels', meta: { title: 'Sentinels', subtitle: 'Run a Sentinel node to help secure the network and earn ZNN and QSR.', panel: 'sentinels', advanced: true }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/accelerator', name: 'net-accelerator', meta: { title: 'Accelerator', subtitle: 'Community-funded projects, voted on by Pillars.', panel: 'accelerator', advanced: true }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/rewards', name: 'net-rewards', meta: { title: 'Rewards', subtitle: 'Everything you have earned from the network, in one place.', panel: 'rewards', advanced: true }, component: () => import('../views/NetworkPage.vue') },
      { path: 'settings', name: 'settings', meta: { title: 'Settings', subtitle: 'Appearance, node connection and wallet security.' }, component: () => import('../views/Settings.vue') },
      { path: 'walletconnect', name: 'walletconnect', meta: { title: 'WalletConnect', subtitle: 'Connect to a Zenon bridge and approve its requests from here.' }, component: () => import('../views/WalletConnect.vue') },
      { path: 'address-book', name: 'address-book', meta: { title: 'Address book', subtitle: 'Save the addresses you send to, so you never paste the wrong one.' }, component: () => import('../views/AddressBook.vue') },
    ],
  },
]

const router = createRouter({ history: createMemoryHistory(), routes })

router.beforeEach((to) => {
  const wallet = useWalletStore()
  const isPublic = PUBLIC_ROUTES.includes(to.name as string)
  if (wallet.locked && !isPublic) return { name: 'unlock' }
  if (!wallet.locked && isPublic) return { name: 'dashboard' }
  // meta.advanced marks operator tools that Everyday keeps out of its
  // navigation; meta.everyday marks screens that only exist in Everyday.
  // Opening an operator tool from Everyday (a link, a top-bar badge, More
  // tools) switches the layout and says so, rather than showing a screen the
  // sidebar cannot get back to.
  const ui = useUiStore()
  if (to.meta.advanced && ui.mode === 'everyday') {
    ui.setMode('advanced', `Switched to Advanced for ${String(to.meta.title ?? 'this tool')}. Switch back from the top bar.`)
  } else if (to.meta.everyday && ui.mode === 'advanced') {
    return { name: 'dashboard' }
  }
  return true
})

router.afterEach(() => {
  // Discard any half-built/finished tx when navigating between screens.
  // Awaiting previews and retryable confirmation errors can both own a backend
  // hold, so both must release it by identity rather than using a bare reset.
  const tx = useTxStore()
  if (tx.status === 'awaiting' || tx.status === 'error') tx.discard()
  else tx.reset()
})

export default router
