<script setup lang="ts">
import { computed } from 'vue'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Textarea from 'primevue/textarea'
import ToggleSwitch from 'primevue/toggleswitch'
import Select from 'primevue/select'
import Editor from 'primevue/editor'
import { BLOCK_REGISTRY, type Block, type BlockField } from '@teggo/blocks'
import type { components } from '@teggo/api/schema'
import { getPath, setPath } from './fields'

type Category = components['schemas']['Category']

const props = defineProps<{ block: Block | null; categories: Category[] }>()

const fields = computed<BlockField[]>(() =>
  props.block ? (BLOCK_REGISTRY[props.block.type]?.fields ?? []) : [],
)

// block.props is the same reactive object held in the page's blocks array, so
// writing through it updates the canvas preview live.
function val(key: string) {
  return props.block ? getPath(props.block.props ?? {}, key) : undefined
}
function set(key: string, value: any) {
  if (props.block) setPath((props.block.props ??= {}), key, value)
}

// link fields: a toggle creates/clears the {label, href} object.
function linkEnabled(key: string) {
  return val(key) != null
}
function toggleLink(key: string, on: boolean) {
  set(key, on ? { label: 'Learn more', href: '/' } : null)
}

// category fields: selecting also stamps source.kind so the backend resolves it.
function setCategory(key: string, value: number | null) {
  set(key, value)
  const sourceKey = key.includes('.') ? key.slice(0, key.lastIndexOf('.')) + '.kind' : 'kind'
  set(sourceKey, 'category')
}
</script>

<template>
  <div class="inspector">
    <template v-if="block">
      <h3>{{ BLOCK_REGISTRY[block.type]?.label ?? block.type }}</h3>
      <div v-for="f in fields" :key="f.key" class="field">
        <label>{{ f.label }}</label>

        <InputText
          v-if="f.kind === 'text'"
          :modelValue="val(f.key)"
          @update:modelValue="set(f.key, $event)"
        />

        <Textarea
          v-else-if="f.kind === 'textarea'"
          :modelValue="val(f.key)"
          rows="6"
          autoResize
          @update:modelValue="set(f.key, $event)"
        />

        <Editor
          v-else-if="f.kind === 'richtext'"
          editorStyle="height: 200px"
          :modelValue="val(f.key)"
          @update:modelValue="set(f.key, $event)"
        />

        <InputNumber
          v-else-if="f.kind === 'number'"
          :modelValue="val(f.key)"
          :min="1"
          :max="48"
          showButtons
          @update:modelValue="set(f.key, $event)"
        />

        <Select
          v-else-if="f.kind === 'category'"
          :modelValue="val(f.key)"
          :options="categories"
          optionLabel="name"
          optionValue="id"
          placeholder="Choose a category"
          filter
          showClear
          @update:modelValue="setCategory(f.key, $event)"
        />

        <template v-else-if="f.kind === 'link'">
          <div class="link-toggle">
            <ToggleSwitch :modelValue="linkEnabled(f.key)" @update:modelValue="toggleLink(f.key, $event)" />
            <span class="muted">{{ linkEnabled(f.key) ? 'Shown' : 'Hidden' }}</span>
          </div>
          <template v-if="linkEnabled(f.key)">
            <InputText
              class="sub"
              placeholder="Button label"
              :modelValue="val(f.key + '.label')"
              @update:modelValue="set(f.key + '.label', $event)"
            />
            <InputText
              class="sub"
              placeholder="Link (e.g. /c/category)"
              :modelValue="val(f.key + '.href')"
              @update:modelValue="set(f.key + '.href', $event)"
            />
          </template>
        </template>
      </div>
    </template>
    <p v-else class="muted empty">Select a block to edit its content.</p>
  </div>
</template>

<style scoped>
.inspector { display: flex; flex-direction: column; gap: 0.25rem; }
.inspector h3 { margin: 0 0 0.5rem; font-size: 0.95rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
.field .sub { margin-top: 0.4rem; }
.field :deep(.p-select), .field :deep(.p-inputnumber) { width: 100%; }
.link-toggle { display: flex; align-items: center; gap: 0.5rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-size: 0.85rem; }
.empty { padding-top: 1rem; }
</style>
