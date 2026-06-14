<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Popover from 'primevue/popover'
import { useNotificationsStore, type Notification } from '@/stores/notifications'

const store = useNotificationsStore()
const router = useRouter()
const op = ref()

const items = computed(() => store.items)
const badge = computed(() => (store.unread > 99 ? '99+' : String(store.unread)))

function toggle(e: Event) {
  op.value?.toggle(e)
}

function onItem(n: Notification) {
  store.markRead(n.id)
  op.value?.hide()
  if (n.link) router.push(n.link).catch(() => {})
}

// Compact relative time: "now", "5m", "3h", "2d", else a date.
function relTime(iso: string): string {
  const then = new Date(iso).getTime()
  const s = Math.max(0, Math.floor((Date.now() - then) / 1000))
  if (s < 45) return 'now'
  if (s < 3600) return `${Math.floor(s / 60)}m`
  if (s < 86400) return `${Math.floor(s / 3600)}h`
  if (s < 604800) return `${Math.floor(s / 86400)}d`
  return new Date(iso).toLocaleDateString()
}
</script>

<template>
  <button type="button" class="bell" :class="{ active: store.unread > 0 }" aria-label="Notifications" @click="toggle">
    <i class="pi pi-bell" />
    <span v-if="store.unread > 0" class="badge">{{ badge }}</span>
  </button>

  <Popover ref="op" class="notif-pop">
    <div class="notif">
      <header class="notif-head">
        <span class="notif-title">Notifications</span>
        <button v-if="store.unread > 0" type="button" class="notif-link" @click="store.markAllRead()">
          Mark all read
        </button>
      </header>

      <ul v-if="items.length" class="notif-list">
        <li
          v-for="n in items"
          :key="n.id"
          class="notif-item"
          :class="{ unread: !n.read }"
          @click="onItem(n)"
        >
          <span class="sev" :data-sev="n.severity" />
          <div class="content">
            <div class="row1">
              <span class="title">{{ n.title }}</span>
              <span class="time">{{ relTime(n.created_at) }}</span>
            </div>
            <div v-if="n.body" class="body">{{ n.body }}</div>
          </div>
        </li>
      </ul>

      <div v-else class="empty">
        <i class="pi pi-check-circle" />
        <span>You're all caught up</span>
      </div>
    </div>
  </Popover>
</template>

<style scoped>
.bell {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.25rem;
  height: 2.25rem;
  border: 1px solid var(--p-surface-200, #e2e8f0);
  border-radius: 6px;
  background: var(--p-surface-0, #fff);
  color: var(--p-text-color, #334155);
  cursor: pointer;
}
.bell:hover {
  background: var(--p-surface-50, #f8fafc);
}
.bell .pi {
  font-size: 1.1rem;
}
.badge {
  position: absolute;
  top: -6px;
  right: -6px;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  border-radius: 9px;
  background: var(--p-red-500, #ef4444);
  color: #fff;
  font-size: 0.68rem;
  font-weight: 700;
  line-height: 18px;
  text-align: center;
}
.notif {
  width: 22rem;
  max-width: 90vw;
}
.notif-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.25rem 0.25rem 0.6rem;
  border-bottom: 1px solid var(--p-surface-200, #e2e8f0);
}
.notif-title {
  font-weight: 700;
}
.notif-link {
  background: none;
  border: none;
  color: var(--p-primary-color, #6366f1);
  font-size: 0.8rem;
  cursor: pointer;
  padding: 0;
}
.notif-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 24rem;
  overflow-y: auto;
}
.notif-item {
  display: flex;
  gap: 0.6rem;
  padding: 0.65rem 0.4rem;
  border-bottom: 1px solid var(--p-surface-100, #f1f5f9);
  cursor: pointer;
}
.notif-item:hover {
  background: var(--p-surface-50, #f8fafc);
}
.notif-item.unread {
  background: var(--p-primary-50, #eef2ff);
}
.sev {
  flex: 0 0 8px;
  width: 8px;
  height: 8px;
  margin-top: 0.35rem;
  border-radius: 50%;
  background: var(--p-surface-300, #cbd5e1);
}
.sev[data-sev='success'] { background: var(--p-green-500, #22c55e); }
.sev[data-sev='warning'] { background: var(--p-amber-500, #f59e0b); }
.sev[data-sev='error'] { background: var(--p-red-500, #ef4444); }
.sev[data-sev='info'] { background: var(--p-blue-500, #3b82f6); }
.content {
  flex: 1;
  min-width: 0;
}
.row1 {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.5rem;
}
.title {
  font-size: 0.88rem;
  font-weight: 600;
  color: var(--p-text-color, #1e293b);
}
.time {
  flex: 0 0 auto;
  font-size: 0.72rem;
  color: var(--p-text-muted-color, #94a3b8);
}
.body {
  margin-top: 0.15rem;
  font-size: 0.8rem;
  color: var(--p-text-muted-color, #64748b);
}
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  padding: 1.5rem 0.5rem;
  color: var(--p-text-muted-color, #94a3b8);
  font-size: 0.85rem;
}
.empty .pi {
  font-size: 1.4rem;
  color: var(--p-green-500, #22c55e);
}
</style>
