// Public surface of @teggo/blocks — shared by the storefront renderer and the
// admin page builder so block rendering and schema never drift.
export { default as BlockRenderer } from './BlockRenderer.vue'
export { default as BlockLink } from './BlockLink.vue'
export { default as PlainLink } from './PlainLink.vue'

// Individual renderers, for the admin builder's per-block preview cards.
export { default as HeroBlock } from './blocks/HeroBlock.vue'
export { default as RichTextBlock } from './blocks/RichTextBlock.vue'
export { default as BannerBlock } from './blocks/BannerBlock.vue'
export { default as ProductGridBlock } from './blocks/ProductGridBlock.vue'
export { default as ProductCard } from './blocks/ProductCard.vue'

export { BLOCK_REGISTRY, BLOCK_TYPES } from './registry'
export type { BlockDefinition, BlockField, FieldKind } from './registry'
export { LinkComponentKey, PreviewModeKey } from './types'
export type { Block, LinkProps } from './types'
