import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { sourceFiles, stripComments } from './source-scan';

// 内部判别串一旦混进 ApiError 的 message，就会顺着 ErrorBoundary 的"原因"格子印到界面上 ——
// 实测截图里就是 `[rate_limit]请求过于频繁，请稍后再试`，用户读到的是方括号里的黑话。
// 而"这是限流"这件事本来就写在 HTTP status 里：一个事实只留一个载体，消费方读 status。
// 这条判据的变异验证：把 `[rate_limit]` 前缀加回 client.ts，第一条立刻红。
const MARKER = '[rate_limit]';

function readSrc(rel: string): string {
  return stripComments(fs.readFileSync(path.resolve(rel), 'utf8'));
}

describe('限流的判别只走 status，不走消息字符串', () => {
  it('源码里不再出现内部判别串 ' + MARKER, () => {
    const hits = sourceFiles()
      .filter((rel) => readSrc(rel).includes(MARKER))
      .map((rel) => `${rel}: ${readSrc(rel).split('\n').find((l) => l.includes(MARKER))?.trim().slice(0, 90)}`);
    expect(hits).toEqual([]);
  }, 30000); // 扫全仓：并发满载时 5s 默认超时不够

  it('消费方按 429 判限流（而不是闻消息）', () => {
    const src = readSrc('src/composables/space/useSpaceInit.ts');
    expect(src).toMatch(/status === 429/);
    expect(src).not.toContain(MARKER);
  });

  it('429 的话术仍交回服务端（三档限流是不同的句子），前端不硬写死', () => {
    const src = readSrc('src/api/client.ts');
    const block = src.slice(src.indexOf('if (status === 429)'), src.indexOf('if (status === 429)') + 900);
    expect(block).toContain('serverMessage(');
  });
});
