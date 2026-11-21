import { config } from '@vue/test-utils';
import i18n from '@/i18n';

// 单测挂的是真实组件树，而组件模板现在用 $t() 取文案。两件事必须一起成立：
// ① 装上插件，否则 $t 是 undefined，断言以看不懂的形失败；
// ② 钉住 zh-CN —— detectLocale 在 happy-dom 里拿到的是 en-US，
//    那样"aria-label 应为『关闭预览』"这类断言会整片红，而产品并没有坏。
i18n.global.locale.value = 'zh-CN';
config.global.plugins = [...(config.global.plugins ?? []), i18n];
