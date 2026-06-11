<script setup lang="ts">
import { inject } from 'vue'
import { LinkComponentKey } from './types'
import PlainLink from './PlainLink.vue'

// Renders the host-provided link component (NuxtLink in the storefront) or a
// plain anchor by default. Keeps block renderers free of any framework router.
defineProps<{ to?: string }>()
const LinkComponent = inject(LinkComponentKey, PlainLink)
</script>

<template>
  <component :is="LinkComponent" :to="to">
    <slot />
  </component>
</template>
