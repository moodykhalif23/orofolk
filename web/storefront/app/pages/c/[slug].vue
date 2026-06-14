<script setup lang="ts">
import ProductCard from '~/components/ProductCard.vue'
import Message from 'primevue/message'
import type { components } from '@teggo/api/schema'

type Product = components['schemas']['StorefrontProduct']

const route = useRoute()
const client = useClient()

// SSR fetch so the catalog page is crawlable. 'all' lists everything; any other
// slug filters by that category's subtree (resolved server-side).
const { data, error } = await useAsyncData(
  () => `products-${route.params.slug}`,
  async () => {
    const slug = route.params.slug as string
    const query: Record<string, string | number> = { page: 1, page_size: 24 }
    if (slug && slug !== 'all') query.category = slug
    const { data, error } = await client.GET('/storefront/products', { params: { query } })
    if (error) throw createError({ statusCode: 502, statusMessage: 'Catalog unavailable' })
    return data
  },
)

useSeoMeta({
  title: () => `Catalog — ${route.params.slug}`,
  description: 'Browse products in the Teggo Store catalog.',
})

// Breadcrumb: Home › Catalog (› category, when filtered to a specific subtree).
const crumbs = computed(() => {
  const slug = String(route.params.slug ?? '')
  if (slug && slug !== 'all') {
    return [{ label: 'Catalog', to: '/c/all' }, { label: slug.replace(/-/g, ' ') }]
  }
  return [{ label: 'Catalog' }]
})

// Grid ↔ list layout toggle for the product listing.
const layout = ref<'grid' | 'list'>('grid')
const layoutOptions = [
  { value: 'grid', icon: 'pi pi-th-large' },
  { value: 'list', icon: 'pi pi-bars' },
]
</script>

<template>
  <section>
    <StoreBreadcrumb :items="crumbs" />
    <h1 class="title">Catalog</h1>

    <Message v-if="error" severity="error" :closable="false">
      Could not load products. Is the API running on the configured base URL?
    </Message>

    <DataView v-else-if="data?.items?.length" :value="data.items" :layout="layout" data-key="public_id">
      <template #header>
        <div class="dv-head">
          <span class="dv-count">{{ data.items.length }} product{{ data.items.length === 1 ? '' : 's' }}</span>
          <SelectButton
            v-model="layout"
            :options="layoutOptions"
            option-value="value"
            data-key="value"
            :allow-empty="false"
            aria-label="Layout"
          >
            <template #option="{ option }"><i :class="option.icon" /></template>
          </SelectButton>
        </div>
      </template>
      <template #grid="{ items }">
        <div class="grid">
          <ProductCard v-for="p in (items as Product[])" :key="p.public_id" :product="p" />
        </div>
      </template>
      <template #list="{ items }">
        <ul class="plp-list">
          <li v-for="p in (items as Product[])" :key="p.public_id">
            <NuxtLink :to="`/p/${p.slug}`" class="row-link">
              <div class="row-thumb">
                <img v-if="p.image" :src="p.image" :alt="p.name" loading="lazy" />
                <div v-else class="row-ph"><i class="pi pi-image" /></div>
              </div>
              <div class="row-info">
                <span class="row-name">{{ p.name }}</span>
                <span class="row-sku">{{ p.sku }}</span>
                <p v-if="p.description" class="row-desc">{{ p.description }}</p>
              </div>
              <Tag :value="`per ${p.unit}`" severity="secondary" class="row-tag" />
            </NuxtLink>
          </li>
        </ul>
      </template>
    </DataView>

    <p v-else class="muted">No products found.</p>
  </section>
</template>

<style scoped>
.title {
  margin: 0 0 1.25rem;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 1rem;
}
.dv-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}
.dv-count {
  font-size: 0.9rem;
  color: var(--p-text-muted-color, #64748b);
}
.plp-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
}
.plp-list li + li {
  border-top: 1px solid var(--p-surface-200, #e2e8f0);
}
.row-link {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.9rem 0.25rem;
  text-decoration: none;
  color: inherit;
}
.row-thumb {
  width: 64px;
  height: 64px;
  flex-shrink: 0;
  border-radius: 8px;
  overflow: hidden;
  background: var(--p-surface-100, #f1f5f9);
}
.row-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.row-ph {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--p-surface-300, #cbd5e1);
}
.row-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.row-name {
  font-weight: 600;
}
.row-sku {
  font-size: 0.8rem;
  color: var(--p-text-muted-color, #64748b);
}
.row-desc {
  margin: 0.1rem 0 0;
  font-size: 0.88rem;
  color: var(--p-text-muted-color, #64748b);
  display: -webkit-box;
  -webkit-line-clamp: 1;
  line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.row-tag {
  flex-shrink: 0;
}
.muted {
  color: var(--p-text-muted-color, #64748b);
}
</style>
