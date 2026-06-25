import { createRouter, createMemoryHistory, type RouteRecordRaw } from 'vue-router'
import { useWalletStore } from '../stores/wallet'

// Public routes are reachable while the wallet is locked. Everything else is
// gated. Routes are lazy-loaded so each screen code-splits and a future plugin
// can register more via router.addRoute().
export const PUBLIC_ROUTES = ['unlock', 'create', 'import']

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: { name: 'unlock' } },
  { path: '/unlock', name: 'unlock', component: () => import('../views/Unlock.vue') },
  { path: '/create', name: 'create', component: () => import('../views/Create.vue') },
  { path: '/import', name: 'import', component: () => import('../views/ImportMnemonic.vue') },
  { path: '/home', name: 'home', component: () => import('../views/Home.vue') },
  { path: '/settings', name: 'settings', component: () => import('../views/Settings.vue') },
  // Tokens reuses the placeholder for now; B4 splits it into its own view.
  { path: '/tokens', name: 'tokens', component: () => import('../views/Settings.vue') },
]

const router = createRouter({ history: createMemoryHistory(), routes })

router.beforeEach((to) => {
  // Instantiate the store inside the guard (after app.use(pinia) has run).
  const wallet = useWalletStore()
  const isPublic = PUBLIC_ROUTES.includes(to.name as string)
  if (wallet.locked && !isPublic) return { name: 'unlock' }
  if (!wallet.locked && isPublic) return { name: 'home' }
  return true
})

export default router
