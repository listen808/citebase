import { describe, expect, it, vi } from 'vitest';
import path from 'node:path';
// @ts-expect-error —— .mjs 工装模块不在任何 tsconfig 的 include 里，只做运行时解析
import { resolveChromium } from '../scripts/lib/chromium-path.mjs';

/**
 * 钉住"运行时工装到底用哪个浏览器起来"这条判据。
 *
 * 加这条的原因很直接：原先这里找不到 ms-playwright 就返回 undefined，而 14 个脚本一律
 * 硬编码 `chromium.launch({ executablePath: findChromium() })`，undefined 会让 Playwright
 * 去用它自己那份并不存在的 bundled 路径 —— 整套运行时巡检在一台没下载过浏览器的机器上
 * 全线起不来，而 `check:style` 那五道静态门禁一条都不会红。判据是纯函数，所以能脱离真机器测。
 */

const norm = (p: string) => p.split(path.sep).join('/');
const PW_ROOT = 'C:/home/AppData/Local/ms-playwright';

/**
 * 假文件系统：给了文件路径就顺带把它的各级父目录也登记成"存在"，
 * 否则 exists(ms-playwright 目录) 会是 false，整条 Playwright 分支根本进不去 ——
 * 那就是测了个不存在的判据。`extraDirs` 用来造"目录在、里面没可用文件"这种形态。
 */
function mkFs(files: string[], dirs: Record<string, string[]> = {}, extraDirs: string[] = []) {
  const set = new Set(files.map(norm));
  for (const f of set) {
    const parts = f.split('/');
    for (let i = 1; i < parts.length; i++) set.add(parts.slice(0, i).join('/'));
  }
  for (const d of extraDirs) set.add(norm(d));
  return {
    exists: (p: string) => set.has(norm(p)),
    readdir: (p: string) => dirs[norm(p)] ?? [],
  };
}

function base(input: { files?: string[]; dirs?: Record<string, string[]>; extraDirs?: string[]; options?: Record<string, unknown> }) {
  const fs = mkFs(input.files ?? [], input.dirs ?? {}, input.extraDirs ?? []);
  const announce = vi.fn();
  const result = resolveChromium({
    home: 'C:/home',
    platform: 'win32',
    exists: fs.exists,
    readdir: fs.readdir,
    announce,
    ...(input.options ?? {}),
  });
  return { result, announce };
}

describe('resolveChromium', () => {
  it('ms-playwright 有货时优先用它，并取 build 号最大的那份', () => {
    const { result, announce } = base({
      files: [
        `${PW_ROOT}/chromium-1100/chrome-win64/chrome.exe`,
        `${PW_ROOT}/chromium-1228/chrome-win64/chrome.exe`,
      ],
      dirs: { [norm(PW_ROOT)]: ['chromium-1100', 'chromium-1228', 'not-a-browser'] },
    });
    expect(result).toContain('chromium-1228');
    // 用 Playwright 自带的浏览器时不该有任何公告——那是文档基线的口径
    expect(announce).not.toHaveBeenCalled();
  });

  it('认得 headless_shell 那种目录名（launch 默认走它）', () => {
    const { result } = base({
      files: [`${PW_ROOT}/chromium_headless_shell-1228/chrome-headless-shell-win64/chrome-headless-shell.exe`],
      dirs: { [norm(PW_ROOT)]: ['chromium_headless_shell-1228'] },
    });
    expect(result).toContain('chrome-headless-shell.exe');
  });

  it('目录在但里面没有可用可执行文件时，也要退到系统浏览器', () => {
    const { result, announce } = base({
      files: ['C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'],
      extraDirs: [PW_ROOT],
    });
    expect(result).toContain('msedge.exe');
    expect(announce).toHaveBeenCalledOnce();
  });

  it('退到系统浏览器必须公告 —— 报表里得看得见"这轮是用谁量的"', () => {
    const { result, announce } = base({
      files: ['C:/Program Files/Google/Chrome/Application/chrome.exe'],
    });
    expect(result).toContain('chrome.exe');
    const msg = String(announce.mock.calls[0]?.[0] ?? '');
    expect(msg).toContain('系统浏览器');
    expect(msg).toContain('基线');
  });

  it('override 指到真实文件时优先级最高，且不公告（变量名 CITEBASE_CHROMIUM_PATH，旧名 AIWIKI_* 兜底）', () => {
    const { result, announce } = base({
      files: ['/opt/pinned/chrome', 'C:/home/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe'],
      dirs: { [norm(PW_ROOT)]: ['chromium-1228'] },
      options: { override: '/opt/pinned/chrome' },
    });
    expect(result).toBe('/opt/pinned/chrome');
    expect(announce).not.toHaveBeenCalled();
  });

  it('指定的路径不存在时必须说出来，不能悄悄退回自动探测', () => {
    const { result, announce } = base({
      files: ['C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'],
      options: { override: '/opt/typo/chrome' },
    });
    expect(result).toContain('msedge.exe');
    // 两条都要说：第一条是"你指定的那份没用上"，第二条是"最后用了谁"
    expect(announce).toHaveBeenCalledTimes(2);
    expect(String(announce.mock.calls[0]?.[0])).toContain('不存在');
    expect(String(announce.mock.calls[1]?.[0])).toContain('系统浏览器');
  });

  it('什么浏览器都没有时返回 undefined，让调用方去抛它自己的错', () => {
    const { result, announce } = base({ files: [] });
    expect(result).toBeUndefined();
    expect(announce).not.toHaveBeenCalled();
  });

  it('非 win32 平台不去找 Windows 路径', () => {
    const fs = mkFs(['C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe']);
    expect(
      resolveChromium({
        home: '/home/u',
        platform: 'linux',
        exists: fs.exists,
        readdir: fs.readdir,
      }),
    ).toBeUndefined();
  });
});
