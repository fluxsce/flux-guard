import type { BoardSection } from './boards'
import type { Panel } from '../api'

const kindIcons: Record<string, string> = {
  redis: 'redis',
  mysql: 'mysql',
  mongo: 'mongodb',
  kafka: 'workflow',
  zk: 'network',
  nacos: 'cloud',
}

const panelIcons: Record<string, string> = {
  overview: 'layout-dashboard',
  memory: 'memory-stick',
  traffic: 'arrow-left-right',
  slow: 'hourglass',
  replica: 'copy',
  keys: 'key',
  connections: 'plug',
  topic: 'layers',
  partition: 'split',
  broker: 'server',
  group: 'users',
  ops: 'activity',
  storage: 'database',
  latency: 'gauge',
  scale: 'boxes',
  registry: 'library',
  terminal: 'terminal',
}

export function kindIcon(kindCode: string) {
  return kindIcons[kindCode] || 'boxes'
}

export function panelIcon(panelCode: string) {
  return panelIcons[panelCode] || 'activity'
}

export function sectionsFor(panels: Panel[], panelCode: string, sections: BoardSection[]) {
  const current = panels.find((item) => item.panelCode === panelCode)
  if (!current || current.terminalFlag === 'Y') return []
  if (current.panelCode === 'overview') {
    const claimed = new Set(panels.flatMap((item) => (item.panelCode === 'overview' ? [] : item.sections ?? [])))
    return sections.filter((item) => !claimed.has(item.code))
  }
  const wanted = new Set(current.sections ?? [])
  return sections.filter((item) => wanted.has(item.code))
}

export function terminalNote(kindCode: string) {
  const notes: Record<string, string> = {
    redis: '按库浏览键。可按类型查看内容，删除键，或设置和取消过期时间。',
    mysql: 'MySQL 终端是 SQL 客户端。',
    mongo: '按库浏览集合和索引。可新建、删除、重命名，按条件查文档，并看历史慢日志。慢操作页可终止正在执行的操作。',
    kafka: '看主题的保留时间和清理策略。可新建、删除、增加分区、改配置，也可删除消费组或重置位点。',
    zk: 'ZooKeeper 终端按树浏览节点，可按名称或路径过滤。右键可刷新、新建、复制路径、移动、改权限和删除子树。',
    nacos: 'Nacos 终端走控制台操作。',
  }
  return notes[kindCode] || '这种组件的终端还没有接入。'
}
