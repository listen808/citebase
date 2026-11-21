import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

// 这台盯的不是某条判据，而是**判据本身会不会骗人**：凡名字里带 CJK / Han / Chinese 的正则常量，
// 都必须"认得中文，且不把 ASCII 当中文"。
//
// 起因（2026-10-12 实测两处）：`[㐀-䶿一-鿿豈-﫿 -〿＀-￯]` 这种字形范围写法，抄写或落盘时
// 掉一个边界字符（䶿、豈），剩下的裸连字符就被 JS 解析成 `U+0020–U+303F` —— 于是这条正则
// **对纯英文行返回 true**。`tests/i18n-closure.test.ts` 与 `scripts/i18n-extract.mjs`
// 各带一条这样的预筛活了许多批：真正的判定另有内嵌的 `\u3400-\u9fff` 兜着，
// 所以坏判据的实际作用是"什么都没拦" —— 不报错、不变红，只是让人以为那里有闸。
// 同一类还有一次：品牌门禁里我照抄这条字形范围，把 122 条英文全判成违例
// （一直红的门禁与没有门禁一样）。所以"哑判据"这件事本身要有一台东西盯着。
//
// 这里刻意只测**行为**（认不认中文、会不会把 ASCII 算进来），不测写法：
// "检查写法"的工具自己也得匹配中文，会绕回同一个坑。
//
// 反向验证：把任一 CJK 正则改回字形范围并删掉一个边界字符 → 三条里必有红的。

const SCAN_DIRS = ['scripts', 'tests', path.join('..', 'tools')];
const NAME_RE = /^\s*(?:const|let|var)\s+(\w*(?:CJK|Han|Chinese)\w*)\s*=\s*\/(.+)\/[a-z]*;?\s*$/;

const CJK_PROBES = ['中文测试', '加载中', '保存成功', '知识库', '空间名已存在'];
// 逐个 ASCII 可打印字符试：正常写法（下界 \u3000 起）一个都不该命中。
const ASCII_PROBES = Array.from({ length: 0x7e - 0x20 + 1 }, (_, i) => String.fromCharCode(0x20 + i));

type Found = { where: string; name: string; source: string; re: RegExp | null };

function safeRegExp(source: string): RegExp | null {
  try {
    return new RegExp(source);
  } catch {
    return null;
  }
}

function collected(): Found[] {
  const found: Found[] = [];
  for (const dir of SCAN_DIRS) {
    const abs = path.resolve(dir);
    if (!fs.existsSync(abs)) continue;
    for (const entry of fs.readdirSync(abs, { recursive: true, encoding: 'utf8' })) {
      const rel = String(entry);
      if (!/\.(mjs|ts)$/.test(rel) || rel.endsWith('.d.ts') || rel.includes('cjk-regex')) continue;
      fs.readFileSync(path.join(abs, rel), 'utf8').split('\n').forEach((line, i) => {
        const m = NAME_RE.exec(line);
        if (!m) return;
        // 编译不了也算扫到（re = null），交给第一条用例点名，别让它安静消失
        const re = safeRegExp(m[2]);
        found.push({ where: `${rel}:${i + 1}`, name: m[1], source: m[2], re });
      });
    }
  }
  return found;
}

const label = ({ where, name, source }: Found) => `${where} ${name} /${source}/`;

describe('CJK 判据不能是哑的', () => {
  const regexes = collected();

  it('至少扫到一条 CJK 正则，且每条都能编译', () => {
    expect(regexes.length).toBeGreaterThan(0);
    expect(regexes.filter(({ re }) => re === null).map(label)).toEqual([]);
  });

  it('每条都必须认得中文（五个探针全部命中）', () => {
    const blind = regexes
      .filter((x): x is Found & { re: RegExp } => x.re !== null)
      .filter(({ re }) => !CJK_PROBES.every((probe) => re.test(probe)))
      .map(label);
    expect(blind).toEqual([]);
  });

  it('每条都必须把 ASCII 判成"不是中文"', () => {
    const wrong = regexes
      .filter((x): x is Found & { re: RegExp } => x.re !== null)
      .map(({ where, name, source, re }) => {
        const hits = ASCII_PROBES.filter((ch) => re.test(ch));
        return hits.length ? `${where} ${name} /${source}/ 把 ASCII 当中文：${hits.slice(0, 10).join(' ')}` : null;
      })
      .filter((x): x is string => x !== null);
    expect(wrong).toEqual([]);
  });
});
