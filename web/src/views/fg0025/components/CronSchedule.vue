<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import CronPreview from './CronPreview.vue'
import {
  buildSchedule,
  cronChoices,
  cronSpecs,
  fieldSummary,
  parseSchedule,
  type CronChoice,
  type CronScheduleState,
  type CronSpec,
} from '../cronSchedule'

const props = defineProps<{
  model: Record<string, unknown>
  disabled?: boolean
}>()

const state = reactive<CronScheduleState>(parseSchedule(String(props.model.cronExpr || '0 2 * * *')))
const expr = computed(() => buildSchedule(state))

watch(expr, (value) => {
  if (props.model.cronExpr !== value) props.model.cronExpr = value
}, { immediate: true })

watch(
  () => props.model.cronExpr,
  (value) => {
    const text = String(value || '').trim()
    if (!text || text === expr.value) return
    const next = parseSchedule(text)
    for (const spec of cronSpecs) Object.assign(state[spec.key], next[spec.key])
  },
)

function setChoice(key: keyof CronScheduleState, choice: CronChoice) {
  if (props.disabled) return
  const spec = cronSpecs.find((item) => item.key === key)
  const field = state[key]
  if (spec && choice === 'values' && field.values.length >= spec.max - spec.min + 1) {
    field.values = [seedValue(spec)]
  }
  field.choice = choice
}

function seedValue(spec: CronSpec) {
  if (spec.key === 'hour') return 2
  if (spec.key === 'week') return 1
  return spec.min
}

function toggle(key: keyof CronScheduleState, n: number) {
  if (props.disabled) return
  const field = state[key]
  const has = field.values.includes(n)
  if (has && field.values.length === 1) return
  field.values = has ? field.values.filter((item) => item !== n) : [...field.values, n].sort((a, b) => a - b)
}

function numbers(min: number, max: number) {
  const out: number[] = []
  for (let n = min; n <= max; n++) out.push(n)
  return out
}

function chipText(spec: CronSpec, n: number) {
  if (spec.key === 'week') return spec.labelOf(n).replace('周', '')
  if (spec.key === 'month') return `${n}月`
  if (spec.key === 'minute' || spec.key === 'hour') return String(n).padStart(2, '0')
  return String(n)
}
</script>

<template>
  <div class="fg-cron-editor">
    <section v-for="spec in cronSpecs" :key="spec.key" class="fg-cron-field">
      <div class="fg-cron-head">
        <strong>{{ spec.label }}</strong>
        <span>{{ fieldSummary(state[spec.key], spec) }}</span>
      </div>
      <div class="fg-cron-body">
      <div class="fg-cron-modes">
        <button
          v-for="item in cronChoices"
          :key="item.value"
          type="button"
          :disabled="disabled"
          :class="{ on: state[spec.key].choice === item.value }"
          @click="setChoice(spec.key, item.value)"
        >
          {{ item.label }}
        </button>
      </div>
      <div v-if="state[spec.key].choice === 'values'" class="fg-cron-chips" :class="spec.key">
        <button
          v-for="n in spec.order"
          :key="n"
          type="button"
          :disabled="disabled"
          :class="{ on: state[spec.key].values.includes(n) }"
          @click="toggle(spec.key, n)"
        >
          {{ chipText(spec, n) }}
        </button>
      </div>
      <div v-else-if="state[spec.key].choice === 'range'" class="fg-cron-row">
        <label>
          从
          <select v-model.number="state[spec.key].from" :disabled="disabled">
            <option v-for="n in numbers(spec.min, spec.max)" :key="n" :value="n">{{ spec.labelOf(n) }}</option>
          </select>
        </label>
        <label>
          到
          <select v-model.number="state[spec.key].to" :disabled="disabled">
            <option v-for="n in numbers(spec.min, spec.max)" :key="n" :value="n">{{ spec.labelOf(n) }}</option>
          </select>
        </label>
      </div>
      <div v-else-if="state[spec.key].choice === 'step'" class="fg-cron-row">
        <label>
          从
          <select v-model.number="state[spec.key].start" :disabled="disabled">
            <option v-for="n in numbers(spec.min, spec.max)" :key="n" :value="n">{{ spec.labelOf(n) }}</option>
          </select>
        </label>
        <label>
          到
          <select v-model.number="state[spec.key].end" :disabled="disabled">
            <option v-for="n in numbers(spec.min, spec.max)" :key="n" :value="n">{{ spec.labelOf(n) }}</option>
          </select>
        </label>
        <label>
          每隔
          <select v-model.number="state[spec.key].step" :disabled="disabled">
            <option v-for="n in numbers(1, spec.max - spec.min || 1)" :key="n" :value="n">{{ n }}</option>
          </select>
          {{ spec.unit }}
        </label>
      </div>
      </div>
    </section>
    <CronPreview :expr="expr" />
  </div>
</template>

<style scoped>
.fg-cron-editor {
  display: flex;
  flex-direction: column;
}
.fg-cron-field {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  gap: 8px 16px;
  align-items: start;
  padding: 12px 0;
  border-bottom: 1px solid var(--rs-border, #e5e7eb);
}
.fg-cron-body {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}
.fg-cron-head strong {
  display: block;
  font-size: 13px;
  font-weight: 500;
}
.fg-cron-head span {
  display: block;
  margin-top: 4px;
  color: var(--rs-text-secondary, #64748b);
  font-size: 12px;
  line-height: 1.4;
}
.fg-cron-modes,
.fg-cron-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.fg-cron-modes {
  display: inline-flex;
  gap: 0;
  border: 1px solid var(--rs-border, #e5e7eb);
  border-radius: 8px;
  overflow: hidden;
}
.fg-cron-modes button,
.fg-cron-chips button,
select {
  font: inherit;
  font-size: 12px;
  color: inherit;
}
.fg-cron-modes button {
  border: 0;
  border-right: 1px solid var(--rs-border, #e5e7eb);
  background: transparent;
  padding: 4px 12px;
  cursor: pointer;
}
.fg-cron-modes button:last-child {
  border-right: 0;
}
.fg-cron-chips {
  display: grid;
  gap: 6px;
  width: 100%;
}
.fg-cron-chips.minute {
  grid-template-columns: repeat(10, minmax(0, 1fr));
}
.fg-cron-chips.hour {
  grid-template-columns: repeat(8, minmax(0, 1fr));
}
.fg-cron-chips.day,
.fg-cron-chips.week {
  grid-template-columns: repeat(7, minmax(0, 1fr));
}
.fg-cron-chips.month {
  grid-template-columns: repeat(6, minmax(0, 1fr));
}
.fg-cron-chips button {
  min-width: 0;
  height: 28px;
  padding: 0;
  border: 1px solid var(--rs-border, #e5e7eb);
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  font-variant-numeric: tabular-nums;
}
.fg-cron-modes button.on,
.fg-cron-chips button.on {
  background: var(--rs-accent, #0f766e);
  border-color: var(--rs-accent, #0f766e);
  color: #fff;
}
button:disabled {
  cursor: default;
}
select {
  height: 28px;
  margin-left: 4px;
  padding: 0 8px;
  border: 1px solid var(--rs-border, #e5e7eb);
  border-radius: 6px;
  background: var(--rs-surface, #fff);
}
</style>
