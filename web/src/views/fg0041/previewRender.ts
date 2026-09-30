/** 与后端 SampleVars 对齐，编辑态本地预览用。 */
export const sampleVars: Record<string, string> = {
  ruleName: 'CPU 使用率过高',
  level: '严重',
  levelCode: 'critical',
  metric: 'cpu.usage',
  metricName: '主机 CPU 使用率',
  value: '95',
  threshold: '80',
  operator: '>',
  hostName: 'web-01',
  agentId: 'agent-local',
  firedAt: '2026-09-28 15:04:05',
  status: '触发',
  message: '主机 web-01 主机 CPU 使用率 当前 95，阈值 > 80',
  scene: '触发',
  contactNames: '张三',
}

export type ChannelKind = 'email' | 'wecom' | 'dingtalk' | 'qq' | 'feishu' | 'webhook' | 'sms' | string

export const channelLabels: Record<string, string> = {
  email: '邮件',
  wecom: '企业微信',
  dingtalk: '钉钉',
  qq: 'QQ',
  feishu: '飞书',
  webhook: 'Webhook',
  sms: '短信',
}

export const contentLabels: Record<string, string> = {
  text: '文本',
  markdown: 'Markdown',
  html: 'HTML',
}

const placeholder = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g

export function applyPlaceholders(text: string, vars: Record<string, string> = sampleVars) {
  if (!text) return text
  return text.replace(placeholder, (all, name: string) => (name in vars ? vars[name] : all))
}

export function escapeHtml(text: string) {
  return text
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

function inlineMarkdown(text: string) {
  return text
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
}

/** 覆盖预警模板常用的粗体、引用、换行，不做完整 CommonMark。 */
export function markdownToHtml(source: string) {
  const lines = escapeHtml(source).split(/\r?\n/)
  const out: string[] = []
  let inQuote = false
  const closeQuote = () => {
    if (!inQuote) return
    out.push('</blockquote>')
    inQuote = false
  }
  for (const line of lines) {
    const quote = /^&gt;\s?(.*)$/.exec(line)
    if (quote) {
      if (!inQuote) {
        out.push('<blockquote>')
        inQuote = true
      }
      out.push(`<p>${inlineMarkdown(quote[1])}</p>`)
      continue
    }
    closeQuote()
    if (line.trim() === '') {
      out.push('<div class="gap"></div>')
      continue
    }
    out.push(`<p>${inlineMarkdown(line)}</p>`)
  }
  closeQuote()
  return out.join('')
}

export function bodyToHtml(contentType: string, body: string) {
  const kind = (contentType || 'text').toLowerCase()
  if (kind === 'html') return body
  if (kind === 'markdown') return markdownToHtml(body)
  return `<pre class="plain">${escapeHtml(body)}</pre>`
}

function themeCss(channelKind: string): string {
  const kind = (channelKind || '').toLowerCase()
  if (kind === 'email') {
    return `
body{font:14px/1.6 "Segoe UI",system-ui,-apple-system,sans-serif;color:#1f2937;background:#fff;padding:18px 20px}
blockquote{margin:10px 0;padding:8px 12px;border-left:3px solid #93c5fd;background:#f8fafc;color:#334155;border-radius:0 6px 6px 0}
p{margin:0 0 10px} .gap{height:8px}
code{font:12px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;padding:1px 5px;border-radius:4px;background:#f1f5f9}
.plain{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit}
strong{font-weight:650}
a{color:#2563eb}
`
  }
  if (kind === 'sms') {
    return `
body{font:14px/1.45 system-ui,-apple-system,sans-serif;color:#111827;background:transparent;padding:4px 2px}
p{margin:0 0 6px} .gap{height:4px}
blockquote{margin:6px 0;padding:0;border:0;color:#374151}
.plain{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit}
strong{font-weight:650}
`
  }
  if (kind === 'webhook') {
    return `
body{font:12.5px/1.55 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;color:#e2e8f0;background:transparent;padding:2px}
p{margin:0 0 8px} .gap{height:6px}
blockquote{margin:8px 0;padding:8px 10px;border-left:2px solid #38bdf8;background:rgba(56,189,248,.08);color:#cbd5e1}
code{color:#7dd3fc;background:rgba(148,163,184,.15);padding:1px 4px;border-radius:3px}
.plain{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit;color:#cbd5e1}
strong{color:#f8fafc;font-weight:650}
`
  }
  // IM：企业微信 / 钉钉 / 飞书 / QQ
  return `
body{font:14px/1.55 system-ui,-apple-system,"PingFang SC","Segoe UI",sans-serif;color:#1f2329;background:transparent;padding:2px 0}
p{margin:0 0 8px} .gap{height:6px}
blockquote{margin:8px 0;padding:8px 10px;border-left:3px solid #c9cdd4;background:#f5f6f7;color:#646a73;border-radius:0 6px 6px 0}
code{font:12px/1.4 ui-monospace,Menlo,monospace;padding:1px 4px;border-radius:3px;background:#eef0f2}
.plain{margin:0;white-space:pre-wrap;overflow-wrap:anywhere;font:inherit}
strong{font-weight:650}
`
}

export function previewSrcdoc(
  contentType: string,
  body: string,
  options?: { sample?: boolean; channelKind?: string },
) {
  const raw = options?.sample ? applyPlaceholders(body) : body
  const html = bodyToHtml(contentType, raw)
  const css = themeCss(options?.channelKind || '')
  return `<!doctype html><html><head><meta charset="utf-8"><style>
html,body{margin:0;min-height:100%;box-sizing:border-box}
*,*::before,*::after{box-sizing:border-box}
${css}
</style></head><body>${html || '<p style="opacity:.45">暂无正文</p>'}</body></html>`
}

export type PreviewSkin = 'email' | 'im' | 'sms' | 'webhook'

export function previewSkin(channelKind: string): PreviewSkin {
  switch ((channelKind || '').toLowerCase()) {
    case 'email':
      return 'email'
    case 'sms':
      return 'sms'
    case 'webhook':
      return 'webhook'
    default:
      return 'im'
  }
}

export function imBrand(channelKind: string) {
  switch ((channelKind || '').toLowerCase()) {
    case 'wecom':
      return { name: '企业微信', accent: '#2b7de9', bot: '预警机器人' }
    case 'dingtalk':
      return { name: '钉钉', accent: '#0089ff', bot: '预警机器人' }
    case 'feishu':
      return { name: '飞书', accent: '#3370ff', bot: '预警机器人' }
    case 'qq':
      return { name: 'QQ', accent: '#12b7f5', bot: '预警机器人' }
    default:
      return { name: channelLabels[channelKind] || '消息', accent: '#64748b', bot: '预警机器人' }
  }
}
