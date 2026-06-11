<script setup lang="ts">
import { resolveComponent, type Component } from 'vue'
import Message from 'primevue/message'
import { BlockRenderer, type Block } from '@teggo/blocks'
import type { components } from '@teggo/api/schema'

type Page = components['schemas']['ContentPage']

const route = useRoute()
const client = useClient()
const slug = route.params.slug as string

// SSR fetch so CMS pages are crawlable. Sends the auth cookie so targeted
// (customer-group) pages resolve for signed-in buyers.
const { data: page, error } = await useAsyncData(`page-${slug}`, async () => {
  const { data, error } = await client.GET('/storefront/pages/{slug}', { params: { path: { slug } } })
  if (error || !data) throw createError({ statusCode: 404, statusMessage: 'Page not found' })
  return data as Page
})

const seo = computed(() => (page.value?.seo ?? {}) as Record<string, string>)
useSeoMeta({
  title: () => seo.value.title || page.value?.title || 'Teggo',
  description: () => seo.value.description || '',
})

const blocks = computed<Block[]>(() => (page.value?.blocks as Block[] | undefined) ?? [])

// Inject NuxtLink so shared block renderers produce real client-side nav.
const NuxtLink = resolveComponent('NuxtLink') as Component
</script>

<template>
  <section>
    <Message v-if="error" severity="error" :closable="false">Page not found.</Message>
    <BlockRenderer v-else-if="page" :blocks="blocks" :link-component="NuxtLink" />
  </section>
</template>
