<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterView, useRouter, useRoute } from 'vue-router'
import Button from 'primevue/button'
import { auth } from '@/lib/auth'

const router = useRouter()
const route = useRoute()

// Mobile sidebar drawer (small screens only). Closes on navigation.
const mobileOpen = ref(false)
watch(() => route.fullPath, () => { mobileOpen.value = false })

const nav = [
  { label: 'Dashboard', icon: 'pi pi-home', routeName: 'dashboard' },
  { label: 'Orders', icon: 'pi pi-shopping-cart', routeName: 'orders' },
  { label: 'Products', icon: 'pi pi-box', routeName: 'products' },
  { label: 'Payouts', icon: 'pi pi-wallet', routeName: 'payouts' },
]

function logout() {
  auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="shell" :class="{ 'drawer-open': mobileOpen }">
    <div class="scrim" @click="mobileOpen = false" />
    <aside class="sidebar">
      <div class="brand"><i class="pi pi-shop" /> Vendor Portal</div>
      <nav class="nav">
        <RouterLink
          v-for="n in nav"
          :key="n.routeName"
          :to="{ name: n.routeName }"
          class="navitem"
          active-class="active"
        >
          <i :class="n.icon" /> <span>{{ n.label }}</span>
        </RouterLink>
      </nav>
      <div class="foot">
        <div class="email">{{ auth.email }}</div>
        <Button label="Sign out" icon="pi pi-sign-out" text size="small" @click="logout" />
      </div>
    </aside>
    <main class="content">
      <header class="mtop">
        <button class="hamburger" aria-label="Menu" @click="mobileOpen = !mobileOpen"><i class="pi pi-bars" /></button>
        <span class="mtop-brand"><i class="pi pi-shop" /> Vendor Portal</span>
      </header>
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.shell { display: grid; grid-template-columns: var(--teggo-sidebar-width) 1fr; height: 100%; }
.sidebar {
  background: #0f172a; color: #e2e8f0; display: flex; flex-direction: column;
  padding: 1rem 0.75rem;
}
.brand { font-weight: 700; font-size: 1.05rem; padding: 0.5rem 0.75rem 1rem; display: flex; gap: 0.5rem; align-items: center; }
.nav { display: flex; flex-direction: column; gap: 0.15rem; flex: 1; }
.navitem {
  display: flex; align-items: center; gap: 0.65rem; padding: 0.6rem 0.75rem;
  border-radius: 8px; color: #cbd5e1; font-size: 0.95rem;
}
.navitem:hover { background: #1e293b; color: #fff; }
.navitem.active { background: #1d4ed8; color: #fff; }
.foot { border-top: 1px solid #1e293b; padding-top: 0.75rem; }
.email { font-size: 0.8rem; color: #94a3b8; padding: 0 0.75rem 0.35rem; overflow: hidden; text-overflow: ellipsis; }
.content { padding: 1.75rem 2rem; overflow: auto; }

/* Mobile topbar (hamburger) is desktop-hidden; the breakpoint turns the sidebar
   into an off-canvas drawer. */
.mtop { display: none; }
.scrim { display: none; }
.hamburger {
  background: none; border: none; cursor: pointer; font-size: 1.3rem;
  color: #0f172a; padding: 0.25rem 0.5rem;
}
@media (max-width: 900px) {
  .shell { grid-template-columns: 1fr; }
  .sidebar {
    position: fixed; top: 0; left: 0; bottom: 0; width: var(--teggo-sidebar-width);
    z-index: 60; transform: translateX(-100%); transition: transform 0.2s ease;
    box-shadow: 0 0 40px rgba(0,0,0,0.35);
  }
  .drawer-open .sidebar { transform: translateX(0); }
  .drawer-open .scrim {
    display: block; position: fixed; inset: 0; z-index: 55; background: rgba(15,23,42,0.45);
  }
  .mtop {
    display: flex; align-items: center; gap: 0.5rem;
    margin: -1.75rem -2rem 1rem; padding: 0.6rem 1rem;
    border-bottom: 1px solid #e2e8f0; background: #fff;
  }
  .mtop-brand { font-weight: 700; display: flex; align-items: center; gap: 0.4rem; }
  .content { padding: 1.25rem 1rem; }
}
</style>
