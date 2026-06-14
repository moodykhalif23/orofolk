import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import Aura from '@primeuix/themes/aura'
import { definePreset } from '@primeuix/themes'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'

// Teggo brand preset — indigo primary (kept in sync with admin & storefront).
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

import 'primeicons/primeicons.css'
import '@/assets/main.css'

import App from '@/App.vue'
import { router } from '@/router'
import { auth } from '@/lib/auth'
import { configureClient } from '@/lib/client'

const app = createApp(App)

// darkModeSelector is a class that's never applied, so the app stays light
// regardless of the OS colour scheme (matches admin & storefront — otherwise
// PrimeVue's default 'system' follows a dark OS and the viewport goes black).
app.use(PrimeVue, { theme: { preset: Teggo, options: { darkModeSelector: '.app-dark' } } })
app.use(ToastService)
app.directive('tooltip', Tooltip)

// Wire the API client to the auth store: token on every request, and a 401
// clears the session and bounces to login.
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
