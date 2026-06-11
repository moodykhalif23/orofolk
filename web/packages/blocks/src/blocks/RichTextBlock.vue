<script setup lang="ts">
const props = defineProps<{ props: Record<string, any> }>()
</script>

<template>
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="rich" v-html="(props.props ?? {}).html" />
</template>

<style scoped>
.rich { line-height: 1.7; }
</style>
