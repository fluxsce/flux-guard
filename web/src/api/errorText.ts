import { getCurrentLocale } from '@/locales'
import enUS from '@/locales/en-US/error'
import zhCN from '@/locales/zh-CN/error'

const catalogs: Record<string, Record<string, string>> = {
  'zh-CN': zhCN,
  'en-US': enUS,
}

/**
 * 按稳定错误码取当前语言的文案。没有对应码时用兜底，通常是后端默认中文。
 * @param code - 失败响应里的 code
 * @param fallback - 没有译文时显示的文案
 * @returns 当前语言的错误说明
 */
export function messageOfCode(code: string | undefined, fallback: string): string {
  const key = code?.trim()
  if (!key) return fallback
  const text = catalogs[getCurrentLocale()]?.[key]
  return text && text.trim() ? text : fallback
}
