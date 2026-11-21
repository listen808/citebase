import { beforeEach, describe, expect, it, vi } from 'vitest';

// 品牌源的替身：这里要验的是"名字从哪来、读不到时怎么办"，不是请求怎么发。
const api = vi.hoisted(() => ({ getSiteInfo: vi.fn() }));

vi.mock('@/api/modules/system/site', () => api);

import { APP_NAME } from './app-info';

// site-brand 把名字放在模块级的 ref 里（就是要跨组件共享），所以每条用例都必须
// resetModules + 动态 import 拿到一份干净的实例。
// 上一版直接静态 import，用例之间共享同一个 ref：把 "读取失败不改名" 排到
// "拉到的值覆盖默认名" 后面就会因为残留值而随机红 —— 时序型门禁的随机红最坑，
// 它本地像好的、CI 上偶发，最后没人信这条判据。
type SiteBrand = typeof import('./site-brand');

async function freshBrand(): Promise<SiteBrand> {
  vi.resetModules();
  api.getSiteInfo.mockReset();
  return import('./site-brand');
}

describe('站点展示名', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('没拉到之前显示代码默认名', async () => {
    // 表现：登录页与标签页标题在首帧就该有名字，不能是空白或 undefined。
    const { currentSiteName, siteDocumentTitle } = await freshBrand();
    expect(currentSiteName()).toBe(APP_NAME);
    expect(siteDocumentTitle()).toBe(APP_NAME);
  });

  it('拉到的值覆盖默认名', async () => {
    const { currentSiteName, loadSiteBrand, siteBrandName, siteDocumentTitle } = await freshBrand();
    api.getSiteInfo.mockResolvedValue({ name: '团队百科' });

    await loadSiteBrand();

    expect(currentSiteName()).toBe('团队百科');
    expect(siteBrandName.value).toBe('团队百科');
    expect(siteDocumentTitle('登录')).toBe('登录 - 团队百科');
  });

  it('服务端回空值或空白时退回默认名，而不是把界面弄出一个空洞', async () => {
    for (const blank of ['', '   ', '\t']) {
      const { currentSiteName, refreshSiteBrand } = await freshBrand();
      api.getSiteInfo.mockResolvedValue({ name: blank });
      await refreshSiteBrand();
      expect(currentSiteName()).toBe(APP_NAME);
    }
  });

  it('读取失败不抛错、不改名 —— 一个名字不值得为它弹错误', async () => {
    const { currentSiteName, refreshSiteBrand } = await freshBrand();
    api.getSiteInfo.mockRejectedValue(new Error('ECONNREFUSED'));

    await expect(refreshSiteBrand()).resolves.toBeUndefined();
    expect(currentSiteName()).toBe(APP_NAME);
  });

  it('一次生命周期只发一个请求，refresh 才重发', async () => {
    const { loadSiteBrand, refreshSiteBrand } = await freshBrand();
    api.getSiteInfo.mockResolvedValue({ name: '科研笔记库' });

    // 并发 load（首屏与别处同时调用）不该变成两次 /site。
    await Promise.all([loadSiteBrand(), loadSiteBrand()]);
    expect(api.getSiteInfo).toHaveBeenCalledTimes(1);

    await refreshSiteBrand();
    expect(api.getSiteInfo).toHaveBeenCalledTimes(2);
  });

  it('管理员改过名字后重新拉取，标题跟着换', async () => {
    const { currentSiteName, loadSiteBrand, refreshSiteBrand, siteDocumentTitle } = await freshBrand();
    api.getSiteInfo.mockResolvedValue({ name: '旧名' });
    await loadSiteBrand();
    expect(siteDocumentTitle('设置')).toBe('设置 - 旧名');

    api.getSiteInfo.mockResolvedValue({ name: '新名' });
    await refreshSiteBrand();

    expect(currentSiteName()).toBe('新名');
    expect(siteDocumentTitle()).toBe('新名');
  });
});

// 描述这一路存在的理由：index.html 里那句 <meta name="description"> 是构建产物，
// 写死中文且不跟界面语言走 —— 英文用户把登录页发出去，抓取器读到的是一句中文。
// 静态 HTML 翻译不了自己，只能在 JS 起来后按"管理员的主页描述 → 语言包"两层盖掉。
describe('站点描述写进 <meta name="description">', () => {
  const DESC = 'meta[name="description"]';

  beforeEach(() => {
    // 上一条用例可能留着一个 content 不同的元素：不清就分不清"这次写的"和"上次残留的"
    document.head.querySelector(DESC)?.remove();
  });

  it('管理员填了「主页描述」就用它（裁掉首尾空白）', async () => {
    const { loadSiteBrand } = await freshBrand();
    api.getSiteInfo.mockResolvedValue({ name: '团队百科', description: '  我们自己的定位  ' });

    await loadSiteBrand();

    expect(document.head.querySelector(DESC)?.getAttribute('content')).toBe('我们自己的定位');
  });

  it('没填就退回语言包那句一句话定位，并带上当前站点名', async () => {
    const brand = await freshBrand();
    const { default: i18n } = await import('@/i18n');
    api.getSiteInfo.mockResolvedValue({ name: '团队百科', description: '' });

    await brand.loadSiteBrand();

    expect(document.head.querySelector(DESC)?.getAttribute('content')).toBe(
      String(i18n.global.t('chrome.tagline', { app: '团队百科' })),
    );
  });

  it('读不到服务端时也得有一句 —— 不能只剩静态默认值那句中文', async () => {
    const brand = await freshBrand();
    const { default: i18n } = await import('@/i18n');
    api.getSiteInfo.mockRejectedValue(new Error('ECONNREFUSED'));

    await brand.refreshSiteBrand();

    expect(document.head.querySelector(DESC)?.getAttribute('content')).toBe(
      String(i18n.global.t('chrome.tagline', { app: APP_NAME })),
    );
  });

  it('换语言重写这一句：只换界面不换它，等于对外仍报着旧语言', async () => {
    const brand = await freshBrand();
    const { default: i18n } = await import('@/i18n');
    api.getSiteInfo.mockResolvedValue({ name: APP_NAME, description: '' });
    await brand.loadSiteBrand();

    // 期望值必须在各自的语言下取好再往下走：断言都写在最后的话，
    // 第一条 expect 求值时 locale 已经切回 zh-CN，拿中文去比英文那一格（时序型假红）。
    i18n.global.locale.value = 'en-US';
    brand.applySiteMetaDescription();
    const en = document.head.querySelector(DESC)?.getAttribute('content');
    const enWant = String(i18n.global.t('chrome.tagline', { app: APP_NAME }));

    i18n.global.locale.value = 'zh-CN';
    brand.applySiteMetaDescription();
    const zh = document.head.querySelector(DESC)?.getAttribute('content');
    const zhWant = String(i18n.global.t('chrome.tagline', { app: APP_NAME }));

    expect(en).toBe(enWant);
    expect(zh).toBe(zhWant);
    expect(zh).not.toBe(en);
  });
});
