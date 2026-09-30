<script setup lang="ts">
import { computed, useTemplateRef } from 'vue'
import { useModuleAi } from '@/ai'
import { useButtonAllowed } from '@/components/buttonAccess'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataForm from '@/components/form/FgDataForm.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsButton, RsDialog, RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import type { Template } from './api'
import TemplatePreviewPane from './TemplatePreviewPane.vue'
import {
  builtInOptions,
  contentOptions,
  flagLabel,
  kindOptions,
  statusOptions,
  templateColumns,
  templateFormFields,
  templateMenuItems,
  templateSearchFields,
  useTemplateCatalog,
} from './composables/useTemplateCatalog'

type FormExpose = {
  getValues: () => Record<string, unknown>
  model: Record<string, unknown>
}

const moduleId = 'fg0041'
const allowed = useButtonAllowed()
const formRef = useTemplateRef<FormExpose>('formRef')
const {
  page,
  pageSize,
  error,
  formOpen,
  formMode,
  current,
  active,
  total,
  pageRows,
  openView,
  openCreate,
  openEdit,
  submitTemplate,
  removeTemplate,
  preview,
  previewOpen,
  previewSubject,
  previewBody,
  previewContent,
  previewChannel,
  onRow,
  onSearch,
  onReset,
} = useTemplateCatalog()

const formFields = computed(() => templateFormFields)
const formTitle = computed(() => {
  if (formMode.value === 'create') return '新增预警模板'
  if (formMode.value === 'edit') return '修改预警模板'
  return '预警模板'
})
const builtIn = computed(() => active.value?.builtInFlag === 'Y')
const canSave = computed(() => {
  if (formMode.value === 'view') return false
  return allowed(moduleId, formMode.value === 'create' ? 'add' : 'edit')
})
const formModel = computed<Record<string, unknown> | undefined>(() => formRef.value?.model)

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'create', label: '新增', icon: 'plus', onClick: openCreate },
  {
    key: 'edit',
    label: '修改',
    icon: 'pencil',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openEdit(active.value)
    },
  },
  {
    key: 'delete',
    label: '删除',
    icon: 'trash',
    disabled: !active.value || builtIn.value,
    onClick: () => {
      if (active.value) void removeTemplate(active.value)
    },
  },
  {
    key: 'preview',
    label: '预览',
    icon: 'file-text',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void preview(active.value)
    },
  },
  {
    key: 'view',
    label: '查看',
    icon: 'eye',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openView(active.value)
    },
  },
])

useModuleAi(
  () => ({
    moduleId,
    title: '预警模板',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      formMode: formMode.value,
      previewOpen: previewOpen.value,
      selectedTemplateId: active.value?.templateId ?? '',
      selectedTemplateName: active.value?.templateName ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0041.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: Template | null }) {
  if (!payload.row) return
  if (payload.key === 'view') openView(payload.row)
  if (payload.key === 'edit') openEdit(payload.row)
  if (payload.key === 'preview') void preview(payload.row)
  if (payload.key === 'delete') void removeTemplate(payload.row)
}

function modelText(model: Record<string, unknown> | undefined, key: string) {
  if (!model) return ''
  const value = model[key]
  return value == null ? '' : String(value)
}

function closeForm() {
  formOpen.value = false
}

function saveForm() {
  void submitTemplate(formRef.value?.getValues() ?? {})
}
</script>

<template>
  <div class="fg-page fg-template" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog>
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="templateSearchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="templateColumns"
        row-key="templateId"
        :module-id="moduleId"
        :menu-items="templateMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #channelKind="{ row }">
          {{ flagLabel(kindOptions, String((row as Template).channelKind ?? '')) }}
        </template>
        <template #contentType="{ row }">
          {{ flagLabel(contentOptions, String((row as Template).contentType ?? '')) }}
        </template>
        <template #builtInFlag="{ row }">
          {{ flagLabel(builtInOptions, String((row as Template).builtInFlag ?? '')) }}
        </template>
        <template #statusFlag="{ row }">
          <RsTag size="sm" :variant="(row as Template).statusFlag === 'Y' ? 'success' : 'default'">
            {{ flagLabel(statusOptions, String((row as Template).statusFlag ?? '')) }}
          </RsTag>
        </template>
      </FgGrid>
      <template #pager>
        <RsPagination
          v-model:page="page"
          v-model:page-size="pageSize"
          :total="total"
          size="sm"
          align="end"
          show-page-size
          :page-size-options="pageSizeChoices"
        />
      </template>
    </FgCatalog>

    <RsDialog
      v-model:open="formOpen"
      :title="formTitle"
      width="980px"
      :modal="false"
      :teleport-to="`#${moduleId}-layer`"
    >
      <div class="fg-tpl-editor">
        <section class="fg-tpl-editor__form">
          <FgDataForm
            ref="formRef"
            :fields="formFields"
            :mode="formMode"
            :module-id="moduleId"
            :initial-data="current"
          />
        </section>
        <aside class="fg-tpl-editor__preview">
          <div class="fg-tpl-editor__preview-head">
            <div class="fg-tpl-editor__preview-title">渠道效果</div>
            <div class="fg-tpl-editor__preview-desc">按种类模拟真实通知外观，正文类型即时生效</div>
          </div>
          <TemplatePreviewPane
            v-if="formModel"
            compact
            sample
            :subject="modelText(formModel, 'subjectText')"
            :body="modelText(formModel, 'bodyText')"
            :content-type="modelText(formModel, 'contentType') || 'text'"
            :channel-kind="modelText(formModel, 'channelKind') || 'webhook'"
          />
        </aside>
      </div>
      <template #footer>
        <RsButton size="sm" @click="closeForm">{{ formMode === 'view' ? '关闭' : '取消' }}</RsButton>
        <RsButton v-if="canSave" variant="primary" size="sm" @click="saveForm">保存</RsButton>
      </template>
    </RsDialog>

    <RsDialog
      v-model:open="previewOpen"
      title="模板预览"
      width="560px"
      :modal="false"
      :teleport-to="`#${moduleId}-layer`"
    >
      <TemplatePreviewPane
        :subject="previewSubject"
        :body="previewBody"
        :content-type="previewContent"
        :channel-kind="previewChannel"
      />
      <template #footer>
        <RsButton size="sm" @click="previewOpen = false">关闭</RsButton>
      </template>
    </RsDialog>
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-template {
  position: relative;
  z-index: 0;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.fg-layer {
  position: absolute;
  inset: 0;
  z-index: 30;
  pointer-events: none;
}
.fg-layer:deep(> *) {
  pointer-events: auto;
}
.fg-tpl-editor {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(300px, 0.95fr);
  gap: 18px;
  align-items: start;
  min-height: 420px;
}
.fg-tpl-editor__form {
  min-width: 0;
  max-height: min(68vh, 640px);
  overflow: auto;
  padding-right: 4px;
}
.fg-tpl-editor__preview {
  position: sticky;
  top: 0;
  min-width: 0;
  padding: 14px;
  border-radius: 12px;
  border: 1px solid var(--rs-border, #e5e7eb);
  background:
    radial-gradient(120% 80% at 100% 0%, color-mix(in srgb, var(--rs-primary, #2563eb) 8%, transparent), transparent 55%),
    var(--rs-surface-muted, #f8fafc);
}
.fg-tpl-editor__preview-head {
  margin-bottom: 12px;
}
.fg-tpl-editor__preview-title {
  font-size: 13px;
  font-weight: 650;
  color: var(--rs-text, #0f172a);
}
.fg-tpl-editor__preview-desc {
  margin-top: 2px;
  font-size: 12px;
  color: var(--rs-muted, #64748b);
}
@media (max-width: 900px) {
  .fg-tpl-editor {
    grid-template-columns: 1fr;
  }
  .fg-tpl-editor__preview {
    position: static;
  }
}
</style>
