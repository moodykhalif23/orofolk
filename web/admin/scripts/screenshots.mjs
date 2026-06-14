// Capture fresh dashboard screenshots for the marketing landing page
// (src/views/LandingView.vue references /landing/*.png).
//
// Usage: pnpm --filter @teggo/admin screenshots
// Requires the admin dev server running (pnpm --filter @teggo/admin dev) and the
// API reachable through its proxy. Override the target/credentials with env vars:
//   ADMIN_URL (default http://localhost:5173)
//   DEMO_EMAIL / DEMO_PASSWORD (default the dev seed admin)
//
// It logs in via the API to mint an admin token, injects it into localStorage so
// the SPA boots authenticated, then screenshots each view at retina resolution.
import { chromium } from 'playwright'
import { mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const BASE = process.env.ADMIN_URL ?? 'http://localhost:5173'
const EMAIL = process.env.DEMO_EMAIL ?? 'admin@demo.test'
const PASSWORD = process.env.DEMO_PASSWORD ?? 'admin1234'
const outDir = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'public', 'landing')

// route → output file. These map to the visuals in LandingView.vue.
const shots = [
  { route: '/', file: 'dashboard.png' },
  { route: '/insights', file: 'insights.png' },
  { route: '/analytics', file: 'analytics.png' },
]

async function main() {
  let token
  try {
    const res = await fetch(`${BASE}/admin/auth/login`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ email: EMAIL, password: PASSWORD }),
    })
    if (!res.ok) throw new Error(`login returned ${res.status}`)
    token = (await res.json()).token
    if (!token) throw new Error('no token in login response')
  } catch (e) {
    console.error(`\n✗ Could not authenticate against ${BASE} (${e.message}).`)
    console.error('  Make sure the admin dev server is running:')
    console.error('    pnpm --filter @teggo/admin dev')
    console.error('  (override with ADMIN_URL, DEMO_EMAIL, DEMO_PASSWORD)\n')
    process.exit(1)
  }

  await mkdir(outDir, { recursive: true })
  const browser = await chromium.launch()
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2 })
  // Seed the token before any app code runs so the router sees an authed session.
  await page.addInitScript(
    ([t, e]) => {
      localStorage.setItem('teggo.admin.token', t)
      localStorage.setItem('teggo.admin.email', e)
    },
    [token, EMAIL],
  )

  for (const { route, file } of shots) {
    await page.goto(`${BASE}${route}`, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(2500) // let lazy charts and data settle
    await page.screenshot({ path: resolve(outDir, file) })
    console.log(`✓ ${route} → public/landing/${file}`)
  }

  await browser.close()
  console.log('\nDone — screenshots written to web/admin/public/landing/.')
}

main().catch((e) => {
  console.error(e)
  process.exit(1)
})
