<script setup lang="ts">
import Card from 'primevue/card'
import Tag from 'primevue/tag'
import type { components } from '@teggo/api/schema'

type Product = components['schemas']['StorefrontProduct']

defineProps<{ product: Product }>()
</script>

<template>
  <NuxtLink :to="`/p/${product.slug}`" class="card-link">
    <Card class="product-card">
      <template #header>
        <div class="thumb">
          <img v-if="product.image" :src="product.image" :alt="product.name" loading="lazy" />
          <div v-else class="thumb-ph"><i class="pi pi-image" /></div>
        </div>
      </template>
      <template #title>
        <span class="name">{{ product.name }}</span>
      </template>
      <template #subtitle>
        <span class="sku">{{ product.sku }}</span>
      </template>
      <template #content>
        <p v-if="product.description" class="desc">{{ product.description }}</p>
        <Tag :value="`per ${product.unit}`" severity="secondary" />
      </template>
    </Card>
  </NuxtLink>
</template>

<style scoped>
.card-link {
  text-decoration: none;
  color: inherit;
  display: block;
  height: 100%;
}
.product-card {
  height: 100%;
  transition: box-shadow 0.15s ease;
  overflow: hidden;
}
.thumb {
  aspect-ratio: 4 / 3;
  background: var(--p-surface-100, #f1f5f9);
}
.thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.thumb-ph {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 2rem;
  color: var(--p-surface-300, #cbd5e1);
}
.product-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}
.name {
  font-size: 1rem;
}
.sku {
  font-size: 0.8rem;
  color: var(--p-text-muted-color, #64748b);
}
.desc {
  margin: 0 0 0.75rem;
  font-size: 0.9rem;
  color: var(--p-text-muted-color, #64748b);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
