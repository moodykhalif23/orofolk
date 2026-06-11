<script setup lang="ts">
import Button from 'primevue/button'
import BlockLink from '../BlockLink.vue'

// Renders both `banner` and `cta` block types — same layout.
const props = defineProps<{ props: Record<string, any> }>()
const p = () => props.props ?? {}
</script>

<template>
  <div class="banner">
    <span>{{ p().heading || p().text }}</span>
    <BlockLink v-if="p().cta" :to="p().cta.href">
      <Button :label="p().cta.label" size="small" />
    </BlockLink>
  </div>
</template>

<style scoped>
.banner { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 1.5rem; background: var(--p-primary-50, #e0f2fe); border-radius: 10px; }
</style>
