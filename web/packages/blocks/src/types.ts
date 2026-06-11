import type { InjectionKey, Component } from 'vue'

// Blocks are an additive JSONB union persisted by the CMS. Renderers read props
// loosely; the registry (registry.ts) is the source of truth for shape + defaults.
export type Block = {
  type: string
  id?: string
  props?: Record<string, unknown>
}

export type LinkProps = { label?: string; href?: string }

// The storefront provides NuxtLink; the admin builder provides a plain anchor.
// Both accept a `to` prop, so renderers stay framework-agnostic.
export const LinkComponentKey: InjectionKey<Component> = Symbol('teggo-link-component')

// True inside the admin builder preview, where server-resolved data (e.g. a
// product-grid's products) is absent — renderers can show a placeholder instead.
export const PreviewModeKey: InjectionKey<boolean> = Symbol('teggo-preview-mode')
