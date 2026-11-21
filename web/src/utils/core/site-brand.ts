import { readonly, ref } from 'vue';
import { getSiteInfo } from '@/api/modules/system/site';
import i18n from '@/i18n';
import { logger } from '@/utils/logger';
import { APP_NAME } from './app-info';

/**
 * 站点展示名的前端唯一来源。
 *
 * 原先名字抄在六个地方：app-info.ts 的 APP_NAME、router/guards.ts 的 APP_TITLE、
 * ApBrand 的 wordmark 默认值、LoginView 的 title、SpaceMobileHeader 的默认值、
 * ShareView 与邮件预览里那句 "Powered by Citebase" —— 而后台"主页标题"那一栏
 * 从头到尾没有任何读取方，管理员改它改的是空气。
 *
 * 现在只有两层：APP_NAME 是代码默认（编译进包里，网络失败也看得见名字），
 * 这个 ref 是服务端 main_title 解析后的值（判据在 internal/brand，含空白退回默认）。
 * 界面上任何出现产品名的地方都取这里的 currentSiteName()，不再各写一份字面量。
 */
const siteName = ref(APP_NAME);
let pending: Promise<void> | null = null;

/** 当前站点名，供非组件环境（路由守卫、i18n 参数、标题拼接）使用 */
export function currentSiteName(): string {
  return siteName.value;
}

/** 组件里用这个：管理员改完名字，下一次渲染就跟过去 */
export const siteBrandName = readonly(siteName);

/**
 * 标签页标题的唯一拼法：有段落就「段落 - 站点名」，没有就只剩站点名。
 * 原先这个格式写在两处（router/guards.ts 与 DocumentEditView 的脏标记那一条），
 * 各自抄了一遍产品名 —— 两处对不上时，用户看到的是"标签页一会儿一个名字一会儿另一个"。
 */
export function siteDocumentTitle(section?: string): string {
  const name = currentSiteName();
  return section ? `${section} - ${name}` : name;
}

/**
 * 拉取站点名。整个应用生命周期只发一次（并发调用共享同一个请求）。
 * 失败就保持代码默认名 —— 登录页在装表之前、后端不可达时都得有名字可显示，
 * 所以这里刻意不弹错误、不重试。
 */
export function loadSiteBrand(): Promise<void> {
  if (!pending) pending = fetchSiteBrand();
  return pending;
}

async function fetchSiteBrand(): Promise<void> {
  try {
    const info = await getSiteInfo();
    const name = typeof info?.name === 'string' ? info.name.trim() : '';
    if (name) siteName.value = name;
    siteDescription.value = typeof info?.description === 'string' ? info.description : '';
  } catch (err) {
    logger.warn('读取站点展示名失败，沿用代码默认名', err);
  } finally {
    // 成功、失败、老后端不回 description 三种情况都要落一次：
    // 静态 HTML 里那份默认描述只有这一次（以及换语言时）被覆盖的机会。
    applySiteMetaDescription();
  }
}

/** 站点描述（管理员「系统设置 → 基础 → 主页描述」那一栏）。空串 = 没填。 */
const siteDescription = ref('');

/** 当前站点描述；没填就是空串，调用方据此退回自己的默认文案 */
export function currentSiteDescription(): string {
  return siteDescription.value;
}

/**
 * 把站点描述写进 `<meta name="description">` —— index.html 那份静态默认值的唯一覆盖点。
 *
 * 为什么必须有这一层：index.html 是构建产物，里面写死的中文描述不跟着界面语言走，
 * 英文用户把登录页链接发出去时，抓取器读到的是一句中文（静态 HTML 不可能翻译自己，
 * 只能在 JS 起来后按当前语言盖掉）。
 *
 * 两层来源与 internal/brand 同构：管理员填了「主页描述」就用它（那一栏此前又是一个
 * "能编辑、不生效"的键），没填退回语言包里那句一句话定位 —— `{app}` 插值，品牌名不进译文。
 */
export function applySiteMetaDescription(): void {
  const fromConfig = siteDescription.value.trim();
  const app = currentSiteName();
  const value = fromConfig || String(i18n.global.t('chrome.tagline', { app }));
  if (!value) return;
  let el = document.head.querySelector<HTMLMetaElement>('meta[name="description"]');
  if (!el) {
    el = document.createElement('meta');
    el.setAttribute('name', 'description');
    document.head.appendChild(el);
  }
  el.setAttribute('content', value);
}

/**
 * 哪几个配置键改的是站点品牌。与后端 internal/brand 的三个常量同名，两侧一致性由
 * tools/repocheck/brand_source_test.go 的 TestFrontendBrandConfigKeysMatchBackend 钉住 ——
 * 少列一个键的症状是"管理员改了，全站不动，必须刷新一次"，多列一个键会白白多发请求。
 */
export const BRAND_CONFIG_KEYS = ['main_title', 'system_name', 'main_description'] as const;

/** 保存完这个键之后需不需要 refreshSiteBrand() */
export function affectsSiteBrand(key: string): boolean {
  return (BRAND_CONFIG_KEYS as readonly string[]).includes(key);
}

/** 管理端保存完「系统设置 → 基础」后调用，让顶栏/标签页标题不用刷新就跟上新名字。 */
export function refreshSiteBrand(): Promise<void> {
  pending = null;
  return loadSiteBrand();
}
