import { fileURLToPath } from 'node:url'
import { readFileSync } from 'node:fs'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

// 与 vite.config.ts 同源：单测里凡是 import 到 app-info（页脚、顶栏、关于页都引它）
// 都需要这个注入值，否则测试环境里 __APP_VERSION__ 是 undefined，
// 于是"关于页显示什么版本"这件事只在构建产物里对、在测试里悄悄是 vundefined。
const pkgVersion = (
  JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')) as { version: string }
).version

export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(pkgVersion),
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    // P2-10: 关闭 globals: true，避免与 TS 自动 import 冲突
    // 测试中应显式 import { describe, it, expect } from 'vitest'
    globals: false,
    environment: 'happy-dom',
    // 组件模板现在用 $t()，挂载时必须带上 i18n 插件，且语言钉在 zh-CN ——
    // 否则断言拿到的是 undefined，测试会以看不懂的形式失败。
    setupFiles: ['tests/setup.ts'],
    // passWithNoTests 已移除: 空测试套件应真实失败而非虚假通过。
    // 当前仓库尚无单测文件，npm run test 会报错——这是预期行为，
    // 补齐 src/**/*.test.ts 后即恢复通过。
    include: ['src/**/*.{test,spec}.{js,ts}', 'tests/**/*.{test,spec}.{js,ts}'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: ['src/**/*.{ts,vue}'],
      exclude: [
        'src/**/*.d.ts',
        'src/**/*.spec.ts',
        'src/**/*.test.ts',
        'src/main.ts',
        'src/App.vue',
        'src/router/**',
        'src/api/**',
        'src/stores/**',
        'src/auto-imports.d.ts',
        'src/components.d.ts',
      ],
    },
  },
})
