<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import { useContactsStore } from '../stores/contacts'
import { useWalletStore } from '../stores/wallet'
import { usePlasmaStore } from '../stores/plasma'
import { formatAmountExact, shortAddress, toBase } from '../lib/format'
import { plasmaLevel } from '../lib/plasma'
import AmountInput from './AmountInput.vue'
import ContactPicker from './ContactPicker.vue'
import { BookIcon } from '@lucide/vue'

// SendForm collects the send INTENT only (recipient/token/amount) and emits it.
// It does NOT build/PoW/sign — the tx store (Task 8) owns that. The backend
// re-validates every field authoritatively.
const emit = defineEmits<{
  send: [intent: { recipient: string; zts: string; amountDecimal: string }]
}>()

const balances = useBalancesStore()
const { items } = storeToRefs(balances)

const contacts = useContactsStore()
const wallet = useWalletStore()
const plasma = usePlasmaStore()

// Saved contacts double as one-tap recipients; the list is display-only and a
// failed load just leaves the chips out.
onMounted(() => { Promise.resolve().then(() => contacts.load()).catch(() => {}) })
const quickContacts = computed(() => contacts.items.slice(0, 4))

const props = defineProps<{ initialRecipient?: string; initialToken?: string }>()

const recipient = ref(props.initialRecipient ?? '')
const zts = ref(props.initialToken ?? '')
const amountDecimal = ref('')
const bookOpen = ref(false)

function onPickContact(addr: string) {
  recipient.value = addr
  bookOpen.value = false
}

// Token order for the picker: ZNN first, QSR second, then the rest alphabetically
// by symbol. So the default selection (first entry) is ZNN.
const TOKEN_RANK: Record<string, number> = { ZNN: 0, QSR: 1 }
const orderedItems = computed(() =>
  [...items.value].sort((a, b) => {
    const ra = TOKEN_RANK[a.symbol] ?? 2
    const rb = TOKEN_RANK[b.symbol] ?? 2
    if (ra !== rb) return ra - rb
    return (a.symbol || a.zts).localeCompare(b.symbol || b.zts)
  }),
)

// Default to the first token (ZNN when present) once balances load.
watch(
  orderedItems,
  (list) => {
    // A pre-selected token that this account does not hold falls back to the default.
    const held = list.some((b) => b.zts === zts.value)
    if ((!zts.value || !held) && list[0]) zts.value = list[0].zts
  },
  { immediate: true },
)

// Selected token + its available balance (commas for display, plain decimal for Max).
const selectedTok = computed(() => items.value.find((b) => b.zts === zts.value))
const balanceLabel = computed(() =>
  balances.available && selectedTok.value ? `${formatAmountExact(selectedTok.value.amount, selectedTok.value.decimals)} ${selectedTok.value.symbol || ''}`.trim() : '',
)
const maxDecimal = computed(() =>
  balances.available && selectedTok.value ? formatAmountExact(selectedTok.value.amount, selectedTok.value.decimals) : '',
)

// z1 bech32: starts z1, lowercase alnum, length ~40. Backend re-validates authoritatively.
const validAddr = computed(() => /^z1[0-9a-z]{38}$/.test(recipient.value))
const validAmount = computed(() => {
  if (!selectedTok.value) return false
  try {
    const amount = BigInt(toBase(amountDecimal.value, selectedTok.value.decimals))
    return amount > 0n && amount <= BigInt(selectedTok.value.amount)
  } catch { return false }
})
const canSend = computed(() => balances.available && validAddr.value && validAmount.value && !!zts.value)

// Summary column: a plain-language echo of the form. It is NOT what gets
// signed — the confirm step renders the block the backend actually built.
const recipientName = computed(() => contacts.items.find((c) => c.address === recipient.value)?.name ?? '')
const fromAccount = computed(() => wallet.accounts.find((a) => a.index === wallet.activeIndex))
const fromLabel = computed(() => fromAccount.value?.label?.trim() || (fromAccount.value ? `Account ${fromAccount.value.index}` : ''))
const primaryTokens = computed(() => orderedItems.value.filter((b) => b.symbol in TOKEN_RANK))
const otherTokens = computed(() => orderedItems.value.filter((b) => !(b.symbol in TOKEN_RANK)))
const otherSelected = computed(() => otherTokens.value.some((b) => b.zts === zts.value))
const needsPow = computed(() => {
  if (plasma.info === null) return false
  const level = plasmaLevel(plasma.info.currentPlasma ?? 0)
  return level === 'None' || level === 'Low'
})

function onSend() {
  if (!canSend.value) return
  emit('send', {
    recipient: recipient.value,
    zts: zts.value,
    amountDecimal: amountDecimal.value,
  })
}
</script>

<template>
  <div class="page-split">
    <div class="panel-card space-y-7">
      <!-- Who -->
      <div class="space-y-2.5">
        <span class="section-title block">Who are you sending to?</span>
        <div class="relative">
          <Input
            v-model="recipient"
            aria-label="recipient"
            placeholder="z1…"
            class="h-12 w-full pr-12 font-mono text-foreground"
          />
          <button
            type="button"
            aria-label="address book"
            title="Address book"
            class="absolute right-2 top-1/2 grid h-8 w-8 -translate-y-1/2 place-items-center rounded-md transition-colors hover:bg-foreground/[0.08]"
            :class="bookOpen ? 'text-foreground' : 'text-muted-foreground'"
            @click="bookOpen = !bookOpen"
          >
            <BookIcon :size="16" />
          </button>
        </div>
        <p v-if="recipient && !validAddr" class="text-xs text-destructive">
          Invalid z1 address
        </p>
        <p v-else-if="validAddr" class="text-xs font-medium text-success">
          {{ recipientName ? `${recipientName} · valid address` : 'Valid address' }}
        </p>
        <div v-if="quickContacts.length && !bookOpen" class="flex flex-wrap items-center gap-2">
          <span class="hint">Saved</span>
          <button
            v-for="c in quickContacts"
            :key="c.address"
            type="button"
            :aria-label="`send to ${c.name}`"
            class="flex h-9 items-center gap-2 rounded-full border pl-1.5 pr-3 text-sm text-foreground transition-colors"
            :class="recipient === c.address ? 'border-primary bg-primary/[0.08]' : 'border-input hover:border-muted-foreground/50'"
            @click="recipient = c.address"
          >
            <span class="grid h-6 w-6 place-items-center rounded-full bg-foreground/10 text-[11px] font-bold">{{ c.name.charAt(0).toUpperCase() }}</span>
            {{ c.name }}
          </button>
        </div>
        <ContactPicker :open="bookOpen" @select="onPickContact" @close="bookOpen = false" />
      </div>

      <!-- What -->
      <div class="space-y-2.5">
        <span class="section-title block">Which asset?</span>
        <div class="grid grid-cols-2 gap-2.5 @xl:grid-cols-3" role="group" aria-label="Token">
          <button
            v-for="b in primaryTokens"
            :key="b.zts"
            type="button"
            class="choice flex flex-col gap-0.5"
            :class="zts === b.zts ? 'choice-on' : ''"
            :aria-pressed="zts === b.zts"
            @click="zts = b.zts"
          >
            <span class="font-semibold">{{ b.symbol }}</span>
            <span class="truncate font-mono text-xs text-muted-foreground">{{ balances.available ? formatAmountExact(b.amount, b.decimals) : '—' }}</span>
          </button>
          <label v-if="otherTokens.length" class="choice flex flex-col gap-0.5" :class="otherSelected ? 'choice-on' : ''">
            <span class="font-semibold">Other token</span>
            <select
              v-model="zts"
              aria-label="Other token"
              class="w-full bg-transparent text-xs text-muted-foreground outline-none"
            >
              <option v-if="!otherSelected" :value="zts" disabled>Choose…</option>
              <option v-for="b in otherTokens" :key="b.zts" :value="b.zts">
                {{ b.symbol || b.zts }}
              </option>
            </select>
          </label>
        </div>
      </div>

      <!-- How much -->
      <div class="space-y-2.5">
        <AmountInput v-model="amountDecimal" label="Amount" heading="How much?" :symbol="selectedTok?.symbol" :max="maxDecimal" />
        <p v-if="balanceLabel" class="hint">
          Balance: <span class="font-medium text-foreground">{{ balanceLabel }}</span>
        </p>
        <p v-if="amountDecimal && !validAmount && balances.available" class="text-xs text-destructive">Enter an amount within your available balance.</p>
      </div>
    </div>

    <aside class="panel-card flex flex-col gap-3.5 text-sm" aria-label="Summary">
      <h2 class="section-title">Summary</h2>
      <div class="flex justify-between gap-3">
        <span class="text-muted-foreground">From</span>
        <span class="text-right text-foreground">{{ fromLabel || '—' }}</span>
      </div>
      <div class="flex justify-between gap-3">
        <span class="text-muted-foreground">To</span>
        <span class="min-w-0 text-right text-foreground">
          <template v-if="validAddr">
            <span v-if="recipientName" class="block">{{ recipientName }}</span>
            <span class="font-mono text-xs text-muted-foreground">{{ shortAddress(recipient) }}</span>
          </template>
          <template v-else>—</template>
        </span>
      </div>
      <div class="flex justify-between gap-3">
        <span class="text-muted-foreground">Amount</span>
        <span class="break-all text-right font-mono text-foreground">{{ validAmount ? `${amountDecimal} ${selectedTok?.symbol ?? ''}` : '—' }}</span>
      </div>
      <div class="flex justify-between gap-3">
        <span class="text-muted-foreground">Network fee</span>
        <span class="font-semibold text-success">Free</span>
      </div>
      <div v-if="needsPow" class="rounded-xl border border-warning/40 bg-warning/10 px-3.5 py-3">
        <p class="font-semibold text-warning">Low plasma</p>
        <p class="hint mt-0.5">The wallet will do a short proof-of-work before sending. Fuse plasma to skip the wait.</p>
      </div>
      <Button
        class="h-12 w-full text-base font-bold"
        aria-label="Send"
        :disabled="!canSend"
        @click="onSend"
      >
        Review and send
      </Button>
      <p class="hint text-center">You will confirm the exact transaction before it is signed.</p>
    </aside>
  </div>
</template>
