// Re-export the generated OpenAPI schema types for ergonomic local use.
// The contract is owned by @teggo/api (generated from openapi.yaml).
import type { components } from '@teggo/api/schema'

export type Product = components['schemas']['StorefrontProduct']
export type ProductList = components['schemas']['StorefrontProductList']
