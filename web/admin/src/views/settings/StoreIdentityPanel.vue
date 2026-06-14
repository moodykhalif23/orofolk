<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import Panel from 'primevue/panel'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Select from 'primevue/select'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import { api, errMessage } from '@/lib/client'

// Store identity (SAAS.md #4): the org-scoped branding + email keys and the
// payment gateway config. Branding/email persist as config_settings keys;
// payments go through the dedicated encrypted endpoint (values never echoed).

const toast = useToast()

const branding = reactive({ store_name: '', brand_color: '', logo_url: '' })
const emailId = reactive({ from_name: '', from_address: '' })
const payments = reactive({
  gateway: 'mock',
  available: [] as string[],
  configuredKeys: [] as string[],
  newCredentials: [] as { key: string; value: string }[],
})
const savingIdentity = ref(false)
const savingPayments = ref(false)

const keyFields: [string, Record<string, string>, string][] = [
  ['branding.store_name', branding, 'store_name'],
  ['branding.brand_color', branding, 'brand_color'],
  ['branding.logo_url', branding, 'logo_url'],
  ['email.from_name', emailId, 'from_name'],
  ['email.from_address', emailId, 'from_address'],
]

async function load() {
  for (const [key, target, field] of keyFields) {
    const { data } = await api.GET('/admin/settings/resolve', { params: { query: { key } } })
    if (data?.found && typeof data.value === 'string') target[field] = data.value
  }
  const { data } = await api.GET('/admin/settings/payments')
  if (data) {
    payments.gateway = data.gateway ?? 'mock'
    payments.available = data.available ?? []
    payments.configuredKeys = data.configured_keys ?? []
  }
}

async function saveIdentity() {
  savingIdentity.value = true
  for (const [key, target, field] of keyFields) {
    const { error: err } = await api.PUT('/admin/settings', {
      body: { scope: 'org', scope_id: null, key, value: target[field] ?? '' },
    })
    if (err) {
      savingIdentity.value = false
      toast.add({ severity: 'error', summary: errMessage(err, `Could not save ${key}`), life: 4000 })
      return
    }
  }
  savingIdentity.value = false
  toast.add({ severity: 'success', summary: 'Store identity saved', life: 2500 })
}

function addCredRow() {
  payments.newCredentials.push({ key: '', value: '' })
}

async function savePayments() {
  savingPayments.value = true
  const creds = payments.newCredentials.filter((c) => c.key.trim())
  const body: { gateway: string; credentials?: Record<string, string> } = { gateway: payments.gateway }
  if (creds.length) {
    body.credentials = Object.fromEntries(creds.map((c) => [c.key.trim(), c.value]))
  }
  const { error: err } = await api.PUT('/admin/settings/payments', { body })
  savingPayments.value = false
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Could not save payment config'), life: 4000 })
    return
  }
  payments.newCredentials = []
  toast.add({ severity: 'success', summary: 'Payment config saved', life: 2500 })
  load()
}

onMounted(load)
</script>

<template>
  <div class="identity">
    <Panel header="Store identity" toggleable>
      <div class="cols">
        <section>
          <h4>Branding</h4>
          <div class="field">
            <label>Store name</label>
            <InputText v-model="branding.store_name" placeholder="Acme Industrial Supply" fluid />
          </div>
          <div class="field">
            <label>Brand color</label>
            <div class="color-row">
              <input v-model="branding.brand_color" type="color" class="swatch" aria-label="Pick brand color" />
              <InputText v-model="branding.brand_color" placeholder="#4f46e5" fluid />
            </div>
          </div>
          <div class="field">
            <label>Logo URL</label>
            <InputText v-model="branding.logo_url" placeholder="/media/… (from the Media library)" fluid />
          </div>
        </section>

        <section>
          <h4>Email sender</h4>
          <div class="field">
            <label>From name</label>
            <InputText v-model="emailId.from_name" placeholder="Acme Industrial" fluid />
          </div>
          <div class="field">
            <label>From address</label>
            <InputText v-model="emailId.from_address" placeholder="orders@acme.com" fluid />
          </div>
          <p class="muted small">
            Applied to buyer-facing email (orders, quotes, invoices, recurring). The platform's
            address stays the technical sender, so deliverability never depends on this.
          </p>
        </section>

        <section>
          <h4>Payments</h4>
          <div class="field">
            <label>Gateway</label>
            <Select v-model="payments.gateway" :options="payments.available" fluid />
          </div>
          <div v-if="payments.configuredKeys.length" class="field">
            <label>Stored credentials</label>
            <div class="cred-tags">
              <Tag v-for="k in payments.configuredKeys" :key="k" :value="k" severity="secondary" />
            </div>
            <small class="muted">Values are encrypted at rest and never shown. Saving new ones replaces all.</small>
          </div>
          <div v-for="(c, i) in payments.newCredentials" :key="i" class="cred-row">
            <InputText v-model="c.key" placeholder="key (e.g. api_key)" />
            <Password v-model="c.value" :feedback="false" toggleMask placeholder="value" />
          </div>
          <Button label="Add credential" icon="pi pi-plus" text size="small" @click="addCredRow" />
        </section>
      </div>
      <div class="actions">
        <Button label="Save identity" :loading="savingIdentity" @click="saveIdentity" />
        <Button label="Save payments" severity="secondary" outlined :loading="savingPayments" @click="savePayments" />
      </div>
    </Panel>
  </div>
</template>

<style scoped>
.identity { margin-bottom: 1.5rem; }
.cols { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1.5rem; }
.cols h4 { margin: 0 0 0.75rem; font-size: 0.9rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 0.9rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
.small { font-size: 0.8rem; margin: 0; }
.color-row { display: flex; gap: 0.5rem; align-items: center; }
.swatch { width: 2.4rem; height: 2.4rem; padding: 0; border: 1px solid var(--teggo-border, #e2e8f0); border-radius: 6px; background: none; cursor: pointer; }
.cred-tags { display: flex; gap: 0.4rem; flex-wrap: wrap; }
.cred-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem; margin-bottom: 0.5rem; }
.actions { display: flex; gap: 0.75rem; justify-content: flex-end; margin-top: 0.5rem; }
</style>
