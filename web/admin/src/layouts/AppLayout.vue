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

// Mobile sidebar drawer (small screens only). Closes on navigation.
const mobileOpen = ref(false)
watch(() => route.fullPath, () => { mobileOpen.value = false })

const can = (p: string) => auth.can(p)
const allows = (f: string) => billing.allows(f)

// Tier 1 — sidebar: every section the user/plan can see. Assistant is pulled
// out as a utility button (a distinct AI entry point), so it's excluded here.
const sideSections = computed(() =>
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

// The active section — drives both the sidebar highlight and the topbar label.
const activeSection = computed<NavSection | null>(() => {
  const k = ctx.value?.key
  return k ? sectionByKey(k) ?? null : null
})

// Tier 2 — sub-nav: the current section's leaves (only when it has any).
const subNavSection = computed<NavSection | null>(() => {
  const s = activeSection.value
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

    <!-- Tier 1: sidebar of sections -->
    <aside class="sidebar">
      <RouterLink :to="{ name: 'dashboard' }" class="brand">
        <span class="brand-badge"><i class="pi pi-bolt" /></span>
        <span class="brand-name">Teggo<span class="brand-sub">Admin</span></span>
      </RouterLink>
      <nav class="nav-scroll">
        <ul class="menu">
          <li v-for="s in sideSections" :key="s.key" class="menu-item">
            <RouterLink :to="sectionTo(s)" class="menu-link" :class="{ active: ctx?.key === s.key }">
              <i :class="s.icon" class="menu-icon" />
              <span class="menu-text">{{ s.label }}</span>
            </RouterLink>
          </li>
          <li v-if="assistant" class="menu-item menu-item--pinned">
            <RouterLink :to="{ name: 'assistant' }" class="menu-link" :class="{ active: ctx?.key === 'assistant' }">
              <i class="pi pi-sparkles menu-icon" />
              <span class="menu-text">{{ assistant.label }}</span>
            </RouterLink>
          </li>
        </ul>
      </nav>
    </aside>

    <div class="main">
      <!-- Top bar: page context on the left, account utilities on the right -->
      <header class="topbar">
        <button type="button" class="hamburger" aria-label="Menu" @click="mobileOpen = !mobileOpen">
          <i class="pi pi-bars" />
        </button>
        <div v-if="activeSection" class="topbar-context">
          <i :class="activeSection.icon" />
          <span>{{ activeSection.label }}</span>
        </div>
        <span class="spacer" />
        <NotificationBell />
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

      <main class="content">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  height: 100vh;
  height: 100dvh;
  overflow: hidden;
}

/* --- Tier 1: sidebar --- */
.sidebar {
  /* Compact icon-over-label rail on large screens — reclaims working space for
     content. The mobile drawer widens back out (see the breakpoint below). */
  width: 108px;
  flex-shrink: 0;
  height: 100%;
  /* Brand indigo rail (deep gradient); nav text/icons go light for contrast. */
  background: linear-gradient(180deg, var(--p-primary-700, #4338ca) 0%, var(--p-primary-900, #312e81) 100%);
  display: flex;
  flex-direction: column;
}
.brand {
  display: flex;
  align-items: center;
  justify-content: center; /* rail: just the badge, centred */
  gap: 0.6rem;
  height: 58px; /* match the topbar so the header line is flush */
  padding: 0 0.5rem;
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
  background: #fff;
  color: var(--p-primary-color, #6366f1);
  font-size: 0.95rem;
  flex-shrink: 0;
}
.brand-name {
  display: none; /* hidden on the rail; shown again in the mobile drawer */
  font-weight: 800;
  font-size: 1.05rem;
  letter-spacing: -0.01em;
  color: #fff;
}
.brand-sub {
  margin-left: 0.35rem;
  font-weight: 500;
  font-size: 0.82rem;
  color: rgba(255, 255, 255, 0.65);
}
.nav-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.25) transparent;
  padding: 0.6rem 0 1rem;
}
.nav-scroll::-webkit-scrollbar { width: 8px; }
.nav-scroll::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.25); border-radius: 8px; }
.menu {
  list-style: none;
  margin: 0;
  padding: 0 0.4rem;
  display: flex;
  flex-direction: column;
  height: 100%;
}
.menu-item { margin-bottom: 4px; }
/* Assistant pinned to the foot of the rail. */
.menu-item--pinned { margin-top: auto; }
/* Rail item: icon stacked over its label, centred. */
.menu-link {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.3rem;
  padding: 0.55rem 0.3rem;
  border-radius: 10px;
  text-decoration: none;
  color: rgba(255, 255, 255, 0.78);
  font-size: 0.7rem;
  font-weight: 600;
  line-height: 1.2;
  text-align: center;
}
.menu-icon {
  font-size: 1.2rem;
  color: var(--teggo-accent, #f59e0b);
  flex-shrink: 0;
}
.menu-text { display: block; width: 100%; }
/* Active marker on the indigo rail: a vertical amber accent pill flush to the
   left edge, plus white icon + label — no cushion. */
.menu-link.active { color: #fff; }
.menu-link.active .menu-icon { color: #fff; }
.menu-link.active::before {
  content: '';
  position: absolute;
  left: -0.4rem; /* cancels .menu's 0.4rem inset → sits on the rail edge */
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 26px;
  border-radius: 0 3px 3px 0;
  background: var(--teggo-accent, #f59e0b);
}

/* --- Main column --- */
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  height: 100%;
  overflow: hidden;
}

/* --- Top bar --- */
.topbar {
  display: flex;
  align-items: center;
  height: 58px;
  flex-shrink: 0;
  padding: 0 1.25rem;
  background: var(--teggo-surface, #fff);
  border-bottom: 1px solid var(--p-surface-200, #e2e8f0);
}
.topbar-context {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 700;
  font-size: 0.95rem;
  letter-spacing: -0.01em;
  color: var(--p-text-color, #0f172a);
}
.topbar-context i { color: var(--p-text-color, #334155); font-size: 0.95rem; }
.spacer { flex: 1; }
/* NotificationBell is a multi-root component, so a class on it doesn't fall
   through — space the cluster from the account button (a real element) instead. */
.account-trigger {
  border: none;
  background: transparent;
  padding: 0;
  margin-left: 0.9rem;
  cursor: pointer;
  display: flex;
  align-items: center;
}
.account-avatar {
  background: var(--p-primary-color, #6366f1);
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
.sub-link.active { background: var(--p-primary-50, #eef2ff); color: var(--p-primary-color, #6366f1); }
.sub-link.active i { color: var(--p-primary-color, #6366f1); }

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

/* --- Mobile: the sidebar becomes an off-canvas drawer --- */
.hamburger {
  display: none;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1.3rem;
  color: var(--p-text-color, #1e293b);
  padding: 0.25rem 0.5rem;
  margin-right: 0.5rem;
}
.scrim { display: none; }

@media (max-width: 1024px) {
  .hamburger { display: inline-flex; }
  .sidebar {
    width: 264px;
    position: fixed;
    top: 0;
    left: 0;
    z-index: 60;
    transform: translateX(-100%);
    transition: transform 0.2s ease;
    box-shadow: 0 0 40px rgba(0, 0, 0, 0.15);
  }
  /* The drawer has room for the comfortable icon-beside-label layout. */
  .brand { justify-content: flex-start; padding: 0 1.1rem; }
  .brand-name { display: inline; }
  .menu { padding: 0 0.75rem; }
  .menu-link {
    flex-direction: row;
    align-items: center;
    justify-content: flex-start;
    gap: 0.7rem;
    padding: 0.6rem 0.7rem;
    font-size: 0.9rem;
    text-align: left;
  }
  .menu-icon { font-size: 1rem; width: 1.2rem; text-align: center; }
  .menu-text { width: auto; flex: 1; min-width: 0; }
  /* Keep the active pill flush to the drawer edge (wider .menu inset here). */
  .menu-link.active::before { left: -0.75rem; height: 22px; }
  .drawer-open .sidebar { transform: translateX(0); }
  .drawer-open .scrim {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 55;
    background: rgba(15, 23, 42, 0.45);
  }
}

/* Phones: tighten the chrome so content gets the space it needs. */
@media (max-width: 640px) {
  .content { padding: 1rem; }
  .topbar { padding: 0 1rem; }
  .subnav-inner { padding: 0 1rem; }
}
</style>
