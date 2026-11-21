import { createI18n } from 'vue-i18n';
import zhCN from './locales/zh-CN';
import enUS from './locales/en-US';
import { serverMessages } from './locales/server-messages';

const LOCALE_KEY = 'app-locale';

function detectLocale(): string {
  const stored = localStorage.getItem(LOCALE_KEY);
  if (stored) return stored;

  const browserLang = navigator.language;
  if (browserLang.startsWith('zh')) return 'zh-CN';
  return 'en-US';
}

const initialLocale = detectLocale();

const i18n = createI18n({
  legacy: false,
  // globalInjection 必须显式开：legacy:false 时模板里的 $t() 不再自动可用，
  // 而 check:gesture / check:geom 那一串工装跑的是真实挂载的组件树 ——
  // 没有它，任何把中文换成 $t('…') 的页面都会以"undefined is not a function"的形式崩，
  // 却能在只 import useI18n 的单测里全绿。
  globalInjection: true,
  locale: initialLocale,
  fallbackLocale: 'zh-CN',
  // server 命名空间是生成物（scripts/gen-server-messages.mjs），上游是后端 internal/response/messages.go。
  // 它是"后端说给人听的话"的译文来源，消费点在 api/client.ts —— 那里是全站唯一读这句话的地方。
  messages: {
    'zh-CN': { ...zhCN, server: serverMessages['zh-CN'].server },
    'en-US': { ...enUS, server: serverMessages['en-US'].server },
  },
});

// index.html 里的 lang 是静态值，实际语言由 detectLocale 决定
document.documentElement.lang = initialLocale;

export default i18n;

/**
 * Intl 用的 locale 标签。界面语言与日期/数字的语言必须跟着同一个开关 ——
 * 英文界面配"2026年10月11日星期日"是最容易被漏掉的一类破口：
 * 它是运行时格式化出来的，源码里根本没有中文字面量，所以 CJK 门禁数不到。
 */
export function appIntlLocale(): string {
  return String(i18n.global.locale.value);
}
