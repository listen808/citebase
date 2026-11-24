#!/usr/bin/env node
/**
 * check:design —— 设计门禁（对齐 standards/前端设计标准.md 的 P0 三项）
 *
 * 扫的是"裸值"，不是"有没有设计"：
 *   1. font-size 裸 px（必须走 --fs-1..6）
 *   2. border-radius 裸 px（必须走 --r-xs/sm/md/lg/pill，50% 与 0 除外）
 *   3. 用 emoji 当图标（导航/按钮里出现 🔴🟠🖥 这类）
 *   4. 视图层硬编码 hex 色（令牌文件本身不算）
 *
 * 用法：node scripts/check-design.mjs [目录，默认当前目录]
 * 退出码：有 bad 级问题 → 1；只有 warn → 0，方便先接入再收口。
 */
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, extname, basename, relative } from 'node:path'

const root = process.argv[2] || '.'
const EXTS = new Set(['.vue', '.svelte', '.css', '.scss', '.html', '.ts'])
// preview/ 是设计原型的静态 mockup（自带一套硬编码样式来模拟旧稿），
// 它由 tokens 同步生成才好比对，本身不进产品门禁
const SKIP_DIRS = new Set(['node_modules', 'dist', 'build', '.git', 'coverage', '.artifacts', 'preview', 'dist-offline'])
// 令牌/主题文件里出现裸值是天经地义的（它就是取值来源）
const TOKEN_FILE = /(^|[\\\\/])(tokens?|theme|app|style|variables)[\w.-]*\.(css|scss)$/i
const EMOJI = /[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u

const problems = []

function walk(dir) {
  for (const name of readdirSync(dir)) {
    if (SKIP_DIRS.has(name)) continue
    const fp = join(dir, name)
    if (statSync(fp).isDirectory()) walk(fp)
    else if (EXTS.has(extname(name))) scan(fp)
  }
}

function scan(fp) {
  const text = readFileSync(fp, 'utf8')
  const rel = relative(root, fp)
  const isToken = TOKEN_FILE.test(basename(fp))
  const lines = text.split('\n')
  lines.forEach((line, i) => {
    const where = `${rel}:${i + 1}`
    // 只管 px：em/rem 是相对字号（markdown 内联 code、富文本预览）的正当用法
    // HTML 内联 style（邮件模板、富文本片段）落不进 CSS 变量，单独放行
    const inlineHtml = /<[^>]+style=/.test(line)
    // 显式豁免：行内写 design-allow 并给一句理由（写进 docs/design.md 的豁免表）
    const allowed = /design-allow/.test(line)
    if (!isToken && !inlineHtml && !allowed && /font-size\s*:\s*\d+(\.\d+)?px/.test(line)) {
      problems.push({ level: 'bad', where, rule: 'font-size 裸 px', line: line.trim() })
    }
    if (!isToken && /border-radius\s*:\s*(?!var|0|50%)[^;]*\d/.test(line)) {
      problems.push({ level: 'warn', where, rule: 'border-radius 裸 px', line: line.trim() })
    }
    if (EMOJI.test(line) && /(class|aria-label|<button|<Icon|<span)/.test(line)) {
      problems.push({ level: 'warn', where, rule: '疑似 emoji 图标', line: line.trim() })
    }
    if (fp.endsWith('.vue') || fp.endsWith('.svelte')) {
      for (const m of line.matchAll(/#[0-9a-fA-F]{6}\b/g)) {
        problems.push({ level: 'warn', where, rule: '视图层 hex 色', line: m[0] })
      }
    }
  })
}

walk(root)

const bad = problems.filter(p => p.level === 'bad')
// 先打 bad 再打 warn：坏问题要一眼看到，警告可以翻页看
const sorted = [...bad, ...problems.filter(p => p.level !== 'bad')]
for (const p of sorted.slice(0, 80)) {
  console.log(`${p.level.toUpperCase()} ${p.where}  ${p.rule}  ${p.line}`)
}
console.log(`\n合计 ${problems.length} 处：bad ${bad.length} / warn ${problems.length - bad.length}`)
if (bad.length) {
  console.log('bad 级必须清零（裸字号请改 var(--fs-N)）')
  process.exit(1)
}
