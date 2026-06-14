// One-off: verify Phase 0 — breadcrumb + detail skeleton on an order detail page.
import { chromium } from 'playwright'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const BASE = process.env.ADMIN_URL ?? 'http://localhost:5173'
const dir = dirname(fileURLToPath(import.meta.url))

const res = await fetch(`${BASE}/demo`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ email: 'demo@teggo.dev', company: 'Northwind Industrial', name: 'Demo' }),
})
if (!res.ok) throw new Error(`/demo returned ${res.status}`)
const token = (await res.json()).token

// Grab a real order id from the API.
const ordersRes = await fetch(`${BASE}/admin/orders`, { headers: { authorization: `Bearer ${token}` } })
const orders = await ordersRes.json()
const id = orders.items?.[0]?.id
if (!id) throw new Error('no orders in demo org')
const detailUrl = `${BASE}/orders/${id}`

const browser = await chromium.launch()
const inject = ([t]) => {
  localStorage.setItem('teggo.admin.token', t)
  localStorage.setItem('teggo.admin.email', 'demo@teggo.dev')
}

// Loaded detail page → breadcrumb + content.
const page = await browser.newPage({ viewport: { width: 1280, height: 860 }, deviceScaleFactor: 1 })
await page.addInitScript(inject, [token])
await page.goto(detailUrl, { waitUntil: 'networkidle' })
await page.waitForTimeout(1200)
await page.screenshot({ path: resolve(dir, '_p0-detail.png') })
console.log('wrote _p0-detail.png  ', detailUrl)

// Throttled fresh load → catch the loading skeleton.
const ctx2 = await browser.newContext({ viewport: { width: 1280, height: 860 } })
const sk = await ctx2.newPage()
await sk.addInitScript(inject, [token])
const cdp = await ctx2.newCDPSession(sk)
await cdp.send('Network.emulateNetworkConditions', { offline: false, latency: 800, downloadThroughput: 40000, uploadThroughput: 40000 })
await sk.goto(detailUrl, { waitUntil: 'commit' }).catch(() => {})
await sk.waitForTimeout(500)
await sk.screenshot({ path: resolve(dir, '_p0-skeleton.png') })
console.log('wrote _p0-skeleton.png')

await browser.close()
