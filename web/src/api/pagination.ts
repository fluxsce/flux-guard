import { computed, ref } from 'vue'

/** 与后端 PageSizeCeiling 一致。 */
export const PAGE_SIZE_CEILING = 200

const PAGE_SIZE_STEPS = [10, 20, 50, 100, 200] as const

const policy = ref({
  defaultPageSize: 20,
  maxPageSize: PAGE_SIZE_CEILING,
})

function clamp(value: number | undefined, min: number, max: number, fallback: number) {
  if (value == null || Number.isNaN(value)) return fallback
  const next = Math.floor(value)
  if (next < min) return min
  if (next > max) return max
  return next
}

/** 用环境设置覆盖列表分页。 */
export function applyPageSizePolicy(defaultPageSize?: number, maxPageSize?: number) {
  const max = clamp(maxPageSize, 1, PAGE_SIZE_CEILING, PAGE_SIZE_CEILING)
  let next = clamp(defaultPageSize, 1, max, 20)
  if (next > max) next = max
  policy.value = { defaultPageSize: next, maxPageSize: max }
}

export function getDefaultPageSize() {
  return policy.value.defaultPageSize
}

export const pageSizePolicy = computed(() => policy.value)

export type PageData<T> = {
  items: T[]
  total: number
  page: number
  pageSize: number
}

/** 列表查询参数。空筛选不传。sort 只在主机这类可排序列表上使用。 */
export function listParams(
  page: number,
  pageSize: number,
  filters: Record<string, string>,
  sort?: { key: string; order: 'asc' | 'desc' } | null,
) {
  const params: Record<string, string | number> = { page, pageSize }
  for (const [key, value] of Object.entries(filters)) {
    const text = value.trim()
    if (text) params[key] = text
  }
  if (sort?.key) {
    params.orderBy = sort.key
    if (sort.order === 'desc') params.desc = 1
  }
  return params
}

export const pageSizeChoices = computed(() => {
  const max = policy.value.maxPageSize
  const options: number[] = PAGE_SIZE_STEPS.filter((size) => size <= max)
  const current = policy.value.defaultPageSize
  if (current <= max && !options.includes(current)) {
    options.push(current)
    options.sort((a, b) => a - b)
  }
  return options.length > 0 ? options : [current]
})
