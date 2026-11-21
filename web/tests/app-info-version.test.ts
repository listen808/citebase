import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import { APP_VERSION } from '@/utils/core/app-info';

/**
 * 这条测试存在的理由：版本号原先是 app-info.ts 里手抄的 'v2.0.0'。
 * 改成从 package.json 注入之后，"注入没接上"在编译期是看不出来的 ——
 * __APP_VERSION__ 未定义时模板字符串会安静地产出 'vundefined'，
 * 类型仍然是 string，vue-tsc 一个错都不报，只有把值打出来才知道。
 *
 * 放在 tests/ 而不是 src/ 旁边：src/** 走浏览器 tsconfig（没有 node 类型），
 * 这里要读 package.json 所以必须落在 tsconfig.node.json 的射程内。
 */
describe('APP_VERSION 的来源', () => {
  const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8')) as { version: string };

  it('等于 package.json 的 version 加前导 v', () => {
    expect(APP_VERSION).toBe(`v${pkg.version}`);
  });

  it('不是注入失败留下的空值', () => {
    expect(APP_VERSION).not.toBe('vundefined');
    expect(APP_VERSION).toMatch(/^v\d+\.\d+\.\d+$/);
  });
});
