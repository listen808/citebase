// 一次性量尺：把 100 个视图里的中文串按"能不能机械抽取"分类，只为定批次规模，不落地。
// 跑法：cd web && node scripts/i18n-inventory.mjs
import fs from 'node:fs';
import path from 'node:path';

const viewsDir = 'src/views';
const compDir = 'src/components';

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (e.name.endsWith('.vue')) out.push(p);
  }
  return out;
}

// 码位写法。这条曾经用字形范围（相邻区段之间只靠一个裸连字符分隔），落盘时掉了一个边界字符，
// 剩下的裸连字符被 JS 解析成 `U+0020–U+303F` —— 于是第 33 行那个"这文件有没有中文"的
// 预筛**对任何含空格的源码都返回 true**，files 就变成"扫过的文件数"而不是"带中文的文件数"
// （曾报 207，实际含中文的要少）。逐条抽取用的是内联码位区间，是对的，所以待译串本身没被污染。
// 由 tests/cjk-regex.test.ts 盯着这类哑判据。
const CJK = /[\u3000-\u303f\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff00-\uffef]/;

function templateOf(src) {
  const s = src.indexOf('<template>');
  if (s < 0) return ['', -1, -1];
  return [src.slice(s, src.lastIndexOf('</template>') + 11), s, src.lastIndexOf('</template>')];
}

const stats = { files: 0, tmplText: 0, tmplAttr: 0, scriptStr: 0, other: 0, uniq: new Set() };
const perFile = [];

for (const root of [viewsDir, compDir]) {
  if (!fs.existsSync(root)) continue;
  for (const file of walk(root)) {
    const src = fs.readFileSync(file, 'utf8');
    if (!CJK.test(src)) continue;
    stats.files++;
    const [tmpl, , tEnd] = templateOf(src);
    let n = 0;

    // 模板里的可见文本节点：>中文< 之间
    for (const m of tmpl.matchAll(/>([^<>]*[一-鿿][^<>]*)</g)) {
      const t = m[1].replace(/\s+/g, ' ').trim();
      if (t) { stats.tmplText++; stats.uniq.add(t); n++; }
    }
    // 模板里带中文的属性值（placeholder / title / label / ok-text …）
    for (const m of tmpl.matchAll(/[\w-]+\s*=\s*"([^"]*[一-鿿][^"]*)"/g)) {
      stats.tmplAttr++; stats.uniq.add(m[1]); n++;
    }
    // 模板里直接写在插值外的裸中文（如 {{ }} 内、或 :ok-text="'删除'"）—— 计数进 other
    for (const _m of tmpl.matchAll(/{{[^}]*[一-鿿][^}]*}}/g)) { stats.other++; n++; }
    // 脚本段里的字符串字面量
    const script = src.slice(tEnd < 0 ? 0 : tEnd);
    for (const m of script.matchAll(/(['"`])(?:\\.|(?!\1)[^\\])*?[一-鿿](?:\\.|(?!\1)[^\\])*?\1/g)) {
      stats.scriptStr++; stats.uniq.add(m[0].slice(1, -1)); n++;
    }
    perFile.push([file, n]);
  }
}

perFile.sort((a, b) => b[1] - a[1]);
console.log(JSON.stringify({
  files: stats.files,
  tmplText: stats.tmplText,
  tmplAttr: stats.tmplAttr,
  scriptStrings: stats.scriptStr,
  interpolation: stats.other,
  distinctChineseStrings: stats.uniq.size,
  top10: perFile.slice(0, 10),
}, null, 2));
