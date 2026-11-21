/**
 * JS/CSS-in-JS 源码里的**样式对象**枚举。
 *
 * 为什么单独成文件：多条判据都需要"同一个对象里同时出现 A 和 B"这种结构判断
 * （padding 简写覆盖长写、手搓浮层表面签名、单轴滚动容器），而**按行数取文本窗口**
 * 会被注释骗过去 —— 这条教训在 R11 上实测过：判据取上下 15 行，注释里出现
 * `gridAutoRows` 五个字就算通过。花括号配对是唯一能说出"这两个键属于同一份样式"的办法。
 *
 * 不处理字符串里的花括号（模板字面量 `${...}` 会带进来）：调用方需要先掩掉插值，
 * 现有判据里 R11 就是这么做的。这里保持只做一件事。
 */
export function eachJsObject(text, cb, maxLen = 4000) {
  for (let i = 0; i < text.length; i++) {
    if (text[i] !== '{') continue;
    let depth = 0;
    const start = i;
    for (; i < text.length; i++) {
      if (text[i] === '{') depth++;
      else if (text[i] === '}' && --depth === 0) break;
    }
    const body = text.slice(start, i + 1);
    if (body.length > maxLen) continue;
    cb(body, start);
  }
}

/** 抹掉注释但**保住行号**：把注释里的非换行字符换成空格，行号偏移才不会失真。 */
export function stripCommentsKeepLines(text) {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (_m, p) => p + ' '.repeat(_m.length - p.length));
}

/** 由源码偏移量算出 1 基行号（判据报错要指到行）。 */
export const lineAt = (text, index) => text.slice(0, index).split('\n').length;
