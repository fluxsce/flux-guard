import { defineConfig, type PluginOption } from 'vite'
import vue from '@vitejs/plugin-vue'
import { niumaUiHost } from 'niuma-ui/vite-plugins/niuma-ui-host'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'

const require = createRequire(import.meta.url)
const uiRoot = dirname(require.resolve('niuma-ui/package.json'))

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
  plugins: [vue(), ...(niumaUiHost() as PluginOption[]), redirectWebRoot()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
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
