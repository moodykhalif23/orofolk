<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useBillingStore } from '@/stores/billing'
import { sectionByKey, visibleLeaves } from '@/nav/sections'
import EmptyState from '@/components/EmptyState.vue'

// The "brief page" for a section: a short headline plus the section's leaves as
// cards that deep-link to each setup page. Generic — driven entirely by the
// /section/:key route param against the shared nav model.
const route = useRoute()
const auth = useAuthStore()
const billing = useBillingStore()

const section = computed(() => sectionByKey(String(route.params.key)))
const leaves = computed(() =>
  section.value ? visibleLeaves(section.value, (p) => auth.can(p), (f) => billing.allows(f)) : [],
)
</script>

<template>
  <div v-if="section" class="hub">
    <header class="hub-head">
      <div>
        <h1>{{ section.label }}</h1>
        <p>{{ section.blurb }}</p>
      </div>
    </header>

    <div v-if="leaves.length" class="hub-grid">
      <RouterLink
        v-for="leaf in leaves"
        :key="leaf.routeName"
        :to="{ name: leaf.routeName }"
        class="hub-card"
      >
        <span class="hub-card__icon"><i :class="leaf.icon" /></span>
        <span class="hub-card__body">
          <span class="hub-card__title">{{ leaf.label }}</span>
          <span class="hub-card__blurb">{{ leaf.blurb }}</span>
        </span>
        <i class="pi pi-arrow-right hub-card__go" />
      </RouterLink>
    </div>

    <EmptyState
      v-else
      icon="pi pi-lock"
      title="Nothing here for you yet"
      message="You don't have access to anything in this section. Ask an administrator if you think that's wrong."
    />
  </div>
</template>

<style scoped>
.hub {
  max-width: 1000px;
  margin: 0 auto;
}
.hub-head {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.75rem;
}
.hub-head h1 {
  margin: 0;
  font-size: 1.55rem;
  font-weight: 800;
  letter-spacing: -0.02em;
}
.hub-head p {
  margin: 0.2rem 0 0;
  color: var(--p-text-muted-color, #64748b);
  font-size: 0.95rem;
}
.hub-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1rem;
}
.hub-card {
  display: flex;
  align-items: center;
  gap: 0.9rem;
  padding: 1.1rem 1.15rem;
  background: var(--teggo-surface, #fff);
  border: 1px solid var(--p-surface-200, #e2e8f0);
  border-radius: 0 18px 0 0; /* 3 sharp edges, top-right curve */
  text-decoration: none;
  color: inherit;
}
.hub-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  color: var(--p-text-color, #334155);
  font-size: 1.4rem;
}
.hub-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
  flex: 1;
}
.hub-card__title {
  font-weight: 700;
  font-size: 0.98rem;
  letter-spacing: -0.01em;
}
.hub-card__blurb {
  color: var(--p-text-muted-color, #64748b);
  font-size: 0.85rem;
  line-height: 1.4;
}
.hub-card__go {
  color: var(--p-text-muted-color, #cbd5e1);
  font-size: 0.85rem;
  flex-shrink: 0;
}
</style>
