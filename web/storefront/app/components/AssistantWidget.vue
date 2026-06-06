<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import type { components } from '@teggo/api/schema'

type Turn = components['schemas']['AssistantTurn']

const client = useClient()
const open = ref(false)
const input = ref('')
const busy = ref(false)
const messages = ref<Turn[]>([])
const body = ref<HTMLElement | null>(null)

const suggestions = ['Where is my latest order?', 'What do I owe?', 'What should I reorder?']

async function send(text?: string) {
  const msg = (text ?? input.value).trim()
  if (!msg || busy.value) return
  input.value = ''
  messages.value.push({ role: 'user', text: msg })
  await scroll()
  busy.value = true
  const history = messages.value.slice(0, -1)
  const { data, error } = await client.POST('/storefront/assistant', { body: { message: msg, history } })
  busy.value = false
  messages.value.push({ role: 'assistant', text: data?.text ?? (error ? 'Sorry, I\'m unavailable right now.' : '') })
  await scroll()
}

async function scroll() {
  await nextTick()
  if (body.value) body.value.scrollTop = body.value.scrollHeight
}
</script>

<template>
  <div class="assistant">
    <Button
      v-if="!open"
      class="fab"
      icon="pi pi-comments"
      rounded
      aria-label="Open assistant"
      @click="open = true"
    />
    <div v-else class="panel">
      <header class="phead">
        <span><i class="pi pi-sparkles" /> Assistant</span>
        <Button icon="pi pi-times" text rounded size="small" aria-label="Close" @click="open = false" />
      </header>
      <div ref="body" class="pbody">
        <div v-if="!messages.length" class="hint">
          <p>Ask me about your orders, invoices or reorders.</p>
          <button v-for="s in suggestions" :key="s" class="chip" @click="send(s)">{{ s }}</button>
        </div>
        <div v-for="(m, i) in messages" :key="i" class="msg" :class="m.role">
          <div class="bubble">{{ m.text }}</div>
        </div>
        <div v-if="busy" class="msg assistant"><div class="bubble">…</div></div>
      </div>
      <form class="pcomposer" @submit.prevent="send()">
        <InputText v-model="input" placeholder="Type a message…" />
        <Button type="submit" icon="pi pi-send" :loading="busy" :disabled="!input.trim()" />
      </form>
    </div>
  </div>
</template>

<style scoped>
.fab { position: fixed; right: 1.5rem; bottom: 1.5rem; width: 3.5rem; height: 3.5rem; z-index: 50; box-shadow: 0 6px 20px rgba(0,0,0,0.18); }
.panel {
  position: fixed; right: 1.5rem; bottom: 1.5rem; z-index: 50;
  width: 22rem; height: 30rem; max-height: 75vh;
  display: flex; flex-direction: column; background: #fff;
  border: 1px solid #e2e8f0; border-radius: 14px; box-shadow: 0 12px 40px rgba(0,0,0,0.2); overflow: hidden;
}
.phead { display: flex; align-items: center; justify-content: space-between; padding: 0.6rem 0.85rem; background: #0f172a; color: #fff; font-weight: 600; }
.pbody { flex: 1; overflow-y: auto; padding: 0.85rem; display: flex; flex-direction: column; gap: 0.5rem; }
.hint { color: #64748b; font-size: 0.9rem; }
.chip { display: inline-block; margin: 0.25rem 0.25rem 0 0; padding: 0.3rem 0.6rem; border: 1px solid #e2e8f0; border-radius: 999px; background: #f8fafc; cursor: pointer; font-size: 0.82rem; }
.chip:hover { background: #eef2ff; }
.msg { display: flex; }
.msg.user { justify-content: flex-end; }
.bubble { max-width: 80%; padding: 0.5rem 0.7rem; border-radius: 12px; white-space: pre-wrap; line-height: 1.4; font-size: 0.9rem; }
.msg.user .bubble { background: #1d4ed8; color: #fff; border-bottom-right-radius: 4px; }
.msg.assistant .bubble { background: #f1f5f9; border-bottom-left-radius: 4px; }
.pcomposer { display: flex; gap: 0.4rem; padding: 0.6rem; border-top: 1px solid #e2e8f0; }
.pcomposer :deep(input) { flex: 1; }
</style>
