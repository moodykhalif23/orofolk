<script setup lang="ts">
// Standard page header for storefront pages: a title with an optional muted
// count/subtitle, and an actions slot (sort control, primary button, etc.).
// Mirrors the admin PageHeader so the two apps read as one product.
defineProps<{
  /** Page title. */
  title: string
  /** Muted suffix after the title — a count "(12)" or a short hint. */
  meta?: string | number
}>()
</script>

<template>
  <div class="page-header">
    <div class="page-header__heading">
      <h1 class="title">
        {{ title }}
        <span v-if="meta !== undefined && meta !== ''" class="page-header__meta">{{
          typeof meta === 'number' ? `(${meta})` : meta
        }}</span>
      </h1>
      <div v-if="$slots.sub" class="page-header__sub"><slot name="sub" /></div>
    </div>
    <div v-if="$slots.actions" class="page-header__actions"><slot name="actions" /></div>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.25rem;
}
.page-header__heading {
  min-width: 0;
}
.page-header .title {
  margin: 0;
}
.page-header__meta {
  color: var(--p-text-muted-color, #64748b);
  font-weight: 400;
  font-size: 1rem;
}
.page-header__sub {
  margin-top: 0.35rem;
  color: var(--p-text-muted-color, #64748b);
  font-size: 0.85rem;
}
.page-header__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}
</style>
