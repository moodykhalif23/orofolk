<script setup lang="ts">
import { computed, inject } from 'vue'
import ProductCard from './ProductCard.vue'
import { PreviewModeKey } from '../types'
import type { components } from '@teggo/api/schema'

type Product = components['schemas']['StorefrontProduct']

// Products are resolved server-side by the storefront and passed in as data, so
// this renderer stays presentational and works in the admin preview too.
const props = defineProps<{ props: Record<string, any> }>()
const p = () => props.props ?? {}
const products = computed<Product[]>(() => (p().products as Product[] | undefined) ?? [])

const preview = inject(PreviewModeKey, false)
const limit = computed<number>(() => Number(p().source?.limit) || 8)
// In the builder there are no resolved products — show a placeholder grid.
const showPlaceholder = computed(() => preview && products.value.length === 0)
</script>

<template>
  <div class="pg">
    <h2 v-if="p().heading">{{ p().heading }}</h2>

    <div v-if="showPlaceholder" class="placeholder">
      <div v-for="n in Math.min(limit, 4)" :key="n" class="skeleton" />
      <p class="hint">
        Up to {{ limit }} products from the selected category — shown live on the storefront.
      </p>
    </div>

    <div v-else class="grid">
      <ProductCard v-for="prod in products" :key="prod.public_id" :product="prod" />
    </div>
  </div>
</template>

<style scoped>
.pg .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 1rem; }
.placeholder { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 0.75rem; }
.skeleton { height: 96px; border-radius: 8px; background: var(--p-surface-100, #f1f5f9); border: 1px dashed var(--p-surface-300, #cbd5e1); }
.hint { grid-column: 1 / -1; margin: 0.25rem 0 0; font-size: 0.85rem; color: var(--p-text-muted-color, #64748b); }
</style>
