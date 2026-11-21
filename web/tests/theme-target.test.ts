import { describe, expect, it } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { join, resolve } from 'node:path';
// @ts-expect-error —— .mjs 工装模块不在 tsconfig include 里
import { stripCommentsKeepLines as stripComments } from '../scripts/lib/js-objects.mjs';
// @ts-expect-error —— .mjs 工装模块不在任何 tsconfig 的 include 里，只做运行时解析
import { parseThemeTarget, themeStorageEntries, MATERIAL_KEY, GLASS_CLEAR_KEY, ALL_THEME_TARGETS, parseThemeList } from '../scripts/lib/theme-target.mjs';

/**
 * 钉住主题轮的两件事：**外观与材质是两条轴**，以及**不许把"没量到"记成"量过了"**。
 *
 * 触发这条的实测缺陷：`check:elements` 与 `snap` 只写 `ui:theme`、从不写 `ui:material`，
 * 于是 AGENTS.md 里那条 `--themes=light,dark,liquid-glass,notion-paper,editorial`
 * 的 `liquid-glass` 一轮被别名解成 base=light + 默认 solid —— 报表看着跑了五套，
 * 其实 light 跑了两遍，全站唯一带 backdrop-filter 与半透明表面的玻璃材质一次都没量。
 */

const ok = (spec: string) => {
  const t = parseThemeTarget(spec);
  if (!t.ok) throw new Error(`应当解析成功：${spec} -> ${t.error}`);
  return t;
};
const fail = (spec: string) => {
  const t = parseThemeTarget(spec);
  if (t.ok) throw new Error(`应当被拒绝：${spec} -> ${t.label}`);
  return t.error;
};

describe('parseThemeTarget', () => {
  it('裸外观名 = solid 材质，label 把两条轴都写出来', () => {
    const t = ok('editorial');
    expect(t).toMatchObject({ base: 'editorial', material: 'solid', clear: false });
    expect(t.label).toBe('editorial+solid');
  });

  it('显式组合与历史别名解成同一件事', () => {
    expect(ok('light+glass').label).toBe('light+glass');
    // 旧命令行里那个独立主题名要继续可用，但必须真的带上 glass
    expect(ok('liquid-glass').label).toBe('light+glass');
  });

  it('clear 是玻璃的变体：dark+clear 是 dark+glass+clear 的简写，solid 配 clear 直接拒', () => {
    expect(ok('dark+glass+clear').clear).toBe(true);
    expect(ok('dark+glass+clear').label).toBe('dark+glass+clear');
    expect(ok('dark+clear').label).toBe('dark+glass+clear');
    expect(fail('light+solid+clear')).toContain('glass');
  });

  it('纸面外观没有玻璃：必须当场拒绝，不能让 app 侧悄悄塌回 solid 再被记成量过 glass', () => {
    expect(fail('editorial+glass')).toContain('没有玻璃形态');
    expect(fail('warm-paper+glass')).toContain('没有玻璃形态');
  });

  it('拼错的外观与材质要说清楚有什么可给', () => {
    expect(fail('ligth')).toContain('light');
    expect(fail('light+frosted')).toContain('solid');
    expect(fail('')).not.toBe('');
  });
});

describe('themeStorageEntries', () => {
  it('九个数键一律显式写，包括 solid 与 false（否则下一轮沿用上一轮的材质或档位）', () => {
    const map = Object.fromEntries(themeStorageEntries(ok('light')));
    expect(map[MATERIAL_KEY]).toBe('solid');
    expect(map[GLASS_CLEAR_KEY]).toBe('false');
    // 4 个外观/材质键 + 5 条行为与个性化档（批1–批4）。
    // 个数写死是故意的：加了新的一档而这里没跟着改，就意味着某一轮会沿用上一轮的档位去量下一档
    // —— 那正是"看着量了八档、实际量的是默认档"那台假绿工装的复发条件。
    expect(Object.keys(map)).toHaveLength(9);
  });

  it('玻璃轮写下的就是 glass，与 light 轮互不污染', () => {
    expect(Object.fromEntries(themeStorageEntries(ok('light+glass')))[MATERIAL_KEY]).toBe('glass');
  });
});

/**
 * 全量枚举这台：判"哪些组合合法"的规则（玻璃能力、clear 依附 glass）只该有一份，
 * 所以清单由 parseThemeTarget 生成。这几条钉的是**覆盖面本身** ——
 * 以前每台工装各写死一个默认档，notion-paper / editorial / dark 全系 / 任何 solid
 * 从没被任何自动化跑过，而清单上的话说的是"运行时已按主题巡检"。
 */
describe('ALL_THEME_TARGETS 与 --themes=all', () => {
  const labels = ALL_THEME_TARGETS.map((t) => t.label);

  it('四种外观都在，且数量等于合法搭配数（不是 4×2×2）', () => {
    expect(ALL_THEME_TARGETS).toHaveLength(8);
    for (const base of ['light', 'dark', 'editorial', 'warm-paper']) {
      expect(labels.some((l) => l.startsWith(base + '+'))).toBe(true);
    }
  });

  it('玻璃只跟着有玻璃能力的外观；纸面外观不得出现在任何 glass 轮里', () => {
    expect(labels.filter((l) => l.includes('+glass'))).toEqual([
      'light+glass',
      'light+glass+clear',
      'dark+glass',
      'dark+glass+clear',
    ]);
    expect(labels.some((l) => /^(editorial|warm-paper)\+glass/.test(l))).toBe(false);
  });

  it('每一档都是 parseThemeTarget 认得的写法（枚举与解析不许有两个真相源）', () => {
    for (const t of ALL_THEME_TARGETS) {
      const again = parseThemeTarget(t.label.replace('+solid', ''));
      expect(again.ok || again.error).toBeTruthy();
      if (!again.ok) throw new Error(`${t.label} 解不回来：${again.error}`);
      expect(again.material).toBe(t.material);
      expect(again.clear).toBe(t.clear);
    }
  });

  it('标签互不重复（重复意味着有一档实际没被覆盖）', () => {
    expect(new Set(labels).size).toBe(labels.length);
  });

  it('--themes=all 取全量；具体名单照解；拼错照拒', () => {
    expect(parseThemeList('all').targets).toHaveLength(8);
    expect(parseThemeList('ALL').targets).toHaveLength(8);
    expect(parseThemeList('light,dark+glass+clear').targets.map((t) => t.label)).toEqual([
      'light+solid',
      'dark+glass+clear',
    ]);
    expect(parseThemeList('light+glass+frosted').ok).toBe(false);
    expect(parseThemeList('').ok).toBe(false);
  });
});

/**
 * 常驻防呆：themeStorageEntries 交出来的键**自带前缀**，谁再拼一次 'citebase:' 谁就在量默认档。
 *
 * 触发这台的实际缺陷：check:glass:runtime 写的是 localStorage.setItem('citebase:' + kk, vv)，
 * 于是落盘的是 citebase:citebase:ui:theme，应用读不到 -> 每一轮都用默认档渲染，
 * 而报表按 --themes 把这一轮标成 light+glass+clear。静态门禁与单元测试都看不见（它俩不跑浏览器），
 * 只有"读源码看有没有人二次拼前缀"这一条拦得住，所以做成常驻扫描。
 */
describe('消费方不许给主题键二次拼前缀', () => {
  const dir = resolve(fileURLToPath(import.meta.url), '..', '..', 'scripts');
  const walk = (at: string): string[] =>
    readdirSync(at, { withFileTypes: true }).flatMap((e) =>
      e.isDirectory() ? walk(join(at, e.name)) : [join(at, e.name)],
    );
  const consumers = walk(dir).filter((f) => f.endsWith('.mjs')).filter((f) => readFileSync(f, 'utf8').includes('lib/theme-target.mjs'));

  it('确实扫到了消费方（一台都没扫到 = 扫描本身坏了）', () => {
    expect(consumers.length).toBeGreaterThanOrEqual(3);
  });

  it("没有任何 setItem('citebase:' + …) 这种二次拼前缀的写法", () => {
    for (const f of consumers) {
      // 先剥注释：这条判据扫的是**会被执行的那行**，而解释这个 bug 的注释里
      // 必然要写出那个错误写法（check-glass-runtime.mjs 就这么写过，被自己拦了一次）。
      const src = stripComments(readFileSync(f, 'utf8'));
      const hit = src.match(/setItem\(\s*['"]citebase:['"]\s*\+/);
      if (hit) throw new Error(`${f} 又拼了一次前缀：${hit[0]} —— 主题轮会静默退回默认档`);
    }
  });

  it('枚举出来的每一个键都已经带 citebase: 前缀（上面那条判据的前提）', () => {
    for (const target of ALL_THEME_TARGETS) {
      for (const [k] of themeStorageEntries(target)) expect(k.startsWith('citebase:')).toBe(true);
    }
  });
});
