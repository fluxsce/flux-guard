import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0003 = createApi(moduleApiPrefix('fg0003'))

export interface PluginInfo {
  pluginCode: string
  pluginName: string
  pluginKind: string
  pluginScope: string
  builtinFlag: string
}

export function queryPlugins() {
  return fg0003.get('/queryPlugins')
}
