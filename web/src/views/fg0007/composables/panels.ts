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
  broker: 'server',
  group: 'users',
  ops: 'activity',
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
    redis: 'Redis 终端走命令通道，和主机上的系统终端不是同一路。',
    mysql: 'MySQL 终端是 SQL 客户端。',
    mongo: 'MongoDB 终端走自己的命令通道。',
    kafka: 'Kafka 终端看主题和消费组，不是系统终端。',
    zk: 'ZooKeeper 终端走四字命令和节点浏览。',
    nacos: 'Nacos 终端走控制台操作。',
  }
  return notes[kindCode] || '这种组件的终端还没有接入。'
}
