<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import AmountInput from './AmountInput.vue'

// SendForm collects the send INTENT only (recipient/token/amount) and emits it.
// It does NOT build/PoW/sign — the tx store (Task 8) owns that. The backend
// re-validates every field authoritatively.
const emit = defineEmits<{
  send: [intent: { recipient: string; zts: string; amountDecimal: string }]
}>()

const { items } = storeToRefs(useBalancesStore())

const recipient = ref('')
const zts = ref('')
const amountDecimal = ref('')

// Default to the first token's zts once balances load (verbatim from the Svelte
// reactive `$: if (!zts && $balances[0]) zts = $balances[0].zts`).
watch(
  items,
  (list) => {
    if (!zts.value && list[0]) zts.value = list[0].zts
  },
  { immediate: true },
)

// z1 bech32: starts z1, lowercase alnum, length ~40. Backend re-validates authoritatively.
const validAddr = computed(() => /^z1[0-9a-z]{38}$/.test(recipient.value))
const validAmount = computed(
  () => amountDecimal.value !== '' && Number(amountDecimal.value) > 0,
)
const canSend = computed(() => validAddr.value && validAmount.value && !!zts.value)

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
    <label class="block text-sm text-muted-foreground"
      >Recipient
      <Input
        v-model="recipient"
        aria-label="recipient"
        placeholder="z1…"
        class="mt-1 w-full font-mono text-foreground"
      />
    </label>
    <p v-if="recipient && !validAddr" class="text-xs text-destructive">
      Invalid z1 address
    </p>

    <label class="block text-sm text-muted-foreground"
      >Token
      <select
        v-model="zts"
        class="mt-1 w-full rounded bg-card px-3 py-2 text-foreground"
      >
        <option v-for="b in items" :key="b.zts" :value="b.zts">
          {{ b.symbol || b.zts }}
        </option>
      </select>
    </label>

    <AmountInput v-model="amountDecimal" label="Amount" />

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
