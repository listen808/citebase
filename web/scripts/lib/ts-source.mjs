/**
 * 在 Node 里取 TS 源码导出的判据值 —— 工装与 src 之间唯一的取值通道。
 *
 * 为什么要它：这些数字（浮层 alpha 的地板与天花板、各主题的表面色、可读前景映射）
 * 在 `src/` 里有一份带单测的实现。工装若手抄一份，改一次源码就静默分叉；
 * 而 esbuild 现编译一遍的成本远低于一次错误口径的代价，所以判据一律从源码取。
 *
 * 用法：`loadTsExports([{ from: './src/utils/core/glass-contrast', names: ['A', 'B'] }])`
 * 返回 `{ A, B }`。不写第二份常量，也不在工装里重做主题解析。
 */
import path from 'path';
import NodeModule from 'node:module';
import { buildSync } from 'esbuild';

export function loadTsExports(specs, root = path.resolve('.')) {
  const stdin = specs
    .map((s) => `export { ${s.names.join(', ')} } from '${s.from}';`)
    .join('\n');
  const bundle = buildSync({
    stdin: { contents: stdin, resolveDir: root, loader: 'ts' },
    bundle: true,
    format: 'cjs',
    platform: 'node',
    write: false,
    absWorkingDir: root,
  });
  const req = NodeModule.createRequire(path.join(root, 'noop.js'));
  const mod = { exports: {} };
  new Function('module', 'exports', 'require', bundle.outputFiles[0].text)(mod, mod.exports, req);
  const out = {};
  for (const s of specs) for (const n of s.names) out[n] = mod.exports[n];
  return out;
}
