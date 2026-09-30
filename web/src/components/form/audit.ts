import type { FgFormField } from './types'

export const basicTab = '基本信息'
export const auditTab = '其它信息'

/** 给字段（含 fieldset 子树）打上同一 tab。 */
export function tabOf(tab: string, fields: FgFormField[]): FgFormField[] {
  return fields.map((field) => ({
    ...field,
    tab,
    children: field.children ? tabOf(tab, field.children) : undefined,
  }))
}

/** 构造 fieldset 节点，对齐 gateway `{ type: 'fieldset', children }`。 */
export function fieldset(
  field: string,
  label: string,
  children: FgFormField[],
  opts?: {
    tips?: string
    show?: boolean | ((formData: Record<string, unknown>) => boolean)
    span?: number
  },
): FgFormField {
  return {
    field,
    label,
    type: 'fieldset',
    span: opts?.span ?? 24,
    tips: opts?.tips,
    show: opts?.show,
    children,
  }
}

/** 创建人和修改人。只读，挂在其它信息页签。 */
export const stampFields: FgFormField[] = [
  { field: 'addTime', label: '创建时间', type: 'datetime', disabled: true, tab: auditTab },
  { field: 'addWho', label: '创建人', disabled: true, tab: auditTab },
  { field: 'editTime', label: '修改时间', type: 'datetime', disabled: true, tab: auditTab },
  { field: 'editWho', label: '修改人', disabled: true, tab: auditTab },
]

/** 创建人、修改人和版本。只读，挂在其它信息页签。 */
export const auditFields: FgFormField[] = [
  ...stampFields,
  { field: 'currentVersion', label: '版本号', type: 'number', disabled: true, tab: auditTab },
  { field: 'oprSeqFlag', label: '操作序列', disabled: true, tab: auditTab },
]
