import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0001 = createApi(moduleApiPrefix('fg0001'))

export interface Overview {
  agentCount: number
  onlineCount: number
  pluginCount: number
  eventCount: number
  lastReceivedTime?: string
}

export function queryOverview() {
  return fg0001.get('/overview')
}
