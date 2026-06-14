<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()

// Screenshots are captured by the Playwright script (pnpm --filter @teggo/admin
// screenshots). Until they exist, show a tasteful placeholder instead of a
// broken-image icon.
const broken = reactive(new Set<string>())

// ---- Request-a-demo flow: provision an instant demo org and log straight in ---
const showDemo = ref(false)
const submitting = ref(false)
const error = ref('')
const form = reactive({ name: '', email: '', company: '' })

function openDemo() {
  error.value = ''
  showDemo.value = true
}

async function requestDemo() {
  error.value = ''
  if (!form.email.trim() || !form.email.includes('@')) {
    error.value = 'A valid work email is required.'
    return
  }
  submitting.value = true
  const { data, error: e } = await api.POST('/demo', {
    body: { name: form.name.trim(), email: form.email.trim(), company: form.company.trim() },
  })
  submitting.value = false
  if (e || !data) {
    error.value = errMessage(e, 'Could not start your demo — please try again.')
    return
  }
  // Auto-login into the freshly provisioned, pre-seeded demo org.
  auth.setToken(data.token)
  auth.setEmail(data.email)
  router.push('/')
}

const rows = [
  {
    eyebrow: 'The whole B2B motion',
    title: 'Catalog → quote → order → invoice → cash, in one place',
    body: 'Per-customer pricing, company accounts with approvals and budgets, RFQ/quote negotiation, multi-warehouse fulfilment and order-to-cash — the B2B realities built in, not bolted on.',
    points: ['Read-time per-customer & contract pricing', 'RFQ → quote → order → invoice → payment', 'Approvals, budgets and credit limits'],
    img: '/landing/dashboard.png',
    alt: 'Teggo admin dashboard',
  },
  {
    eyebrow: 'AI woven into the workflow',
    title: 'A weekly executive briefing that writes itself',
    body: 'Not a chatbot you have to ask — an engine that runs on a schedule, computes your metrics, detects anomalies with explainable rules, and writes the briefing with recommended actions that deep-link straight into the work.',
    points: ['Revenue, margin, AR aging, churn-risk & concentration', 'AI-authored narrative (with a deterministic fallback)', 'Recommended actions, one click from the fix'],
    img: '/landing/insights.png',
    alt: 'Teggo executive insights & briefings',
    reversed: true,
  },
  {
    eyebrow: 'Decisions backed by data',
    title: 'Live dashboards, a report builder and one-click exports',
    body: 'Data is the modern oil — funding, budgeting and planning all run on it. Live dashboards, a safe custom report builder, gross-margin analytics, and full-record CSV / Excel exports straight into your finance and BI tools.',
    points: ['Live aggregates — no overnight lag', 'Gross-margin & profitability', 'CSV / Excel exports + a complete audit trail'],
    img: '/landing/analytics.png',
    alt: 'Teggo analytics & reporting',
  },
]

const caps = [
  { icon: 'pi pi-shopping-cart', title: 'Order-to-cash', body: 'Carts, orders, approvals, invoicing and payments — end to end.' },
  { icon: 'pi pi-dollar', title: 'Catalog & pricing', body: 'Products, categories, attributes and per-customer price lists.' },
  { icon: 'pi pi-users', title: 'CRM & accounts', body: 'Companies, buyers, roles, credit limits and account health.' },
  { icon: 'pi pi-chart-bar', title: 'Insights & audit', body: 'AI briefings, live dashboards, exports and a full audit trail.' },
  { icon: 'pi pi-shop', title: 'Marketplace', body: 'Vendor onboarding, product approval, order splitting and payouts.' },
  { icon: 'pi pi-sparkles', title: 'AI assistant', body: 'Answers across orders, catalog, customers and stock — under your permissions.' },
]

const stats = [
  { k: '1', v: 'platform — commerce, CRM & workflow' },
  { k: '7d', v: 'instant demo, isolated & pre-seeded' },
  { k: '100%', v: 'your data, multi-tenant & RLS-isolated' },
  { k: 'AI', v: 'woven into the weekly workflow' },
]
</script>

<template>
  <div class="landing">
    <header class="topbar">
      <div class="brand">
        <span class="brand-badge"><i class="pi pi-bolt" /></span>
        <span class="brand-name">Teggo</span>
      </div>
      <nav class="topnav">
        <RouterLink to="/login" class="signin">Sign in</RouterLink>
        <Button label="Request a demo" icon="pi pi-arrow-right" icon-pos="right" @click="openDemo" />
      </nav>
    </header>

    <section class="hero">
      <span class="eyebrow">B2B commerce, CRM &amp; workflow — one platform</span>
      <h1 class="hero-title">The commerce platform your buyers and your team actually want.</h1>
      <p class="hero-sub">
        Teggo runs the entire B2B motion — catalog, quoting, orders, fulfilment, invoicing and cash —
        with per-customer pricing, approvals and an AI insights engine built in. See it with your own
        instant demo, pre-loaded with real data.
      </p>
      <div class="cta-row">
        <Button label="Request a demo" size="large" icon="pi pi-arrow-right" icon-pos="right" @click="openDemo" />
        <RouterLink to="/login"><Button label="Sign in" size="large" severity="secondary" outlined /></RouterLink>
      </div>
      <div class="browser hero-shot">
        <div class="browser-bar"><span /><span /><span /><div class="browser-url">app.teggo.example</div></div>
        <img v-show="!broken.has('/landing/dashboard.png')" src="/landing/dashboard.png" alt="Teggo admin dashboard" loading="lazy" @error="broken.add('/landing/dashboard.png')" />
        <div v-if="broken.has('/landing/dashboard.png')" class="shot-ph"><i class="pi pi-chart-bar" /><span>Dashboard</span></div>
      </div>
    </section>

    <section v-for="r in rows" :key="r.title" class="row" :class="{ reversed: r.reversed }">
      <div class="row-copy">
        <span class="row-eyebrow">{{ r.eyebrow }}</span>
        <h2 class="row-title">{{ r.title }}</h2>
        <p class="row-body">{{ r.body }}</p>
        <ul class="checks">
          <li v-for="p in r.points" :key="p"><i class="pi pi-check" /> {{ p }}</li>
        </ul>
        <Button label="Request a demo" icon="pi pi-arrow-right" icon-pos="right" text @click="openDemo" />
      </div>
      <div class="row-visual">
        <div class="browser">
          <div class="browser-bar"><span /><span /><span /><div class="browser-url">app.teggo.example</div></div>
          <img v-show="!broken.has(r.img)" :src="r.img" :alt="r.alt" loading="lazy" @error="broken.add(r.img)" />
          <div v-if="broken.has(r.img)" class="shot-ph"><i class="pi pi-image" /><span>{{ r.alt }}</span></div>
        </div>
      </div>
    </section>

    <section class="caps">
      <h2 class="section-title">The whole platform, not a stack of integrations</h2>
      <div class="cap-grid">
        <div v-for="c in caps" :key="c.title" class="cap">
          <span class="cap-icon"><i :class="c.icon" /></span>
          <h3>{{ c.title }}</h3>
          <p>{{ c.body }}</p>
        </div>
      </div>
    </section>

    <section class="stats">
      <div v-for="s in stats" :key="s.v" class="stat">
        <div class="stat-k">{{ s.k }}</div>
        <div class="stat-v">{{ s.v }}</div>
      </div>
    </section>

    <section class="final">
      <h2>See the full system in your own demo.</h2>
      <p>We spin up an isolated, pre-seeded demo organization and log you straight in — no setup, no sales call required.</p>
      <Button label="Request a demo" size="large" icon="pi pi-arrow-right" icon-pos="right" @click="openDemo" />
    </section>

    <footer class="foot">
      <span>© {{ new Date().getFullYear() }} Teggo — B2B commerce platform.</span>
      <RouterLink to="/login" class="signin">Sign in</RouterLink>
    </footer>

    <Dialog v-model:visible="showDemo" modal header="Request your instant demo" :style="{ width: '440px' }">
      <p class="demo-lede">We'll provision a private, pre-seeded demo organization and sign you in right away.</p>
      <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
      <form class="demo-form" @submit.prevent="requestDemo">
        <div class="field">
          <label>Work email</label>
          <InputText v-model="form.email" type="email" placeholder="you@company.com" fluid autofocus />
        </div>
        <div class="field">
          <label>Name <span class="opt">(optional)</span></label>
          <InputText v-model="form.name" placeholder="Your name" fluid />
        </div>
        <div class="field">
          <label>Company <span class="opt">(optional)</span></label>
          <InputText v-model="form.company" placeholder="Company name" fluid />
        </div>
      </form>
      <template #footer>
        <Button label="Cancel" severity="secondary" text :disabled="submitting" @click="showDemo = false" />
        <Button label="Start my demo" icon="pi pi-arrow-right" icon-pos="right" :loading="submitting" @click="requestDemo" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.landing {
  --green: #16a34a;
  --ink: #0f172a;
  --muted: #64748b;
  color: var(--ink);
  background: #fff;
  min-height: 100vh;
  font-family: 'Open Sans', system-ui, sans-serif;
}
.topbar {
  display: flex; align-items: center; justify-content: space-between;
  max-width: 1100px; margin: 0 auto; padding: 1rem 1.5rem;
}
.brand { display: flex; align-items: center; gap: 0.55rem; }
.brand-badge {
  display: inline-flex; align-items: center; justify-content: center;
  width: 30px; height: 30px; border-radius: 8px; background: var(--green); color: #fff;
}
.brand-name { font-weight: 800; font-size: 1.15rem; letter-spacing: -0.01em; }
.topnav { display: flex; align-items: center; gap: 1.1rem; }
.signin { color: var(--ink); font-weight: 600; text-decoration: none; }
.signin:hover { color: var(--green); }

.hero {
  max-width: 880px; margin: 0 auto; padding: 3rem 1.5rem 2rem; text-align: center;
}
.eyebrow {
  display: inline-block; font-size: 0.8rem; font-weight: 700; letter-spacing: 0.04em;
  text-transform: uppercase; color: var(--green); background: #f0fdf4;
  padding: 0.3rem 0.7rem; border-radius: 999px; margin-bottom: 1rem;
}
.hero-title { font-size: clamp(1.9rem, 4vw, 2.9rem); font-weight: 800; line-height: 1.1; letter-spacing: -0.02em; margin: 0 0 1rem; }
.hero-sub { font-size: 1.1rem; line-height: 1.65; color: var(--muted); max-width: 44rem; margin: 0 auto 1.6rem; }
.cta-row { display: flex; gap: 0.75rem; justify-content: center; flex-wrap: wrap; margin-bottom: 2.5rem; }

.browser {
  border: 1px solid #e2e8f0; border-radius: 12px; overflow: hidden;
  box-shadow: 0 24px 60px -28px rgba(15, 23, 42, 0.4); background: #fff;
}
.browser-bar { display: flex; align-items: center; gap: 6px; padding: 9px 12px; background: #f1f5f9; border-bottom: 1px solid #e2e8f0; }
.browser-bar > span { width: 10px; height: 10px; border-radius: 50%; background: #cbd5e1; }
.browser-url { margin-left: 10px; font-size: 0.75rem; color: #94a3b8; background: #fff; border-radius: 6px; padding: 2px 10px; }
.browser img { display: block; width: 100%; height: auto; }
.shot-ph {
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 0.5rem;
  aspect-ratio: 16 / 10; background: linear-gradient(135deg, #f0fdf4, #ecfeff);
  color: var(--green); font-weight: 600;
}
.shot-ph .pi { font-size: 2rem; opacity: 0.8; }
.hero-shot { max-width: 980px; margin: 0 auto; }

.row {
  display: grid; grid-template-columns: 1fr 1fr; gap: 3rem; align-items: center;
  max-width: 1100px; margin: 0 auto; padding: 3.5rem 1.5rem;
}
.row.reversed .row-copy { order: 2; }
.row-eyebrow { font-size: 0.78rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.04em; color: var(--green); }
.row-title { font-size: 1.7rem; font-weight: 800; line-height: 1.15; letter-spacing: -0.01em; margin: 0.5rem 0 0.8rem; }
.row-body { color: var(--muted); line-height: 1.65; margin: 0 0 1rem; }
.checks { list-style: none; padding: 0; margin: 0 0 1.2rem; display: flex; flex-direction: column; gap: 0.5rem; }
.checks li { display: flex; align-items: center; gap: 0.55rem; font-weight: 500; }
.checks .pi-check { color: var(--green); font-size: 0.8rem; }

.caps { max-width: 1100px; margin: 0 auto; padding: 2.5rem 1.5rem; }
.section-title { text-align: center; font-size: 1.7rem; font-weight: 800; letter-spacing: -0.01em; margin: 0 0 2rem; }
.cap-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1.2rem; }
.cap { border: 1px solid #e2e8f0; border-radius: 12px; padding: 1.4rem; }
.cap-icon { display: inline-flex; align-items: center; justify-content: center; width: 40px; height: 40px; border-radius: 10px; background: #f0fdf4; color: var(--green); font-size: 1.1rem; margin-bottom: 0.8rem; }
.cap h3 { margin: 0 0 0.35rem; font-size: 1.05rem; }
.cap p { margin: 0; color: var(--muted); line-height: 1.55; font-size: 0.92rem; }

.stats {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem;
  max-width: 1100px; margin: 1.5rem auto; padding: 2rem 1.5rem;
  border-top: 1px solid #e2e8f0; border-bottom: 1px solid #e2e8f0;
}
.stat { text-align: center; }
.stat-k { font-size: 2rem; font-weight: 800; color: var(--green); }
.stat-v { color: var(--muted); font-size: 0.92rem; }

.final { text-align: center; max-width: 720px; margin: 0 auto; padding: 3.5rem 1.5rem; }
.final h2 { font-size: 1.9rem; font-weight: 800; letter-spacing: -0.01em; margin: 0 0 0.6rem; }
.final p { color: var(--muted); line-height: 1.65; margin: 0 0 1.4rem; }

.foot {
  display: flex; align-items: center; justify-content: space-between;
  max-width: 1100px; margin: 0 auto; padding: 1.5rem; border-top: 1px solid #e2e8f0;
  color: var(--muted); font-size: 0.88rem;
}

.demo-lede { color: var(--muted); margin: 0 0 1rem; line-height: 1.55; }
.demo-form { display: flex; flex-direction: column; gap: 0.85rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.82rem; font-weight: 600; }
.field .opt { color: var(--muted); font-weight: 400; }
.mb { margin-bottom: 0.8rem; }

@media (max-width: 760px) {
  .row { grid-template-columns: 1fr; gap: 1.5rem; }
  .row.reversed .row-copy { order: 0; }
}
</style>
