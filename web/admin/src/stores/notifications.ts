// Admin notification feed: durable history fetched over HTTP, with real-time
// delivery via Pusher when the API reports it's configured. When it isn't, the
// store falls back to polling the unread count, so the bell stays accurate
// either way. The notification endpoints aren't in the OpenAPI contract, so we
// use plain fetch with the same base URL + bearer token as the typed client.
import { defineStore } from 'pinia'
import Pusher from 'pusher-js'
import { useAuthStore } from './auth'

export interface Notification {
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
const PREFIX = '/admin/notifications'
const POLL_MS = 30_000

// Kept module-scoped (not in reactive state) so Pusher/timer internals aren't proxied.
let pusher: Pusher | null = null
let pollTimer: number | null = null

export const useNotificationsStore = defineStore('notifications', {
  state: () => ({
    items: [] as Notification[],
    unread: 0,
    realtime: null as Realtime | null,
  }),
  actions: {
    authHeaders(): Record<string, string> {
      const t = useAuthStore().token
      return t ? { Authorization: `Bearer ${t}` } : {}
    },

    // load fetches the latest feed + unread count, then (re)establishes delivery.
    async load() {
      try {
        const res = await fetch(`${BASE}${PREFIX}?limit=20`, { headers: this.authHeaders() })
        if (!res.ok) return
        const d = await res.json()
        this.items = d.items ?? []
        this.unread = d.unread_count ?? 0
        this.realtime = d.realtime ?? null
        this.connectRealtime()
      } catch {
        /* offline — the next poll/realtime event recovers */
      }
    },

    async markRead(id: string) {
      const n = this.items.find((x) => x.id === id)
      if (n && !n.read) {
        n.read = true
        this.unread = Math.max(0, this.unread - 1)
      }
      try {
        const res = await fetch(`${BASE}${PREFIX}/${id}/read`, { method: 'POST', headers: this.authHeaders() })
        if (res.ok) this.unread = (await res.json()).unread_count ?? this.unread
      } catch {
        /* optimistic update already applied */
      }
    },

    async markAllRead() {
      this.items.forEach((n) => (n.read = true))
      this.unread = 0
      try {
        await fetch(`${BASE}${PREFIX}/read-all`, { method: 'POST', headers: this.authHeaders() })
      } catch {
        /* optimistic */
      }
    },

    connectRealtime() {
      if (this.realtime?.enabled && !pusher) {
        pusher = new Pusher(this.realtime.key, {
          cluster: this.realtime.cluster,
          channelAuthorization: {
            endpoint: `${BASE}${PREFIX}/pusher-auth`,
            transport: 'ajax',
            headers: this.authHeaders(),
          },
        })
        const ch = pusher.subscribe(this.realtime.channel)
        ch.bind('notification.created', (n: Notification) => {
          if (this.items.some((x) => x.id === n.id)) return
          this.items = [n, ...this.items].slice(0, 50)
          if (!n.read) this.unread += 1
        })
      } else if (!this.realtime?.enabled) {
        // Poll-only mode: refresh the feed on a timer.
        if (pollTimer === null) pollTimer = window.setInterval(() => this.load(), POLL_MS)
      }
    },

    // start is called once the user is authenticated; stop on logout.
    start() {
      this.load()
    },
    stop() {
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
  },
})
