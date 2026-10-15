import type { AppInstance } from '../api'

/** 目录树里的一行：文件夹或文件。 */
export interface FileEntry {
  key: string
  name: string
  kind: 'dir' | 'file'
  depth: number
  /** 相对站点根的目录前缀，根为 '' */
  relDir: string
  row?: AppInstance
}

function normSlash(path: string) {
  return path.replace(/\\/g, '/')
}

function baseName(path: string) {
  const n = normSlash(path)
  const parts = n.split('/').filter(Boolean)
  return parts[parts.length - 1] || n || '—'
}

/** 算出相对 base 的路径；对不上则退回文件名。 */
export function relativeToBase(artifactPath: string | undefined, basePath: string | undefined) {
  const full = normSlash(artifactPath || '')
  const base = normSlash(basePath || '').replace(/\/$/, '')
  if (full && base && (full === base || full.startsWith(base + '/'))) {
    return full.slice(base.length).replace(/^\//, '')
  }
  return baseName(full)
}

/** 当前目录下要展示的文件夹与文件（不递归展开）。 */
export function entriesInDir(files: AppInstance[], basePath: string | undefined, cwd: string): FileEntry[] {
  const cwdNorm = normSlash(cwd).replace(/\/$/, '')
  const dirs = new Map<string, FileEntry>()
  const fileRows: FileEntry[] = []

  for (const row of files) {
    if (row.artifactKind === 'dir') {
      const rel = relativeToBase(row.artifactPath, basePath) || normSlash(row.appCode)
      const parent = parentDir(rel)
      if (parent !== cwdNorm) continue
      const name = rel.split('/').filter(Boolean).pop() || rel
      if (!dirs.has(rel)) {
        dirs.set(rel, {
          key: `dir:${rel}`,
          name,
          kind: 'dir',
          depth: 0,
          relDir: rel,
          row,
        })
      }
      continue
    }
    const rel = relativeToBase(row.artifactPath, basePath)
    const parent = parentDir(rel)
    if (parent !== cwdNorm) {
      // 文件在更深层：补一层可见子目录
      if (cwdNorm === '' || rel.startsWith(cwdNorm + '/')) {
        const rest = cwdNorm ? rel.slice(cwdNorm.length + 1) : rel
        const next = rest.split('/')[0]
        if (next && rest.includes('/')) {
          const childDir = cwdNorm ? `${cwdNorm}/${next}` : next
          if (!dirs.has(childDir)) {
            dirs.set(childDir, {
              key: `dir:${childDir}`,
              name: next,
              kind: 'dir',
              depth: 0,
              relDir: childDir,
            })
          }
        }
      }
      continue
    }
    fileRows.push({
      key: row.instId,
      name: baseName(rel) || row.appCode || '—',
      kind: 'file',
      depth: 0,
      relDir: cwdNorm,
      row,
    })
  }

  const dirList = [...dirs.values()].sort((a, b) => a.name.localeCompare(b.name))
  fileRows.sort((a, b) => a.name.localeCompare(b.name))
  return [...dirList, ...fileRows]
}

function parentDir(rel: string) {
  const n = normSlash(rel).replace(/\/$/, '')
  const i = n.lastIndexOf('/')
  if (i < 0) return ''
  return n.slice(0, i)
}

/** 面包屑：首段为 base 末级目录名（完整路径见页面 Base 行），其后为相对 cwd 各段。 */
export function breadcrumbOf(cwd: string, basePath?: string): { label: string; path: string }[] {
  const base = normSlash(basePath || '').replace(/\/$/, '')
  const rootLabel = base ? baseName(base) : '根目录'
  const out: { label: string; path: string }[] = [{ label: rootLabel, path: '' }]
  const parts = normSlash(cwd).split('/').filter(Boolean)
  let acc = ''
  for (const part of parts) {
    acc = acc ? `${acc}/${part}` : part
    out.push({ label: part, path: acc })
  }
  return out
}
