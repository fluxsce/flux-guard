import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0026 = createApi(moduleApiPrefix('fg0026'))

export interface PluginMethod {
  name: string
  ops?: string[]
}

export interface PluginInfo {
  pluginCode: string
  pluginName: string
  version: string
  pluginKind: string
  pluginScope: string
  builtinFlag: string
  component: string
  lang: string
  startup: string
  status: string
  pid?: number
  methods: PluginMethod[]
}

export function queryPlugins() {
  return fg0026.get('/queryPlugins')
}
