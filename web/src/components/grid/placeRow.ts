/**
 * 保存接口返回的一行写回表格后的结果。
 * created 为真表示这是新行，调用方通常把分页回到第一页。
 */
export interface PlacedRows<T> {
  rows: T[]
  created: boolean
}

/**
 * 用新增或修改返回的一行更新表格。主键已在列表中则原位替换，否则插到开头。
 * 与列表查询使用同一行结构，不再整表查询。
 * @param rows - 当前表格数据
 * @param row - 保存接口返回的这一行
 * @param same - 判断是否为同一行，通常比较主键
 * @returns 更新后的行，以及是否为新增
 */
export function placeSavedRow<T>(rows: readonly T[], row: T, same: (item: T) => boolean): PlacedRows<T> {
  const index = rows.findIndex(same)
  if (index >= 0) {
    const next = rows.slice()
    next[index] = row
    return { rows: next, created: false }
  }
  return { rows: [row, ...rows], created: true }
}
