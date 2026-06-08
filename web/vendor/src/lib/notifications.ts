import { reactive } from 'vue'
import Pusher from 'pusher-js'
import { auth } from './auth'

// Vendor notification feed (no Pinia, mirroring lib/auth.ts). Durable history is
// fetched over HTTP; real-time arrives via Pusher when the API reports it's
// configured, otherwise the unread state is refreshed by polling. Endpoints
// aren't in the OpenAPI contract, so we use plain fetch + bearer token.
export interface VNotification {
  id: string
  type: string
  title: string
  body?: string
  link?: string
  severity: 'info' | 'success' | 'warning' | 'error'
  read: boolean
  created_at: string
}

interface Realtime {
  enabled: boolean
  key: string
  cluster: string
  channel: string
}

const BASE = import.meta.env.VITE_API_BASE_URL ?? ''
const PREFIX = '/vendor/notifications'
const POLL_MS = 30_000

let pusher: Pusher | null = null
let pollTimer: number | null = null

export const notifications = reactive({
  items: [] as VNotification[],
  unread: 0,
  realtime: null as Realtime | null,

  headers(): Record<string, string> {
    return auth.token ? { Authorization: `Bearer ${auth.token}` } : {}
  },

  async load(): Promise<void> {
    try {
      const res = await fetch(`${BASE}${PREFIX}?limit=20`, { headers: this.headers() })
      if (!res.ok) return
      const d = await res.json()
      this.items = d.items ?? []
      this.unread = d.unread_count ?? 0
      this.realtime = d.realtime ?? null
      this.connectRealtime()
    } catch {
      /* offline — recovered by the next poll/realtime event */
    }
  },

  async markRead(id: string): Promise<void> {
    const n = this.items.find((x) => x.id === id)
    if (n && !n.read) {
      n.read = true
      this.unread = Math.max(0, this.unread - 1)
    }
    try {
      const res = await fetch(`${BASE}${PREFIX}/${id}/read`, { method: 'POST', headers: this.headers() })
      if (res.ok) this.unread = (await res.json()).unread_count ?? this.unread
    } catch {
      /* optimistic */
    }
  },

  async markAllRead(): Promise<void> {
    this.items.forEach((n) => (n.read = true))
    this.unread = 0
    try {
      await fetch(`${BASE}${PREFIX}/read-all`, { method: 'POST', headers: this.headers() })
    } catch {
      /* optimistic */
    }
  },

  connectRealtime(): void {
    if (this.realtime?.enabled && !pusher) {
      pusher = new Pusher(this.realtime.key, {
        cluster: this.realtime.cluster,
        channelAuthorization: {
          endpoint: `${BASE}${PREFIX}/pusher-auth`,
          transport: 'ajax',
          headers: this.headers(),
        },
      })
      const ch = pusher.subscribe(this.realtime.channel)
      ch.bind('notification.created', (n: VNotification) => {
        if (this.items.some((x) => x.id === n.id)) return
        this.items = [n, ...this.items].slice(0, 50)
        if (!n.read) this.unread += 1
      })
    } else if (!this.realtime?.enabled) {
      if (pollTimer === null) pollTimer = window.setInterval(() => this.load(), POLL_MS)
    }
  },

  start(): void {
    this.load()
  },
  stop(): void {
    if (pusher) {
      pusher.disconnect()
      pusher = null
    }
    if (pollTimer !== null) {
      clearInterval(pollTimer)
      pollTimer = null
    }
    this.items = []
    this.unread = 0
    this.realtime = null
  },
})
