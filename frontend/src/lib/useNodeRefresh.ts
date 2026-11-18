import { onMounted, watch } from 'vue'
import { useNodeStore } from '../stores/node'

// Panels stay mounted while disconnected to preserve unsubmitted drafts.
// Reload their snapshots when the connection returns as well as on first mount.
export function useNodeRefresh(refresh: () => unknown) {
  const node = useNodeStore()
  onMounted(() => { refresh() })
  watch(() => node.connected, (connected) => { if (connected) refresh() }, { flush: 'sync' })
}
