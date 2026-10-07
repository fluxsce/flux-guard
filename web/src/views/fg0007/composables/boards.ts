import type { ComponentConn, MetricDef, Point } from '../api'

export interface BoardCard {
  label: string
  value: string
  level: 'ok' | 'watch' | 'alert'
  help?: string
}

export interface BoardSection {
  code: string
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

export function formatMetric(unit: string, value: number | null) {
  return textOf(unit, value)
}

// listText 是实例列表上的当前值。AOF 用采集里的最近写入状态：未写入表示没开。
export function listText(code: string, unit: string, value: number | null) {
  if (code === 'aofOk') {
    if (value == null || value < 0) return '未开启'
    return value > 0 ? '开启' : '失败'
  }
  if (code === 'memUsagePercent' && (value == null || value < 0)) return '未设置'
  return textOf(unit, value)
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

function card(label: string, value: string, level: BoardCard['level'] = 'ok', help?: string): BoardCard {
  return help ? { label, value, level, help } : { label, value, level }
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
  if (kind === 'mongo') return mongoBoard(up, role, version, point, problem)
  if (kind === 'kafka') return kafkaBoard(up, version, point, problem)
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
    { code: 'availability', title: '可用性', cards: [up, role, version], metrics: [] },
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
      code: 'sentinel',
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
      code: 'cluster',
      title: '集群',
      cards: [
        up,
        card('状态', state, stateLevel),
        card('节点', textOf('', num(point, 'knownNodes'))),
        card('分片', textOf('', num(point, 'connectedSlaves'))),
        card('失败槽', textOf('', fail), fail != null && fail > 0 ? 'alert' : 'ok', '集群里处于失败状态的哈希槽。大于 0 表示有槽没有可用主节点。'),
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
  const maxMem = num(point, 'maxMemoryBytes')
  const maxText = maxMem == null ? '—' : maxMem <= 0 ? '未设置' : textOf('byte', maxMem)
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
    card('复制延迟', textOf('s', lag), lagLevel, '从节点落后主节点的秒数。主节点自己没有这项。'),
    card('主链路', linkText(link), link != null && link > 0 ? 'alert' : 'ok', '从节点到主节点的连接是否还在。主节点显示 —。'),
  ]
  if (showSlaves) replicaCards.unshift(card('从节点', textOf('', num(point, 'connectedSlaves'))))
  if (withUptime) replicaCards.push(card('运行', textOf('s', num(point, 'uptimeSec'))))
  return [
    {
      code: 'memory',
      title: '内存',
      cards: [
        card('已用', textOf('byte', num(point, 'usedMemoryBytes'))),
        card('内存上限', maxText, 'ok', 'maxmemory。未设置表示不限制内存。'),
        card('上限占用', maxMem != null && maxMem <= 0 ? '未设置' : textOf('%', usage), usageLevel, '已用内存占上限的比例。没有设置上限时这里是未设置。'),
        card('峰值', textOf('byte', num(point, 'usedMemoryPeakBytes')), 'ok', '这次启动以来用过的最高内存。'),
        card('碎片率', textOf('ratio', frag), fragLevel, '分配给 Redis 的内存除以实际存放数据的内存。接近 1 比较健康，持续高于 1.5 说明碎片偏多。'),
      ],
      metrics: [
        { code: 'usedMemoryBytes', name: '已用内存', unit: 'byte' },
        ...(maxMem != null && maxMem > 0 ? [{ code: 'maxMemoryBytes', name: '内存上限', unit: 'byte' as const }] : []),
        { code: 'memUsagePercent', name: '上限占用', unit: '%' },
      ],
    },
    {
      code: 'access',
      title: '访问',
      cards: [
        card('每秒操作', textOf('', num(point, 'opsPerSec'))),
        card('区间命中率', textOf('%', hit), hitLevel, '两次采集之间的键查找命中比例，不是从启动到现在的累计命中。没有查找时显示 —。'),
        card('连接数', textOf('', num(point, 'connectedClients'))),
        card('阻塞', textOf('', blocked), blocked != null && blocked > 0 ? (blocked >= 10 ? 'alert' : 'watch') : 'ok', '正在等待 BLPOP 这类阻塞命令的客户端数量。'),
      ],
      metrics: [
        { code: 'opsPerSec', name: '每秒操作' },
        { code: 'hitRate', name: '区间命中率', unit: '%' },
        { code: 'connectedClients', name: '连接数' },
        { code: 'blockedClients', name: '阻塞客户端' },
      ],
    },
    {
      code: 'traffic',
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
      code: 'replica',
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
      code: 'persist',
      title: '持久化',
      cards: [
        card('RDB', flagText(rdb, '成功', '失败'), rdb === 0 ? 'alert' : 'ok', '最近一次 RDB 快照是否保存成功。'),
        card('AOF', aofText(aof), aof === 0 ? 'alert' : 'ok', '没开 AOF 时显示未开启。开了则看最近一次写入是否成功。'),
      ],
      metrics: [],
    },
    {
      code: 'keys',
      title: '键',
      cards: [
        card('键数量', textOf('', num(point, 'keyCount'))),
        card('淘汰', textOf('/s', evicted), evicted != null && evicted > 0 ? (evicted >= 1 ? 'alert' : 'watch') : 'ok', '内存达到上限后被删掉的键，按每秒计。'),
        card('过期', textOf('/s', num(point, 'expiredPerSec')), 'ok', '到达过期时间后被删掉的键，按每秒计。'),
        card('拒绝连接', textOf('/s', rejected), rejected != null && rejected > 0 ? 'alert' : 'ok', '客户端因为连不上而被拒绝的次数，按每秒计。'),
      ],
      metrics: [{ code: 'keyCount', name: '键数量' }],
    },
    {
      code: 'slow',
      title: '慢操作',
      cards: [card('日志条数', textOf('', slow), slow != null && slow > 0 ? 'watch' : 'ok', '慢操作日志里当前还留着多少条。下面的表最多列出最近 128 条。')],
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
    { code: 'availability', title: '可用性', cards: [up, role, version], metrics: [] },
    {
      code: 'connections',
      title: '连接',
      cards: [
        card('连接数', textOf('', num(point, 'threadsConnected'))),
        card('活跃线程', textOf('', num(point, 'threadsRunning')), 'ok', '正在执行语句的线程，不是全部连接。'),
      ],
      metrics: [
        { code: 'threadsConnected', name: '连接数' },
        { code: 'threadsRunning', name: '活跃线程' },
      ],
    },
    {
      code: 'slow',
      title: '缓冲池与慢查询',
      cards: [
        card('缓冲池命中', textOf('%', hit), hitLevel, '数据页从缓冲池读到的比例。偏低说明磁盘读变多。'),
        card('累计慢查询', textOf('', slow), 'ok', '从启动到现在超过慢查询阈值的语句次数，不是当前还在跑的慢语句。'),
      ],
      metrics: [{ code: 'bufferPoolHitRate', name: '缓冲池命中', unit: '%' }],
    },
    {
      code: 'replica',
      title: '复制',
      cards: [card('延迟', lagText, lagLevel, '从库落后主库的秒数。看不出主从时显示无主从数据。')],
      metrics: [{ code: 'replicaLagSec', name: '复制延迟', unit: 's' }],
    },
  ]
}

function mongoRole(code: string) {
  const names: Record<string, string> = {
    primary: '主节点',
    secondary: '从节点',
    arbiter: '仲裁',
    mongos: '路由',
    standalone: '单机',
  }
  return names[code] || code || '—'
}

function mongoBoard(
  up: BoardCard,
  role: BoardCard,
  version: BoardCard,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const usage = num(point, 'connUsagePercent')
  let usageLevel: BoardCard['level'] = 'ok'
  if (usage != null && usage >= 90) {
    usageLevel = 'alert'
    problem(`连接已用到上限的 ${usage.toFixed(1)}%`, 'alert')
  } else if (usage != null && usage >= 80) {
    usageLevel = 'watch'
    problem(`连接已用到上限的 ${usage.toFixed(1)}%`, 'watch')
  }
  const dirty = num(point, 'cacheDirtyPercent')
  let dirtyLevel: BoardCard['level'] = 'ok'
  if (dirty != null && dirty >= 20) {
    dirtyLevel = 'alert'
    problem(`脏数据占缓存上限的 ${dirty.toFixed(1)}%`, 'alert')
  } else if (dirty != null && dirty >= 10) {
    dirtyLevel = 'watch'
    problem(`脏数据占缓存上限的 ${dirty.toFixed(1)}%`, 'watch')
  }
  const lag = num(point, 'replicaLagSec')
  const members = num(point, 'memberCount')
  const inSet = role.value === 'primary' || role.value === 'secondary' || role.value === 'arbiter' || Boolean(point?.detail?.setName)
  let lagLevel: BoardCard['level'] = 'ok'
  let lagText = '—'
  if (!inSet && (lag == null || lag < 0)) lagText = '无副本集'
  else if (lag != null && lag >= 0) {
    lagText = textOf('s', lag)
    if (lag >= 30) {
      lagLevel = 'alert'
      problem(`复制延迟 ${Math.round(lag)} 秒`, 'alert')
    } else if (lag >= 10) {
      lagLevel = 'watch'
      problem(`复制延迟 ${Math.round(lag)} 秒`, 'watch')
    }
  }
  const unhealthy = num(point, 'unhealthyMembers')
  if (unhealthy != null && unhealthy > 0) problem(`异常成员 ${Math.round(unhealthy)}`, 'alert')
  const readers = num(point, 'queuedReaders')
  const writers = num(point, 'queuedWriters')
  const readerLevel = queueLevel(readers)
  const writerLevel = queueLevel(writers)
  if (readerLevel !== 'ok' && readers != null) problem(`读排队 ${Math.round(readers)}`, readerLevel)
  if (writerLevel !== 'ok' && writers != null) problem(`写排队 ${Math.round(writers)}`, writerLevel)
  const readTickets = num(point, 'readTickets')
  const writeTickets = num(point, 'writeTickets')
  if (readTickets === 0) problem('读票据已用完', 'alert')
  if (writeTickets === 0) problem('写票据已用完', 'alert')
  const asserts = num(point, 'assertPerSec')
  let assertLevel: BoardCard['level'] = 'ok'
  if (asserts != null && asserts >= 1) {
    assertLevel = 'alert'
    problem(`断言 ${asserts.toFixed(2)}/秒`, 'alert')
  } else if (asserts != null && asserts > 0) {
    assertLevel = 'watch'
    problem(`断言 ${asserts.toFixed(2)}/秒`, 'watch')
  }
  const slow = num(point, 'slowOps')
  let slowLevel: BoardCard['level'] = 'ok'
  if (slow != null && slow >= 5) {
    slowLevel = 'alert'
    problem(`正在执行的慢操作 ${Math.round(slow)}`, 'alert')
  } else if (slow != null && slow > 0) {
    slowLevel = 'watch'
    problem(`正在执行的慢操作 ${Math.round(slow)}`, 'watch')
  }
  const setName = point?.detail?.setName?.trim() || ''
  return [
    {
      code: 'availability',
      title: '可用性',
      cards: [
        up,
        card('角色', mongoRole(role.value)),
        version,
        card('运行', textOf('s', num(point, 'uptimeSec'))),
        card('副本集', setName || (inSet ? '—' : '无'), 'ok', '副本集名称。单机和路由没有这项。'),
      ],
      metrics: [],
    },
    {
      code: 'connections',
      title: '连接',
      cards: [
        card('当前', textOf('', num(point, 'connectionsCurrent'))),
        card('可用', textOf('', num(point, 'connectionsAvailable'))),
        card('活跃', textOf('', num(point, 'connectionsActive')), 'ok', '正在执行操作的连接。空闲连接不算在这里。'),
        card('占用', textOf('%', usage), usageLevel, '当前连接占当前加可用的比例，也就是接近连接上限的程度。'),
      ],
      metrics: [
        { code: 'connectionsCurrent', name: '当前连接' },
        { code: 'connectionsActive', name: '活跃连接' },
        { code: 'connUsagePercent', name: '占用', unit: '%' },
      ],
    },
    {
      code: 'memory',
      title: '内存',
      cards: [
        card('常驻', textOf('byte', num(point, 'residentBytes')), 'ok', '进程常驻内存。'),
        card('缓存已用', textOf('byte', num(point, 'cacheUsedBytes'))),
        card('缓存上限', textOf('byte', num(point, 'cacheMaxBytes')), 'ok', 'WiredTiger 缓存配置的上限。'),
        card('缓存占用', textOf('%', num(point, 'cacheUsagePercent')), 'ok', '已用缓存除以上限。WiredTiger 会把缓存用到接近上限，单看这个比例不能判断故障。'),
        card('脏数据', textOf('byte', num(point, 'cacheDirtyBytes'))),
        card('脏数据占比', textOf('%', dirty), dirtyLevel, '脏数据占缓存上限的比例。大约到 5% 开始淘汰，到 20% 会明显变慢。'),
      ],
      metrics: [
        { code: 'cacheUsedBytes', name: '缓存已用', unit: 'byte' },
        { code: 'cacheDirtyBytes', name: '脏数据', unit: 'byte' },
        { code: 'residentBytes', name: '常驻内存', unit: 'byte' },
      ],
    },
    {
      code: 'ops',
      title: '操作',
      cards: [
        card('每秒', textOf('/s', num(point, 'opsPerSec')), 'ok', '两次采集之间，插入、查询、更新、删除、命令和取更多的合计。第一拍还没有上一拍时显示 —。'),
        card('查询', textOf('/s', num(point, 'opQueryPerSec'))),
        card('取更多', textOf('/s', num(point, 'opGetmorePerSec')), 'ok', 'getmore，把游标里剩下的结果取回来。'),
        card('插入', textOf('/s', num(point, 'opInsertPerSec'))),
        card('更新', textOf('/s', num(point, 'opUpdatePerSec'))),
        card('删除', textOf('/s', num(point, 'opDeletePerSec'))),
        card('命令', textOf('/s', num(point, 'opCommandPerSec'))),
      ],
      metrics: [
        { code: 'opQueryPerSec', name: '查询', unit: '/s' },
        { code: 'opGetmorePerSec', name: '取更多', unit: '/s' },
        { code: 'opInsertPerSec', name: '插入', unit: '/s' },
        { code: 'opUpdatePerSec', name: '更新', unit: '/s' },
        { code: 'opDeletePerSec', name: '删除', unit: '/s' },
        { code: 'opCommandPerSec', name: '命令', unit: '/s' },
      ],
    },
    {
      code: 'traffic',
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
      code: 'replica',
      title: '复制',
      cards: [
        card('延迟', lagText, lagLevel, '主节点显示最慢从节点落后的秒数。从节点显示自己落后主节点的秒数。单机和路由没有这项。'),
        card('成员', textOf('', members)),
        card('异常成员', textOf('', unhealthy), unhealthy != null && unhealthy > 0 ? 'alert' : 'ok', '健康检查不是 1 的成员。'),
      ],
      metrics: [{ code: 'replicaLagSec', name: '复制延迟', unit: 's' }],
    },
    {
      code: 'queue',
      title: '排队',
      cards: [
        card('读排队', textOf('', readers), readerLevel, '已经在等全局锁的读操作。'),
        card('写排队', textOf('', writers), writerLevel, '已经在等全局锁的写操作。'),
        card('读票据', textOf('', readTickets), readTickets === 0 ? 'alert' : 'ok', 'WiredTiger 还能同时开始的读事务数。变成 0 时新的读要等。'),
        card('写票据', textOf('', writeTickets), writeTickets === 0 ? 'alert' : 'ok', 'WiredTiger 还能同时开始的写事务数。变成 0 时新的写要等。'),
        card('断言', textOf('/s', asserts), assertLevel, '两次采集之间新出现的断言，按每秒计。包含 regular、warning、msg 和 user。正常是 0。'),
      ],
      metrics: [
        { code: 'queuedReaders', name: '读排队' },
        { code: 'queuedWriters', name: '写排队' },
        { code: 'readTickets', name: '读票据' },
        { code: 'writeTickets', name: '写票据' },
      ],
    },
    {
      code: 'storage',
      title: '库',
      cards: [card('占用', textOf('byte', num(point, 'dataBytes')), 'ok', '各库 sizeOnDisk 的合计。')],
      metrics: [{ code: 'dataBytes', name: '占用', unit: 'byte' }],
    },
    {
      code: 'slow',
      title: '慢操作',
      cards: [card('正在执行', textOf('', slow), slowLevel, '这一刻还在执行、并且已经超过 1 秒的操作。不是历史慢日志。')],
      metrics: [{ code: 'slowOps', name: '慢操作' }],
    },
  ]
}

function queueLevel(value: number | null): BoardCard['level'] {
  if (value == null || value < 0 || value === 0) return 'ok'
  return value >= 10 ? 'alert' : 'watch'
}

function kafkaBoard(
  up: BoardCard,
  version: BoardCard,
  point: Point | null,
  problem: (text: string, level: Board['level']) => void,
): BoardSection[] {
  const offlineBrokers = num(point, 'offlineBrokers')
  const offline = num(point, 'offlinePartitions')
  const under = num(point, 'underReplicated')
  const preferred = num(point, 'preferredReplicas')
  const minIsr = num(point, 'underMinIsr')
  const rebalance = num(point, 'rebalanceGroups')
  const online = num(point, 'onlineBrokers')
  const brokers = num(point, 'brokerCount')
  if (offlineBrokers != null && offlineBrokers > 0) problem(`不可达 Broker ${offlineBrokers}`, 'alert')
  if (offline != null && offline > 0) problem(`离线分区 ${offline}`, 'alert')
  if (under != null && under > 0) problem(`未同步分区 ${under}`, 'watch')
  if (minIsr != null && minIsr > 0) problem(`低于最小 ISR ${minIsr}`, 'alert')
  if (preferred != null && preferred > 0) problem(`首选副本偏离 ${preferred}`, 'watch')
  if (rebalance != null && rebalance > 0) problem(`再均衡 ${rebalance}`, 'watch')
  const lag = num(point, 'maxLag')
  let lagLevel: BoardCard['level'] = 'ok'
  if (lag != null && lag >= 10000) {
    lagLevel = 'alert'
    problem(`分区延迟 ${Math.round(lag)}`, 'alert')
  } else if (lag != null && lag >= 1000) {
    lagLevel = 'watch'
    problem(`分区延迟 ${Math.round(lag)}`, 'watch')
  }
  const brokerText =
    online == null && brokers == null ? '—' : `${textOf('', online)}/${textOf('', brokers)}`
  return [
    {
      code: 'cluster',
      title: '集群',
      cards: [
        up,
        version,
        card('Broker', brokerText, offlineBrokers != null && offlineBrokers > 0 ? 'alert' : 'ok'),
        card('离线分区', textOf('', offline), offline != null && offline > 0 ? 'alert' : 'ok', '没有可用 Leader 的分区。这些分区暂时不能读写。'),
        card('未同步分区', textOf('', under), under != null && under > 0 ? 'watch' : 'ok', '副本没有跟上 Leader 的分区。'),
        card('首选副本偏离', textOf('', preferred), preferred != null && preferred > 0 ? 'watch' : 'ok', 'Leader 不在配置的首选副本上的分区。'),
        card('低于最小 ISR', textOf('', minIsr), minIsr != null && minIsr > 0 ? 'alert' : 'ok', '同步副本数低于 min.insync.replicas。主题单独配置时按主题，否则用 Broker 默认值。'),
        card('日志占用', textOf('byte', num(point, 'logBytes')), 'ok', '各可达 Broker 的日志目录占用合计。读不到时是 —。'),
      ],
      metrics: [
        { code: 'onlineBrokers', name: '可达 Broker' },
        { code: 'offlineBrokers', name: '不可达 Broker' },
        { code: 'offlinePartitions', name: '离线分区' },
        { code: 'underReplicated', name: '未同步分区' },
        { code: 'preferredReplicas', name: '首选副本偏离' },
        { code: 'underMinIsr', name: '低于最小 ISR' },
        { code: 'logBytes', name: '日志占用', unit: 'byte' },
      ],
    },
    {
      code: 'consume',
      title: '消费',
      cards: [
        card('Topic', textOf('', num(point, 'topicCount'))),
        card('消费组', textOf('', num(point, 'groupCount'))),
        card('再均衡', textOf('', rebalance), rebalance != null && rebalance > 0 ? 'watch' : 'ok', '正在重新分配分区的消费组数量。'),
        card('最大分区延迟', textOf('', lag), lagLevel, '所有消费组里，单个分区还没消费的最大条数。'),
        card('总延迟', textOf('', num(point, 'totalLag')), 'ok', '各消费组未消费消息条数相加。'),
      ],
      metrics: [
        { code: 'groupCount', name: '消费组' },
        { code: 'rebalanceGroups', name: '再均衡' },
        { code: 'maxLag', name: '最大分区延迟' },
        { code: 'totalLag', name: '总延迟' },
      ],
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
    { code: 'availability', title: '可用性', cards: [up, role, version], metrics: [] },
    {
      code: 'latency',
      title: '压力',
      cards: [
        card('平均延迟', latency == null ? '—' : `${Math.round(latency)} ms`, latencyLevel),
        card('堆积请求', textOf('', outstanding), outstanding != null && outstanding >= 10 ? 'watch' : 'ok', '已经收到、还没处理完的请求。'),
      ],
      metrics: [
        { code: 'avgLatency', name: '平均延迟', unit: 'ms' },
        { code: 'outstandingRequests', name: '堆积请求' },
      ],
    },
    {
      code: 'scale',
      title: '规模',
      cards: [
        card('节点', textOf('', num(point, 'znodeCount'))),
        card('跟随者', textOf('', num(point, 'followers')), 'ok', '当前 Leader 看到的跟随者数量。单机没有这项。'),
        card('Watch', textOf('', num(point, 'watchCount')), 'ok', '客户端注册、还没触发的监听数量。'),
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
    { code: 'availability', title: '可用性', cards: [up], metrics: [] },
    {
      code: 'registry',
      title: '配置与服务',
      cards: [
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
