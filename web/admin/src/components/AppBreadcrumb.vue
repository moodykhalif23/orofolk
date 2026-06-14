<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import Breadcrumb from 'primevue/breadcrumb'

// A calm breadcrumb for detail/sub pages: Home › … › current. Items with a
// `route` render as router links; the last (current) item is plain text.
export interface Crumb {
  label: string
  route?: RouteLocationRaw
  icon?: string
}

defineProps<{ items: Crumb[] }>()

const home = { icon: 'pi pi-home', route: { name: 'dashboard' } as RouteLocationRaw }
</script>

<template>
  <Breadcrumb :home="home" :model="items" class="app-breadcrumb">
    <template #item="{ item }">
      <RouterLink v-if="item.route" :to="item.route" class="bc-link">
        <i v-if="item.icon" :class="item.icon" />
        <span v-if="item.label">{{ item.label }}</span>
      </RouterLink>
      <span v-else class="bc-current">{{ item.label }}</span>
    </template>
  </Breadcrumb>
</template>

<style scoped>
/* Strip the default panel chrome — a breadcrumb should read as quiet context,
   not a boxed surface. */
.app-breadcrumb {
  background: transparent;
  border: none;
  padding: 0 0 0.75rem;
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
