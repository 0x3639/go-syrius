import { createRouter, createMemoryHistory, type RouteRecordRaw } from 'vue-router'
import { useWalletStore } from '../stores/wallet'
import { useTxStore } from '../stores/tx'
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
      { path: 'tokens', name: 'tokens', meta: { title: 'Tokens', subtitle: 'Everything this account holds, and the tokens you issued.' }, component: () => import('../views/Tokens.vue') },
      { path: 'network/plasma', name: 'net-plasma', meta: { title: 'Plasma', subtitle: 'Plasma makes your transactions instant. You get it by locking QSR, and you can unlock it later.', panel: 'plasma' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/staking', name: 'net-staking', meta: { title: 'Staking', subtitle: 'Lock ZNN for a set time and earn QSR until you withdraw it.', panel: 'staking' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/pillars', name: 'net-pillars', meta: { title: 'Pillars', subtitle: 'Back a Pillar with your ZNN and share its rewards. Your ZNN never leaves your wallet.', panel: 'pillars' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/sentinels', name: 'net-sentinels', meta: { title: 'Sentinels', subtitle: 'Run a Sentinel node to help secure the network and earn ZNN and QSR.', panel: 'sentinels' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/accelerator', name: 'net-accelerator', meta: { title: 'Accelerator', subtitle: 'Community-funded projects, voted on by Pillars.', panel: 'accelerator' }, component: () => import('../views/NetworkPage.vue') },
      { path: 'network/rewards', name: 'net-rewards', meta: { title: 'Rewards', subtitle: 'Everything you have earned from the network, in one place.', panel: 'rewards' }, component: () => import('../views/NetworkPage.vue') },
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
