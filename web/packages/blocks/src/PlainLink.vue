<script setup lang="ts">
// Default link used when no host link component is injected (e.g. the admin
// builder preview, where navigation is inert). Mirrors NuxtLink's `to` prop.
defineProps<{ to?: string }>()
</script>

<template>
  <a :href="to">
    <slot />
  </a>
</template>
