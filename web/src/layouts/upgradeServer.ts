import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0001 = createApi(moduleApiPrefix('fg0001'))

/**
 * 上传中央服务发行包并安排重启。
 * @param file - flux-guard-版本-系统-架构.tar.gz
 * @param sha256 - 该文件的 SHA256
 * @returns 服务端受理结果。成功后进程会退出。
 */
export function upgradeServer(file: File, sha256: string) {
  const body = new FormData()
  body.append('file', file)
  body.append('sha256', sha256.trim())
  return fg0001.post<{ message: string }>('/upgradeServer', body, undefined, { timeout: 30 * 60 * 1000 })
}
