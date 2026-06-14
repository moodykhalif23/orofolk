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

// Teggo brand preset: deep teal primary. Success/danger/warn stay semantic
// (green/red/amber) — only the brand `primary.*` ramp changes. 900 = #013440,
// the dark-teal fill colour. Keep in sync with the storefront & vendor apps.
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
