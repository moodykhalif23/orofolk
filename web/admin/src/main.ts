import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import Tooltip from 'primevue/tooltip'

import 'primeicons/primeicons.css'
import '@/assets/main.css'
import Aura from '@primeuix/themes/aura'
import { definePreset } from '@primeuix/themes'

import App from '@/App.vue'
import { router } from '@/router'
import { useAuthStore } from '@/stores/auth'
import { configureClient } from '@/lib/client'

// Teggo brand preset: ULTIMA-style indigo primary. Success/danger/warn stay
// semantic (green/red/amber) — only the brand `primary.*` ramp changes.
// Keep this palette in sync with the storefront (nuxt.config.ts) and vendor app.
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

const app = createApp(App)

app.use(createPinia())
app.use(PrimeVue, {
  theme: {
    preset: Teggo,
    options: {
      darkModeSelector: '.app-dark',
    },
  },
})
app.use(ToastService)
app.use(ConfirmationService)
app.directive('tooltip', Tooltip)

// Wire the API client to the auth store: token on every request, and a 401
// clears the session and bounces to login.
const auth = useAuthStore()
configureClient({
  getToken: () => auth.token,
  onUnauthorized: () => {
    auth.logout()
    if (router.currentRoute.value.name !== 'login') {
      router.push({ name: 'login' })
    }
  },
})

app.use(router)
app.mount('#app')
