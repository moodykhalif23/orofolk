// Single source of truth for storefront block types.
//
// This registry drives THREE consumers so they can never drift:
//   1. The storefront renderer (BlockRenderer.vue) — which component renders a type.
//   2. The admin page builder — the block palette and the property inspector.
//   3. (later) the AI design tool — it emits blocks against these defaults.
//
// Adding a new block type = add one entry here + one renderer component, matching
// the additive JSONB model the backend already uses (no migration required).

// Inspector field kinds the admin builder knows how to render.
export type FieldKind = 'text' | 'textarea' | 'richtext' | 'link' | 'number' | 'category'

export type BlockField = {
  // Path into the block's `props`. Dotted paths (e.g. 'cta.label') are supported.
  key: string
  label: string
  kind: FieldKind
}

export type BlockDefinition = {
  type: string
  label: string
  icon: string // primeicons class, e.g. 'pi pi-star'
  // Default props applied when a block is inserted in the builder.
  defaultProps: () => Record<string, unknown>
  // Fields exposed in the inspector, in display order.
  fields: BlockField[]
}

export const BLOCK_REGISTRY: Record<string, BlockDefinition> = {
  hero: {
    type: 'hero',
    label: 'Hero',
    icon: 'pi pi-star',
    defaultProps: () => ({ heading: 'Your headline', subheading: '', cta: null }),
    fields: [
      { key: 'heading', label: 'Heading', kind: 'text' },
      { key: 'subheading', label: 'Subheading', kind: 'text' },
      { key: 'cta', label: 'Call to action', kind: 'link' },
    ],
  },
  'rich-text': {
    type: 'rich-text',
    label: 'Rich text',
    icon: 'pi pi-align-left',
    defaultProps: () => ({ html: '<p>Edit me</p>' }),
    fields: [{ key: 'html', label: 'Content', kind: 'richtext' }],
  },
  banner: {
    type: 'banner',
    label: 'Banner',
    icon: 'pi pi-flag',
    defaultProps: () => ({ heading: 'Announcement', cta: null }),
    fields: [
      { key: 'heading', label: 'Text', kind: 'text' },
      { key: 'cta', label: 'Call to action', kind: 'link' },
    ],
  },
  cta: {
    type: 'cta',
    label: 'Call to action',
    icon: 'pi pi-megaphone',
    defaultProps: () => ({ heading: 'Ready to order?', cta: { label: 'Get started', href: '/' } }),
    fields: [
      { key: 'heading', label: 'Text', kind: 'text' },
      { key: 'cta', label: 'Button', kind: 'link' },
    ],
  },
  'product-grid': {
    type: 'product-grid',
    label: 'Product grid',
    icon: 'pi pi-th-large',
    // Products are resolved server-side from a category source at render time
    // (see internal/modules/cms/public.go). The block stores the reference only.
    defaultProps: () => ({ heading: '', source: { kind: 'category', category_id: null, limit: 8 } }),
    fields: [
      { key: 'heading', label: 'Heading', kind: 'text' },
      { key: 'source.category_id', label: 'Category', kind: 'category' },
      { key: 'source.limit', label: 'Max products', kind: 'number' },
    ],
  },
}

// Ordered list for palette display.
export const BLOCK_TYPES = Object.keys(BLOCK_REGISTRY)
