import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const api = createApi(moduleApiPrefix('fg0042'))

export interface Channel {
  channelId: string
  channelName: string
  channelKind: string
  endpoint: string
  port: number
  username: string
  sender: string
  target: string
  optionText: string
  extraText: string
  templateId: string
  templateName: string
  secretSet: string
  statusFlag: string
  activeFlag: string
  noteText: string
  addTime: string
  addWho: string
  editTime: string
  editWho: string
  currentVersion: number
  oprSeqFlag: string
}

export interface ChannelInput {
  channelId: string
  channelName: string
  channelKind: string
  endpoint: string
  port: number
  username: string
  secret: string
  sender: string
  target: string
  optionText: string
  extraText: string
  templateId: string
  statusFlag: string
  activeFlag: string
  noteText: string
}

export function queryChannels(params: Record<string, string | number>): Promise<ApiResult<PageData<Channel>>> {
  return api.get<PageData<Channel>>('/queryChannels', params)
}

export function createChannel(body: ChannelInput): Promise<ApiResult<Channel>> {
  return api.post<Channel>('/createChannel', body)
}

export function updateChannel(body: ChannelInput): Promise<ApiResult<Channel>> {
  return api.post<Channel>('/updateChannel', body)
}

export function deleteChannel(channelId: string) {
  return api.post('/deleteChannel', { channelId })
}

export function testChannel(channelId: string) {
  return api.post('/testChannel', { channelId })
}
