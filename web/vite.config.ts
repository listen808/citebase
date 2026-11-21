import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import Components from 'unplugin-vue-components/vite'
import AutoImport from 'unplugin-auto-import/vite'
import { VitePWA } from 'vite-plugin-pwa'
import compression from 'vite-plugin-compression'

// 版本号只有一个来源：package.json。以前 app-info.ts 里另抄了一份 'v2.0.0'，
// 改版本时要记得改两处，而没有任何东西会提醒你忘了第二处。
const pkgVersion = (
  JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')) as { version: string }
).version

export default defineConfig(({ command, mode }) => {
  const isDev = command === 'serve';
  
  return {
    base: '/',
    define: {
      __APP_VERSION__: JSON.stringify(pkgVersion),
      // vue-i18n 默认用 `new Function` 编译消息，会被后端的 CSP script-src 'self' 挡掉，
      // 表现为 bootstrap() 在 app.mount() 前抛 EvalError、页面全白。
      // JIT 走 AST 解释器，不碰 eval；开着 CSP 的部署必须置 true。
      __INTLIFY_JIT_COMPILATION__: true,
    },
    plugins: [
      vue(),
      // 仅在开发模式下启用 Vue DevTools
      isDev && vueDevTools(),
      VitePWA({
        registerType: 'autoUpdate',
        includeAssets: ['favicon.ico'],
        manifest: {
          name: 'Citebase',
          short_name: 'Citebase',
          // 装出来的应用语言标签原先是插件默认的 "en"，而 index.html 是 zh-CN —— 两边不一致
          lang: 'zh-CN',
          dir: 'ltr',
          id: '/',
          description: '轻盈知识库管理系统',
          theme_color: '#ffffff',
          background_color: '#f5f5f7',
          display: 'standalone',
          scope: '/',
          start_url: '/',
          icons: [
            // 位图一栏全是 `npm run gen:brand-icons` 从 public/favicon.svg / public/pwa-icon.svg
            // 生成的派生物（两枚 SVG 与顶栏 logo 同一图形与色值，见 pwa-icon.svg 内注释）。
            // sizes 不是手填的：web/tests/brand-closure.test.ts 把每条声明对回文件里的真实像素，
            // 并把每枚位图内嵌的几何戳对回 ApBrand.vue。
            // 原先这里只有一条 `{ src: '/favicon.ico', sizes: '64x64' }` —— 那个文件实际是
            // 改名前那枚旧标记的单条 32×32，声明与内容两头都是假的，而 Android 的安装门槛要 192+。
            { src: '/pwa-icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
            { src: '/pwa-icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'maskable' },
            { src: '/pwa-icon-192.png', sizes: '192x192', type: 'image/png' },
            { src: '/pwa-icon-512.png', sizes: '512x512', type: 'image/png' },
            // maskable 另出一枚满幅方形：瓦片那圈圆角会被系统遮罩切走，边缘就剩透明缺口
            { src: '/pwa-icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
            // .ico 是多条载荷，sizes 就把真实条目全列出来，Windows 挑尺寸才有的比
            { src: '/favicon.ico', sizes: '16x16 32x32 48x48', type: 'image/x-icon' },
          ],
        },
        workbox: {
          maximumFileSizeToCacheInBytes: 2 * 1024 * 1024,
          globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
          // 大体积按需库不进安装期清单：monaco / mermaid / highlight 都是
          // "这一页可能根本不出现"的 chunk（highlight.js 全语言包实测 963kB / gzip 307kB），
          // precached 意味着每个首次访问的人不管看没看过代码块都要先下 1MB。
          // 它们改由下面那条 CacheFirst 运行时规则在真正被请求时才缓存 ——
          // 与 monaco/mermaid 同一套机制，不是新开一条。
          globIgnores: ['**/vendor-monaco-*.js', '**/vendor-mermaid-*.js', '**/vendor-highlight-*.js', '**/*.worker-*.js'],
          runtimeCaching: [
            // P2: 文档正文（/api/v1/documents/.*/content）不再缓存到 Cache Storage。
            // 原配置用 NetworkFirst + 24h/50 条缓存，登出/换账号后离线仍可读到私有文档，
            // 构成越权读取风险。如需离线阅读，应配合登出时 caches.delete('doc-content-cache')
            // 并按用户隔离缓存键，当前暂不启用。
            {
              urlPattern: /\/(?:assets|dist\/assets)\/(?:vendor-monaco-[^/]+|vendor-mermaid-[^/]+|vendor-highlight-[^/]+|[^/]*\.worker[^/]*)\.js$/,
              handler: 'CacheFirst',
              options: {
                cacheName: 'large-vendor-cache',
                expiration: { maxEntries: 20, maxAgeSeconds: 30 * 24 * 60 * 60 },
              },
            },
          ],
        },
      }),
      Components({
        dirs: ['src/components/apple'],
        dts: 'src/components.d.ts',
        directoryAsNamespace: false,
        include: [/\.vue$/],
      }),
      AutoImport({
        imports: ['vue', 'vue-router', 'pinia'],
        dts: 'src/auto-imports.d.ts',
        dirs: ['src/composables/**', 'src/stores/**'],
        vueTemplate: true,
      }),
      // 仅在生产模式下启用压缩
      !isDev && compression({
        algorithm: 'gzip',
        ext: '.gz',
        threshold: 1024,
        deleteOriginFile: false,
      }),
      !isDev && compression({
        algorithm: 'brotliCompress',
        ext: '.br',
        threshold: 1024,
        deleteOriginFile: false,
      }),
    ].filter(Boolean),
    server: {
      port: 3000,
      host: '0.0.0.0', // 允许局域网设备访问
      strictPort: true,
      proxy: {
        '/api/v1/ws': {
          target: 'ws://localhost:3001',
          ws: true,
          changeOrigin: true,
        },
        '/api/v1/collab/ws': {
          target: 'ws://localhost:3001',
          ws: true,
          changeOrigin: true,
        },
        '/api/v1': {
          target: 'http://localhost:3001',
          changeOrigin: true,
        },
        '/images': {
          target: 'http://localhost:3001',
          changeOrigin: true,
        },
        '/attachments': {
          target: 'http://localhost:3001',
          changeOrigin: true,
        },
      },
      // 优化开发服务器性能
      watch: {
        ignored: ['**/node_modules/**', '**/.git/**', '**/data/**'],
      },
    },
    build: {
      cssMinify: 'esbuild',
      cssCodeSplit: true,
      sourcemap: mode === 'development', // 仅在开发模式生成 sourcemap
      target: 'es2020',
      minify: 'esbuild',
      reportCompressedSize: false,
      chunkSizeWarningLimit: 1000, // 提高 chunk 警告阈值
      rollupOptions: {
        output: {
          manualChunks(id: string) {
            if (id.includes('node_modules/monaco-editor')) return 'vendor-monaco';
            if (id.includes('node_modules/katex')) return 'vendor-katex';
            if (id.includes('node_modules/mermaid')) return 'vendor-mermaid';
            if (id.includes('node_modules/highlight.js')) return 'vendor-highlight';
            if (id.includes('node_modules/vue') || id.includes('node_modules/vue-router') || id.includes('node_modules/pinia')) return 'vendor-vue';
            if (id.includes('node_modules/lucide-vue-next')) return 'vendor-lucide';
            if (id.includes('node_modules/marked')) return 'vendor-markdown';
            if (id.includes('node_modules')) return 'vendor-other'; // 其他第三方库统一打包
          },
        },
      },
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      },
    },
    // 优化依赖预构建
    optimizeDeps: {
      include: [
        'vue', 'vue-router', 'pinia', 'axios',
        'marked', 'highlight.js', 'katex', 'mermaid',
        // morphicons 形变引擎（图标几何数据已内联在 components/apple/morph-icon-data.ts，
        // 不再引 lucide 数据包，免得 dev 里反复 "Failed to resolve import"）。
        'morphicons/vue',
      ],
    },
  };
})
