import { createI18n } from 'vue-i18n';

const msgs = {
  'zh-CN': {
    p: {
      at: "本文档的 {'@'}提及名单",
      json: "regex 方式填正则，partial 方式可填 JSON，如 {'{'}\"keep\":4{'}'}",
      mixed: "请 {a} 复核，{'@'}lisi 补充；{'@'}zhangsan",
    },
  },
};
const i18n = createI18n({ legacy: false, locale: 'zh-CN', messages: msgs });
for (const k of ['at', 'json', 'mixed']) {
  console.log(k, '=>', JSON.stringify(i18n.global.t(`p.${k}`, { a: '张三' })));
}
