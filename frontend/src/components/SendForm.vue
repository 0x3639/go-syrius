<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import { formatAmountExact, toBase } from '../lib/format'
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

const recipient = ref('')
const zts = ref('')
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
    if (!zts.value && list[0]) zts.value = list[0].zts
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
  <div class="space-y-3">
    <div>
      <span class="text-sm text-muted-foreground">Recipient</span>
      <div class="relative mt-1">
        <Input
          v-model="recipient"
          aria-label="recipient"
          placeholder="z1…"
          class="w-full pr-11 font-mono text-foreground"
        />
        <button
          type="button"
          aria-label="address book"
          title="Address book"
          class="absolute right-1.5 top-1/2 grid h-7 w-7 -translate-y-1/2 place-items-center rounded transition-colors hover:bg-foreground/[0.08]"
          :class="bookOpen ? 'text-foreground' : 'text-muted-foreground'"
          @click="bookOpen = !bookOpen"
        >
          <BookIcon :size="16" />
        </button>
      </div>
      <ContactPicker class="mt-2" :open="bookOpen" @select="onPickContact" @close="bookOpen = false" />
    </div>
    <p v-if="recipient && !validAddr" class="text-xs text-destructive">
      Invalid z1 address
    </p>

    <label class="block text-sm text-muted-foreground"
      >Token
      <select
        v-model="zts"
        class="mt-1 w-full rounded-md border border-input bg-transparent px-3 py-2 text-foreground"
      >
        <option v-for="b in orderedItems" :key="b.zts" :value="b.zts">
          {{ b.symbol || b.zts }}
        </option>
      </select>
    </label>
    <p v-if="balanceLabel" class="-mt-1 text-xs text-muted-foreground">
      Balance: <span class="font-medium text-foreground">{{ balanceLabel }}</span>
    </p>

    <AmountInput v-model="amountDecimal" label="Amount" :max="maxDecimal" />
    <p v-if="amountDecimal && !validAmount && balances.available" class="text-xs text-destructive">Enter an amount within your available balance.</p>

    <Button
      class="w-full"
      aria-label="Send"
      :disabled="!canSend"
      @click="onSend"
    >
      Send
    </Button>
  </div>
</template>
