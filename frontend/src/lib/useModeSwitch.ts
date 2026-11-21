import { useRoute, useRouter } from 'vue-router'
import { useUiStore, type UiMode } from '../stores/ui'

// Switching layout keeps the account, network and current screen. The only
// time it navigates is when the current screen does not exist in the target
// layout (an operator tool in Everyday; Earn or More tools in Advanced).
export function useModeSwitch() {
  const ui = useUiStore()
  const router = useRouter()
  const route = useRoute()
  return (mode: UiMode) => {
    if (ui.mode === mode) return
    const meta = route?.meta ?? {}
    const leaves = mode === 'everyday' ? meta.advanced : meta.everyday
    ui.setMode(mode)
    if (leaves) router.push('/dashboard')
  }
}
