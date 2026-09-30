import { ref } from 'vue'
import { useRsToast } from '@/ui'
import { useSessionStore } from '@/stores/session'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { queryHostDetail, queryHostSeries, type HostCurve, type HostDetail, type HostMonitor, type HostSeries, type SeriesRange } from '../api'

export function useHostDetail() {
  const toast = useRsToast()
  const session = useSessionStore()
  const detail = ref<HostDetail | null>(null)
  const curves = ref<HostCurve[]>([])
  const error = ref('')
  const loading = ref(false)
  let seq = 0

  function clear() {
    seq += 1
    detail.value = null
    curves.value = []
    error.value = ''
    loading.value = false
  }

  async function load(row: HostMonitor, keep = false, range?: SeriesRange | null) {
    if (!session.allowButton('fg0005:view')) return
    const current = ++seq
    error.value = ''
    if (!keep) {
      detail.value = null
      curves.value = []
      loading.value = true
    }
    try {
      const result = await queryHostDetail(row.agentId)
      if (current !== seq) return
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载明细失败')
        toast.error(error.value)
        return
      }
      detail.value = parseApiData<HostDetail>(result) ?? null
      const series = await queryHostSeries(row.agentId, range)
      if (current !== seq) return
      if (!isApiSuccess(series)) {
        error.value = getApiMessage(series, '加载曲线失败')
        toast.error(error.value)
        return
      }
      curves.value = parseApiData<HostSeries>(series)?.curves ?? []
    } catch (err) {
      if (current !== seq) return
      error.value = getHttpErrorMessage(err, '加载明细失败')
      toast.error(error.value)
    } finally {
      if (current === seq) loading.value = false
    }
  }

  return { detail, curves, error, loading, load, clear }
}
