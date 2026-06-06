<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'

const { isAuthenticated, logout } = useAuth()
const router = useRouter()
const route = useRoute()

const term = ref((route.query.q as string) ?? '')
// Mobile nav drawer (small screens only). Closes on navigation.
const mobileOpen = ref(false)
watch(() => route.fullPath, () => { mobileOpen.value = false })

function search() {
  const q = term.value.trim()
  mobileOpen.value = false
  if (q) router.push({ path: '/search', query: { q } })
}

function signOut() {
  logout()
  router.push('/')
}
</script>

<template>
  <div class="shell">
    <header class="header">
      <NuxtLink to="/" class="brand"><i class="pi pi-shopping-bag" /> Teggo Store</NuxtLink>
      <button class="hamburger" :aria-expanded="mobileOpen" aria-label="Menu" @click="mobileOpen = !mobileOpen">
        <i :class="mobileOpen ? 'pi pi-times' : 'pi pi-bars'" />
      </button>
      <nav class="nav">
        <NuxtLink to="/">Home</NuxtLink>
        <NuxtLink to="/c/all">Catalog</NuxtLink>
        <NuxtLink to="/contact">Contact</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/quick-order">Quick order</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/reorder">Reorder</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/lists">Lists</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/rfqs">RFQs</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/quotes">Quotes</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/orders">Orders</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/returns">Returns</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/invoices">Invoices</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/budgets">Budgets</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/account/settings">Account</NuxtLink>
      </nav>
      <span class="spacer" />
      <span class="search">
        <i class="pi pi-search" />
        <InputText
          v-model="term"
          placeholder="Search products…"
          class="search-input"
          @keyup.enter="search"
        />
      </span>
      <NuxtLink to="/cart" class="cart-link">
        <Button icon="pi pi-shopping-cart" label="Cart" severity="secondary" outlined />
      </NuxtLink>
      <NuxtLink v-if="!isAuthenticated" to="/login" class="auth-link">
        <Button icon="pi pi-user" label="Sign in" text />
      </NuxtLink>
      <Button v-else class="auth-link" icon="pi pi-sign-out" label="Sign out" text @click="signOut" />
    </header>

    <!-- Mobile navigation drawer: shown only on small screens via the hamburger. -->
    <nav v-show="mobileOpen" class="mobile-menu">
      <span class="mm-search">
        <i class="pi pi-search" />
        <InputText v-model="term" placeholder="Search products…" @keyup.enter="search" />
      </span>
      <NuxtLink to="/">Home</NuxtLink>
      <NuxtLink to="/c/all">Catalog</NuxtLink>
      <NuxtLink to="/contact">Contact</NuxtLink>
      <template v-if="isAuthenticated">
        <NuxtLink to="/quick-order">Quick order</NuxtLink>
        <NuxtLink to="/account/reorder">Reorder</NuxtLink>
        <NuxtLink to="/account/lists">Lists</NuxtLink>
        <NuxtLink to="/account/rfqs">RFQs</NuxtLink>
        <NuxtLink to="/account/quotes">Quotes</NuxtLink>
        <NuxtLink to="/account/orders">Orders</NuxtLink>
        <NuxtLink to="/account/returns">Returns</NuxtLink>
        <NuxtLink to="/account/invoices">Invoices</NuxtLink>
        <NuxtLink to="/account/budgets">Budgets</NuxtLink>
        <NuxtLink to="/account/settings">Account</NuxtLink>
      </template>
      <NuxtLink v-if="!isAuthenticated" to="/login">Sign in</NuxtLink>
      <button v-else class="mm-signout" @click="signOut">Sign out</button>
    </nav>

    <main class="content">
      <slot />
    </main>

    <footer class="footer">
      <p>Teggo storefront — server-rendered for SEO. © {{ new Date().getFullYear() }}</p>
    </footer>

    <AssistantWidget v-if="isAuthenticated" />
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.header {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  padding: 0.9rem 1.5rem;
  border-bottom: 1px solid var(--p-surface-200, #e2e8f0);
  background: var(--p-surface-0, #fff);
}
.brand {
  font-weight: 700;
  font-size: 1.15rem;
  text-decoration: none;
  color: inherit;
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.nav {
  display: flex;
  gap: 1rem;
}
.nav a {
  text-decoration: none;
  color: var(--p-text-muted-color, #64748b);
}
.nav a.router-link-active {
  color: var(--p-primary-color, #0ea5e9);
  font-weight: 600;
}
.spacer {
  flex: 1;
}
.search {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--p-text-muted-color, #64748b);
}
.search-input {
  width: 16rem;
  max-width: 30vw;
}
.content {
  flex: 1;
  padding: 1.5rem;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}
.footer {
  padding: 1.5rem;
  text-align: center;
  color: var(--p-text-muted-color, #64748b);
  border-top: 1px solid var(--p-surface-200, #e2e8f0);
}

/* Hamburger + mobile drawer are desktop-hidden; the breakpoint reveals them. */
.hamburger {
  display: none;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1.35rem;
  color: var(--p-text-color, #1e293b);
  padding: 0.25rem;
}
.mobile-menu {
  display: none;
}

@media (max-width: 860px) {
  .header {
    gap: 0.75rem;
    padding: 0.75rem 1rem;
  }
  /* Collapse the desktop nav, inline search and auth button into the drawer. */
  .nav,
  .header .search,
  .auth-link {
    display: none;
  }
  .spacer {
    flex: 1;
  }
  .hamburger {
    display: inline-flex;
    order: 3; /* keep hamburger at the far right, after brand + cart */
  }
  .cart-link :deep(.p-button-label) {
    display: none; /* icon-only cart on mobile to save room */
  }

  .mobile-menu {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    padding: 0.5rem 1rem 1rem;
    border-bottom: 1px solid var(--p-surface-200, #e2e8f0);
    background: var(--p-surface-0, #fff);
  }
  .mobile-menu a,
  .mm-signout {
    padding: 0.7rem 0.25rem;
    text-decoration: none;
    color: var(--p-text-color, #1e293b);
    border-bottom: 1px solid var(--p-surface-100, #f1f5f9);
    background: none;
    border-left: none;
    border-right: none;
    border-top: none;
    text-align: left;
    font: inherit;
    cursor: pointer;
  }
  .mobile-menu a.router-link-active {
    color: var(--p-primary-color, #0ea5e9);
    font-weight: 600;
  }
  .mm-search {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 0;
    color: var(--p-text-muted-color, #64748b);
  }
  .mm-search :deep(.p-inputtext) {
    width: 100%;
  }
  .content {
    padding: 1rem;
  }
}
</style>
