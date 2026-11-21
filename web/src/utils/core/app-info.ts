/**
 * 关于软件的事实只留一处：顶栏的帮助菜单、页脚、关于页都从这里取，
 * 避免同一个版本号 / 同一个外链在三个文件里各写一遍。
 */
export const APP_NAME = 'Citebase';
/** 一句话定位走语言包（chrome.tagline），这里不再留一份只会在中文里出现的常量 */
/** 由 vite define 注入，源头是 package.json 的 version —— 不要再在这里抄一个字面量 */
export const APP_VERSION = `v${__APP_VERSION__}`;
export const APP_REPO_URL = 'https://github.com/listen808/citebase';
export const APP_REPO_LABEL = 'github.com/listen808/citebase';
export const APP_ISSUES_URL = 'https://github.com/listen808/citebase/issues';
export const APP_LICENSE_URL = 'https://github.com/listen808/citebase/blob/main/LICENSE';
export const APP_RELEASES_URL = 'https://github.com/listen808/citebase/releases';
export const APP_AUTHOR_URL = 'https://github.com/listen808';
export const APP_AUTHOR_NAME = 'listen808';

export interface AppLink {
  /** 语言包里的键；渲染处用 $t 取，别在这里写死文案 */
  labelKey: string;
  href?: string;
  /** 站内路由；给了就不用 href */
  to?: string;
}

/** 帮助入口：顶栏下拉与页脚共用同一份，顺序即展示顺序 */
export const APP_HELP_LINKS: AppLink[] = [
  { labelKey: 'chrome.helpIssues', href: APP_ISSUES_URL },
  { labelKey: 'chrome.helpDocs', href: APP_REPO_URL },
  { labelKey: 'chrome.helpAbout', to: '/about' },
];
