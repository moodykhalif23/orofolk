<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { RouterView, RouterLink, useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useBillingStore } from '@/stores/billing'
import { useNotificationsStore } from '@/stores/notifications'
import Avatar from 'primevue/avatar'
import Popover from 'primevue/popover'
import Button from 'primevue/button'
import NotificationBell from '@/components/NotificationBell.vue'
import {
  sections,
  sectionByKey,
  isSectionVisible,
  visibleLeaves,
  type NavSection,
} from '@/nav/sections'

const auth = useAuthStore()
const billing = useBillingStore()
const notifications = useNotificationsStore()

// The layout only renders for an authenticated session, so start the feed on
// mount and tear it down on sign-out. Plan entitlements load once so the nav
// mirrors what the API would allow anyway.
onMounted(() => billing.load())
onMounted(() => notifications.start())
onBeforeUnmount(() => notifications.stop())
const router = useRouter()
const route = useRoute()

// Mobile drawer (small screens only). Closes on navigation.
const mobileOpen = ref(false)
watch(() => route.fullPath, () => { mobileOpen.value = false })

const can = (p: string) => auth.can(p)
const allows = (f: string) => billing.allows(f)

// Tier 1 — head nav: every section the user/plan can see. Assistant is pulled
// out as a utility button (a distinct AI entry point), so it's excluded here.
const headSections = computed(() =>
  sections.filter((s) => s.key !== 'assistant' && isSectionVisible(s, can, allows)),
)
const assistant = computed(() => {
  const a = sectionByKey('assistant')
  return a && isSectionVisible(a, can, allows) ? a : null
})

// Resolve every leaf (and standalone) to its concrete path once, so the active
// section/leaf can be found by longest-prefix match — which keeps detail routes
// (e.g. /orders/123) highlighting their parent.
const leafIndex = computed(() => {
  const out: { key: string; routeName: string; path: string }[] = []
  for (const s of sections) {
    if (s.routeName) out.push({ key: s.key, routeName: s.routeName, path: router.resolve({ name: s.routeName }).path })
    for (const it of s.items ?? []) out.push({ key: s.key, routeName: it.routeName, path: router.resolve({ name: it.routeName }).path })
  }
  return out
})

interface NavContext {
  key: string
  leafName: string | null
  isHub: boolean
}
const ctx = computed<NavContext | null>(() => {
  if (route.name === 'section') {
    return { key: String(route.params.key), leafName: null, isHub: true }
  }
  let best: { key: string; routeName: string; path: string } | null = null
  for (const e of leafIndex.value) {
    if (route.path === e.path || (e.path !== '/' && route.path.startsWith(e.path + '/'))) {
      if (!best || e.path.length > best.path.length) best = e
    }
  }
  return best ? { key: best.key, leafName: best.routeName, isHub: false } : null
})

// Tier 2 — sub-nav: the current section's leaves (only when it has any).
const subNavSection = computed<NavSection | null>(() => {
  const k = ctx.value?.key
  const s = k ? sectionByKey(k) : undefined
  return s && s.items?.length ? s : null
})
const subNavLeaves = computed(() =>
  subNavSection.value ? visibleLeaves(subNavSection.value, can, allows) : [],
)

function sectionTo(s: NavSection) {
  return s.routeName ? { name: s.routeName } : { name: 'section', params: { key: s.key } }
}

const account = ref()
function toggleAccount(e: Event) {
  account.value?.toggle(e)
}
function logout() {
  auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="layout" :class="{ 'drawer-open': mobileOpen }">
    <div class="scrim" @click="mobileOpen = false" />

    <!-- Tier 1: head nav bar -->
    <header class="head">
      <div class="head-inner">
        <button type="button" class="hamburger" aria-label="Menu" @click="mobileOpen = !mobileOpen">
          <i class="pi pi-bars" />
        </button>
        <RouterLink :to="{ name: 'dashboard' }" class="brand">
          <span class="brand-badge"><i class="pi pi-bolt" /></span>
          <span class="brand-name">Teggo<span class="brand-sub">Admin</span></span>
        </RouterLink>

        <nav class="primary">
          <RouterLink
            v-for="s in headSections"
            :key="s.key"
            :to="sectionTo(s)"
            class="primary-link"
            :class="{ active: ctx?.key === s.key }"
          >{{ s.label }}</RouterLink>
        </nav>

        <div class="head-utils">
          <RouterLink
            v-if="assistant"
            :to="{ name: 'assistant' }"
            class="util-btn"
            :class="{ active: ctx?.key === 'assistant' }"
            aria-label="Assistant"
            title="Assistant"
          ><i class="pi pi-sparkles" /></RouterLink>
          <NotificationBell class="util-bell" />
          <button type="button" class="account-trigger" @click="toggleAccount" aria-label="Account">
            <Avatar :label="auth.initials" shape="square" class="account-avatar" />
          </button>
          <Popover ref="account" class="account-pop">
            <div class="account-card">
              <div class="account-head">
                <Avatar :label="auth.initials" shape="square" size="large" class="account-avatar" />
                <div class="account-id">
                  <div class="account-email">{{ auth.email ?? 'Signed in' }}</div>
                  <div class="account-org">Organization {{ auth.orgId ?? '—' }}</div>
                </div>
              </div>
              <div class="account-meta">
                <i class="pi pi-shield" />
                <span>{{ auth.permissions.length }} permissions</span>
              </div>
              <Button
                icon="pi pi-sign-out"
                label="Sign out"
                severity="secondary"
                outlined
                class="account-signout"
                @click="logout"
              />
            </div>
          </Popover>
        </div>
      </div>
    </header>

    <!-- Tier 2: per-section sub-nav bar -->
    <div v-if="subNavSection && subNavLeaves.length" class="subnav">
      <div class="subnav-inner">
        <RouterLink
          :to="{ name: 'section', params: { key: subNavSection.key } }"
          class="sub-link sub-overview"
          :class="{ active: ctx?.isHub }"
        >
          <i class="pi pi-th-large" /><span>Overview</span>
        </RouterLink>
        <span class="sub-divider" />
        <RouterLink
          v-for="leaf in subNavLeaves"
          :key="leaf.routeName"
          :to="{ name: leaf.routeName }"
          class="sub-link"
          :class="{ active: ctx?.leafName === leaf.routeName }"
        >
          <i :class="leaf.icon" /><span>{{ leaf.label }}</span>
        </RouterLink>
      </div>
    </div>

    <!-- Mobile drawer: the section list -->
    <aside class="drawer">
      <nav class="drawer-nav">
        <RouterLink
          v-for="s in headSections"
          :key="s.key"
          :to="sectionTo(s)"
          class="drawer-link"
          :class="{ active: ctx?.key === s.key }"
        >
          <i :class="s.icon" /><span>{{ s.label }}</span>
        </RouterLink>
        <RouterLink
          v-if="assistant"
          :to="{ name: 'assistant' }"
          class="drawer-link"
          :class="{ active: ctx?.key === 'assistant' }"
        >
          <i class="pi pi-sparkles" /><span>Assistant</span>
        </RouterLink>
      </nav>
    </aside>

    <main class="content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  height: 100dvh;
  overflow: hidden;
}

/* --- Tier 1: head nav --- */
.head {
  flex-shrink: 0;
  background: var(--teggo-surface, #fff);
  border-bottom: 1px solid var(--p-surface-200, #e2e8f0);
}
.head-inner {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  height: 58px;
  padding: 0 1.25rem;
}
.brand {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-shrink: 0;
  text-decoration: none;
}
.brand-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 0 10px 0 0; /* brand corner motif */
  background: var(--p-primary-color, #16a34a);
  color: #fff;
  font-size: 0.95rem;
  flex-shrink: 0;
}
.brand-name {
  font-weight: 800;
  font-size: 1.05rem;
  letter-spacing: -0.01em;
  color: var(--p-text-color, #0f172a);
}
.brand-sub {
  margin-left: 0.35rem;
  font-weight: 500;
  font-size: 0.82rem;
  color: var(--p-text-muted-color, #94a3b8);
}

.primary {
  display: flex;
  align-items: center;
  gap: 0.15rem;
  flex: 1;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: none;
  height: 100%;
}
.primary::-webkit-scrollbar { display: none; }
.primary-link {
  position: relative;
  display: inline-flex;
  align-items: center;
  height: 100%;
  padding: 0 0.7rem;
  white-space: nowrap;
  text-decoration: none;
  color: var(--p-text-color, #475569);
  font-size: 0.9rem;
  font-weight: 600;
  border-bottom: 2px solid transparent;
  transition: color 0.15s ease, border-color 0.15s ease;
}
.primary-link:hover { color: var(--p-primary-color, #16a34a); }
.primary-link.active {
  color: var(--p-primary-color, #16a34a);
  border-bottom-color: var(--p-primary-color, #16a34a);
}

.head-utils {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-shrink: 0;
}
.util-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 9px;
  color: var(--p-text-muted-color, #64748b);
  text-decoration: none;
  transition: background-color 0.15s ease, color 0.15s ease;
}
.util-btn:hover { background: var(--p-surface-100, #f1f5f9); color: var(--p-text-color, #0f172a); }
.util-btn.active { background: var(--p-primary-50, #f0fdf4); color: var(--p-primary-color, #16a34a); }
.account-trigger {
  border: none;
  background: transparent;
  padding: 0;
  cursor: pointer;
  display: flex;
  align-items: center;
}
.account-avatar {
  background: var(--p-primary-color, #16a34a);
  color: #fff;
  font-weight: 700;
  font-size: 0.8rem;
}

/* --- Tier 2: sub-nav --- */
.subnav {
  flex-shrink: 0;
  background: var(--teggo-surface, #fff);
  border-bottom: 1px solid var(--p-surface-200, #e2e8f0);
}
.subnav-inner {
  display: flex;
  align-items: center;
  gap: 0.2rem;
  height: 48px;
  padding: 0 1.25rem;
  overflow-x: auto;
  scrollbar-width: none;
}
.subnav-inner::-webkit-scrollbar { display: none; }
.sub-divider {
  width: 1px;
  height: 20px;
  background: var(--p-surface-200, #e2e8f0);
  margin: 0 0.4rem;
  flex-shrink: 0;
}
.sub-link {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.4rem 0.7rem;
  border-radius: 8px;
  white-space: nowrap;
  text-decoration: none;
  color: var(--p-text-color, #475569);
  font-size: 0.85rem;
  font-weight: 600;
  transition: background-color 0.12s ease, color 0.12s ease;
}
.sub-link i {
  font-size: 0.85rem;
  color: var(--p-text-muted-color, #94a3b8);
  transition: color 0.12s ease;
}
.sub-link:hover { background: var(--p-surface-100, #f1f5f9); }
.sub-link.active { background: var(--p-primary-50, #f0fdf4); color: var(--p-primary-color, #16a34a); }
.sub-link.active i { color: var(--p-primary-color, #16a34a); }

/* --- Content --- */
.content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 1.5rem;
  background: var(--p-content-background, #f8fafc);
}

/* --- Account popover --- */
.account-card { width: 230px; display: flex; flex-direction: column; gap: 0.85rem; }
.account-head { display: flex; align-items: center; gap: 0.7rem; }
.account-id { min-width: 0; }
.account-email {
  font-weight: 600; font-size: 0.9rem;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.account-org { font-size: 0.78rem; color: var(--p-text-muted-color, #64748b); }
.account-meta {
  display: flex; align-items: center; gap: 0.45rem;
  font-size: 0.8rem; color: var(--p-text-muted-color, #64748b);
  padding-top: 0.6rem; border-top: 1px solid var(--teggo-border, #e2e8f0);
}
.account-signout { width: 100%; }

/* --- Mobile drawer (hidden on desktop) --- */
.hamburger {
  display: none;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1.3rem;
  color: var(--p-text-color, #1e293b);
  padding: 0.25rem 0.4rem;
  margin-right: -0.25rem;
}
.drawer { display: none; }
.scrim { display: none; }

@media (max-width: 1024px) {
  .hamburger { display: inline-flex; }
  /* The horizontal section list collapses into the drawer on small screens. */
  .primary { display: none; }
  .drawer {
    display: block;
    position: fixed;
    top: 0;
    left: 0;
    z-index: 60;
    width: 260px;
    height: 100%;
    background: var(--teggo-surface, #fff);
    border-right: 1px solid var(--p-surface-200, #e2e8f0);
    transform: translateX(-100%);
    transition: transform 0.2s ease;
    box-shadow: 0 0 40px rgba(0, 0, 0, 0.15);
    overflow-y: auto;
    padding: 0.75rem;
  }
  .drawer-open .drawer { transform: translateX(0); }
  .drawer-open .scrim {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 55;
    background: rgba(15, 23, 42, 0.45);
  }
  .drawer-nav { display: flex; flex-direction: column; gap: 2px; }
  .drawer-link {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.6rem 0.7rem;
    border-radius: 8px;
    text-decoration: none;
    color: var(--p-text-color, #334155);
    font-size: 0.9rem;
    font-weight: 600;
  }
  .drawer-link i { color: var(--p-text-muted-color, #94a3b8); width: 1.15rem; text-align: center; }
  .drawer-link:hover { background: var(--p-surface-100, #f1f5f9); }
  .drawer-link.active { background: var(--p-primary-50, #f0fdf4); color: var(--p-primary-color, #16a34a); }
  .drawer-link.active i { color: var(--p-primary-color, #16a34a); }
}
</style>
