import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import Aura from '@primeuix/themes/aura'
import { definePreset } from '@primeuix/themes'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'

// Teggo brand preset — deep teal primary (kept in sync with admin & storefront).
const Teggo = definePreset(Aura, {
  semantic: {
    primary: {
      50: '#eef9fb',
      100: '#d3eff4',
      200: '#ade0e9',
      300: '#79cad9',
      400: '#3fa6bd',
      500: '#0e7490',
      600: '#0c6178',
      700: '#0a4e60',
      800: '#083e4c',
      900: '#013440',
      950: '#01242d',
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
