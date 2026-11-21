/**
 * 语法高亮颜色 Token - 使用 CSS 变量支持主题切换
 * 遵循 Apple Design 规范，禁止硬编码颜色
 *
 * 调色板按「主题 → 调色板」的穷举映射注入（Record<ThemeId, ...> 由类型系统保证不漏主题），
 * 新增一套主题时若忘了配调色板，type-check 会直接失败。
 */
import { THEME_REGISTRY, isDarkThemeId, supportsGlass, type ThemeId } from './themes';

export const HighlightColors = {
  // 浅色主题
  light: {
    keyword: '#d73a49',
    string: '#032f62',
    comment: '#6a737d',
    number: '#005cc5',
    title: '#6f42c1',
    built_in: '#e36209',
    meta: '#032f62',
    name: '#22863a',
    attr: '#6f42c1',
    variable: '#e36209',
    function: '#6f42c1',
    params: '#24292e',
    literal: '#005cc5',
    symbol: '#032f62',
    selectorTag: '#22863a',
    selectorClass: '#6f42c1',
    selectorId: '#005cc5',
    subst: '#24292e',
    punctuation: '#24292e',
    operator: '#24292e',
    section: '#005cc5',
    tag: '#22863a',
    attribute: '#6f42c1',
    regexp: '#032f62',
    link: '#032f62',
    bullet: '#005cc5',
    code: '#24292e',
    formula: '#24292e',
    doctag: '#6a737d',
    templateTag: '#22863a',
    templateVariable: '#e36209',
    addition: '#22863a',
    deletion: '#b31d28',
    quote: '#6a737d',
    type: '#d73a49',
    class: '#6f42c1',
  },
  // 暗色主题
  dark: {
    keyword: '#ff7b72',
    string: '#a5d6ff',
    comment: '#8b949e',
    number: '#79c0ff',
    title: '#d2a8ff',
    built_in: '#ffa657',
    meta: '#a5d6ff',
    name: '#7ee787',
    attr: '#d2a8ff',
    variable: '#ffa657',
    function: '#d2a8ff',
    params: '#c9d1d9',
    literal: '#79c0ff',
    symbol: '#a5d6ff',
    selectorTag: '#7ee787',
    selectorClass: '#d2a8ff',
    selectorId: '#79c0ff',
    subst: '#c9d1d9',
    punctuation: '#c9d1d9',
    operator: '#c9d1d9',
    section: '#79c0ff',
    tag: '#7ee787',
    attribute: '#d2a8ff',
    regexp: '#a5d6ff',
    link: '#a5d6ff',
    bullet: '#79c0ff',
    code: '#c9d1d9',
    formula: '#c9d1d9',
    doctag: '#8b949e',
    templateTag: '#7ee787',
    templateVariable: '#ffa657',
    addition: '#7ee787',
    deletion: '#ff7b72',
    quote: '#8b949e',
    type: '#ff7b72',
    class: '#d2a8ff',
  },
} as const;

/** 调色板名 */
export type HighlightPalette = keyof typeof HighlightColors;

/** base → 高亮调色板。玻璃是材质轴，不改代码文字色，故只按 base 分派（glass 跟随其外观）。 */
export const HIGHLIGHT_PALETTE_BY_THEME: Record<ThemeId, HighlightPalette> = {
  light: 'light',
  dark: 'dark',
  editorial: 'light',
  'warm-paper': 'light',
};

/**
 * 注入语法高亮 CSS 变量到 :root
 * 同一调色板的主题合并成一条选择器；与 :root 默认值相同的调色板不重复输出。
 */
export function injectHighlightCSSVariables(): void {
  const styleId = 'highlight-colors';
  if (document.getElementById(styleId)) return;

  const renderVars = (palette: HighlightPalette) =>
    Object.entries(HighlightColors[palette])
      .map(([key, value]) => `  --hl-${key}: ${value};`)
      .join('\n');

  const defaultPalette = HIGHLIGHT_PALETTE_BY_THEME.light;
  const groups = new Map<HighlightPalette, ThemeId[]>();
  for (const { id } of THEME_REGISTRY) {
    if (id === 'light') continue;
    const palette = HIGHLIGHT_PALETTE_BY_THEME[id];
    const list = groups.get(palette);
    if (list) list.push(id);
    else groups.set(palette, [id]);
  }

  const blocks = [`:root {\n${renderVars(defaultPalette)}\n}`];
  for (const [palette, ids] of groups) {
    if (palette === defaultPalette) continue;
    const selectors = ids.map((id) => `:root[data-theme='${id}']`).join(',\n');
    blocks.push(`${selectors} {\n${renderVars(palette)}\n}`);
  }

  const style = document.createElement('style');
  style.id = styleId;
  style.textContent = blocks.join('\n\n');
  document.head.appendChild(style);
}

/**
 * 生成语法高亮样式规则
 */
export function getHighlightStyleRules(): string {
  return `
.hljs-keyword { color: var(--hl-keyword) !important; }
.hljs-string { color: var(--hl-string) !important; }
.hljs-comment { color: var(--hl-comment) !important; }
.hljs-number { color: var(--hl-number) !important; }
.hljs-title { color: var(--hl-title) !important; }
.hljs-built_in { color: var(--hl-built_in) !important; }
.hljs-meta { color: var(--hl-meta) !important; }
.hljs-name { color: var(--hl-name) !important; }
.hljs-attr { color: var(--hl-attr) !important; }
.hljs-variable { color: var(--hl-variable) !important; }
.hljs-function { color: var(--hl-function) !important; }
.hljs-params { color: var(--hl-params) !important; }
.hljs-literal { color: var(--hl-literal) !important; }
.hljs-symbol { color: var(--hl-symbol) !important; }
.hljs-selector-tag { color: var(--hl-selectorTag) !important; }
.hljs-selector-class { color: var(--hl-selectorClass) !important; }
.hljs-selector-id { color: var(--hl-selectorId) !important; }
.hljs-subst { color: var(--hl-subst) !important; }
.hljs-emphasis { font-style: italic !important; }
.hljs-strong { font-weight: bold !important; }
.hljs-punctuation { color: var(--hl-punctuation) !important; }
.hljs-operator { color: var(--hl-operator) !important; }
.hljs-section { color: var(--hl-section) !important; }
.hljs-tag { color: var(--hl-tag) !important; }
.hljs-attribute { color: var(--hl-attribute) !important; }
.hljs-regexp { color: var(--hl-regexp) !important; }
.hljs-link { color: var(--hl-link) !important; }
.hljs-bullet { color: var(--hl-bullet) !important; }
.hljs-code { color: var(--hl-code) !important; }
.hljs-formula { color: var(--hl-formula) !important; }
.hljs-doctag { color: var(--hl-doctag) !important; }
.hljs-template-tag { color: var(--hl-templateTag) !important; }
.hljs-template-variable { color: var(--hl-templateVariable) !important; }
.hljs-addition { color: var(--hl-addition) !important; }
.hljs-deletion { color: var(--hl-deletion) !important; }
.hljs-quote { color: var(--hl-quote) !important; }
.hljs-type { color: var(--hl-type) !important; }
.hljs-class { color: var(--hl-class) !important; }
.hljs-title.class_ { color: var(--hl-class) !important; }
.hljs-title.function_ { color: var(--hl-function) !important; }
.hljs-variable.language_ { color: var(--hl-code) !important; }
.hljs-meta.prompt_,
.hljs-meta .hljs-prompt,
.prompt_ { color: var(--hl-meta) !important; }
.hljs-meta.keyword_,
.hljs-meta .hljs-keyword,
.keyword_ { color: var(--hl-keyword) !important; }
.hljs-meta.string_,
.hljs-meta .hljs-string,
.string_ { color: var(--hl-string) !important; }
`;
}

/**
 * 初始化语法高亮样式（在 main.ts 中调用）
 */
export function initHighlightStyles(): void {
  injectHighlightCSSVariables();

  const styleId = 'highlight-styles';
  if (document.getElementById(styleId)) return;

  const style = document.createElement('style');
  style.id = styleId;
  style.textContent = getHighlightStyleRules();
  document.head.appendChild(style);
}

// ============================================================
// Monaco 编辑器主题：与 --hl-* 调色板同源，保证编辑器与预览色系统一
// ============================================================

export const MONACO_APPLE_THEMES = {
  light: 'apple-light',
  dark: 'apple-dark',
  /** 液态玻璃（浅）：浅色高亮 + 透明编辑器底（玻璃面板透出） */
  glass: 'apple-glass',
  /** 液态玻璃（深）：暗色高亮 + 透明编辑器底 */
  glassDark: 'apple-glass-dark',
  /** 杂志排版：更暖一档的纸底 + 直角 */
  editorial: 'apple-editorial',
  /** 暖纸（Oil Book 同款）：更浅的暖纸底 */
  warmPaper: 'apple-warm-paper',
} as const;

/**
 * (base, material) → Monaco 主题名。玻璃材质用透明编辑器底透出面板，明/暗各一套；
 * 其余按 base 外观分派。未知 base 回落浅色。
 */
export function monacoThemeForUi(
  base: string | null | undefined,
  material: string | null | undefined = 'solid',
): string {
  const dark = isDarkThemeId(base);
  if (material === 'glass' && supportsGlass(base as ThemeId)) {
    return dark ? MONACO_APPLE_THEMES.glassDark : MONACO_APPLE_THEMES.glass;
  }
  if (base === 'editorial') return MONACO_APPLE_THEMES.editorial;
  if (base === 'warm-paper') return MONACO_APPLE_THEMES.warmPaper;
  return dark ? MONACO_APPLE_THEMES.dark : MONACO_APPLE_THEMES.light;
}

/** monaco 主题色要求无 # 前缀的 6 位 hex */
function stripHash(hex: string): string {
  return hex.replace(/^#/, '');
}

/**
 * 用 HighlightColors 调色板注册 apple-light / apple-dark 两个 Monaco 主题。
 * 幂等：重复调用会更新同名主题（monaco.defineTheme 允许覆盖）。
 */
export function defineMonacoThemes(monaco: typeof import('monaco-editor')): void {
  const buildRules = (c: Record<keyof typeof HighlightColors['light'], string>) => [
    { token: 'comment', foreground: stripHash(c.comment) },
    { token: 'keyword', foreground: stripHash(c.keyword) },
    { token: 'string', foreground: stripHash(c.string) },
    { token: 'regexp', foreground: stripHash(c.regexp) },
    { token: 'number', foreground: stripHash(c.number) },
    { token: 'type', foreground: stripHash(c.class) },
    { token: 'tag', foreground: stripHash(c.name) },
    { token: 'attribute.name', foreground: stripHash(c.attr) },
    { token: 'attribute.value', foreground: stripHash(c.string) },
    { token: 'metatag', foreground: stripHash(c.number) },
    { token: 'variable', foreground: stripHash(c.variable) },
    { token: 'function', foreground: stripHash(c.function) },
  ];

  monaco.editor.defineTheme(MONACO_APPLE_THEMES.light, {
    base: 'vs',
    inherit: true,
    rules: buildRules(HighlightColors.light),
    // monaco 运行时要求 colors 字段必须存在（空对象即继承 base 主题的界面色）
    colors: {},
  });
  monaco.editor.defineTheme(MONACO_APPLE_THEMES.dark, {
    base: 'vs-dark',
    inherit: true,
    rules: buildRules(HighlightColors.dark),
    colors: {},
  });
  monaco.editor.defineTheme(MONACO_APPLE_THEMES.glass, {
    base: 'vs',
    inherit: true,
    rules: buildRules(HighlightColors.light),
    colors: {
      'editor.background': '#ffffff00',
      'editor.lineHighlightBackground': '#ffffff40',
      'editorLineNumber.foreground': '#7d8494',
    },
  });
  monaco.editor.defineTheme(MONACO_APPLE_THEMES.glassDark, {
    base: 'vs-dark',
    inherit: true,
    rules: buildRules(HighlightColors.dark),
    colors: {
      'editor.background': '#00000000',
      'editor.lineHighlightBackground': '#ffffff14',
      'editorLineNumber.foreground': '#8e8e93',
    },
  });
  monaco.editor.defineTheme(MONACO_APPLE_THEMES.editorial, {
    base: 'vs',
    inherit: true,
    rules: buildRules(HighlightColors.light),
    colors: {
      'editor.background': '#fbfaf7',
      'editor.lineHighlightBackground': '#f3f0e9',
      'editorLineNumber.foreground': '#998f83',
    },
  });
  monaco.editor.defineTheme(MONACO_APPLE_THEMES.warmPaper, {
    base: 'vs',
    inherit: true,
    rules: buildRules(HighlightColors.light),
    colors: {
      'editor.background': '#faf8f4',
      'editor.lineHighlightBackground': '#f2ece1',
      'editorLineNumber.foreground': '#a29a8d',
    },
  });
}

/**
 * mermaid 取色：直接读当前主题的 CSS 变量，而不是只有明/暗两套预设，
 * 这样暖纸主题不会长出 macOS 蓝的流程图。
 */
export function mermaidThemeVariables(): Record<string, string> {
  const styles = getComputedStyle(document.documentElement);
  const read = (name: string, fallback: string) => styles.getPropertyValue(name).trim() || fallback;
  return {
    background: read('--apple-surface-panel', 'transparent'),
    primaryColor: read('--apple-bg-secondary', '#f5f5f7'),
    primaryBorderColor: read('--apple-border', '#d2d2d7'),
    primaryTextColor: read('--apple-text-primary', '#1d1d1f'),
    secondaryColor: read('--apple-bg-tertiary', '#f0f0f0'),
    tertiaryColor: read('--apple-bg-hover', '#f5f5f7'),
    lineColor: read('--apple-blue', '#0071e3'),
    edgeLabelBackground: read('--apple-surface-panel', '#ffffff'),
    clusterBkg: read('--apple-bg-secondary', '#f5f5f7'),
    clusterBorder: read('--apple-border', '#d2d2d7'),
    titleColor: read('--apple-text-primary', '#1d1d1f'),
    fontFamily: read('--apple-font-text', 'sans-serif'),
  };
}

/** 把外部传入的主题名（vs / vs-dark）映射为 apple 统一主题 */
export function toAppleMonacoTheme(theme: string | undefined): string {
  return theme === 'vs-dark' ? MONACO_APPLE_THEMES.dark : MONACO_APPLE_THEMES.light;
}
