import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0023 = createApi(moduleApiPrefix('fg0023'))

export interface Resource {
  resourceId: string
  resourceName: string
  resourceCode: string
  resourceType: string
  resourcePath?: string
  addTime?: string
  addWho?: string
  editTime?: string
  editWho?: string
  children?: Resource[]
}

export interface ResourceQuery {
  resourceName?: string
  resourceCode?: string
  resourceType?: string
}

export function queryResources(query: ResourceQuery = {}) {
  return fg0023.get('/queryResources', query)
}

export function getResource(resourceId: string) {
  return fg0023.get('/getResource', { resourceId })
}
