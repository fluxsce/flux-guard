<script setup lang="ts">
import { computed } from 'vue'
import { previewCron } from '../cronPreview'

const props = defineProps<{
  expr?: string
}>()

const preview = computed(() => previewCron(props.expr || ''))
</script>

<template>
  <section class="fg-cron-preview" aria-label="执行时间预览">
    <p v-if="preview.error" class="fg-cron-error">{{ preview.error }}</p>
    <template v-else>
      <p class="fg-cron-summary">{{ preview.summary }}</p>
      <p v-if="preview.note" class="fg-cron-note">{{ preview.note }}</p>
      <p class="fg-cron-next-title">接下来 5 次</p>
      <ol v-if="preview.next.length">
        <li v-for="item in preview.next" :key="item">{{ item }}</li>
      </ol>
      <p v-else class="fg-cron-note">366 天内没有命中的分钟。</p>
    </template>
  </section>
</template>

<style scoped>
.fg-cron-preview {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid var(--rs-border, #e5e7eb);
  border-radius: 8px;
  background: var(--rs-surface-muted, #f8fafc);
}
.fg-cron-summary {
  margin: 0;
  font-weight: 600;
}
.fg-cron-note,
.fg-cron-next-title {
  margin: 10px 0 0;
  color: var(--rs-text-secondary, #64748b);
  font-style: normal;
  font-size: 12px;
}
ol {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 8px 0 0;
  padding-left: 1.4em;
}
li {
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.fg-cron-error {
  margin: 0;
  color: var(--rs-danger, #b91c1c);
}
</style>
