<script setup lang="ts">
// Consistent empty/zero state — an icon, a title, an optional message, and an
// optional call-to-action slot. Used in DataTable #empty slots and standalone
// "nothing here yet" panels so every module reads the same way to a first-time,
// non-technical user instead of a bare line of text.
withDefaults(
  defineProps<{
    /** PrimeIcons class, e.g. "pi pi-inbox". */
    icon?: string
    /** Short headline, e.g. "No customers yet". */
    title: string
    /** Optional supporting sentence explaining the next step. */
    message?: string
  }>(),
  { icon: 'pi pi-inbox' },
)
</script>

<template>
  <div class="empty-state">
    <i :class="icon" class="empty-state__icon" />
    <p class="empty-state__title">{{ title }}</p>
    <p v-if="message" class="empty-state__message">{{ message }}</p>
    <div v-if="$slots.default" class="empty-state__action"><slot /></div>
  </div>
</template>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 2.5rem 1rem;
  color: var(--p-text-muted-color, #64748b);
}
.empty-state__icon {
  font-size: 1.75rem;
  opacity: 0.5;
  margin-bottom: 0.75rem;
}
.empty-state__title {
  margin: 0;
  font-weight: 600;
  color: var(--p-text-color, #334155);
}
.empty-state__message {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
  max-width: 28rem;
}
.empty-state__action {
  margin-top: 1rem;
}
</style>
