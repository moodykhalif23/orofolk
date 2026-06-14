import Aura from '@primeuix/themes/aura'
import { definePreset } from '@primeuix/themes'

// Teggo brand preset: ULTIMA-style indigo primary (success/danger stay
// semantic). Keep this palette in sync with the admin & vendor apps. Tenants
// can still white-label by overriding --p-primary-color at runtime.
const Teggo = definePreset(Aura, {
  semantic: {
    primary: {
      50: '#eef2ff',
      100: '#e0e7ff',
      200: '#c7d2fe',
      300: '#a5b4fc',
      400: '#818cf8',
      500: '#6366f1',
      600: '#4f46e5',
      700: '#4338ca',
      800: '#3730a3',
      900: '#312e81',
      950: '#1e1b4b',
    },
  },
})

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-01-01',
  devtools: { enabled: true },

  modules: ['@primevue/nuxt-module'],

  // @teggo/api and @teggo/blocks ship as source (workspace packages) — transpile them.
  build: { transpile: ['@teggo/api', '@teggo/blocks'] },

  primevue: {
    // Components auto-import by default; directives must be listed explicitly.
    directives: { include: ['Tooltip'] },
    options: {
      theme: {
        preset: Teggo,
        options: {
          darkModeSelector: '.app-dark',
        },
      },
    },
  },

  css: ['primeicons/primeicons.css', '~/assets/css/main.css'],

  runtimeConfig: {
    public: {
      // Base URL of the Go storefront API. Override with NUXT_PUBLIC_API_BASE.
      apiBase: 'http://localhost:8080',
    },
  },

  $production: {
    routeRules: {
      '/': { swr: 600 },
      '/c/**': { swr: 600 },
      '/p/**': { swr: 600 },
    },
  },

  app: {
    head: {
      htmlAttrs: { lang: 'en' },
      title: 'Teggo Storefront',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&display=swap',
        },
      ],
    },
  },
})
