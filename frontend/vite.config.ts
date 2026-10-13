import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// 代理目标可经环境变量注入，便于在 docker-compose 中将 /api 转发到 backend 服务。
// 本地未设置时默认指向本机 8080（与本地 go run 启动的后端一致）。
const proxyTarget = process.env.VITE_PROXY_TARGET ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    host: true,
    port: 3000,
    proxy: {
      '/uploads': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/healthz': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/readyz': {
        target: proxyTarget,
        changeOrigin: true,
      },
    },
  },
})
