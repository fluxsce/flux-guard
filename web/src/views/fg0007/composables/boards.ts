import type { ComponentConn, MetricDef, Point } from '../api'

export interface BoardCard {
  label: string
  value: string
  level: 'ok' | 'watch' | 'alert'
}

export interface BoardSection {
  title: string
  cards: BoardCard[]
  metrics: MetricDef[]
}

export interface Board {
  level: 'ok' | 'watch' | 'alert'
  finding: string
  sections: BoardSection[]
}

function num(point: Point | null, code: string) {
  const value = point?.values?.[code]
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function textOf(unit: string, value: number | null) {
  if (value == null || value < 0) return '—'
  if (unit === 'byte') {
    if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
    if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
    if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`
    return `${Math.round(value)} B`
  }
  if (unit === '%') return `${value.toFixed(1)}%`
  if (unit === 'ratio') return value.toFixed(2)
  if (unit === 'KB/s') return `${value.toFixed(1)} KB/s`
  if (unit === '/s') return `${value.toFixed(2)}/s`
  if (unit === 's' && value >= 3600) return `${(value / 3600).toFixed(1)} 小时`
  if (Number.isInteger(value)) return String(value)
  return value.toFixed(1)
}

function card(label: string, value: string, level: BoardCard['level'] = 'ok'): BoardCard {
  return { label, value, level }
}

function worse(current: Board['level'], next: Board['level']): Board['level'] {
  const rank = { ok: 0, watch: 1, alert: 2 }
  return rank[next] > rank[current] ? next : current
}

export function inspect(conn: ComponentConn, point: Point | null): Board {
  const findings: string[] = []
  let level: Board['level'] = conn.upFlag === 'Y' ? 'ok' : 'alert'
  if (conn.upFlag !== 'Y') findings.push(conn.errorText || '连不上')
  else if (conn.errorText) findings.push(conn.errorText)

  const deploy = (conn.optionText || '').split(':')[0]
  const sections = sectionsOf(conn.kindCode, deploy, conn, point, (text, next) => {
    findings.push(text)
    level = worse(level, next)
  })
  return {
    level,
    finding: findings.filter(Boolean).join('；'),
    sections,
  }
}

function sectionsOf(
  kind: string,
  deploy: string,
  conn: ComponentConn,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const up = card('连通', conn.upFlag === 'Y' ? '正常' : '不通', conn.upFlag === 'Y' ? 'ok' : 'alert')
  const role = card('角色', point?.roleName || conn.roleName || '—')
  const version = card('版本', point?.versionText || conn.versionText || '—')
  if (kind === 'redis' && deploy === 'cluster') return redisCluster(up, point, problem)
  if (kind === 'redis' && deploy === 'sentinel') return redisSentinel(up, conn, point, problem)
  if (kind === 'redis') return redisStandalone(up, role, version, point, problem)
  if (kind === 'mysql') return mysqlBoard(up, role, version, point, problem)
  if (kind === 'mongo') return mongoBoard(up, role, version, point)
  if (kind === 'kafka') return kafkaBoard(up, point, problem)
  if (kind === 'zk') return zkBoard(up, role, version, point, problem)
  return nacosBoard(up, point)
}

function redisStandalone(
  up: BoardCard,
  role: BoardCard,
  version: BoardCard,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  return [
    { title: '可用性', cards: [up, role, version], metrics: [] },
    ...redisRuntime(point, problem, true, true),
  ]
}

function redisSentinel(
  up: BoardCard,
  conn: ComponentConn,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const down = num(point, 'failSlots')
  if (down != null && down > 0) problem('主节点已下线', 'alert')
  return [
    {
      title: '哨兵',
      cards: [
        up,
        card('主节点', conn.databaseName || '—', down != null && down > 0 ? 'alert' : 'ok'),
        card('哨兵数', textOf('', num(point, 'knownNodes'))),
        card('从节点', textOf('', num(point, 'connectedSlaves'))),
      ],
      metrics: [
        { code: 'knownNodes', name: '哨兵数' },
        { code: 'connectedSlaves', name: '从节点' },
      ],
    },
    ...redisRuntime(point, problem, false, true),
  ]
}

function redisCluster(
  up: BoardCard,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const state = point?.roleName || '—'
  const fail = num(point, 'failSlots')
  if (state === 'fail') problem('集群状态为 fail', 'alert')
  if (fail != null && fail > 0) problem(`失败槽 ${fail}`, 'alert')
  const stateLevel: BoardCard['level'] = state === 'ok' ? 'ok' : state === '—' ? 'ok' : 'alert'
  return [
    {
      title: '集群',
      cards: [
        up,
        card('状态', state, stateLevel),
        card('节点', textOf('', num(point, 'knownNodes'))),
        card('分片', textOf('', num(point, 'connectedSlaves'))),
        card('失败槽', textOf('', fail), fail != null && fail > 0 ? 'alert' : 'ok'),
      ],
      metrics: [
        { code: 'knownNodes', name: '节点' },
        { code: 'failSlots', name: '失败槽' },
      ],
    },
    ...redisRuntime(point, problem, false, false),
  ]
}

function redisRuntime(
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
  withUptime: boolean,
  showSlaves: boolean,
): BoardSection[] {
  const hit = num(point, 'hitRate')
  let hitLevel: BoardCard['level'] = 'ok'
  if (hit != null && hit >= 0 && hit < 50) {
    hitLevel = 'alert'
    problem(`区间命中率只有 ${hit.toFixed(1)}%`, 'alert')
  } else if (hit != null && hit >= 0 && hit < 80) {
    hitLevel = 'watch'
    problem(`区间命中率 ${hit.toFixed(1)}%，偏低`, 'watch')
  }
  const usage = num(point, 'memUsagePercent')
  let usageLevel: BoardCard['level'] = 'ok'
  if (usage != null && usage >= 90) {
    usageLevel = 'alert'
    problem(`内存已用到上限的 ${usage.toFixed(1)}%`, 'alert')
  } else if (usage != null && usage >= 80) {
    usageLevel = 'watch'
    problem(`内存已用到上限的 ${usage.toFixed(1)}%`, 'watch')
  }
  const frag = num(point, 'memFragmentation')
  let fragLevel: BoardCard['level'] = 'ok'
  if (frag != null && frag >= 2) {
    fragLevel = 'alert'
    problem(`内存碎片率 ${frag.toFixed(2)}`, 'alert')
  } else if (frag != null && frag >= 1.5) {
    fragLevel = 'watch'
    problem(`内存碎片率 ${frag.toFixed(2)}`, 'watch')
  }
  const blocked = num(point, 'blockedClients')
  if (blocked != null && blocked > 0) problem(`阻塞客户端 ${Math.round(blocked)}`, blocked >= 10 ? 'alert' : 'watch')
  const evicted = num(point, 'evictedPerSec')
  if (evicted != null && evicted > 0) problem(`正在淘汰键，${evicted.toFixed(2)}/秒`, evicted >= 1 ? 'alert' : 'watch')
  const rejected = num(point, 'rejectedPerSec')
  if (rejected != null && rejected > 0) problem(`正在拒绝连接，${rejected.toFixed(2)}/秒`, 'alert')
  const lag = num(point, 'replicaLagSec')
  let lagLevel: BoardCard['level'] = 'ok'
  if (lag != null && lag >= 30) {
    lagLevel = 'alert'
    problem(`复制延迟 ${Math.round(lag)} 秒`, 'alert')
  } else if (lag != null && lag >= 10) {
    lagLevel = 'watch'
    problem(`复制延迟 ${Math.round(lag)} 秒`, 'watch')
  }
  const link = num(point, 'masterLinkDown')
  if (link != null && link > 0) problem('从节点到主节点的链路已断开', 'alert')
  const rdb = num(point, 'rdbOk')
  if (rdb != null && rdb === 0) problem('最近一次 RDB 保存失败', 'alert')
  const aof = num(point, 'aofOk')
  if (aof != null && aof === 0) problem('最近一次 AOF 写入失败', 'alert')
  const slow = num(point, 'slowlogLen')
  const replicaCards = [
    card('复制延迟', textOf('s', lag), lagLevel),
    card('主链路', linkText(link), link != null && link > 0 ? 'alert' : 'ok'),
  ]
  if (showSlaves) replicaCards.unshift(card('从节点', textOf('', num(point, 'connectedSlaves'))))
  if (withUptime) replicaCards.push(card('运行', textOf('s', num(point, 'uptimeSec'))))
  return [
    {
      title: '内存',
      cards: [
        card('已用', textOf('byte', num(point, 'usedMemoryBytes'))),
        card('上限占用', textOf('%', usage), usageLevel),
        card('峰值', textOf('byte', num(point, 'usedMemoryPeakBytes'))),
        card('碎片率', textOf('ratio', frag), fragLevel),
      ],
      metrics: [
        { code: 'usedMemoryBytes', name: '已用内存', unit: 'byte' },
        { code: 'memUsagePercent', name: '上限占用', unit: '%' },
      ],
    },
    {
      title: '访问',
      cards: [
        card('每秒操作', textOf('', num(point, 'opsPerSec'))),
        card('区间命中率', textOf('%', hit), hitLevel),
        card('连接数', textOf('', num(point, 'connectedClients'))),
        card('阻塞', textOf('', blocked), blocked != null && blocked > 0 ? (blocked >= 10 ? 'alert' : 'watch') : 'ok'),
      ],
      metrics: [
        { code: 'opsPerSec', name: '每秒操作' },
        { code: 'hitRate', name: '区间命中率', unit: '%' },
        { code: 'connectedClients', name: '连接数' },
        { code: 'blockedClients', name: '阻塞客户端' },
      ],
    },
    {
      title: '流量',
      cards: [
        card('入站', textOf('KB/s', num(point, 'netInKbps'))),
        card('出站', textOf('KB/s', num(point, 'netOutKbps'))),
      ],
      metrics: [
        { code: 'netInKbps', name: '入站', unit: 'KB/s' },
        { code: 'netOutKbps', name: '出站', unit: 'KB/s' },
      ],
    },
    {
      title: '复制',
      cards: replicaCards,
      metrics: showSlaves
        ? [
            { code: 'replicaLagSec', name: '复制延迟', unit: 's' },
            { code: 'connectedSlaves', name: '从节点' },
          ]
        : [{ code: 'replicaLagSec', name: '复制延迟', unit: 's' }],
    },
    {
      title: '持久化',
      cards: [
        card('RDB', flagText(rdb, '成功', '失败'), rdb === 0 ? 'alert' : 'ok'),
        card('AOF', aofText(aof), aof === 0 ? 'alert' : 'ok'),
      ],
      metrics: [],
    },
    {
      title: '键',
      cards: [
        card('键数量', textOf('', num(point, 'keyCount'))),
        card('淘汰', textOf('/s', evicted), evicted != null && evicted > 0 ? (evicted >= 1 ? 'alert' : 'watch') : 'ok'),
        card('过期', textOf('/s', num(point, 'expiredPerSec'))),
        card('拒绝连接', textOf('/s', rejected), rejected != null && rejected > 0 ? 'alert' : 'ok'),
      ],
      metrics: [{ code: 'keyCount', name: '键数量' }],
    },
    {
      title: '慢操作',
      cards: [card('日志条数', textOf('', slow), slow != null && slow > 0 ? 'watch' : 'ok')],
      metrics: [{ code: 'slowlogLen', name: '慢操作' }],
    },
  ]
}

function linkText(value: number | null) {
  if (value == null || value < 0) return '—'
  return value > 0 ? '断开' : '正常'
}

function flagText(value: number | null, ok: string, bad: string) {
  if (value == null || value < 0) return '—'
  return value > 0 ? ok : bad
}

function aofText(value: number | null) {
  if (value == null || value < 0) return '未开启'
  return value > 0 ? '成功' : '失败'
}

function mysqlBoard(
  up: BoardCard,
  role: BoardCard,
  version: BoardCard,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const lag = num(point, 'replicaLagSec')
  let lagLevel: BoardCard['level'] = 'ok'
  let lagText = '无主从数据'
  if (lag != null && lag >= 0) {
    lagText = `${Math.round(lag)} 秒`
    if (lag >= 60) {
      lagLevel = 'alert'
      problem(`复制延迟 ${Math.round(lag)} 秒`, 'alert')
    } else if (lag >= 10) {
      lagLevel = 'watch'
      problem(`复制延迟 ${Math.round(lag)} 秒`, 'watch')
    }
  }
  const hit = num(point, 'bufferPoolHitRate')
  let hitLevel: BoardCard['level'] = 'ok'
  if (hit != null && hit >= 0 && hit < 95) {
    hitLevel = 'watch'
    problem(`缓冲池命中率 ${hit.toFixed(1)}%`, 'watch')
  }
  const slow = num(point, 'slowQueries')
  return [
    { title: '可用性', cards: [up, role, version], metrics: [] },
    {
      title: '连接',
      cards: [
        card('连接数', textOf('', num(point, 'threadsConnected'))),
        card('活跃线程', textOf('', num(point, 'threadsRunning'))),
      ],
      metrics: [
        { code: 'threadsConnected', name: '连接数' },
        { code: 'threadsRunning', name: '活跃线程' },
      ],
    },
    {
      title: '缓冲池与慢查询',
      cards: [
        card('缓冲池命中', textOf('%', hit), hitLevel),
        card('累计慢查询', textOf('', slow)),
      ],
      metrics: [{ code: 'bufferPoolHitRate', name: '缓冲池命中', unit: '%' }],
    },
    {
      title: '复制',
      cards: [card('延迟', lagText, lagLevel)],
      metrics: [{ code: 'replicaLagSec', name: '复制延迟', unit: 's' }],
    },
  ]
}

function mongoBoard(up: BoardCard, role: BoardCard, version: BoardCard, point: Point | null): BoardSection[] {
  return [
    { title: '可用性', cards: [up, role, version], metrics: [] },
    {
      title: '连接',
      cards: [
        card('当前连接', textOf('', num(point, 'connectionsCurrent'))),
        card('可用连接', textOf('', num(point, 'connectionsAvailable'))),
      ],
      metrics: [
        { code: 'connectionsCurrent', name: '当前连接' },
        { code: 'connectionsAvailable', name: '可用连接' },
      ],
    },
    {
      title: '累计操作',
      cards: [
        card('插入', textOf('', num(point, 'opInsert'))),
        card('查询', textOf('', num(point, 'opQuery'))),
        card('更新', textOf('', num(point, 'opUpdate'))),
        card('删除', textOf('', num(point, 'opDelete'))),
      ],
      metrics: [
        { code: 'opQuery', name: '查询' },
        { code: 'opInsert', name: '插入' },
        { code: 'opUpdate', name: '更新' },
        { code: 'opDelete', name: '删除' },
      ],
    },
  ]
}

function kafkaBoard(
  up: BoardCard,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const offline = num(point, 'offlinePartitions')
  const under = num(point, 'underReplicated')
  if (offline != null && offline > 0) problem(`离线分区 ${offline}`, 'alert')
  if (under != null && under > 0) problem(`未同步分区 ${under}`, 'watch')
  const lag = num(point, 'maxLag')
  let lagLevel: BoardCard['level'] = 'ok'
  if (lag != null && lag >= 10000) {
    lagLevel = 'alert'
    problem(`消费延迟 ${Math.round(lag)}`, 'alert')
  } else if (lag != null && lag >= 1000) {
    lagLevel = 'watch'
    problem(`消费延迟 ${Math.round(lag)}`, 'watch')
  }
  return [
    {
      title: '集群',
      cards: [
        up,
        card('Broker', textOf('', num(point, 'brokerCount'))),
        card('Topic', textOf('', num(point, 'topicCount'))),
        card('离线分区', textOf('', offline), offline != null && offline > 0 ? 'alert' : 'ok'),
        card('未同步分区', textOf('', under), under != null && under > 0 ? 'watch' : 'ok'),
      ],
      metrics: [
        { code: 'brokerCount', name: 'Broker' },
        { code: 'offlinePartitions', name: '离线分区' },
        { code: 'underReplicated', name: '未同步分区' },
      ],
    },
    {
      title: '消费',
      cards: [
        card('消费组', textOf('', num(point, 'groupCount'))),
        card('最大延迟', textOf('', lag), lagLevel),
      ],
      metrics: [{ code: 'maxLag', name: '最大延迟' }],
    },
  ]
}

function zkBoard(
  up: BoardCard,
  role: BoardCard,
  version: BoardCard,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const latency = num(point, 'avgLatency')
  const outstanding = num(point, 'outstandingRequests')
  let latencyLevel: BoardCard['level'] = 'ok'
  if (latency != null && latency >= 100) {
    latencyLevel = latency >= 500 ? 'alert' : 'watch'
    problem(`平均延迟 ${Math.round(latency)} ms`, latencyLevel)
  }
  if (outstanding != null && outstanding >= 10) problem(`堆积请求 ${outstanding}`, 'watch')
  return [
    { title: '可用性', cards: [up, role, version], metrics: [] },
    {
      title: '压力',
      cards: [
        card('平均延迟', latency == null ? '—' : `${Math.round(latency)} ms`, latencyLevel),
        card('堆积请求', textOf('', outstanding), outstanding != null && outstanding >= 10 ? 'watch' : 'ok'),
      ],
      metrics: [
        { code: 'avgLatency', name: '平均延迟', unit: 'ms' },
        { code: 'outstandingRequests', name: '堆积请求' },
      ],
    },
    {
      title: '规模',
      cards: [
        card('节点', textOf('', num(point, 'znodeCount'))),
        card('跟随者', textOf('', num(point, 'followers'))),
        card('Watch', textOf('', num(point, 'watchCount'))),
      ],
      metrics: [
        { code: 'znodeCount', name: '节点' },
        { code: 'watchCount', name: 'Watch' },
      ],
    },
  ]
}

function nacosBoard(up: BoardCard, point: Point | null): BoardSection[] {
  return [
    {
      title: '健康',
      cards: [
        up,
        card('配置', textOf('', num(point, 'configCount'))),
        card('服务', textOf('', num(point, 'serviceCount'))),
      ],
      metrics: [
        { code: 'configCount', name: '配置' },
        { code: 'serviceCount', name: '服务' },
      ],
    },
  ]
}
