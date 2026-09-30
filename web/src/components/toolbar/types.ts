import type { FgAiRisk } from '@/ai/types'

/** 对齐 RsButton 形态。 */
export type FgToolbarButtonVariant =
  | 'primary'
  | 'default'
  | 'secondary'
  | 'ghost'
  | 'danger'
  | 'link'
  | 'text'

/** 对齐 RsButton 语义色。gateway 的 success / warning / info / error 会落到这里。 */
export type FgToolbarButtonTone = 'neutral' | 'primary' | 'danger' | 'success' | 'warning' | 'info'

/** gateway ToolbarButtonType。error 映射为 danger。 */
export type FgToolbarButtonType = 'default' | 'primary' | 'success' | 'warning' | 'error' | 'info'

export type FgToolbarSize = 'ssm' | 'sm' | 'md' | 'lg'

export type FgToolbarAlign = 'left' | 'center' | 'right' | 'space-between'

export type FgToolbarBorder = 'bottom' | 'top' | 'both' | 'none'

export interface FgToolbarDropdownOption {
  key: string
  label: string
  /** Lucide 图标名 */
  icon?: string
  disabled?: boolean
  /** 该项之前画分隔线 */
  divider?: boolean
  onClick?: (key: string) => void | Promise<void>
}

/**
 * 工具条按钮。图标用 Lucide 名称。
 * 传入模块号时，`key` 作为按钮动作，例如 add → fg0021:add。
 * `code` 写完整权限码时优先用它。`show: false` 时不渲染。`atEnd` 放到右侧。
 */
export interface FgToolbarButton {
  key: string
  /** 完整权限码。不写时由工具条用模块号和 key 拼接。 */
  code?: string
  label?: string
  /** Lucide 图标名，如 search、rotate-ccw */
  icon?: string
  variant?: FgToolbarButtonVariant
  /** 与 variant 二选一，便于沿用 gateway 的 type */
  type?: FgToolbarButtonType
  tone?: FgToolbarButtonTone
  size?: FgToolbarSize
  disabled?: boolean
  show?: boolean
  loading?: boolean
  tooltip?: string
  atEnd?: boolean
  dropdown?: boolean
  dropdownOptions?: FgToolbarDropdownOption[]
  /** 给全平台助手的风险。不写时查看类为 read，其余为 write。 */
  aiRisk?: FgAiRisk
  onClick?: (key: string) => void | Promise<void>
}

export interface FgToolbarGroup {
  key: string
  title?: string
  buttons: FgToolbarButton[]
  /** 与前一组之间画分隔线 */
  divider?: boolean
}
