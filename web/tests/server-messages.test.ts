import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { generate, readCatalog, buildCatalog } from '../scripts/gen-server-messages.mjs';
import { serverMessages } from '@/i18n/locales/server-messages';

// 这台的射程：**后端那张话术表，前端必须一条不落地认得**。
//
// 为什么需要它：消费点只有一处（api/client.ts 在构造 ApiError 时按 message_key 查
// server.<key>），查不到就走"退回服务端原文"那一支。退回中文不是崩，不会有人在英文界面上
// 把"零星几句中文"当 bug 报 —— 所以这类"安静失败"必须有门禁，不能靠人眼。
//
// 三条判据：
//   ① 生成物没过期（改了 messages.go 或 en.json 却没重跑生成 → 红）；
//   ② 键集双向相等：后端有的键英文必须有，英文里不许留后端已删的键；
//   ③ 每条译文都非空，且英文侧不许夹中文（夹中文就是没翻）。
//
// 反向验证：往 en.json 里删一个键 → ② 红；把生成物里的一个值改一个字 → ① 红；
// 把某条英文改成中文 → ③ 红。

const repoRoot = path.resolve(import.meta.dirname, '..', '..');
const generatedFile = path.resolve('src/i18n/locales/server-messages.ts');

describe('后端话术表的前端消费面', () => {
  it('生成物与上游一致（改完表必须重跑 gen:server-messages）', () => {
    const { content } = generate();
    const onDisk = fs.readFileSync(generatedFile, 'utf8');
    // 逐字符比：只比键集的话，"值改了但没重生成"会漏过去，而那正是最常见的失效形态。
    expect(onDisk).toBe(content);
  });

  it('后端每个键都有中文与英文，且不残留后端已不认的键', () => {
    const zh = readCatalog(fs.readFileSync(path.join(repoRoot, 'internal', 'response', 'messages.go'), 'utf8'));
    const en = JSON.parse(fs.readFileSync(path.join(repoRoot, 'web', 'scripts', 'server-messages.en.json'), 'utf8'));
    const built = buildCatalog(zh, en); // 缺键/多键/空文案/插值都在这里抛

    const packZh = serverMessages['zh-CN'].server as Record<string, string>;
    const packEn = serverMessages['en-US'].server as Record<string, string>;

    expect(Object.keys(packZh).sort()).toEqual(built.slugs);
    expect(Object.keys(packEn).sort()).toEqual(built.slugs);
    // 语言包与上游文案也必须一致（生成物里那份就是表里的值，不接受第三份真相）
    for (const slug of built.slugs) {
      expect(packZh[slug]).toBe(zh[slug]);
      expect(packEn[slug]).toBe(en[slug]);
    }
  });

  it('英文侧不许夹中文（漏翻在界面上就是中文混进英文）', () => {
    const packEn = serverMessages['en-US'].server as Record<string, string>;
    // 显式码位区间，不抄字形范围：上一版从 i18n-closure 抄过来的那段范围在这里匹配了**全部** 122 条
    // —— 抄写时一个边界字符掉了，判据就从"挑出漏翻"变成"全都算漏翻"，而这种失真只会一直红，
    // 一直红的门禁和没有门禁一样。
    const cjk = /[\u3000-\u303F\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF\uFF00-\uFFEF]/;
    const offenders = Object.entries(packEn).filter(([, v]) => cjk.test(String(v)));
    expect(offenders.map(([k]) => k)).toEqual([]);
  });
});
