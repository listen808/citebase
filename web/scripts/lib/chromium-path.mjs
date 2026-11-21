// 本机已装的 chromium build 号可能与 playwright 期望的不一致，直接要下载的那份
// 往往不存在（`chromium.launch()` 默认走 headless-shell，缺文件就整个脚本起不来）。
// 这里挑 ms-playwright 下 build 号最大的可用可执行文件，避免为了跑一次巡检去联网下载
// ——本机走 ZTS 内网，外连下载本身是受限动作。
//
// 两个工装（visual-snapshot、check-ui-consistency）共用这一份实现：
// 各写一套的话，哪天只修了一边，另一边就会在同一个坑上反复失败。
//
// 找不到 ms-playwright 时会退到系统装的 Edge/Chrome。此前这里直接返回 undefined，
// 而 14 个脚本一律硬编码 `chromium.launch({ executablePath: findChromium() })` ——
// undefined 会让 Playwright 回落到自己那份并不存在的 bundled 路径，于是整套运行时
// 工装（check:ui/geom/boxes/elements/parity/stress/motion/gesture/visual/dialog/detail/apple）
// 在一台没下载过 playwright 浏览器的机器上全部起不来，而静态门禁一个都不报。
// 退到系统浏览器意味着**量到的像素可能与文档里用 Chromium 记的基线差一点**，
// 所以退路一律往 stderr 打一行公告，让报表里看得见"这轮是用谁量的"。
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const PLAYWRIGHT_DIR = 'AppData/Local/ms-playwright';

/** 系统浏览器的常见落点；顺序即优先级（先 Edge 后 Chrome，与 Windows 上的装机率一致） */
const SYSTEM_BROWSER_CANDIDATES = {
  win32: [
    'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
    'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
    'C:/Program Files/Google/Chrome/Application/chrome.exe',
    'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
  ],
  darwin: [
    '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Chromium.app/Contents/MacOS/Chromium',
  ],
  linux: [
    '/usr/bin/microsoft-edge',
    '/usr/bin/google-chrome',
    '/usr/bin/google-chrome-stable',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
  ],
};

/**
 * 纯函数：把「有哪些文件 / 目录读得到」当参数传进来，判据本身才测得到。
 * @param {{ home: string, platform: string, exists: (p: string) => boolean, readdir: (p: string) => string[], override?: string, overrideName?: string, announce?: (msg: string) => void }} opts
 */
export function resolveChromium({ home, platform, exists, readdir, override, overrideName, announce }) {
  if (override) {
    if (exists(override)) return override;
    // 显式指定却指到一个不存在的文件：不能默默退回自动探测，否则人会以为
    // 自己指定的那份真的被用了。这里也不抛 —— 抛了会让 14 个工装一起起不来。
    announce?.(`${overrideName ?? 'CHROMIUM 路径'} 指向的文件不存在：${override}（改回自动探测）`);
  }

  const pwRoot = path.join(home, PLAYWRIGHT_DIR);
  if (exists(pwRoot)) {
    const cands = [];
    for (const dir of readdir(pwRoot)) {
      const m = /^chromium(_headless_shell)?-(\d+)$/.exec(dir);
      if (!m) continue;
      const exe = m[1]
        ? path.join(pwRoot, dir, 'chrome-headless-shell-win64', 'chrome-headless-shell.exe')
        : path.join(pwRoot, dir, 'chrome-win64', 'chrome.exe');
      if (exists(exe)) cands.push([Number(m[2]), exe]);
    }
    cands.sort((a, b) => b[0] - a[0]);
    if (cands.length) return cands[0][1];
  }

  const system = (SYSTEM_BROWSER_CANDIDATES[platform] || SYSTEM_BROWSER_CANDIDATES.linux).find((p) =>
    exists(p),
  );
  if (system) {
    announce?.(
      `运行时工装退到系统浏览器：${system}\n` +
        `  —— 文档里的基线是用 Playwright 自带的 Chromium 量的，换浏览器可能有个位像素差；` +
        `要逐字复现基线请跑 npx playwright install chromium`,
    );
    return system;
  }

  return undefined;
}

export function findChromium() {
  // 新名优先、旧名兜底（项目 2026-10-07 从 AIWiki 改名而来，两套名字都可能在别人的 shell 里）。
  const legacy = process.env.AIWIKI_CHROMIUM_PATH;
  const override = process.env.CITEBASE_CHROMIUM_PATH || legacy;
  return resolveChromium({
    home: process.env.USERPROFILE || process.env.HOME || os.homedir(),
    platform: process.platform,
    exists: (p) => fs.existsSync(p),
    readdir: (p) => fs.readdirSync(p),
    override,
    overrideName: process.env.CITEBASE_CHROMIUM_PATH ? 'CITEBASE_CHROMIUM_PATH' : 'AIWIKI_CHROMIUM_PATH',
    announce: (msg) => process.stderr.write(`ℹ ${msg}\n`),
  });
}
