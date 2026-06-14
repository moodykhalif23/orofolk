<script setup lang="ts">
import Button from 'primevue/button'
import type { components } from '@teggo/api/schema'

type SuggestItem = components['schemas']['SuggestItem']

const { isAuthenticated, logout } = useAuth()
const router = useRouter()
const route = useRoute()
const client = useClient()

// Per-tenant branding (SAAS.md #4): name, accent color and logo resolve from
// the serving host server-side, so the first paint is already on-brand.
const { data: branding } = await useAsyncData('branding', async () => {
  const { data } = await client.GET('/storefront/branding')
  return data ?? null
})
const storeName = computed(() => branding.value?.store_name || 'Teggo Store')
const brandStyle = computed(() =>
  branding.value?.brand_color ? { '--p-primary-color': branding.value.brand_color } : undefined,
)

// i18n: content-locale selector (populated from the store's configured locales).
const locale = useLocale()
const localeOptions = ref<string[]>([])
const localeDefault = ref('')
onMounted(async () => {
  const { data } = await client.GET('/storefront/locales')
  if (data) {
    localeOptions.value = data.locales ?? []
    localeDefault.value = data.default ?? ''
  }
})

const term = ref((route.query.q as string) ?? '')
// Mobile nav drawer (small screens only). Closes on navigation.
const mobileOpen = ref(false)
// Account dropdown (desktop): a single menu for all B2B/portal links so the
// primary bar stays about shopping. Closes on navigation.
const accountOp = ref()
function toggleAccount(e: Event) { accountOp.value?.toggle(e) }
watch(() => route.fullPath, () => { mobileOpen.value = false; accountOp.value?.hide() })

// Search typeahead: fetch suggestions while typing; selecting one jumps
// straight to that product, while Enter runs a full search.
const suggestions = ref<SuggestItem[]>([])
async function onComplete(e: { query: string }) {
  const t = e.query.trim()
  if (t.length < 2) {
    suggestions.value = []
    return
  }
  const { data } = await client.GET('/storefront/suggest', { params: { query: { q: t } } })
  suggestions.value = data?.items ?? []
}
function onSelect(e: { value: SuggestItem }) {
  mobileOpen.value = false
  term.value = ''
  router.push(`/p/${e.value.slug}`)
}
function search() {
  const q = (typeof term.value === 'string' ? term.value : '').trim()
  mobileOpen.value = false
  if (q) router.push({ path: '/search', query: { q } })
}

function signOut() {
  accountOp.value?.hide()
  logout()
  router.push('/')
}
</script>

<template>
  <div class="shell" :style="brandStyle">
    <header class="header">
      <!-- Top tier: brand + search + cart/auth. Stays on one line. -->
      <div class="bar">
        <NuxtLink to="/" class="brand">
          <img v-if="branding?.logo_url" :src="branding.logo_url" :alt="storeName" class="brand-logo" />
          <i v-else class="pi pi-shopping-bag" />
          {{ storeName }}
        </NuxtLink>
        <button class="hamburger" :aria-expanded="mobileOpen" aria-label="Menu" @click="mobileOpen = !mobileOpen">
          <i :class="mobileOpen ? 'pi pi-times' : 'pi pi-bars'" />
        </button>
        <span class="spacer" />
        <span class="search">
          <i class="pi pi-search" />
          <AutoComplete
            v-model="term"
            :suggestions="suggestions"
            option-label="name"
            placeholder="Search products…"
            class="search-input"
            :complete-on-focus="false"
            @complete="onComplete"
            @item-select="onSelect"
            @keyup.enter="search"
          >
            <template #option="{ option }">
              <div class="sug">
                <span class="sug-name">{{ option.name }}</span>
                <span class="sug-sku">{{ option.sku }}</span>
              </div>
            </template>
          </AutoComplete>
        </span>
        <select v-if="localeOptions.length" v-model="locale" class="locale-select" aria-label="Language">
          <option value="">{{ localeDefault || 'Default' }}</option>
          <option v-for="l in localeOptions" :key="l" :value="l">{{ l }}</option>
        </select>
        <ClientOnly>
          <NotificationBell v-if="isAuthenticated" class="header-bell" />
        </ClientOnly>
        <NuxtLink to="/cart" class="cart-link">
          <Button icon="pi pi-shopping-cart" label="Cart" severity="secondary" outlined />
        </NuxtLink>
        <NuxtLink v-if="!isAuthenticated" to="/login" class="auth-link">
          <Button icon="pi pi-user" label="Sign in" text />
        </NuxtLink>
      </div>

      <!-- Bottom tier (desktop only): primary shopping nav. All B2B/portal links
           live in the Account menu so the bar never overflows. -->
      <nav class="nav">
        <NuxtLink to="/">Home</NuxtLink>
        <NuxtLink to="/c/all">Catalog</NuxtLink>
        <NuxtLink v-if="isAuthenticated" to="/quick-order">Quick order</NuxtLink>
        <NuxtLink to="/contact">Contact</NuxtLink>
        <span class="spacer" />
        <button
          v-if="isAuthenticated"
          type="button"
          class="nav-account"
          aria-haspopup="true"
          @click="toggleAccount"
        >
          <i class="pi pi-user-edit" /> Account <i class="pi pi-chevron-down chev" />
        </button>
      </nav>
      <Popover ref="accountOp" class="account-pop">
        <div class="acct-menu">
          <div class="acct-col">
            <span class="acct-h">Orders &amp; documents</span>
            <NuxtLink to="/account/orders" class="acct-link"><i class="pi pi-box" /><span>Orders</span></NuxtLink>
            <NuxtLink to="/account/quotes" class="acct-link"><i class="pi pi-file-edit" /><span>Quotes</span></NuxtLink>
            <NuxtLink to="/account/rfqs" class="acct-link"><i class="pi pi-comments" /><span>RFQs</span></NuxtLink>
            <NuxtLink to="/account/invoices" class="acct-link"><i class="pi pi-receipt" /><span>Invoices</span></NuxtLink>
            <NuxtLink to="/account/returns" class="acct-link"><i class="pi pi-replay" /><span>Returns</span></NuxtLink>
            <NuxtLink to="/account/subscriptions" class="acct-link"><i class="pi pi-sync" /><span>Recurring</span></NuxtLink>
          </div>
          <div class="acct-col">
            <span class="acct-h">Buying tools</span>
            <NuxtLink to="/account/reorder" class="acct-link"><i class="pi pi-history" /><span>Reorder</span></NuxtLink>
            <NuxtLink to="/account/lists" class="acct-link"><i class="pi pi-list" /><span>Lists</span></NuxtLink>
            <span class="acct-h">Programs</span>
            <NuxtLink to="/account/rebates" class="acct-link"><i class="pi pi-percentage" /><span>Rebates</span></NuxtLink>
            <NuxtLink to="/account/budgets" class="acct-link"><i class="pi pi-wallet" /><span>Budgets</span></NuxtLink>
            <div class="acct-sep" />
            <NuxtLink to="/account/settings" class="acct-link"><i class="pi pi-cog" /><span>Settings</span></NuxtLink>
            <button type="button" class="acct-link acct-signout" @click="signOut"><i class="pi pi-sign-out" /><span>Sign out</span></button>
          </div>
        </div>
      </Popover>
    </header>

    <!-- Mobile navigation drawer: shown only on small screens via the hamburger. -->
    <nav v-show="mobileOpen" class="mobile-menu">
      <span class="mm-search">
        <i class="pi pi-search" />
        <AutoComplete
          v-model="term"
          :suggestions="suggestions"
          option-label="name"
          placeholder="Search products…"
          :complete-on-focus="false"
          @complete="onComplete"
          @item-select="onSelect"
          @keyup.enter="search"
        />
      </span>
      <NuxtLink to="/">Home</NuxtLink>
      <NuxtLink to="/c/all">Catalog</NuxtLink>
      <NuxtLink to="/contact">Contact</NuxtLink>
      <template v-if="isAuthenticated">
        <NuxtLink to="/quick-order">Quick order</NuxtLink>
        <span class="mm-group">Orders &amp; documents</span>
        <NuxtLink to="/account/orders">Orders</NuxtLink>
        <NuxtLink to="/account/quotes">Quotes</NuxtLink>
        <NuxtLink to="/account/rfqs">RFQs</NuxtLink>
        <NuxtLink to="/account/invoices">Invoices</NuxtLink>
        <NuxtLink to="/account/returns">Returns</NuxtLink>
        <NuxtLink to="/account/subscriptions">Recurring</NuxtLink>
        <span class="mm-group">Buying tools</span>
        <NuxtLink to="/account/reorder">Reorder</NuxtLink>
        <NuxtLink to="/account/lists">Lists</NuxtLink>
        <span class="mm-group">Programs</span>
        <NuxtLink to="/account/rebates">Rebates</NuxtLink>
        <NuxtLink to="/account/budgets">Budgets</NuxtLink>
        <span class="mm-group">Account</span>
        <NuxtLink to="/account/settings">Settings</NuxtLink>
      </template>
      <NuxtLink v-if="!isAuthenticated" to="/login">Sign in</NuxtLink>
      <button v-else class="mm-signout" @click="signOut">Sign out</button>
    </nav>

    <main class="content">
      <slot />
    </main>

    <footer class="footer">
      <p>{{ storeName }} — powered by Teggo. © {{ new Date().getFullYear() }}</p>
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
  border-bottom: 1px solid var(--p-surface-200, #e2e8f0);
  background: var(--p-surface-0, #fff);
}
/* Top tier: brand on the left, search + cart/auth pushed to the right. */
.bar {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  padding: 0.9rem 1.5rem;
}
.brand {
  font-weight: 700;
  font-size: 1.15rem;
  text-decoration: none;
  color: inherit;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  white-space: nowrap;
}
.brand-logo {
  height: 1.6rem;
  width: auto;
  display: block;
}
/* Bottom tier: navigation as a wrapping row of pill-buttons. */
.nav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  padding: 0 1.5rem 0.85rem;
}
.nav a {
  display: inline-flex;
  align-items: center;
  padding: 0.45rem 0.85rem;
  border-radius: 8px;
  border: 1px solid transparent;
  text-decoration: none;
  white-space: nowrap;
  font-size: 0.92rem;
  line-height: 1.2;
  color: var(--p-text-muted-color, #475569);
  transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}
.nav a:hover {
  background: var(--p-surface-100, #f1f5f9);
  color: var(--p-text-color, #0f172a);
}
.nav a.router-link-active {
  background: var(--p-primary-50, #eef2ff);
  border-color: var(--p-primary-200, #c7d2fe);
  color: var(--p-primary-700, #4338ca);
  font-weight: 600;
}
/* Account dropdown trigger — matches the nav pills. */
.nav-account {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.85rem;
  border-radius: 8px;
  border: 1px solid transparent;
  background: none;
  font: inherit;
  font-size: 0.92rem;
  line-height: 1.2;
  color: var(--p-text-muted-color, #475569);
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}
.nav-account:hover {
  background: var(--p-surface-100, #f1f5f9);
  color: var(--p-text-color, #0f172a);
}
.nav-account .chev {
  font-size: 0.7rem;
}
/* Account popover — two balanced columns of grouped portal links. */
.acct-menu {
  display: grid;
  grid-template-columns: repeat(2, minmax(11rem, 1fr));
  gap: 0 1.75rem;
}
.acct-col {
  display: flex;
  flex-direction: column;
}
.acct-h {
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--p-text-muted-color, #94a3b8);
  padding: 0.55rem 0.5rem 0.2rem;
}
.acct-col > .acct-h:first-child {
  padding-top: 0.1rem;
}
.acct-link {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.4rem 0.5rem;
  border-radius: 6px;
  text-decoration: none;
  color: var(--p-text-color, #334155);
  font: inherit;
  font-size: 0.9rem;
  background: none;
  border: none;
  cursor: pointer;
  text-align: left;
  width: 100%;
}
.acct-link:hover {
  background: var(--p-surface-100, #f1f5f9);
}
.acct-link i {
  color: var(--p-text-muted-color, #94a3b8);
  width: 1rem;
  text-align: center;
}
.acct-link.router-link-active {
  background: var(--p-primary-50, #eef2ff);
  color: var(--p-primary-700, #4338ca);
}
.acct-link.router-link-active i {
  color: var(--p-primary-600, #4f46e5);
}
.acct-sep {
  border-top: 1px solid var(--p-surface-200, #e2e8f0);
  margin: 0.4rem 0;
}
.acct-signout {
  color: var(--p-red-600, #dc2626);
}
.acct-signout i {
  color: var(--p-red-500, #ef4444);
}
.spacer {
  flex: 1;
}
/* Keep header button labels from breaking mid-word on tight widths. */
.cart-link :deep(.p-button-label),
.auth-link :deep(.p-button-label) {
  white-space: nowrap;
}
.locale-select {
  border: 1px solid var(--p-surface-300, #cbd5e1);
  border-radius: 8px;
  padding: 0.4rem 0.5rem;
  background: var(--p-surface-0, #fff);
  color: var(--p-text-color, #334155);
  font: inherit;
  cursor: pointer;
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
.search-input :deep(input) { width: 100%; }
.sug { display: flex; justify-content: space-between; gap: 1rem; align-items: baseline; }
.sug-sku { color: var(--p-text-muted-color, #64748b); font-size: 0.82rem; }
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
  .bar {
    gap: 0.75rem;
    padding: 0.75rem 1rem;
  }
  /* Collapse the desktop nav, inline search and auth button into the drawer. */
  .nav,
  .bar .search,
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
    color: var(--p-primary-color, #6366f1);
    font-weight: 600;
  }
  .mm-group {
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--p-text-muted-color, #94a3b8);
    padding: 0.75rem 0.25rem 0.15rem;
  }
  .mm-search {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 0;
    color: var(--p-text-muted-color, #64748b);
  }
  .mm-search :deep(.p-autocomplete),
  .mm-search :deep(input) {
    width: 100%;
  }
  .content {
    padding: 1rem;
  }
}
</style>
