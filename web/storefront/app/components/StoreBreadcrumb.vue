<script setup lang="ts">
import Breadcrumb from 'primevue/breadcrumb'

// Calm storefront breadcrumb: Home › … › current. Items with `to` are links;
// the last (current) item is plain text. Auto-imported by Nuxt.
interface Crumb {
  label: string
  to?: string
  icon?: string
}
defineProps<{ items: Crumb[] }>()

const home = { icon: 'pi pi-home', to: '/' }
</script>

<template>
  <Breadcrumb :home="home" :model="items" class="store-breadcrumb">
    <template #item="{ item }">
      <NuxtLink v-if="item.to" :to="item.to" class="bc-link">
        <i v-if="item.icon" :class="item.icon" />
        <span v-if="item.label">{{ item.label }}</span>
      </NuxtLink>
      <span v-else class="bc-current">{{ item.label }}</span>
    </template>
  </Breadcrumb>
</template>

<style scoped>
.store-breadcrumb {
  background: transparent;
  border: none;
  padding: 0 0 1rem;
}
.bc-link {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  text-decoration: none;
  color: var(--p-text-muted-color, #64748b);
  font-size: 0.85rem;
  font-weight: 500;
  transition: color 0.15s ease;
}
.bc-link:hover {
  color: var(--p-primary-color, #6366f1);
}
.bc-current {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--p-text-color, #334155);
}
</style>
