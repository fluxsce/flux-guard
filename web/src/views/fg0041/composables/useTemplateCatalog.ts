import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { auditFields, basicTab, tabOf } from '@/components/form/audit'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import { placeSavedRow } from '@/components/grid/placeRow'
import type { FgGridMenuItem } from '@/components/grid/types'
import { rsConfirm, useRsToast } from '@/ui'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { createTemplate, deleteTemplate, previewTemplate, queryTemplates, updateTemplate, type Template, type TemplateInput } from '../api'

export type FlagOption = { label: string; value: string }

export const statusOptions: FlagOption[] = [
  { label: '启用', value: 'Y' },
  { label: '停用', value: 'N' },
]
export const kindOptions: FlagOption[] = [
  { label: '邮件', value: 'email' },
  { label: '企业微信', value: 'wecom' },
  { label: '钉钉', value: 'dingtalk' },
  { label: 'QQ', value: 'qq' },
  { label: '飞书', value: 'feishu' },
  { label: 'Webhook', value: 'webhook' },
  { label: '短信', value: 'sms' },
]
export const contentOptions: FlagOption[] = [
  { label: '文本', value: 'text' },
  { label: 'Markdown', value: 'markdown' },
  { label: 'HTML', value: 'html' },
]
export const builtInOptions: FlagOption[] = [
  { label: '内置', value: 'Y' },
  { label: '自定义', value: 'N' },
]

export const templateColumns = [
  { key: 'templateName', title: '名称', minWidth: 140 },
  { key: 'channelKind', title: '种类', width: 110 },
  { key: 'contentType', title: '正文类型', width: 110 },
  { key: 'subjectText', title: '标题', minWidth: 180 },
  { key: 'builtInFlag', title: '来源', width: 88 },
  { key: 'statusFlag', title: '状态', width: 88 },
  { key: 'noteText', title: '备注', minWidth: 140 },
]

export const templateMenuItems: FgGridMenuItem[] = [
  { key: 'view', label: '查看', icon: 'eye' },
  { key: 'edit', label: '修改', icon: 'pencil' },
  { key: 'preview', label: '预览', icon: 'file-text' },
  { key: 'delete', label: '删除', icon: 'trash', disabled: (row) => (row as Template | null)?.builtInFlag === 'Y' },
]

export const templateFormFields: FgFormField[] = [
  ...tabOf(basicTab, [
    { field: 'templateId', label: '编码', primary: true, placeholder: '留空则自动生成' },
    { field: 'templateName', label: '名称', required: true },
    { field: 'channelKind', label: '种类', type: 'select', required: true, options: kindOptions },
    { field: 'contentType', label: '正文类型', type: 'select', required: true, options: contentOptions },
    { field: 'subjectText', label: '标题', span: 24, required: true, placeholder: '可用 {{level}} {{scene}} {{ruleName}} {{hostName}}' },
    { field: 'bodyText', label: '正文', type: 'textarea', span: 24, required: true, placeholder: '邮件用 HTML，企业微信和钉钉用 Markdown' },
    { field: 'statusFlag', label: '状态', type: 'select', options: statusOptions },
    { field: 'noteText', label: '备注', type: 'textarea', span: 24 },
  ]),
  { field: 'builtInFlag', label: '来源', type: 'select', options: builtInOptions, disabled: true, tab: '其它信息' },
  ...auditFields,
]

export const templateSearchFields: FgSearchField[] = [
  { field: 'templateName', label: '名称', placeholder: '请输入名称' },
  { field: 'channelKind', label: '种类', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...kindOptions] },
  { field: 'statusFlag', label: '状态', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...statusOptions] },
]

export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

function text(row: Template, key: keyof Template) {
  const value = row[key]
  if (typeof value === 'string') return value
  return ''
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

export function useTemplateCatalog() {
  const toast = useRsToast()
  const rows = ref<Template[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ templateName: '', channelKind: '', statusFlag: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const current = ref<Record<string, unknown>>({})
  const active = ref<Template | null>(null)
  const previewOpen = ref(false)
  const previewSubject = ref('')
  const previewBody = ref('')
  const previewContent = ref('text')
  const previewChannel = ref('webhook')
  const pageRows = computed(() => rows.value)

  function fill(row: Template) {
    return {
      templateId: text(row, 'templateId'),
      templateName: text(row, 'templateName'),
      channelKind: text(row, 'channelKind'),
      contentType: text(row, 'contentType') || 'text',
      subjectText: text(row, 'subjectText'),
      bodyText: text(row, 'bodyText'),
      builtInFlag: text(row, 'builtInFlag'),
      statusFlag: text(row, 'statusFlag'),
      noteText: text(row, 'noteText'),
      addTime: text(row, 'addTime'),
      addWho: text(row, 'addWho'),
      editTime: text(row, 'editTime'),
      editWho: text(row, 'editWho'),
      currentVersion: row.currentVersion == null ? '' : String(row.currentVersion),
      oprSeqFlag: text(row, 'oprSeqFlag'),
    }
  }

  function openView(row: Template) {
    active.value = row
    formMode.value = 'view'
    current.value = fill(row)
    formOpen.value = true
  }

  function openCreate() {
    formMode.value = 'create'
    current.value = {
      templateId: '',
      templateName: '',
      channelKind: 'email',
      contentType: 'html',
      subjectText: '',
      bodyText: '',
      statusFlag: 'Y',
      noteText: '',
    }
    formOpen.value = true
  }

  function openEdit(row: Template) {
    active.value = row
    formMode.value = 'edit'
    current.value = fill(row)
    formOpen.value = true
  }

  function inputOf(values: Record<string, unknown>): TemplateInput {
    return {
      templateId: str(values.templateId).trim(),
      templateName: str(values.templateName).trim(),
      channelKind: str(values.channelKind),
      subjectText: str(values.subjectText).trim(),
      bodyText: str(values.bodyText),
      contentType: str(values.contentType) || 'text',
      statusFlag: str(values.statusFlag) || 'Y',
      activeFlag: 'Y',
      noteText: str(values.noteText).trim(),
    }
  }

  async function reload() {
    const result = await queryTemplates(listParams(page.value, pageSize.value, query.value))
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const data = parseApiData<PageData<Template>>(result)
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
    if (active.value && !rows.value.some((item) => item.templateId === active.value?.templateId)) {
      active.value = null
    }
  }

  function placeTemplate(saved: Template) {
    const placed = placeSavedRow(rows.value, saved, (item) => item.templateId === saved.templateId)
    rows.value = placed.rows
    if (placed.created) {
      total.value += 1
      page.value = 1
    }
    active.value = saved
  }

  async function submitTemplate(values: Record<string, unknown>) {
    error.value = ''
    try {
      const body = inputOf(values)
      const result = formMode.value === 'create' ? await createTemplate(body) : await updateTemplate(body)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const saved = result.data
      if (!saved?.templateId) {
        error.value = '保存失败'
        toast.error(error.value)
        return
      }
      placeTemplate(saved)
      formOpen.value = false
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    }
  }

  async function removeTemplate(row: Template) {
    if (row.builtInFlag === 'Y') {
      error.value = '内置模板不能删除'
      toast.error(error.value)
      return
    }
    const ok = await rsConfirm.confirm({
      title: '删除模板',
      description: `确定删除模板 ${row.templateName}？`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0041',
    })
    if (!ok) return
    error.value = ''
    try {
      const result = await deleteTemplate(row.templateId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '删除失败')
        toast.error(error.value)
        return
      }
      if (active.value?.templateId === row.templateId) active.value = null
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '删除失败')
      toast.error(error.value)
    }
  }

  async function preview(row: Template) {
    error.value = ''
    try {
      const result = await previewTemplate({
        templateId: row.templateId,
        channelKind: row.channelKind,
        subjectText: row.subjectText,
        bodyText: row.bodyText,
        contentType: row.contentType,
      })
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '预览失败')
        toast.error(error.value)
        return
      }
      previewSubject.value = result.data?.subjectText ?? ''
      previewBody.value = result.data?.bodyText ?? ''
      previewContent.value = result.data?.contentType || row.contentType || 'text'
      previewChannel.value = row.channelKind || 'webhook'
      previewOpen.value = true
    } catch (err) {
      error.value = getHttpErrorMessage(err, '预览失败')
      toast.error(error.value)
    }
  }

  function onRow(row: Template) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      templateName: str(values.templateName),
      channelKind: str(values.channelKind),
      statusFlag: str(values.statusFlag),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { templateName: '', channelKind: '', statusFlag: '' }
  }

  watch([page, pageSize, query], () => {
    void reload()
  })

  watch(pageSizePolicy, (policy) => {
    if (pageSize.value > policy.maxPageSize || pageSize.value < 1) {
      pageSize.value = policy.defaultPageSize
    }
  })

  onMounted(async () => {
    try {
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  })

  return {
    page,
    pageSize,
    total,
    error,
    formOpen,
    formMode,
    current,
    active,
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
  }
}
