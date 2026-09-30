/** 树表列。key 同时作为单元格插槽名。 */
export interface FgGridColumn {
  key: string
  title: string
  dataIndex?: string
  width?: number | string
  minWidth?: number | string
  ellipsis?: boolean
  align?: 'left' | 'center' | 'right'
  sortable?: boolean
}

/** 单列排序。点表头在升序、降序、取消之间切换。 */
export interface FgGridSort {
  key: string
  order: 'asc' | 'desc'
}

/** 行上右键菜单。 */
export interface FgGridMenuItem {
  key: string
  label: string
  icon?: string
  danger?: boolean
  /** 完整权限码。不写时由表格用模块号和 key 拼接。 */
  code?: string
  /** 按当前行禁用。内置数据不可删除走这里。 */
  disabled?: boolean | ((row: any) => boolean)
}
