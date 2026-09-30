import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const api = createApi(moduleApiPrefix('fg0041'))

export interface Template {
  templateId: string
  templateName: string
  channelKind: string
  subjectText: string
  bodyText: string
  contentType: string
  builtInFlag: string
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

export interface TemplateInput {
  templateId: string
  templateName: string
  channelKind: string
  subjectText: string
  bodyText: string
  contentType: string
  statusFlag: string
  activeFlag: string
  noteText: string
}

export interface PreviewResult {
  subjectText: string
  bodyText: string
  contentType: string
}

export function queryTemplates(params: Record<string, string | number>): Promise<ApiResult<PageData<Template>>> {
  return api.get<PageData<Template>>('/queryTemplates', params)
}

export function createTemplate(body: TemplateInput): Promise<ApiResult<Template>> {
  return api.post<Template>('/createTemplate', body)
}

export function updateTemplate(body: TemplateInput): Promise<ApiResult<Template>> {
  return api.post<Template>('/updateTemplate', body)
}

export function deleteTemplate(templateId: string) {
  return api.post('/deleteTemplate', { templateId })
}

export function previewTemplate(body: Pick<TemplateInput, 'templateId' | 'channelKind' | 'subjectText' | 'bodyText' | 'contentType'>) {
  return api.post<PreviewResult>('/previewTemplate', { ...body, sceneCode: 'fire' })
}
