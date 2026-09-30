/** 数据表单字段。对齐 gateway RsDataForm：tab / fieldset+children / show。view 只读，edit 时主键不可改。 */
export interface FgFormField {
  field: string
  label: string
  type?: 'input' | 'textarea' | 'select' | 'number' | 'password' | 'date' | 'datetime' | 'fieldset' | 'slot'
  /** 只在这些表单模式下显示。空则创建、修改、查看都显示。 */
  modes?: Array<'create' | 'edit' | 'view'>
  placeholder?: string
  required?: boolean
  disabled?: boolean
  /** 编辑时锁定，避免改掉主键。 */
  primary?: boolean
  span?: number
  options?: { label: string; value: string }[]
  /**
   * 页签。同一表单出现两个及以上不同 tab 时分成页签。
   * 对应 gateway 的 tabKey。
   */
  tab?: string
  /**
   * 是否显示。函数可读取当前表单值，例如 `(m) => m.channelKind === 'email'`。
   * 对应 gateway 的 show。
   * @default true
   */
  show?: boolean | ((formData: Record<string, unknown>) => boolean)
  /** 字段或分组标题旁的帮助提示。悬停图标显示。 */
  tips?: string
  /** 仅 type=fieldset：子字段。对应 gateway children。 */
  children?: FgFormField[]
}

export type FgFormMode = 'create' | 'edit' | 'view'

/** 搜索条件。span 按 24 栅格，默认一行四个。 */
export type FgSearchFieldType =
  | 'input'
  | 'select'
  | 'number'
  | 'switch'
  | 'date'
  | 'daterange'
  | 'datetime'
  | 'datetimerange'

export interface FgSearchField {
  field: string
  label: string
  type?: FgSearchFieldType
  placeholder?: string
  span?: number
  clearable?: boolean
  disabled?: boolean
  options?: { label: string; value: string; disabled?: boolean }[]
}

/** 遍历叶子字段（跳过 fieldset 容器本身）。 */
export function walkLeafFields(fields: FgFormField[], visit: (field: FgFormField) => void) {
  for (const field of fields) {
    if (field.type === 'fieldset') {
      walkLeafFields(field.children ?? [], visit)
      continue
    }
    visit(field)
  }
}
