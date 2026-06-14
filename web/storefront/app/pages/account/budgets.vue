<script setup lang="ts">
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import ProgressBar from 'primevue/progressbar'
import Message from 'primevue/message'

definePageMeta({ middleware: 'auth' })
useSeoMeta({ title: 'Budgets — Teggo Store' })

const client = useClient()

const { data, error } = await useAsyncData('account-budgets', async () => {
  const { data, error } = await client.GET('/storefront/account/budgets')
  if (error) throw createError({ statusCode: 502, statusMessage: 'Could not load budgets' })
  return data
})

function pct(spent: string, amount: string) {
  const a = Number(amount)
  if (!a) return 0
  return Math.min(100, Math.round((Number(spent) / a) * 100))
}
</script>

<template>
  <section class="wrap">
    <h1 class="title">Spending budgets</h1>
    <p class="muted">Your cost-center spend limits for the current period. Orders that would exceed a budget are blocked at checkout.</p>

    <Message v-if="error" severity="error" :closable="false">Could not load your budgets.</Message>

    <DataTable :value="data?.items ?? []" dataKey="cost_center" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-wallet" title="No budgets configured" message="Your administrator hasn't set spending limits. Orders won't be blocked at checkout." />
      </template>
      <Column header="Cost center"><template #body="{ data }">{{ data.cost_center || 'Company-wide' }}</template></Column>
      <Column field="period" header="Period" />
      <Column header="Spent / budget">
        <template #body="{ data }">
          <div class="bar">
            <ProgressBar :value="pct(data.spent, data.amount)" :showValue="false" />
            <span class="amt">{{ data.spent }} / {{ data.amount }} {{ data.currency }}</span>
          </div>
        </template>
      </Column>
      <Column header="Remaining"><template #body="{ data }">{{ data.remaining }} {{ data.currency }}</template></Column>
    </DataTable>
  </section>
</template>

<style scoped>
.wrap { max-width: 760px; margin-inline: auto; }
.title { margin: 0 0 0.4rem; }
.muted { color: var(--p-text-muted-color, #64748b); margin-bottom: 1rem; }
.bar { display: flex; flex-direction: column; gap: 0.2rem; min-width: 14rem; }
.amt { font-size: 0.8rem; color: var(--p-text-muted-color, #64748b); font-variant-numeric: tabular-nums; }
</style>
