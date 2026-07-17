<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Button, Input } from 'nom-ui'
import { GlobeIcon, LinkIcon, ShieldCheckIcon, UnplugIcon } from '@lucide/vue'
import { useWalletConnectStore } from '../stores/walletconnect'

const wc = useWalletConnectStore()
const uri = ref('')
const localError = ref('')
const configured = computed(() => {
  const id = wc.projectId()
  return Boolean(id) && id !== 'REPLACE_ME_WC_PROJECT_ID'
})

async function pair() {
  localError.value = ''
  try {
    await wc.pair(uri.value)
    uri.value = ''
  } catch (error: any) { localError.value = error?.message ?? String(error) }
}

async function approve() {
  localError.value = ''
  try { await wc.approveProposal() }
  catch (error: any) { localError.value = error?.message ?? String(error) }
}

onMounted(() => { if (configured.value) wc.ensureClient().catch((error) => { localError.value = error?.message ?? String(error) }) })
</script>

<template>
  <div class="mx-auto max-w-[48rem] space-y-6">
    <section class="rounded-xl border border-border bg-card p-5 space-y-4">
      <div class="flex items-start gap-3">
        <div class="rounded-lg bg-primary/10 p-2 text-primary"><LinkIcon :size="20" /></div>
        <div>
          <h2 class="font-semibold text-foreground">Connect a bridge</h2>
          <p class="text-sm text-muted-foreground">
            Pair with the current Zenon Bridge or the new NoM Bridge using the same zenon:1 WalletConnect session.
          </p>
        </div>
      </div>

      <div v-if="!configured" class="rounded border border-warning/40 bg-warning/5 p-3 text-sm text-warning" role="alert">
        WalletConnect is not configured in this build. Set <span class="font-mono">VITE_WALLETCONNECT_PROJECT_ID</span> and rebuild.
      </div>
      <template v-else>
        <Input v-model="uri" aria-label="WalletConnect URI" placeholder="wc:…" class="font-mono" />
        <Button :disabled="wc.pairing || !uri.trim()" @click="pair">
          {{ wc.pairing ? 'Pairing…' : 'Pair' }}
        </Button>
        <p class="text-xs text-muted-foreground">
          In the bridge, choose WalletConnect, copy the pairing URI, then paste it here. Only zenon:1 sessions are accepted.
        </p>
      </template>
      <p v-if="localError || wc.error" class="text-sm text-destructive" role="alert">{{ localError || wc.error }}</p>
    </section>

    <section v-if="wc.proposal" class="rounded-xl border border-primary/50 bg-card p-5 space-y-4">
      <div class="flex items-center gap-3">
        <!-- Peer metadata is untrusted: never fetch its icon URL from the
             privileged WebView (IP disclosure, loopback/LAN probing). -->
        <div class="grid h-10 w-10 place-items-center rounded-lg bg-primary/10 text-primary"><GlobeIcon :size="20" /></div>
        <div>
          <h2 class="font-semibold">{{ wc.proposal.name }} wants to connect</h2>
          <p class="text-xs text-muted-foreground break-all">{{ wc.proposal.url }}</p>
        </div>
      </div>
      <p v-if="wc.proposal.isScam" class="rounded border border-destructive/40 bg-destructive/5 p-2 text-sm text-destructive" role="alert">
        WalletConnect Verify flagged this dapp as a known scam. It cannot be approved.
      </p>
      <p v-else-if="wc.proposal.validation === 'VALID'" class="text-xs text-success">
        Verified origin: <span class="break-all font-mono">{{ wc.proposal.verifiedOrigin }}</span>
      </p>
      <p v-else class="text-xs text-warning" role="alert">
        Dapp origin not verified by WalletConnect — the name and URL above are claimed by the dapp, not proven.
      </p>
      <p v-if="wc.proposal.description" class="text-sm text-muted-foreground">{{ wc.proposal.description }}</p>
      <div class="rounded border border-border p-3 text-sm space-y-2">
        <div class="flex justify-between gap-3"><span class="text-muted-foreground">Network</span><span class="font-mono">zenon:1</span></div>
        <div class="flex justify-between gap-3"><span class="text-muted-foreground">Methods</span><span class="text-right font-mono">{{ wc.proposal.methods.join(', ') }}</span></div>
        <div class="flex justify-between gap-3"><span class="text-muted-foreground">Account</span><span class="break-all text-right font-mono">Current active account</span></div>
      </div>
      <div class="flex gap-2">
        <Button class="flex-1" @click="approve">Approve</Button>
        <Button class="flex-1" variant="outline" @click="wc.rejectProposal()">Reject</Button>
      </div>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-3">
      <div class="flex items-center gap-2">
        <ShieldCheckIcon :size="18" class="text-success" />
        <h2 class="font-semibold">Connected dapps</h2>
      </div>
      <p v-if="!wc.sessions.length" class="text-sm text-muted-foreground">No active WalletConnect sessions.</p>
      <div v-for="session in wc.sessions" :key="session.topic" class="flex items-center gap-3 rounded border border-border p-3">
        <div class="grid h-9 w-9 shrink-0 place-items-center rounded bg-primary/10 text-primary"><GlobeIcon :size="18" /></div>
        <div class="min-w-0 flex-1">
          <p class="font-medium">{{ session.name }}</p>
          <p class="truncate text-xs text-muted-foreground">{{ session.url || session.topic }}</p>
          <p class="truncate font-mono text-xs text-muted-foreground">{{ session.accounts[0] }}</p>
        </div>
        <Button variant="outline" :aria-label="`Disconnect ${session.name}`" @click="wc.disconnect(session.topic)">
          <UnplugIcon :size="16" />
        </Button>
      </div>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm font-medium">Security boundary</h2>
      <p class="text-xs text-muted-foreground">
        Pairing exposes only your active address. Bridge transactions are reconstructed and decoded in Go, and every request requires confirmation before PoW, signing, and publication. Arbitrary message signing is disabled.
      </p>
    </section>
  </div>
</template>
