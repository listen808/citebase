import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { applySiteMetaDescription } from '@/utils/core/site-brand';

const LOCALE_KEY = 'app-locale';

export function useLocale() {
  const { locale, t } = useI18n({ useScope: 'global' });

  const currentLocale = computed(() => locale.value);

  function setLocale(lang: string) {
    locale.value = lang;
    localStorage.setItem(LOCALE_KEY, lang);
    // 读屏软件按 html lang 选择语音引擎，切语言必须同步文档语言
    document.documentElement.lang = lang;
    // meta description 的兜底文案来自语言包（chrome.tagline），换语言要跟着换 ——
    // 只换界面不换它，等于英文界面对外仍报着一句中文
    applySiteMetaDescription();
  }

  return {
    locale: currentLocale,
    setLocale,
    t,
  };
}
