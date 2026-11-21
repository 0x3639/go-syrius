<script setup lang="ts">
import TxHistory from '../components/TxHistory.vue'
import BalanceNotice from '../components/BalanceNotice.vue'
</script>

<template>
  <div class="page-content">
    <BalanceNotice />
    <section class="panel-list">
      <h2 class="section-title px-6 pb-1 pt-5">All activity</h2>
      <div class="px-3 pb-3">
        <TxHistory />
      </div>
    </section>
  </div>
</template>
