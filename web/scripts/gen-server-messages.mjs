// 从 internal/response/messages.go 生成前端语言包里的 server 命名空间。
//
// 为什么是生成而不是再手抄一份：那张表是"后端说给人听的话"的唯一目录
// （键 + 中文，中文还要原样放进响应 message 字段给 curl、CLI 和旧部署看），
// 英文由前端拥有（web/scripts/server-messages.en.json）。
// 手抄的失败方式很安静：后端加了一句、前端查不到译文，界面就退回中文 ——
// 那不会报错，也不会有人提 issue，所以键集必须由脚本与门禁双向核对。
//
// 用法：cd web && npm run gen:server-messages
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, '..');
const repoRoot = path.resolve(webRoot, '..');

const registryFile = path.join(repoRoot, 'internal', 'response', 'messages.go');
const enFile = path.join(webRoot, 'scripts', 'server-messages.en.json');
const outFile = path.join(webRoot, 'src', 'i18n', 'locales', 'server-messages.ts');

/** 读后端那张表：常量名 → 键，常量名 → 中文 */
export function readCatalog(src) {
  const keyOf = new Map();
  for (const m of src.matchAll(/^\t(Msg\w+)\s*=\s*"([^"]+)"/gm)) keyOf.set(m[1], m[2]);

  const textOf = new Map();
  for (const m of src.matchAll(/^\t(Msg\w+):\s+(.+?),\s*$/gm)) {
    const raw = m[2];
    // 登记值两种写法：`…` 原始串（内含反斜杠/引号）与 "…" 转义串。
    // 转义串交给 JSON.parse —— Go 的双引号字面量转义集是它的子集，够用了。
    textOf.set(m[1], raw.startsWith('`') ? raw.slice(1, -1) : JSON.parse(raw));
  }

  const noText = [...keyOf.keys()].filter((n) => !textOf.has(n));
  const noConst = [...textOf.keys()].filter((n) => !keyOf.has(n));
  if (noText.length) throw new Error(`messages.go：${noText.length} 个常量没有登记文案：${noText.join(', ')}`);
  if (noConst.length) throw new Error(`messages.go：${noConst.length} 条文案没有对应常量：${noConst.join(', ')}`);

  const zh = {};
  for (const [name, key] of keyOf) zh[key] = textOf.get(name);
  return zh;
}

/** 键集双向核对 + 两条"安静失败"的拦截 */
export function buildCatalog(zh, en) {
  const slugs = Object.keys(zh).sort();
  const missingEn = slugs.filter((k) => !(k in en));
  if (missingEn.length) throw new Error(`server-messages.en.json 缺 ${missingEn.length} 个键：${missingEn.join(', ')}`);
  const staleEn = Object.keys(en).sort().filter((k) => !(k in zh));
  if (staleEn.length) throw new Error(`server-messages.en.json 里有后端已不认的键：${staleEn.join(', ')}`);

  const blank = slugs.filter((k) => !String(zh[k]).trim() || !String(en[k]).trim());
  if (blank.length) throw new Error(`空文案：${blank.join(', ')}`);

  // 响应封套里没有参数位 —— 带插值的键会把 "%s"、"{n}" 原样印到界面上。
  // 要带参数就另开一条键，别在这里放占位符。
  const interpolated = slugs.filter((k) => /%[sdv]\b|\{\w+\}/.test(String(zh[k])) || /%[sdv]\b|\{\w+\}/.test(String(en[k])));
  if (interpolated.length) throw new Error(`文案含插值（封套没有参数位）：${interpolated.join(', ')}`);

  return { slugs, zh, en };
}

const table = (name, slugs, map) =>
  `const ${name}: Record<string, string> = {\n${slugs.map((k) => `  ${JSON.stringify(k)}: ${JSON.stringify(map[k])},`).join('\n')}\n};`;

export function render({ slugs, zh, en }) {
  return `// 生成物，请勿手工编辑。
//   中文上游：internal/response/messages.go（后端把同一句放进响应的 message 字段）
//   英文上游：web/scripts/server-messages.en.json
//   重生成：cd web && npm run gen:server-messages
//   校验：web/tests/server-messages.test.ts（双向核对键集，并比对这份产物是否过期）
//
// 这里的键是稳定契约：只增不改不删。改了旧前端就查不到译文、退回服务端那句中文 ——
// 表现不是报错，是"英文界面里零星几句中文"，所以必须有门禁。
// 唯一消费点是 web/src/api/client.ts：它在构造 ApiError 时按 message_key 取这里的文案。

${table('serverZh', slugs, zh)}

${table('serverEn', slugs, en)}

export const serverMessages = {
  'zh-CN': { server: serverZh },
  'en-US': { server: serverEn },
};

export default serverMessages;
`;
}

export function generate() {
  const zh = readCatalog(fs.readFileSync(registryFile, 'utf8'));
  const en = JSON.parse(fs.readFileSync(enFile, 'utf8'));
  return { content: render(buildCatalog(zh, en)), count: Object.keys(zh).length };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const { content, count } = generate();
  fs.writeFileSync(outFile, content, { encoding: 'utf8' });
  console.log(`生成 src/i18n/locales/server-messages.ts（${count} 个键）`);
}
