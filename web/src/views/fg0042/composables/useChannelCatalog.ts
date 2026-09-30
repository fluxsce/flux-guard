import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { auditFields, basicTab, fieldset, tabOf } from '@/components/form/audit'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import { placeSavedRow } from '@/components/grid/placeRow'
import type { FgGridMenuItem } from '@/components/grid/types'
import { rsConfirm, useRsToast } from '@/ui'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import {
  createChannel,
  deleteChannel,
  queryChannels,
  testChannel,
  updateChannel,
  type Channel,
  type ChannelInput,
} from '../api'
import { queryTemplates } from '@/views/fg0041/api'

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
export const secretOptions: FlagOption[] = [
  { label: '已设置', value: 'Y' },
  { label: '未设置', value: 'N' },
]
export const qqTargetOptions: FlagOption[] = [
  { label: '群', value: 'group' },
  { label: '私聊', value: 'private' },
]
export const tlsOptions: FlagOption[] = [
  { label: '否', value: 'N' },
  { label: '是（STARTTLS）', value: 'Y' },
]

const kindIs = (...kinds: string[]) => (formData: Record<string, unknown>) =>
  kinds.includes(String(formData.channelKind ?? ''))

export const channelColumns = [
  { key: 'channelName', title: '名称', minWidth: 140 },
  { key: 'channelKind', title: '种类', width: 110 },
  { key: 'templateName', title: '模板', minWidth: 140 },
  { key: 'endpoint', title: '地址', minWidth: 180 },
  { key: 'target', title: '目标', minWidth: 140 },
  { key: 'secretSet', title: '密钥', width: 88 },
  { key: 'statusFlag', title: '状态', width: 88 },
  { key: 'noteText', title: '备注', minWidth: 140 },
]

export const channelMenuItems: FgGridMenuItem[] = [
  { key: 'view', label: '查看', icon: 'eye' },
  { key: 'edit', label: '修改', icon: 'pencil' },
  { key: 'delete', label: '删除', icon: 'trash' },
  { key: 'test', label: '测试', icon: 'send' },
]

export function channelFormFields(templates: FlagOption[]): FgFormField[] {
  return [
    ...tabOf(basicTab, [
      fieldset('basicGroup', '基本信息', [
        { field: 'channelId', label: '编码', primary: true, placeholder: '留空则自动生成' },
        { field: 'channelName', label: '名称', required: true },
        { field: 'channelKind', label: '种类', type: 'select', required: true, options: kindOptions },
        {
          field: 'templateId',
          label: '模板',
          type: 'select',
          span: 16,
          options: [{ label: '内置模板', value: '' }, ...templates],
        },
        { field: 'templateName', label: '模板名称', disabled: true, modes: ['view'] },
        { field: 'statusFlag', label: '状态', type: 'select', options: statusOptions },
        { field: 'noteText', label: '备注', type: 'textarea', span: 24 },
      ]),
      fieldset(
        'emailGroup',
        '邮件 SMTP',
        [
          { field: 'endpoint', label: 'SMTP 主机', span: 16, required: true, placeholder: 'smtp.example.com' },
          { field: 'port', label: '端口', type: 'number', placeholder: '465' },
          { field: 'username', label: '账号' },
          {
            field: 'secret',
            label: '密码',
            type: 'password',
            modes: ['create', 'edit'],
            placeholder: '留空表示不修改',
          },
          {
            field: 'secretSet',
            label: '密码',
            type: 'select',
            options: secretOptions,
            disabled: true,
            modes: ['view'],
          },
          { field: 'sender', label: '发件人', placeholder: 'ops@example.com' },
          { field: 'target', label: '收件人', span: 16, placeholder: '多个用逗号分隔' },
          { field: 'optionText', label: 'STARTTLS', type: 'select', options: tlsOptions },
        ],
        { tips: '465 走隐式 TLS；587 且 STARTTLS 选「是」', show: kindIs('email') },
      ),
      fieldset(
        'botGroup',
        '机器人 Webhook',
        [
          { field: 'endpoint', label: 'Webhook 地址', span: 24, required: true, placeholder: 'https://…' },
          {
            field: 'secret',
            label: '加签密钥',
            type: 'password',
            modes: ['create', 'edit'],
            placeholder: '钉钉 / 飞书可选，留空表示不修改',
          },
          {
            field: 'secretSet',
            label: '加签密钥',
            type: 'select',
            options: secretOptions,
            disabled: true,
            modes: ['view'],
          },
        ],
        { tips: '企业微信、钉钉、飞书填机器人 Webhook', show: kindIs('wecom', 'dingtalk', 'feishu') },
      ),
      fieldset(
        'qqGroup',
        'QQ OneBot',
        [
          { field: 'endpoint', label: 'OneBot 地址', span: 24, required: true, placeholder: 'http://127.0.0.1:5700' },
          {
            field: 'secret',
            label: 'accessToken',
            type: 'password',
            modes: ['create', 'edit'],
            placeholder: '留空表示不修改',
          },
          {
            field: 'secretSet',
            label: 'accessToken',
            type: 'select',
            options: secretOptions,
            disabled: true,
            modes: ['view'],
          },
          { field: 'optionText', label: '目标类型', type: 'select', options: qqTargetOptions },
          { field: 'target', label: '群号 / QQ 号', span: 16, placeholder: '与目标类型对应' },
        ],
        { tips: '走 OneBot HTTP，不是官方 QQ 开放平台', show: kindIs('qq') },
      ),
      fieldset(
        'webhookGroup',
        '通用 Webhook',
        [
          { field: 'endpoint', label: '请求地址', span: 24, required: true, placeholder: 'https://…' },
          {
            field: 'secret',
            label: 'Token',
            type: 'password',
            modes: ['create', 'edit'],
            placeholder: '可选，留空表示不修改',
          },
          {
            field: 'secretSet',
            label: 'Token',
            type: 'select',
            options: secretOptions,
            disabled: true,
            modes: ['view'],
          },
          {
            field: 'extraText',
            label: '扩展 JSON',
            type: 'textarea',
            span: 24,
            placeholder: '例如 {"bodyMode":"raw"}',
          },
        ],
        { tips: '自定义 HTTP 回调', show: kindIs('webhook') },
      ),
      fieldset(
        'smsGroup',
        '短信网关',
        [
          { field: 'endpoint', label: '网关地址', span: 24, required: true, placeholder: 'https://sms-gateway/…' },
          {
            field: 'secret',
            label: 'Token',
            type: 'password',
            modes: ['create', 'edit'],
            placeholder: '可选，留空表示不修改',
          },
          {
            field: 'secretSet',
            label: 'Token',
            type: 'select',
            options: secretOptions,
            disabled: true,
            modes: ['view'],
          },
          { field: 'target', label: '手机号', span: 16, placeholder: '多个用逗号分隔' },
        ],
        { tips: '对配置的短信网关 POST，不是运营商 SDK', show: kindIs('sms') },
      ),
    ]),
    ...auditFields,
  ]
}

export const channelSearchFields: FgSearchField[] = [
  { field: 'channelName', label: '名称', placeholder: '请输入名称' },
  { field: 'channelKind', label: '种类', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...kindOptions] },
  { field: 'statusFlag', label: '状态', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...statusOptions] },
]
export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

function text(row: Channel, key: keyof Channel) {
  const value = row[key]
  if (typeof value === 'string') return value
  if (typeof value === 'number') return value.toString()
  return ''
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

export function useChannelCatalog() {
  const toast = useRsToast()
  const rows = ref<Channel[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ channelName: '', channelKind: '', statusFlag: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const current = ref<Record<string, unknown>>({})
  const active = ref<Channel | null>(null)
  const templateOptions = ref<FlagOption[]>([])

  const pageRows = computed(() => rows.value)
  const formFields = computed(() => channelFormFields(templateOptions.value))

  function fill(row: Channel) {
    return {
      channelId: text(row, 'channelId'),
      channelName: text(row, 'channelName'),
      channelKind: text(row, 'channelKind'),
      endpoint: text(row, 'endpoint'),
      port: row.port,
      username: text(row, 'username'),
      secret: '',
      sender: text(row, 'sender'),
      target: text(row, 'target'),
      optionText: text(row, 'optionText'),
      extraText: text(row, 'extraText'),
      templateId: text(row, 'templateId'),
      templateName: text(row, 'templateName'),
      secretSet: text(row, 'secretSet'),
      statusFlag: text(row, 'statusFlag'),
      noteText: text(row, 'noteText'),
      addTime: text(row, 'addTime'),
      addWho: text(row, 'addWho'),
      editTime: text(row, 'editTime'),
      editWho: text(row, 'editWho'),
      currentVersion: row.currentVersion,
      oprSeqFlag: text(row, 'oprSeqFlag'),
    }
  }

  function openView(row: Channel) {
    active.value = row
    formMode.value = 'view'
    current.value = fill(row)
    formOpen.value = true
  }

  function openCreate() {
    formMode.value = 'create'
    current.value = {
      channelId: '',
      channelName: '',
      channelKind: 'email',
      endpoint: '',
      port: 465,
      username: '',
      secret: '',
      sender: '',
      target: '',
      optionText: 'N',
      extraText: '',
      templateId: '',
      statusFlag: 'Y',
      activeFlag: 'Y',
      noteText: '',
    }
    formOpen.value = true
  }

  function openEdit(row: Channel) {
    active.value = row
    formMode.value = 'edit'
    current.value = fill(row)
    formOpen.value = true
  }

  function inputOf(values: Record<string, unknown>): ChannelInput {
    const port = Number(str(values.port) || '0')
    const kind = str(values.channelKind)
    let optionText = str(values.optionText).trim()
    if (kind === 'email' && optionText !== 'Y' && optionText !== 'N') optionText = 'N'
    if (kind === 'qq' && optionText !== 'group' && optionText !== 'private') optionText = 'group'
    return {
      channelId: str(values.channelId).trim(),
      channelName: str(values.channelName).trim(),
      channelKind: kind,
      endpoint: str(values.endpoint).trim(),
      port: Number.isFinite(port) ? port : 0,
      username: str(values.username).trim(),
      secret: str(values.secret),
      sender: str(values.sender).trim(),
      target: str(values.target).trim(),
      optionText,
      extraText: str(values.extraText).trim(),
      templateId: str(values.templateId).trim(),
      statusFlag: str(values.statusFlag) || 'Y',
      activeFlag: 'Y',
      noteText: str(values.noteText).trim(),
    }
  }

  async function reload() {
    const result = await queryChannels(listParams(page.value, pageSize.value, query.value))
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const data = parseApiData<PageData<Channel>>(result)
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
    if (active.value && !rows.value.some((item) => item.channelId === active.value?.channelId)) {
      active.value = null
    }
  }

  function placeChannel(saved: Channel) {
    const placed = placeSavedRow(rows.value, saved, (item) => item.channelId === saved.channelId)
    rows.value = placed.rows
    if (placed.created) {
      total.value += 1
      page.value = 1
    }
    active.value = saved
  }

  async function submitChannel(values: Record<string, unknown>) {
    error.value = ''
    try {
      const body = inputOf(values)
      const result = formMode.value === 'create' ? await createChannel(body) : await updateChannel(body)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const saved = result.data
      if (!saved?.channelId) {
        error.value = '保存失败'
        toast.error(error.value)
        return
      }
      placeChannel(saved)
      formOpen.value = false
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    }
  }

  async function removeChannel(row: Channel) {
    const ok = await rsConfirm.confirm({
      title: '删除渠道',
      description: `确定删除渠道 ${row.channelName}？`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0042',
    })
    if (!ok) return
    error.value = ''
    try {
      const result = await deleteChannel(row.channelId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '删除失败')
        toast.error(error.value)
        return
      }
      if (active.value?.channelId === row.channelId) active.value = null
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '删除失败')
      toast.error(error.value)
    }
  }

  async function sendTest(row: Channel) {
    error.value = ''
    try {
      const result = await testChannel(row.channelId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '发送失败')
        toast.error(error.value)
        return
      }
      toast.success('测试消息已提交，结果见预警事件页的投递记录')
    } catch (err) {
      error.value = getHttpErrorMessage(err, '发送失败')
      toast.error(error.value)
    }
  }

  function onRow(row: Channel) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      channelName: str(values.channelName),
      channelKind: str(values.channelKind),
      statusFlag: str(values.statusFlag),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { channelName: '', channelKind: '', statusFlag: '' }
  }

  watch([page, pageSize, query], () => {
    void reload()
  })

  watch(pageSizePolicy, (policy) => {
    if (pageSize.value > policy.maxPageSize || pageSize.value < 1) {
      pageSize.value = policy.defaultPageSize
    }
  })

  async function loadTemplates() {
    const result = await queryTemplates(listParams(1, 200, { statusFlag: 'Y' }))
    if (!isApiSuccess(result)) return
    const data = parseApiData<PageData<{ templateId: string; templateName: string; channelKind: string }>>(result)
    templateOptions.value = (data?.items ?? []).map((item) => ({
      value: item.templateId,
      label: `${item.templateName}（${item.channelKind}）`,
    }))
  }

  onMounted(async () => {
    try {
      await loadTemplates()
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
    submitChannel,
    removeChannel,
    sendTest,
    formFields,
    onRow,
    onSearch,
    onReset,
  }
}
