<script setup lang="ts">
import Skeleton from 'primevue/skeleton'

// Content-shaped placeholder for detail pages while the record loads, so the
// page settles into its real shape instead of flashing blank then full.
withDefaults(defineProps<{ cards?: number }>(), { cards: 2 })
</script>

<template>
  <div class="detail-skeleton" aria-hidden="true">
    <div class="ds-head">
      <Skeleton width="16rem" height="1.6rem" />
      <Skeleton width="9rem" height="1rem" />
    </div>
    <div class="ds-cards">
      <div v-for="n in cards" :key="n" class="ds-card">
        <Skeleton width="40%" height="1rem" class="ds-row" />
        <Skeleton width="100%" height="0.8rem" class="ds-row" />
        <Skeleton width="92%" height="0.8rem" class="ds-row" />
        <Skeleton width="75%" height="0.8rem" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.detail-skeleton {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}
.ds-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}
.ds-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1rem;
}
.ds-card {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  padding: 1.25rem;
  background: var(--teggo-surface, #fff);
  border: 1px solid var(--p-surface-200, #e2e8f0);
  border-radius: var(--teggo-radius, 6px);
}
.ds-row {
  margin-bottom: 0.1rem;
}
</style>
