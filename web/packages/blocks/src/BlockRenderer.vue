<script setup lang="ts">
import { provide, type Component } from 'vue'
import { LinkComponentKey, PreviewModeKey, type Block } from './types'
import HeroBlock from './blocks/HeroBlock.vue'
import RichTextBlock from './blocks/RichTextBlock.vue'
import BannerBlock from './blocks/BannerBlock.vue'
import ProductGridBlock from './blocks/ProductGridBlock.vue'

const props = defineProps<{
  blocks: Block[]
  // Host link component (NuxtLink in the storefront). Omitted in the admin
  // preview, where BlockLink falls back to a plain anchor.
  linkComponent?: Component
  // Set by the admin builder so renderers can show placeholders for data that
  // is only resolved server-side on the live storefront.
  preview?: boolean
}>()

if (props.linkComponent) provide(LinkComponentKey, props.linkComponent)
provide(PreviewModeKey, props.preview ?? false)

// type -> renderer. `banner` and `cta` share one component.
const RENDERERS: Record<string, Component> = {
  hero: HeroBlock,
  'rich-text': RichTextBlock,
  banner: BannerBlock,
  cta: BannerBlock,
  'product-grid': ProductGridBlock,
}

function rendererFor(type: string): Component | null {
  return RENDERERS[type] ?? null
}
</script>

<template>
  <div
    v-for="(b, i) in blocks"
    :key="b.id || i"
    class="block"
  >
    <component :is="rendererFor(b.type)" v-if="rendererFor(b.type)" :props="b.props ?? {}" />
  </div>
</template>

<style scoped>
.block { margin-bottom: 2rem; }
</style>
