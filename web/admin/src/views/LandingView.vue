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

// Reveal-on-scroll: a featherweight local directive. Elements fade + rise into
// view once, then stop being observed. Honours reduced-motion and any
// environment without IntersectionObserver by showing instantly.
const vReveal = {
  mounted(el: HTMLElement) {
    const noMotion =
      typeof IntersectionObserver === 'undefined' ||
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
    if (noMotion) {
      el.classList.add('is-visible')
      return
    }
    el.classList.add('reveal')
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) {
            e.target.classList.add('is-visible')
            io.unobserve(e.target)
          }
        }
      },
      { threshold: 0.12, rootMargin: '0px 0px -48px 0px' },
    )
    io.observe(el)
  },
}

// Scroll parallax: drifts a layer at a fraction of scroll speed for depth.
// Writes a `--py` CSS variable (so it composes with any existing transform on
// the element); throttled with rAF; a no-op under reduced-motion.
type ParaEl = HTMLElement & { __para?: () => void }
const vParallax = {
  mounted(el: ParaEl, binding: { value?: number }) {
    if (
      typeof IntersectionObserver === 'undefined' ||
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
    )
      return
    const speed = binding.value ?? 0.08
    let ticking = false
    let visible = false
    const apply = () => {
      ticking = false
      const rect = el.getBoundingClientRect()
      const fromCentre = rect.top + rect.height / 2 - window.innerHeight / 2
      el.style.setProperty('--py', `${(-fromCentre * speed).toFixed(1)}px`)
    }
    const onScroll = () => {
      if (!ticking && visible) {
        ticking = true
        requestAnimationFrame(apply)
      }
    }
    // Only listen while the layer is on (or near) screen.
    const io = new IntersectionObserver(
      ([e]) => {
        visible = e.isIntersecting
        if (visible) apply()
      },
      { rootMargin: '200px 0px 200px 0px' },
    )
    io.observe(el)
    apply()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll, { passive: true })
    el.__para = () => {
      io.disconnect()
      window.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', onScroll)
    }
  },
  unmounted(el: ParaEl) {
    el.__para?.()
  },
}

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
      <div class="topbar-inner">
        <div class="brand">
          <span class="brand-badge"><i class="pi pi-bolt" /></span>
          <span class="brand-name">Teggo</span>
        </div>
        <nav class="topnav">
          <RouterLink to="/login" class="signin">Sign in</RouterLink>
          <Button label="Request a demo" icon="pi pi-arrow-right" icon-pos="right" rounded @click="openDemo" />
        </nav>
      </div>
    </header>

    <section class="hero">
      <div v-parallax="0.06" class="hero-glow" aria-hidden="true" />
      <div class="hero-inner">
        <span v-reveal class="eyebrow">B2B commerce, CRM &amp; workflow — one platform</span>
        <h1 v-reveal class="hero-title" style="--delay: 0.05s">
          The commerce platform your buyers<br class="brk" /> and your team actually want.
        </h1>
        <p v-reveal class="hero-sub" style="--delay: 0.1s">
          Teggo runs the entire B2B motion — catalog, quoting, orders, fulfilment, invoicing and cash —
          with per-customer pricing, approvals and an AI insights engine built in. See it with your own
          instant demo, pre-loaded with real data.
        </p>
        <div v-reveal class="cta-row" style="--delay: 0.15s">
          <Button label="Request a demo" size="large" icon="pi pi-arrow-right" icon-pos="right" rounded @click="openDemo" />
          <RouterLink to="/login"><Button label="Sign in" size="large" severity="secondary" outlined rounded /></RouterLink>
        </div>
        <p v-reveal class="hero-note" style="--delay: 0.2s">
          <i class="pi pi-check-circle" /> No setup. No sales call. Live in seconds.
        </p>
        <div v-reveal class="browser hero-shot" style="--delay: 0.25s">
          <div class="browser-bar"><span /><span /><span /><div class="browser-url">app.teggo.example</div></div>
          <img v-show="!broken.has('/landing/dashboard.png')" src="/landing/dashboard.png" alt="Teggo admin dashboard" loading="lazy" @error="broken.add('/landing/dashboard.png')" />
          <div v-if="broken.has('/landing/dashboard.png')" class="shot-ph"><i class="pi pi-chart-bar" /><span>Dashboard</span></div>
        </div>
      </div>
    </section>

    <section class="rows">
      <div v-for="r in rows" :key="r.title" v-reveal class="row" :class="{ reversed: r.reversed }">
        <div class="row-copy">
          <span class="row-eyebrow">{{ r.eyebrow }}</span>
          <h2 class="row-title">{{ r.title }}</h2>
          <p class="row-body">{{ r.body }}</p>
          <ul class="checks">
            <li v-for="p in r.points" :key="p"><i class="pi pi-check" /> {{ p }}</li>
          </ul>
          <Button label="Request a demo" icon="pi pi-arrow-right" icon-pos="right" text @click="openDemo" />
        </div>
        <div v-parallax="0.05" class="row-visual">
          <div class="browser">
            <div class="browser-bar"><span /><span /><span /><div class="browser-url">app.teggo.example</div></div>
            <img v-show="!broken.has(r.img)" :src="r.img" :alt="r.alt" loading="lazy" @error="broken.add(r.img)" />
            <div v-if="broken.has(r.img)" class="shot-ph"><i class="pi pi-image" /><span>{{ r.alt }}</span></div>
          </div>
        </div>
      </div>
    </section>

    <section class="caps">
      <div v-parallax="0.13" class="caps-blob caps-blob-a" aria-hidden="true" />
      <div v-parallax="-0.1" class="caps-blob caps-blob-b" aria-hidden="true" />
      <div class="caps-inner">
        <h2 v-reveal class="section-title">The whole platform, not a stack of integrations</h2>
        <p v-reveal class="section-sub" style="--delay: 0.05s">One login, one data model, one bill — every part of the B2B motion already talking to the others.</p>
        <div class="cap-grid">
          <div v-for="(c, i) in caps" :key="c.title" v-reveal class="cap" :style="{ '--delay': `${i * 0.05}s` }">
            <span class="cap-icon"><i :class="c.icon" /></span>
            <h3>{{ c.title }}</h3>
            <p>{{ c.body }}</p>
          </div>
        </div>
      </div>
    </section>

    <section class="stats">
      <div v-reveal class="stats-panel">
        <div v-for="s in stats" :key="s.v" class="stat">
          <div class="stat-k">{{ s.k }}</div>
          <div class="stat-v">{{ s.v }}</div>
        </div>
      </div>
    </section>

    <section class="final">
      <div v-reveal class="final-inner">
        <h2>See the full system in your own demo.</h2>
        <p>We spin up an isolated, pre-seeded demo organization and log you straight in — no setup, no sales call required.</p>
        <Button label="Request a demo" size="large" icon="pi pi-arrow-right" icon-pos="right" rounded @click="openDemo" />
      </div>
    </section>

    <footer class="foot">
      <div class="foot-inner">
        <div class="brand">
          <span class="brand-badge"><i class="pi pi-bolt" /></span>
          <span class="brand-name">Teggo</span>
        </div>
        <span class="foot-copy">© {{ new Date().getFullYear() }} Teggo — B2B commerce platform.</span>
        <RouterLink to="/login" class="signin">Sign in</RouterLink>
      </div>
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
  --green-700: #15803d;
  --green-50: #f0fdf4;
  --ink: #0f172a;
  --ink-soft: #334155;
  --muted: #64748b;
  --line: #eaeef4;
  --tint: #f7faf9;
  --maxw: 1120px;
  color: var(--ink);
  background: #fff;
  min-height: 100vh;
  font-family: 'Open Sans', system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
}

/* ---- Reveal-on-scroll ---- */
.reveal {
  opacity: 0;
  transform: translateY(18px);
  transition:
    opacity 0.7s cubic-bezier(0.16, 1, 0.3, 1),
    transform 0.7s cubic-bezier(0.16, 1, 0.3, 1);
  transition-delay: var(--delay, 0s);
  will-change: opacity, transform;
}
.reveal.is-visible {
  opacity: 1;
  transform: none;
}

/* ---- Top bar (sticky, quietly translucent) ---- */
.topbar {
  position: sticky;
  top: 0;
  z-index: 20;
  background: rgba(255, 255, 255, 0.78);
  backdrop-filter: saturate(150%) blur(12px);
  border-bottom: 1px solid transparent;
  transition: border-color 0.3s ease;
}
.topbar-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  max-width: var(--maxw);
  margin: 0 auto;
  padding: 0.9rem 1.5rem;
}
.brand { display: flex; align-items: center; gap: 0.55rem; }
.brand-badge {
  display: inline-flex; align-items: center; justify-content: center;
  width: 30px; height: 30px; border-radius: 9px;
  background: linear-gradient(135deg, #22c55e, var(--green-700));
  color: #fff;
  box-shadow: 0 4px 12px -4px rgba(22, 163, 74, 0.6);
}
.brand-name { font-weight: 800; font-size: 1.15rem; letter-spacing: -0.01em; }
.topnav { display: flex; align-items: center; gap: 1.25rem; }
.signin { color: var(--ink-soft); font-weight: 600; text-decoration: none; transition: color 0.2s ease; }
.signin:hover { color: var(--green); }

/* ---- Hero ---- */
.hero { position: relative; overflow: hidden; }
.hero-glow {
  position: absolute;
  top: -22%;
  left: 50%;
  transform: translateX(-50%) translateY(var(--py, 0));
  width: min(1100px, 130%);
  height: 620px;
  background:
    radial-gradient(closest-side, rgba(34, 197, 94, 0.16), transparent 70%),
    radial-gradient(closest-side, rgba(56, 189, 248, 0.1), transparent 70%);
  background-position: 30% 0, 75% 30%;
  background-repeat: no-repeat;
  background-size: 60% 100%, 55% 90%;
  filter: blur(8px);
  pointer-events: none;
}
.hero-inner {
  position: relative;
  max-width: 920px;
  margin: 0 auto;
  padding: 4rem 1.5rem 2.5rem;
  text-align: center;
}
.eyebrow {
  display: inline-block; font-size: 0.78rem; font-weight: 700; letter-spacing: 0.04em;
  text-transform: uppercase; color: var(--green-700); background: var(--green-50);
  padding: 0.34rem 0.8rem; border-radius: 999px; margin-bottom: 1.25rem;
  border: 1px solid rgba(22, 163, 74, 0.12);
}
.hero-title { font-size: clamp(2rem, 4.4vw, 3.1rem); font-weight: 800; line-height: 1.08; letter-spacing: -0.025em; margin: 0 0 1.1rem; }
.hero-sub { font-size: 1.12rem; line-height: 1.65; color: var(--muted); max-width: 42rem; margin: 0 auto 1.8rem; }
.cta-row { display: flex; gap: 0.75rem; justify-content: center; flex-wrap: wrap; margin-bottom: 0.9rem; }
.hero-note { font-size: 0.9rem; color: var(--muted); margin: 0 0 2.75rem; display: inline-flex; align-items: center; gap: 0.45rem; }
.hero-note .pi { color: var(--green); }
.brk { display: none; }

/* ---- Browser frame ---- */
.browser {
  border: 1px solid var(--line); border-radius: 14px; overflow: hidden;
  box-shadow: 0 30px 70px -32px rgba(15, 23, 42, 0.45);
  background: #fff;
}
.browser-bar { display: flex; align-items: center; gap: 6px; padding: 9px 13px; background: #f5f7fa; border-bottom: 1px solid var(--line); }
.browser-bar > span { width: 10px; height: 10px; border-radius: 50%; background: #d4dae2; }
.browser-url { margin-left: 10px; font-size: 0.74rem; color: #97a3b4; background: #fff; border-radius: 6px; padding: 3px 11px; }
.browser img { display: block; width: 100%; height: auto; }
.shot-ph {
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 0.5rem;
  aspect-ratio: 16 / 10; background: linear-gradient(135deg, var(--green-50), #ecfeff);
  color: var(--green); font-weight: 600;
}
.shot-ph .pi { font-size: 2rem; opacity: 0.8; }
.hero-shot { max-width: 1000px; margin: 0 auto; }

/* gentle float, only when motion is welcome — applied to an inner element so it
   never fights the reveal transform on .hero-shot itself. */
@media (prefers-reduced-motion: no-preference) {
  .hero-shot.is-visible { animation: float 8s ease-in-out 0.9s infinite; }
  @keyframes float {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-9px); }
  }
}

/* ---- Feature rows ---- */
.rows { max-width: var(--maxw); margin: 0 auto; }
.row-visual { transform: translate3d(0, var(--py, 0), 0); will-change: transform; }
.row {
  display: grid; grid-template-columns: 1fr 1.1fr; gap: 3.5rem; align-items: center;
  padding: 4.5rem 1.5rem;
}
.row + .row { border-top: 1px solid var(--line); }
.row.reversed .row-copy { order: 2; }
.row-eyebrow { font-size: 0.76rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--green-700); }
.row-title { font-size: clamp(1.5rem, 2.6vw, 1.85rem); font-weight: 800; line-height: 1.18; letter-spacing: -0.02em; margin: 0.55rem 0 0.85rem; }
.row-body { color: var(--muted); line-height: 1.68; margin: 0 0 1.2rem; }
.checks { list-style: none; padding: 0; margin: 0 0 1.4rem; display: flex; flex-direction: column; gap: 0.6rem; }
.checks li { display: flex; align-items: center; gap: 0.6rem; font-weight: 500; color: var(--ink-soft); }
.checks .pi-check {
  color: var(--green); font-size: 0.7rem;
  background: var(--green-50); border-radius: 50%; width: 1.15rem; height: 1.15rem;
  display: inline-flex; align-items: center; justify-content: center; flex: none;
}

/* ---- Capabilities ---- */
.caps {
  position: relative;
  overflow: hidden;
  background: var(--tint);
  border-top: 1px solid var(--line);
  border-bottom: 1px solid var(--line);
}
.caps-blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(28px);
  pointer-events: none;
  transform: translate3d(0, var(--py, 0), 0);
  will-change: transform;
}
.caps-blob-a {
  top: -90px; left: -60px; width: 320px; height: 320px;
  background: radial-gradient(closest-side, rgba(34, 197, 94, 0.18), transparent 70%);
}
.caps-blob-b {
  bottom: -120px; right: -50px; width: 360px; height: 360px;
  background: radial-gradient(closest-side, rgba(56, 189, 248, 0.16), transparent 70%);
}
.caps-inner { position: relative; max-width: var(--maxw); margin: 0 auto; padding: 4.5rem 1.5rem; }
.section-title { text-align: center; font-size: clamp(1.55rem, 2.8vw, 1.95rem); font-weight: 800; letter-spacing: -0.02em; margin: 0 0 0.6rem; }
.section-sub { text-align: center; color: var(--muted); max-width: 36rem; margin: 0 auto 2.4rem; line-height: 1.6; }
.cap-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(270px, 1fr)); gap: 1.1rem; }
.cap {
  background: #fff; border: 1px solid var(--line); padding: 1.5rem;
  /* 3 sharp edges, only the top-right corner curves. */
  border-radius: 0 24px 0 0;
}
.cap-icon {
  display: inline-flex; align-items: center; justify-content: center; width: 42px; height: 42px;
  border-radius: 11px; background: var(--green-50); color: var(--green-700); font-size: 1.15rem; margin-bottom: 0.9rem;
}
.cap h3 { margin: 0 0 0.4rem; font-size: 1.05rem; }
.cap p { margin: 0; color: var(--muted); line-height: 1.58; font-size: 0.92rem; }

/* ---- Stats ---- */
.stats { max-width: var(--maxw); margin: 0 auto; padding: 4rem 1.5rem; }
.stats-panel {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1.5rem 1rem;
  padding: 2.5rem 2rem;
  background: linear-gradient(135deg, #0f172a, #15803d);
  border-radius: 18px;
  box-shadow: 0 30px 70px -40px rgba(15, 23, 42, 0.55);
}
.stat { text-align: center; }
.stat-k { font-size: 2.1rem; font-weight: 800; color: #fff; letter-spacing: -0.02em; }
.stat-v { color: rgba(255, 255, 255, 0.78); font-size: 0.9rem; line-height: 1.45; margin-top: 0.25rem; }

/* ---- Final CTA ---- */
.final { text-align: center; }
.final-inner { max-width: 680px; margin: 0 auto; padding: 1.5rem 1.5rem 5rem; }
.final h2 { font-size: clamp(1.6rem, 3vw, 2.05rem); font-weight: 800; letter-spacing: -0.02em; margin: 0 0 0.7rem; }
.final p { color: var(--muted); line-height: 1.68; margin: 0 0 1.6rem; }

/* ---- Footer ---- */
.foot { border-top: 1px solid var(--line); background: var(--tint); }
.foot-inner {
  display: flex; align-items: center; gap: 1rem; justify-content: space-between;
  max-width: var(--maxw); margin: 0 auto; padding: 1.6rem 1.5rem;
  color: var(--muted); font-size: 0.88rem;
}
.foot-copy { flex: 1; text-align: center; }

/* ---- Demo dialog ---- */
.demo-lede { color: var(--muted); margin: 0 0 1rem; line-height: 1.55; }
.demo-form { display: flex; flex-direction: column; gap: 0.85rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.82rem; font-weight: 600; }
.field .opt { color: var(--muted); font-weight: 400; }
.mb { margin-bottom: 0.8rem; }

@media (max-width: 860px) {
  .row { grid-template-columns: 1fr; gap: 1.75rem; padding: 3.25rem 1.5rem; }
  .row.reversed .row-copy { order: 0; }
  .hero-inner { padding-top: 3rem; }
}
@media (max-width: 560px) {
  .foot-inner { flex-direction: column; gap: 0.5rem; }
  .foot-copy { order: 3; }
}
</style>
