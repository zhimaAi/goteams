import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve, sep } from 'path'
import { fileURLToPath } from 'url'
import fs from 'fs'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

// Local upload file directory (must match the Go backend lib_storage write directory: <project root>/data/uploads).
const uploadsDir = resolve(__dirname, '..', 'data', 'uploads')

// Local dev/preview serves uploaded files directly: when the browser requests /uploads/..., it is read from the local data/uploads first.
// No unauthenticated Go route is needed; OSS mode returns a full URL and never hits this path.
function serveLocalUploads(server) {
  server.middlewares.use('/uploads', (req, res) => {
    const rel = (req.url || '')
      .replace(/^\/uploads\/?/, '')
      .replace(/^\/+/, '')
    const target = resolve(uploadsDir, decodeURIComponent(rel))
    // Path-traversal protection: the resolved path must remain inside uploadsDir.
    if (target !== uploadsDir && !target.startsWith(uploadsDir + sep)) {
      res.statusCode = 404
      res.end('Not Found')
      return
    }
    fs.stat(target, (err, stat) => {
      if (err || !stat.isFile()) {
        res.statusCode = 404
        res.end('Not Found')
        return
      }
      res.setHeader('Cache-Control', 'no-store')
      const stream = fs.createReadStream(target)
      stream.on('error', () => {
        res.statusCode = 404
        res.end('Not Found')
      })
      stream.pipe(res)
    })
  })
}

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const port = Number(env.VITE_PORT || 3000)
  const apiPrefix = env.VITE_API_PREFIX || '/api'
  const uploadsPrefix = env.VITE_UPLOAD_PREFIX || '/uploads'
  const apiProxyTarget = env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:8080'

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
      },
    },
    server: {
      port,
      proxy: {
        [apiPrefix]: {
          target: apiProxyTarget,
          changeOrigin: true,
          secure: false,
          ws: true,
        },
        [uploadsPrefix]: {
          target: apiProxyTarget,
          changeOrigin: true,
          secure: false,
        },
      },
    },
    preview: {
      port,
    },
    configureServer: (server) => serveLocalUploads(server),
    configurePreviewServer: (server) => serveLocalUploads(server),
  }
})
