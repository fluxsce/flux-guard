import { onMounted, ref } from 'vue'
import { API_PREFIX } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'

export interface ProductBuild {
  version: string
  commit: string
  date: string
  /** 中央发行包平台，与 agents/ 包装一致。 */
  goos?: string
  goarch?: string
}

let pending: Promise<ProductBuild | null> | undefined

export function loadProductBuild(): Promise<ProductBuild | null> {
  if (!pending) {
    pending = fetch(`${API_PREFIX}version`, { credentials: 'include' })
      .then(async (res) => {
        if (!res.ok) return null
        const body = (await res.json()) as ApiResult<ProductBuild>
        if (!body.ok || !body.data?.version) return null
        return body.data
      })
      .catch(() => null)
  }
  return pending
}

export function useProductVersion() {
  const version = ref(__FLUX_GUARD_VERSION__)
  onMounted(() => {
    void loadProductBuild().then((build) => {
      if (build?.version) version.value = build.version
    })
  })
  return { version }
}
