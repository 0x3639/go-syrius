<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Button, Input } from 'nom-ui'
import { GlobeIcon, ShieldCheckIcon, UnplugIcon } from '@lucide/vue'
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
  <div class="page-content">
    <div class="grid grid-cols-1 items-start gap-5 @3xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
      <div class="min-w-0 space-y-5">
        <section class="panel-card space-y-5">
          <h2 class="section-title">Connect a bridge</h2>

          <div v-if="!configured" class="rounded-xl border border-warning/40 bg-warning/5 p-3.5 text-sm text-warning" role="alert">
            WalletConnect is unavailable in this version of the app. You can still send and receive directly from your wallet.
          </div>
          <template v-else>
            <ol class="space-y-3 text-sm text-foreground">
              <li v-for="(step, i) in ['In the bridge, choose WalletConnect.', 'Copy the connection link it shows. It starts with wc:', 'Paste it below and connect.']" :key="i" class="flex items-start gap-3">
                <span class="grid h-6.5 w-6.5 flex-none place-items-center rounded-full bg-primary/15 text-xs font-bold text-primary">{{ i + 1 }}</span>
                <span class="leading-6.5">{{ step }}</span>
              </li>
            </ol>
            <label class="field-label flex flex-col gap-2">Connection link
              <div class="flex flex-wrap gap-2">
                <Input v-model="uri" aria-label="WalletConnect URI" placeholder="wc:…" class="h-12 min-w-0 flex-1 basis-56 font-mono" />
                <Button class="h-12 px-6 text-base font-bold" :disabled="wc.pairing || !uri.trim()" @click="pair">
                  {{ wc.pairing ? 'Connecting…' : 'Connect' }}
                </Button>
              </div>
            </label>
            <p class="hint">Supports Zenon mainnet.</p>
          </template>
          <p v-if="localError || wc.error" class="text-sm text-destructive" role="alert">{{ localError || wc.error }}</p>
        </section>

        <section v-if="wc.proposal" class="space-y-4 rounded-2xl border border-primary/50 bg-card p-6">
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
      </div>

      <div class="min-w-0 space-y-5">
        <section class="panel-list">
          <h2 class="section-title px-6 py-4">Connected apps <span class="font-normal text-muted-foreground">· {{ wc.sessions.length }}</span></h2>
          <p v-if="!wc.sessions.length" class="border-t border-border/70 px-6 py-6 text-sm text-muted-foreground">No active WalletConnect sessions.</p>
          <div v-for="session in wc.sessions" :key="session.topic" class="list-row">
            <div class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-primary/15 text-primary"><GlobeIcon :size="18" /></div>
            <div class="min-w-0 flex-1">
              <p class="font-semibold text-foreground">{{ session.name }}</p>
              <p class="truncate text-xs text-muted-foreground">{{ session.url || session.topic }}</p>
              <p class="truncate font-mono text-xs text-muted-foreground">{{ session.accounts[0] }}</p>
            </div>
            <Button variant="outline" :aria-label="`Disconnect ${session.name}`" @click="wc.disconnect(session.topic)">
              <UnplugIcon :size="16" /> Disconnect
            </Button>
          </div>
        </section>

        <section class="panel-card space-y-3">
          <h2 class="section-title">You stay in control</h2>
          <p v-for="point in ['Connecting shares your active address, never your recovery phrase.', 'Review each transaction before approving it.', 'You can disconnect a bridge at any time.']" :key="point" class="flex items-start gap-2.5 text-sm text-muted-foreground">
            <ShieldCheckIcon :size="17" class="mt-0.5 flex-none text-success" />{{ point }}
          </p>
        </section>
      </div>
    </div>
  </div>
</template>
