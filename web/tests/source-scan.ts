import fs from 'node:fs';
import path from 'node:path';

/**
 * 去掉注释：块注释、行注释、以及 vue 模板里的 `<!-- -->`。
 *
 * 为什么单独成一个模块：i18n-closure（"收口过的文件不许再长出硬编码中文"）与
 * brand-closure（"产品名只准写一处"）判的是不同的字面量，但**都需要先排除注释** ——
 * 本仓的注释规矩是中文说理，历史里也会点名提到旧写法（"以前这里写死 'Citebase'"），
 * 不剥注释就会把说理当成违例。两台门禁各抄一份剥注释的正则，
 * 迟早一份改了另一份没改，于是一条判据静默失效。
 */
export function stripComments(src: string): string {
  return src
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|[^:])\/\/[^\n]*$/gm, '$1');
}

/** 递归列出 src 下的源码文件（相对 web/），排掉测试与类型声明 */
export function sourceFiles(dir = 'src'): string[] {
  const out: string[] = [];
  for (const entry of fs.readdirSync(path.resolve(dir), { withFileTypes: true })) {
    const rel = path.posix.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...sourceFiles(rel));
      continue;
    }
    if (!/\.(vue|ts)$/.test(entry.name) || /\.(test|spec)\.ts$/.test(entry.name) || entry.name.endsWith('.d.ts')) continue;
    out.push(rel);
  }
  return out;
}
