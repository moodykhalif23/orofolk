// Typed storefront API client (generated from the OpenAPI contract via @teggo/api).
// Created per call with the configured base URL so it works in SSR and on the
// client. Catalog reads are public; customer-session auth is added later.
import { createApiClient, type ApiClient } from '@teggo/api'

export function useClient(): ApiClient {
  const config = useRuntimeConfig()
  return createApiClient({ baseUrl: config.public.apiBase })
}
