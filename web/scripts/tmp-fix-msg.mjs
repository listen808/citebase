// 一次性修语言包里的消息格式违例（@ 与 { 未转义），修完即可删。
import fs from 'node:fs';

const LEAF = /^( {4}([A-Za-z0-9_]+): )(['`])((?:\\.|(?!\3).)*)\3,$/;
const esc = (m, q) => (q === "'" ? m.replace(/\\/g, '\\\\').replace(/'/g, "\\'") : m.replace(/'/g, "\\'"));

for (const p of ['src/i18n/locales/zh-CN.ts', 'src/i18n/locales/en-US.ts']) {
  const lines = fs.readFileSync(p, 'utf8').split('\n');
  let n = 0;
  const out = lines.map((line) => {
    const m = line.match(LEAF);
    if (!m) return line;
    const [, prefix, , quote, raw] = m;
    const value = raw.replace(/\\([\s\S])/g, (_, c) => c); // 成品形态
    let fixed = value.replace(/@/g, "{'@'}");
    if (/\{"keep":4\}/.test(fixed)) {
      fixed = fixed.replace(/\{"keep":4\}/g, '{\'{\'}"keep":4{\'}\'}');
    }
    if (fixed === value) return line;
    n++;
    return `${prefix}${quote}${esc(fixed, quote)}${quote},`;
  });
  fs.writeFileSync(p, out.join('\n'), 'utf8');
  console.log(`${p} 修了 ${n} 行`);
}
