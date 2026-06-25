<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import QRCode from 'qrcode'
import { Address } from 'nom-ui'

const props = defineProps<{ address: string }>()

const dataUrl = ref('')
let reqId = 0

async function renderQR(addr: string) {
  const id = ++reqId
  if (!addr) {
    dataUrl.value = ''
    return
  }
  try {
    const u = await QRCode.toDataURL(addr, { margin: 1, width: 160 })
    if (id === reqId) dataUrl.value = u
  } catch {
    if (id === reqId) dataUrl.value = ''
  }
}

onMounted(() => renderQR(props.address))
watch(() => props.address, (a) => renderQR(a))
</script>

<template>
  <div class="flex items-center gap-4 rounded bg-card p-4">
    <img
      v-if="dataUrl"
      :src="dataUrl"
      alt="address QR"
      class="h-32 w-32 rounded bg-white p-1"
    />
    <div class="min-w-0">
      <Address
        :address="props.address"
        :truncate="false"
        :tooltip="false"
        :copy="true"
        wrap
        class="text-foreground"
      />
    </div>
  </div>
</template>
