import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = env.VITE_PROXY_API_TARGET || 'http://127.0.0.1:8080'

  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': {
          target: apiTarget,
          // Preserve the browser's Host header so the backend's strict
          // same-origin CSRF check sees the original admin origin during
          // local development. Production uses the native reverse proxy.
          changeOrigin: false
        }
      }
    }
  }
})
