<script setup lang="ts">
import Button from 'primevue/button'
import BlockLink from '../BlockLink.vue'

const props = defineProps<{ props: Record<string, any> }>()
const p = () => props.props ?? {}
</script>

<template>
  <div class="hero">
    <h1>{{ p().heading }}</h1>
    <p v-if="p().subheading" class="sub">{{ p().subheading }}</p>
    <BlockLink v-if="p().cta" :to="p().cta.href">
      <Button :label="p().cta.label" />
    </BlockLink>
  </div>
</template>

<style scoped>
.hero { text-align: center; padding: 3rem 1rem; background: var(--p-surface-50, #f8fafc); border-radius: 12px; }
.hero h1 { margin: 0 0 0.5rem; font-size: 2rem; }
.sub { color: var(--p-text-muted-color, #64748b); margin-bottom: 1rem; }
</style>
