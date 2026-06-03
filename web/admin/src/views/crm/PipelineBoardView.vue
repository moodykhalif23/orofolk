<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Card from 'primevue/card'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type BoardStage = components['schemas']['PipelineBoardStage']

const stages = ref<BoardStage[]>([])
const pipelineName = ref('')
const loading = ref(false)
const error = ref('')

// Total weighted forecast = Σ weighted_amount across stages (Pack 2 §1.2).
const weightedTotal = computed(() =>
  stages.value.reduce((sum, s) => sum + Number(s.weighted_amount || 0), 0).toFixed(2),
)
const openTotal = computed(() => stages.value.reduce((sum, s) => sum + Number(s.open_count || 0), 0))

async function load() {
  loading.value = true
  error.value = ''
  const { data: pls, error: e1 } = await api.GET('/admin/pipelines')
  if (e1 || !pls?.items?.length) {
    loading.value = false
    error.value = errMessage(e1, 'No pipeline configured')
    return
  }
  const pl = pls.items[0]
  pipelineName.value = pl.name
  const { data, error: e2 } = await api.GET('/admin/pipelines/{id}/board', { params: { path: { id: pl.id } } })
  loading.value = false
  if (e2 || !data) {
    error.value = errMessage(e2, 'Failed to load board')
    return
  }
  stages.value = data.items ?? []
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="header">
      <h1>Pipeline <span class="muted">{{ pipelineName }}</span></h1>
      <div class="forecast">
        <div class="f-val">{{ weightedTotal }}</div>
        <div class="f-lbl">weighted forecast · {{ openTotal }} open</div>
      </div>
    </div>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <div class="board">
      <Card v-for="s in stages" :key="s.id" class="stage">
        <template #title>
          <div class="stage-head">
            <span>{{ s.label }}</span>
            <span class="prob">{{ Number(s.probability).toFixed(0) }}%</span>
          </div>
        </template>
        <template #content>
          <div class="count">{{ s.open_count }} <span class="muted">open</span></div>
          <div class="amt"><span class="muted">Total</span> <strong>{{ s.total_amount }}</strong></div>
          <div class="amt"><span class="muted">Weighted</span> <strong>{{ s.weighted_amount }}</strong></div>
        </template>
      </Card>
    </div>
  </div>
</template>

<style scoped>
.header { display: flex; align-items: flex-end; justify-content: space-between; margin-bottom: 1.25rem; }
.header h1 { margin: 0; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
.forecast { text-align: right; }
.f-val { font-size: 1.8rem; font-weight: 800; line-height: 1; color: var(--p-primary-color, #0ea5e9); }
.f-lbl { font-size: 0.8rem; color: var(--p-text-muted-color, #64748b); }
.mb { margin-bottom: 1rem; }
.board { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; align-items: start; }
.stage-head { display: flex; justify-content: space-between; align-items: center; font-size: 1rem; }
.prob { font-size: 0.8rem; color: var(--p-text-muted-color, #64748b); }
.count { font-size: 1.5rem; font-weight: 700; margin-bottom: 0.5rem; }
.amt { display: flex; justify-content: space-between; font-size: 0.9rem; padding: 0.15rem 0; font-variant-numeric: tabular-nums; }
</style>
