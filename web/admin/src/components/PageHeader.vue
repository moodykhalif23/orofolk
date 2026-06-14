<script setup lang="ts">
// Standard page header used by every list/detail view: a title with an optional
// muted count/subtitle on the left, and an actions slot on the right. Replaces
// the per-view `.header` markup + scoped CSS that had drifted across ~29 views,
// so spacing and typography are now identical everywhere. Purely presentational.
defineProps<{
  /** Page title. */
  title: string
  /** Muted suffix after the title — e.g. a row count "(42)" or a short hint. */
  meta?: string | number
}>()
</script>

<template>
  <div class="page-header">
    <div class="page-header__heading">
      <h1>
        {{ title }}
        <span v-if="meta !== undefined && meta !== ''" class="page-header__meta">{{
          typeof meta === 'number' ? `(${meta})` : meta
        }}</span>
      </h1>
      <!-- Optional secondary line (filters, description) rendered under the title. -->
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
  gap: 0.75rem 1rem;
  margin-bottom: 1rem;
  /* When the title + actions can't fit on one line, the actions wrap beneath
     rather than overflowing or crushing the title. */
  flex-wrap: wrap;
}
.page-header__heading {
  min-width: 0;
}
.page-header h1 {
  /* Mirrors `.page > h1` in main.css — kept here so the title renders identically
     whether a view uses <PageHeader> or the legacy inline header markup. */
  margin: 0;
  font-size: 1.35rem;
  font-weight: 700;
  letter-spacing: -0.01em;
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
  flex-wrap: wrap;
}
</style>
