import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { publicRoutes } from '@/router/routes-public';
import { MATERIAL_REGISTRY, THEME_REGISTRY } from '@/styles/theme/themes';
import { stripComments } from './source-scan';

// 这台的射程：**已经收口成 i18n 的文件不许再长出硬编码中文**。
//
// 为什么需要它：全仓 207 个带中文的文件要按域一批批搬进语言包，而"搬完的那一批会不会又被
// 写回中文"没有任何东西拦着 —— 加新页面的人复制相邻文件的写法，就会把硬编码文案带回来。
// 所以每批收口时把文件登记进下面的清单，登记即门禁：清单里的文件出现新的中文字面量立刻红，
// 而不是等到下一次全站走查才发现。
//
// 只查**会被用户看见**的位置：模板文本节点、模板属性值、脚本里的字符串字面量。
// 注释不参与（本仓的注释规矩是中文说理，把它们转成英文只会让代码更难读）。
//
// 反向验证：往登记表里任一文件塞一句 `const x = '探针'` → 第一条红；
// 把某个公开路由的 titleKey 换回 `title: '登录'` → 第二条红。

/** 已收口的文件（相对 web/）。新增一批就加一行；不要从这里删文件来"修红"。 */
const CLOSURED = [
  'src/views/auth/LoginView.vue',
  'src/views/auth/AuthorLoginView.vue',
  'src/components/auth/LoginForm.vue',
  'src/components/apple/ApFooter.vue',
  'src/components/apple/ApThemeMenu.vue',
  'src/router/guards.ts',
  'src/utils/core/app-info.ts',
];

// 码位写法，不写字形范围：这条以前是 `（相邻区段之间只靠一个裸连字符分隔）`，抄写/落盘时掉一个边界字符
// 就会多出一段 `U+0020–U+303F`，于是它对**纯英文行返回 true**（2026-10-12 实测）。
// 本文件没被它骗到，是因为下面三条形状判据各自内嵌了正确的码位区间 ——
// 也就是说这个常量的实际作用是"什么都没拦"。这类"看着在拦"的判据由
// tests/cjk-regex.test.ts 统一点名（它把每条 CJK 正则拿去试 ASCII 与中文）。
const CJK = /[\u3000-\u303f\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff00-\uffef]/;

// stripComments 住在 ./source-scan —— brand-closure 也要用它，两台门禁各抄一份迟早一份失效。
function visibleChineseLines(src: string): string[] {
  const cleaned = stripComments(src);
  const bad: string[] = [];
  cleaned.split('\n').forEach((line, i) => {
    // 只认会被渲染或被当值用的形状：>文本<、属性="…"、字符串字面量。
    const hits =
      />([^<>]*[\u3400-\u9fff][^<>]*)</.test(line) ||
      /[\w-]+\s*=\s*"[^"]*[\u3400-\u9fff][^"]*"/.test(line) ||
      /(['"`])[^'"`]*[\u3400-\u9fff][^'"`]*\1/.test(line);
    if (hits && CJK.test(line)) bad.push(`${i + 1}: ${line.trim().slice(0, 80)}`);
  });
  return bad;
}

describe('i18n 收口批次不回退', () => {
  it('登记过的文件里不再出现硬编码中文', () => {
    const offenders: string[] = [];
    for (const rel of CLOSURED) {
      const p = path.resolve(rel);
      if (!fs.existsSync(p)) {
        offenders.push(`${rel} 登记在表里但文件不存在`);
        continue;
      }
      const bad = visibleChineseLines(fs.readFileSync(p, 'utf8'));
      if (bad.length) offenders.push(`${rel}\n    ${bad.join('\n    ')}`);
    }
    expect(offenders).toEqual([]);
  }, 30000); // 扫登记清单（几十个文件）：并发满载时 5s 不够

  it('公开路由的标签页标题一律走 titleKey（英文界面不许出现中文标题）', () => {
    const flat = JSON.stringify(publicRoutes);
    expect(flat).not.toMatch(/"title":/);
    const missing = publicRoutes
      .filter((r) => !(r.meta as Record<string, unknown> | undefined)?.titleKey)
      .map((r) => String(r.path));
    expect(missing).toEqual([]);
  });

  it('路由 meta 里的每个 titleKey 在两套语言包中都存在', () => {
    // 路由标题是"英文界面冒出中文"最容易漏的一处：它不在页面上，在浏览器标签上。
    const zh = fs.readFileSync(path.resolve('src/i18n/locales/zh-CN.ts'), 'utf8');
    const en = fs.readFileSync(path.resolve('src/i18n/locales/en-US.ts'), 'utf8');
    const missing: string[] = [];
    for (const rel of ['src/router/routes-main.ts', 'src/router/routes-admin.ts', 'src/router/routes-public.ts', 'src/router/index.ts']) {
      const src = fs.readFileSync(path.resolve(rel), 'utf8');
      for (const m of src.matchAll(/titleKey: '([\w.]+)'/g)) {
        const leaf = m[1].split('.').pop() as string;
        // 纯字符串找缩进 + 键名，不拼正则：正则里的 \s 经过一层生成脚本就成了字面 "s"，
        // 于是这条判据永远"找不到键"，报出 50 条假违例（第一版就这样）。
        const has = (pack: string) => pack.includes(`\n    ${leaf}:`);
        if (!has(zh) || !has(en)) missing.push(`${rel}: ${m[1]}`);
      }
    }
    expect(missing).toEqual([]);
  });

  it('en-US 语言包自己不许夹中文（漏翻就是英文界面里的中文）', () => {
    const en = fs.readFileSync(path.resolve('src/i18n/locales/en-US.ts'), 'utf8');
    const lines = en.split('\n').map((l, i) => [i + 1, l] as const)
      .filter(([, l]) => !/^\s*(\/\/|\*|\/\*)/.test(l))
      .filter(([, l]) => /[\u3400-\u9fff]/.test(l));
    expect(lines.map(([n, l]) => `${n}: ${l.trim().slice(0, 70)}`)).toEqual([]);
  });

  it('新增外观必须同时有语言包里的外观名', () => {
    // 主题菜单按 id 建表（不改 THEME_REGISTRY 的 label，那是另一批的射程）。
    // 表是手工的，所以"加了第 5 套外观但没加名字"这种错只能靠数数发现 ——
    // 少一个键的表现是英文界面上那一项渲染成 undefined，而不是报错。
    const zh = fs.readFileSync(path.resolve('src/i18n/locales/zh-CN.ts'), 'utf8');
    const themeKeys = [...zh.matchAll(/^\s{4}(theme[A-Z][A-Za-z]*):/gm)].map((m) => m[1]);
    expect(themeKeys.length).toBeGreaterThanOrEqual(THEME_REGISTRY.length);
    const materials = [...zh.matchAll(/^\s{4}(material[A-Z][A-Za-z]*):/gm)].map((m) => m[1]);
    expect(materials.length).toBeGreaterThanOrEqual(MATERIAL_REGISTRY.length);
  });

  // 运行时生成的中文是 CJK 字面量门禁的**盲区**：源码里根本没有中文字符，
  // "2026年10月11日星期日"是 Intl 按写死的 'zh-CN' 算出来的。界面切英文、日期还留着中文，
  // 就是这一类。判据只盯格式化调用的第一个参数：要么不写（跟随运行环境），要么写 appIntlLocale()。
  it('日期/数字格式化的语言跟着界面开关，不许写死', () => {
    const RE = /(toLocale(?:String|DateString|TimeString|Number)|new Intl\.\w+)\(\s*['"](zh|en)/;
    const hits: string[] = [];
    const walk = (dir: string) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) {
          if (e.name === 'i18n') continue; // 开关本体住在这里，它当然认识 'zh-CN'
          walk(p);
          continue;
        }
        if (!/\.(vue|ts)$/.test(e.name) || /\.test\.ts$/.test(e.name) || e.name.endsWith('.d.ts')) continue;
        fs
          .readFileSync(p, 'utf8')
          .split('\n')
          .forEach((l, i) => {
            if (/^\s*(\/\/|\*|\/\*)/.test(l)) return;
            if (RE.test(l)) hits.push(`${path.relative(process.cwd(), p)}:${i + 1}: ${l.trim().slice(0, 76)}`);
          });
      }
    };
    walk(path.resolve('src'));
    expect(hits).toEqual([]);
  }, 30000); // 递归扫 src：默认 5s 在钩子并发满载时不够（实测 16s）

  it('日期语言门禁拦得住写死的 locale（自证不是哑的）', () => {
    const RE = /(toLocale(?:String|DateString|TimeString|Number)|new Intl\.\w+)\(\s*['"](zh|en)/;
    expect(RE.test("  return date.toLocaleDateString('zh-CN');")).toBe(true);
    expect(RE.test('  return date.toLocaleDateString(appIntlLocale());')).toBe(false);
    expect(RE.test('  return date.toLocaleString();')).toBe(false);
  });

  // 模块顶层的常量数组/对象里调 t()，等于**在 import 那一刻把语言钉死**：
  // 之后切界面语言不会重算，界面上留着旧语言。表现是"切了语言但某些列表还是中文"，
  // 而 type-check / eslint / 单测 / 设计契约全绿（批次十三在 useAIWriter 与 docTemplates 上踩到）。
  // 正确写法：把它做成函数（defaultTemplates()）或 computed，在渲染期取值。
  it('模块顶层常量里不许直接调 t()（语言会在 import 时被钉死）', () => {
    const hits: string[] = [];
    const walk = (dir: string) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) {
          if (e.name === 'i18n' || e.name === 'locales') continue;
          walk(p);
          continue;
        }
        if (!/\.ts$/.test(e.name) || /\.test\.ts$/.test(e.name) || e.name.endsWith('.d.ts')) continue;
        const lines = fs.readFileSync(p, 'utf8').split('\n');
        lines.forEach((l, i) => {
          const m = l.match(/^(?:export )?const (\w+)[^=]*= (\[|\{)$/);
          if (!m) return;
          const open = m[2];
          const close = open === '[' ? ']' : '}';
          let depth = 0;
          let j = i;
          for (; j < lines.length; j++) {
            for (const ch of lines[j]) {
              if (ch === open) depth++;
              else if (ch === close) depth--;
            }
            if (depth <= 0) break;
          }
          const body = lines.slice(i, j + 1).join('\n');
          if (/\bt\('/.test(body)) hits.push(`${path.relative(process.cwd(), p)}:${i + 1} ${m[1]}`);
        });
      }
    };
    walk(path.resolve('src'));
    expect(hits).toEqual([]);
  }, 30000); // 同上：递归扫 src 的第二遍

  it('这条"顶层常量钉死语言"门禁拦得住真实写法（自证不是哑的）', () => {
    const probe = "export const DEFAULT_TEMPLATES: AITemplate[] = [\n  { name: t('x.y') },\n];";
    expect(/^(?:export )?const (\w+)[^=]*= \[$/m.test(probe.split('\n')[0])).toBe(true);
    expect(/\bt\('/.test(probe)).toBe(true);
  });
});
