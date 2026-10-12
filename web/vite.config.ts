import { defineConfig, type PluginOption } from 'vite'
import vue from '@vitejs/plugin-vue'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'

const productVersion = JSON.parse(readFileSync(resolve(__dirname, '../package.json'), 'utf8')).version as string

const require = createRequire(import.meta.url)
const uiRoot = dirname(require.resolve('ensis-ui/package.json'))
const uiRequire = createRequire(resolve(uiRoot, 'package.json'))

// 发布包没有 src。ensisUiHost 会把桶导入改成未导出的深层路径，
// 预构建还会把 monaco 的 ?worker 当成文件名。开发直接走 dist 入口。
const dayjsAliases: { find: RegExp; replacement: string }[] = [
  { find: /^dayjs$/, replacement: uiRequire.resolve('dayjs/esm') },
  {
    find: /^dayjs\/plugin\/customParseFormat(?:\.js)?$/,
    replacement: uiRequire.resolve('dayjs/esm/plugin/customParseFormat'),
  },
]

function redirectWebRoot(): PluginOption {
  return {
    name: 'fluxguard-web-root',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = req.url || ''
        if (url === '/' || url.startsWith('/?')) {
          res.statusCode = 302
          res.setHeader('Location', '/fluxguard/' + url.slice(1))
          res.end()
          return
        }
        next()
      })
    },
  }
}

export default defineConfig({
  base: '/fluxguard/',
  define: {
    __FLUX_GUARD_VERSION__: JSON.stringify(productVersion),
  },
  plugins: [vue(), redirectWebRoot()],
  optimizeDeps: {
    exclude: ['ensis-ui'],
  },
  resolve: {
    alias: [{ find: '@', replacement: resolve(__dirname, 'src') }, ...dayjsAliases],
  },
  server: {
    port: 5174,
    fs: {
      allow: [resolve(__dirname), uiRoot],
    },
    proxy: {
      '/fluxguard/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
