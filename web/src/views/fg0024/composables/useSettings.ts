import { computed, onMounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData, updateTimeout } from '@/api/request'
import { applyPageSizePolicy } from '@/api/pagination'
import { useRsToast, type RsMenuItem } from '@/ui'
import {
  querySettings,
  saveIngestToken,
  saveSettings,
  type EnvVars,
  type IngestToken,
  type Settings,
  type WebTimeout,
} from '../api'

export const settingsGroups: RsMenuItem[] = [
  { key: 'webTimeout', label: 'Web访问设置', icon: 'timer' },
  { key: 'envVars', label: '全局环境变量', icon: 'key' },
  { key: 'ingestToken', label: '上报令牌', icon: 'shield' },
]

export function useSettings() {
  const toast = useRsToast()
  const tab = ref('webTimeout')
  const activeLabel = computed(() => settingsGroups.find((item) => item.key === tab.value)?.label || '环境设置')
  const form = ref<WebTimeout>({
    requestTimeoutSeconds: 120,
    sessionExpireHours: 12,
    defaultPageSize: 20,
    maxPageSize: 200,
    currentVersion: 0,
    editable: false,
  })
  const envVars = ref<EnvVars>({ items: [], currentVersion: 0 })
  const ingestToken = ref<IngestToken>({ configured: false, statusFlag: 'Y', hint: '', currentVersion: 0 })
  const error = ref('')
  const saving = ref(false)

  function applyTimeout(value: WebTimeout) {
    form.value = value
    updateTimeout(value.requestTimeoutSeconds * 1000)
    applyPageSizePolicy(value.defaultPageSize, value.maxPageSize)
  }

  function applyAll(data: Settings) {
    applyTimeout({ ...data.webTimeout, editable: data.editable })
    envVars.value = {
      items: data.envVars?.items ?? [],
      currentVersion: data.envVars?.currentVersion ?? 0,
    }
    ingestToken.value = data.ingestToken ?? { configured: false, statusFlag: 'Y', hint: '', currentVersion: 0 }
  }

  async function load() {
    error.value = ''
    try {
      const result = await querySettings()
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<Settings>(result)
      if (data) applyAll(data)
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  async function save() {
    saving.value = true
    error.value = ''
    try {
      const current = form.value
      const result = await saveSettings({
        requestTimeoutSeconds: Number(current.requestTimeoutSeconds),
        sessionExpireHours: Number(current.sessionExpireHours),
        defaultPageSize: Number(current.defaultPageSize),
        maxPageSize: Number(current.maxPageSize),
        currentVersion: current.currentVersion,
      })
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<WebTimeout>(result)
      if (data) applyTimeout({ ...data, editable: true })
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    } finally {
      saving.value = false
    }
  }

  function onEnvSaved(vars: EnvVars) {
    error.value = ''
    envVars.value = { items: vars.items ?? [], currentVersion: vars.currentVersion }
  }

  async function changeIngest(action: 'rotate' | 'enable' | 'disable') {
    saving.value = true
    error.value = ''
    try {
      const result = await saveIngestToken(ingestToken.value.currentVersion, action)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<IngestToken>(result)
      if (data) ingestToken.value = data
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    } finally {
      saving.value = false
    }
  }

  onMounted(load)

  return {
    tab,
    activeLabel,
    form,
    envVars,
    ingestToken,
    error,
    saving,
    save,
    onEnvSaved,
    changeIngest,
  }
}
