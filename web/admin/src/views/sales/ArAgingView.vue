<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Message from 'primevue/message'
import MeterGroup from 'primevue/metergroup'
import { api, errMessage } from '@/lib/client'
import { useCustomerOptions } from '@/composables/useRecordOptions'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Report = components['schemas']['ARAgingReport']

const report = ref<Report | null>(null)
const loading = ref(false)
const sweeping = ref(false)
const error = ref('')
const toast = useToast()
const { customers, loadCustomers } = useCustomerOptions()

const order = ['current', '1-30', '31-60', '61-90', '90+']

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/invoices/aging')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load aging report')
    return
  }
  report.value = data
}

async function sweep() {
  sweeping.value = true
  const { data, error: err } = await api.POST('/admin/invoices/overdue-sweep')
  sweeping.value = false
  if (err || !data) {
    toast.add({ severity: 'error', summary: 'Sweep failed', detail: errMessage(err), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: `Marked ${data.marked_overdue} overdue`, detail: 'Dunning notices queued', life: 3000 })
  load()
}

// Aging buckets as a proportional MeterGroup (segments of the open total),
// escalating from the brand colour (current) to deep red (90+).
const BUCKET_META: Record<string, { label: string; color: string }> = {
  current: { label: 'Current', color: '#6366f1' },
  '1-30': { label: '1–30 days', color: '#f59e0b' },
  '31-60': { label: '31–60 days', color: '#f97316' },
  '61-90': { label: '61–90 days', color: '#ef4444' },
  '90+': { label: '90+ days', color: '#b91c1c' },
}
const meterValue = computed(() =>
  order.map((b) => ({
    label: BUCKET_META[b].label,
    color: BUCKET_META[b].color,
    value: Number(report.value?.buckets?.[b] ?? 0),
    money: report.value?.buckets?.[b] ?? '0',
  })),
)
const meterMax = computed(() => meterValue.value.reduce((s, m) => s + m.value, 0) || 1)
function custName(id: number) {
  return customers.value.find((c) => c.id === id)?.name ?? `#${id}`
}
function sev(bucket: string) {
  return bucket === 'current' ? 'secondary' : bucket === '90+' || bucket === '61-90' ? 'danger' : 'warn'
}

onMounted(() => {
  load()
  loadCustomers()
})
</script>

<template>
  <div class="page">
    <PageHeader title="AR aging">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button label="Run overdue sweep" icon="pi pi-flag" :loading="sweeping" @click="sweep" />
      </template>
    </PageHeader>
    <p class="muted">Open (issued + overdue) invoices bucketed by days past due. The sweep flips past-due invoices to overdue and queues a dunning notice to each customer.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <div v-if="report" class="aging">
      <div class="aging-head">
        <span class="muted">Open receivables</span>
        <strong class="aging-total">{{ report.open_total }}</strong>
      </div>
      <MeterGroup :value="meterValue" :max="meterMax" class="aging-meter">
        <template #label>
          <div class="aging-legend">
            <span v-for="m in meterValue" :key="m.label" class="leg">
              <span class="leg-dot" :style="{ background: m.color }" />
              <span class="leg-label">{{ m.label }}</span>
              <span class="leg-val">{{ m.money }}</span>
            </span>
          </div>
        </template>
      </MeterGroup>
    </div>

    <DataTable :value="report?.items ?? []" :loading="loading" dataKey="public_id" stripedRows class="mt">
      <template #empty>
        <EmptyState icon="pi pi-check-circle" title="No open invoices" message="Nothing is outstanding — every issued invoice has been paid. Nice." />
      </template>
      <Column header="Invoice"><template #body="{ data }">{{ data.public_id.slice(0, 8) }}…</template></Column>
      <Column header="Customer"><template #body="{ data }">{{ custName(data.customer_id) }}</template></Column>
      <Column header="Status"><template #body="{ data }"><Tag :value="data.status" :severity="data.status === 'overdue' ? 'danger' : 'info'" /></template></Column>
      <Column header="Amount"><template #body="{ data }">{{ data.grand_total }} {{ data.currency }}</template></Column>
      <Column header="Days overdue"><template #body="{ data }">{{ data.days_overdue > 0 ? data.days_overdue : '—' }}</template></Column>
      <Column header="Bucket"><template #body="{ data }"><Tag :value="data.bucket" :severity="sev(data.bucket)" /></template></Column>
    </DataTable>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.mt { margin-top: 1rem; }
.aging { margin: 1rem 0; border: 1px solid var(--p-surface-200, #e2e8f0); border-radius: 8px; padding: 1rem 1.25rem; }
.aging-head { display: flex; align-items: baseline; justify-content: space-between; margin-bottom: 0.75rem; }
.aging-total { font-size: 1.4rem; font-weight: 700; font-variant-numeric: tabular-nums; }
.aging-legend { display: flex; flex-wrap: wrap; gap: 0.4rem 1.25rem; margin-top: 0.85rem; }
.leg { display: inline-flex; align-items: center; gap: 0.4rem; font-size: 0.85rem; }
.leg-dot { width: 0.7rem; height: 0.7rem; border-radius: 2px; display: inline-block; flex-shrink: 0; }
.leg-label { color: var(--p-text-muted-color, #64748b); }
.leg-val { font-weight: 600; font-variant-numeric: tabular-nums; }
</style>
