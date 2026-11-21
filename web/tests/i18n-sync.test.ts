import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { baseCompile } from '@intlify/message-compiler';
import { createI18n } from 'vue-i18n';
import zhCN from '@/i18n/locales/zh-CN';
import enUS from '@/i18n/locales/en-US';
import { stripComments } from './source-scan';

const SRC = path.resolve('src');

function flatten(obj: unknown, prefix = ''): string[] {
  if (obj === null || typeof obj !== 'object') return [prefix];
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) => flatten(v, prefix ? `${prefix}.${k}` : k));
}

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(p, out);
    else if (/\.(vue|ts)$/.test(entry.name) && !entry.name.endsWith('.d.ts')) out.push(p);
  }
  return out;
}

function hasKey(messages: unknown, keyPath: string): boolean {
  return keyPath.split('.').reduce<unknown>((acc, part) => {
    if (acc && typeof acc === 'object') return (acc as Record<string, unknown>)[part];
    return undefined;
  }, messages) !== undefined;
}

const zhKeys = flatten(zhCN).sort();
const enKeys = flatten(enUS).sort();

// 先剥注释再取键：本仓注释用中文说理，历史写法常常就写成 t('...') 的样子，
// 不剥就会把一句解释当成一个真调用点（然后门禁去问"这个键在哪套语言包里"，白白红一次）。
function keysCalledIn(src: string): string[] {
  return [...src.matchAll(/\$?\bt\(\s*'([a-zA-Z0-9_.]+)'/g)].map((m) => m[1] as string);
}

const usedKeys = new Set<string>();
for (const file of walk(SRC)) {
  for (const key of keysCalledIn(stripComments(fs.readFileSync(file, 'utf8')))) usedKeys.add(key);
}

describe('i18n 语言包一致性', () => {
  it('zh-CN 与 en-US 的 key 集合完全一致', () => {
    expect(enKeys.filter((k) => !zhKeys.includes(k))).toEqual([]);
    expect(zhKeys.filter((k) => !enKeys.includes(k))).toEqual([]);
  });

  it('不存在空文案', () => {
    const empty = zhKeys.filter((k) => {
      const value = k.split('.').reduce<unknown>((acc, part) => (acc as Record<string, unknown>)?.[part], zhCN);
      return typeof value === 'string' && value.trim() === '';
    });
    expect(empty).toEqual([]);
  });
});

describe('i18n 调用点', () => {
  it('代码里用到的每个 key 在两套语言包中都存在', () => {
    const missing = [...usedKeys].filter((k) => !hasKey(zhCN, k) || !hasKey(enUS, k)).sort();
    expect(missing).toEqual([]);
  });

  it('确实存在被翻译的调用点（防止 i18n 退化为死代码）', () => {
    expect(usedKeys.size).toBeGreaterThan(0);
  });

  it('注释里的 t(...) 不算调用点（自证抽取器不是哑的）', () => {
    expect(keysCalledIn("const a = t('ws.k1');")).toEqual(['ws.k1']);
    expect(keysCalledIn("<!-- $t('ws.k2') -->")).toEqual(['ws.k2']); // 还没剥注释时它确实看得见
    expect(keysCalledIn(stripComments("// 以前这里写 t('ws.k2')"))).toEqual([]);
    expect(keysCalledIn(stripComments("<!-- 已下线的 $t('ws.k2') -->"))).toEqual([]);
    expect(keysCalledIn(stripComments("/* t('ws.k3') */"))).toEqual([]);
  });
});

// en-US 文件里混进英式拼写（favourite / colour / centre / analyse）只有母语读者能发现，
// 而批量翻译时人眼最容易被漏掉的就是这一类。这里一次拦下整族词形。
// 词尾 -ise 不能裸匹配：enterprise / emphasis 都是正当拼法，所以逐族列动词词干。
describe('en-US 拼写口径', () => {
  const BRITISH =
    /\b(favourit\w*|colour\w*|behaviour\w*|centre|labelled|analyse\w*|organis(?:e|es|ed|ing|ation)|initialis(?:e|es|ed|ing|ation)|visualis(?:e|es|ed|ing|ation)|authoris(?:e|es|ed|ing|ation)|prioritis\w*|customis\w*|recognis\w*|normalis\w*|maximis\w*|minimis\w*|summaris\w*|standardis\w*)\b/i;
  const file = path.resolve(SRC, 'i18n/locales/en-US.ts');

  it('语言包不含英式拼写', () => {
    const hits = fs
      .readFileSync(file, 'utf8')
      .split('\n')
      .map((l, i) => ({ l, n: i + 1 }))
      .filter((x) => BRITISH.test(x.l))
      .map((x) => `${x.n}: ${x.l.trim()}`);
    expect(hits).toEqual([]);
  });

  it('这条规则真的能拦住违规（先自证门禁不是哑的）', () => {
    expect("    k2: 'My favourites list'").toMatch(BRITISH);
    expect("    k3: 'Initialise the workspace'").toMatch(BRITISH);
    expect("    k2: 'My favorites list'").not.toMatch(BRITISH);
    // 这两条是误伤检查：以 -ise 结尾的正当词、以及 analysis 不能被拦
    expect("    k4: 'Enterprise analysis'").not.toMatch(BRITISH);
  });
});

// vue-i18n 的消息格式里 `@` 与 `{` 是**语法**：`@:key` 是链接消息、`{name}` 是插值占位符。
// 于是"@提及""可填 JSON，如 {"keep":4}"这类文案会让消息编译器在**渲染那一刻**报错，
// 页面靠回退拿到原文所以看不出坏，控制台却每条都刷红 —— 而且哪天回退策略变了就是白屏。
// 转义写法是字面量模式：`{'@'}`、`{'{'}`、`{'}'}`。
// 这条门禁不猜词形，直接把两套语言包喂给真的编译器逐条 t()，把 console.error/warn 收下来数。
describe('消息格式能被编译', () => {
  // 直接用 vue-i18n 底层的消息编译器逐条 compile，收 onError 的 token。
  // 不收 console：单测环境里 i18n 的默认错误打印会被拦走，靠"数控制台行数"会数出假结果
  // （第一版就是这么写的，24 条真违例只报了 2 条）。
  const un编译 = (messages: Record<string, unknown>) => {
    const bad: string[] = [];
    for (const key of flatten(messages)) {
      const value = key.split('.').reduce<unknown>((acc, part) => (acc as Record<string, unknown>)?.[part], messages);
      if (typeof value !== 'string') continue;
      const codes: string[] = [];
      try {
        baseCompile(value, { onError: (t) => codes.push(String(t.code)), onWarn: (t) => codes.push(String(t.code)) });
      } catch (e) {
        codes.push('throw:' + String((e as Error).message).slice(0, 60));
      }
      if (codes.length > 0) bad.push(`${key} [${[...new Set(codes)].join(',')}] ${value.slice(0, 60)}`);
    }
    return bad;
  };

  it('zh-CN 每条消息都能编译', () => {
    expect(un编译(zhCN as unknown as Record<string, unknown>)).toEqual([]);
  });

  it('en-US 每条消息都能编译', () => {
    expect(un编译(enUS as unknown as Record<string, unknown>)).toEqual([]);
  });

  it('这条门禁真的拦得住未转义的 @ 与 {（自证不是哑的）', () => {
    expect(un编译({ p: { a: '该文档还没有 @提及' } })).not.toEqual([]);
    expect(un编译({ p: { a: '可填 JSON，如 {"keep":4}' } })).not.toEqual([]);
    expect(un编译({ p: { a: '使用 {{nodeID}} 引用上游' } })).not.toEqual([]);
    expect(un编译({ p: { a: "该文档还没有 {'@'}提及" } })).toEqual([]);
    expect(un编译({ p: { a: '第{page}页' } })).toEqual([]);
  });

  // 转义是为了让编译器闭嘴，但**页面上必须还是原来的字**。这条断言防的是另一种失败：
  // 编译过了、渲染出来却带着一堆语法碎片（{\'@\'} 直接印在界面上）。
  it('转义写法渲染出来就是原字，不是语法碎片', () => {
    const i18n = createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': zhCN, 'en-US': enUS } });
    expect(i18n.global.t('ws.k49')).toBe('本文档的 @提及名单');
    expect(i18n.global.t('ws.k88')).toBe('该文档还没有 @提及');
    expect(i18n.global.t('aiSet.k95')).toContain('如 {"keep":4}');
    i18n.global.locale.value = 'en-US';
    expect(i18n.global.t('ws.k49')).toBe('@mentions in this document');
    expect(i18n.global.t('aiSet.k95')).toContain('e.g. {"keep":4}');
  });
});

// vue-i18n 把两档消息按 CLDR 的 **one | other** 取位：第 0 档是单数那一档。
// 批量翻译时最容易写反的就是这个顺序 —— time.minutesAgo / hoursAgo / daysAgo、
// common.itemCount、vWorkspaceUi.spaceCount 六条全都反着写过，于是英文界面显示
// "3 hour ago"（由 format-time.test.ts 实测坐实）。这类错编译得过、渲染也不报错，
// 只有母语读者看得出来，所以要机器判：把两档逐词比，唯一的差异必须是 x 与 x(+e)s。
describe('en-US 复数分支顺序', () => {
  function valueAt(messages: unknown, keyPath: string): unknown {
    return keyPath.split('.').reduce<unknown>((acc, part) => {
      if (acc && typeof acc === 'object') return (acc as Record<string, unknown>)[part];
      return undefined;
    }, messages);
  }

  // 返回 null = 这一条没问题；返回字符串 = 问题描述。
  function pluralOrderIssue(twoBranch: string): string | null {
    const [first, second] = twoBranch.split(' | ');
    const a = first.trim().split(/\s+/);
    const b = second.trim().split(/\s+/);
    if (a.length !== b.length) return `两档词数不同（${first} / ${second}），机器判不了单复数，请人工确认第 0 档是单数`;
    const diffs = a.map((word, i) => (word === b[i] ? null : [word, b[i]])).filter((x): x is [string, string] => x !== null);
    if (diffs.length === 0) return `两档完全相同（${first}），复数分支等于没写`;
    if (diffs.length > 1) return `两档差异不止一处（${first} / ${second}），请改成只带词尾 -s 的形式`;
    const [one, other] = diffs[0];
    if (other === `${one}s` || other === `${one}es`) return null;
    return `第 0 档 "${one}" 不是第 1 档 "${other}" 的单数形式 —— 顺序写反了`;
  }

  const check = (messages: unknown, keys: string[]) => {
    const bad: string[] = [];
    for (const key of keys) {
      const value = valueAt(messages, key);
      if (typeof value !== 'string') continue;
      if (!value.includes(' | ') || value.split(' | ').length !== 2) continue;
      if (!/\{[^}]*\}/.test(value)) continue; // 没有插值计数参数的两档不归这里管
      const issue = pluralOrderIssue(value);
      if (issue) bad.push(`${key}: ${issue}`);
    }
    return bad;
  };

  it('en-US 每条两档消息的第 0 档都是单数', () => {
    expect(check(enUS, enKeys)).toEqual([]);
  });

  it('这条门禁真的拦得住写反的顺序（自证不是哑的）', () => {
    // 反例就是这次实际写错的那几条
    expect(check({ p: { a: '{n} hours ago | {n} hour ago' } }, ['p.a'])[0]).toContain('顺序写反');
    expect(check({ p: { a: '{n} items | {n} item' } }, ['p.a'])[0]).toContain('顺序写反');
    // 正例
    expect(check({ p: { a: '{n} hour ago | {n} hours ago' } }, ['p.a'])).toEqual([]);
    expect(check({ p: { a: '{n} box | {n} boxes' } }, ['p.a'])).toEqual([]);
    // 三档（zero | one | other）与不带插值的两档不由这条管
    expect(check({ p: { a: 'No items | One item | {n} items' } }, ['p.a'])).toEqual([]);
    expect(check({ p: { a: 'On | Off' } }, ['p.a'])).toEqual([]);
  });

  it('真的把渲染也跑一遍：n=1 与 n=3 必须各取一档', () => {
    const i18n = createI18n({ legacy: false, locale: 'en-US', messages: { 'en-US': enUS } });
    expect(i18n.global.t('time.hoursAgo', { n: 1 })).toBe('1 hour ago');
    expect(i18n.global.t('time.hoursAgo', { n: 3 })).toBe('3 hours ago');
    expect(i18n.global.t('common.itemCount', { n: 1 })).toBe('1 item');
    expect(i18n.global.t('common.itemCount', { n: 12 })).toBe('12 items');
  });
});
