import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// 前端单元测试配置。
//   - 环境：jsdom（request.ts 依赖 window/localStorage/document）
//   - 仅运行 .test.ts 文件，避免误抓 .spec 或生产代码
//   - 复用 vite 的 '@' 别名；'@user' 指向 user-frontend，便于跨项目复用同一套请求/鉴权测试
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@user': resolve(__dirname, '../user-frontend/src'),
      // user-frontend 自带 node_modules/axios，其 request.ts 的 refresh 走的是自己的 axios 副本；
      // 将其统一别名到根 axios，使测试中通过 axios.defaults.adapter 注入的 mock 对两者都生效。
      axios: resolve(__dirname, 'node_modules/axios'),
    },
  },
  server: {
    fs: {
      // 允许测试访问 user-frontend 源码（默认仅限 frontend 根目录）
      allow: [resolve(__dirname), resolve(__dirname, '../user-frontend')],
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    // 冷启动首次懒加载重型视图（含图表依赖）的 Vite 编译可能超过默认 5s，
    // 提高超时避免误报超时（对挂载重型视图的用例也更稳健）。
    testTimeout: 20000,
    hookTimeout: 20000,
    setupFiles: ['./vitest.setup.ts'],
    include: ['src/**/*.test.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      // 本机 safe-delete 拦截了对 coverage 目录的 rm，导致收尾清理由 vitest 触发 shim 崩溃；
      // 关闭自动清理并改用独立输出目录，避免触发删除操作。
      clean: false,
      reportsDirectory: 'coverage-report',
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/**/*.test.ts', 'src/main.ts', 'src/App.vue'],
    },
  },
})
