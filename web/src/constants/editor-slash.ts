export interface SlashCommandDef {
  label: string;
  /** i18n key：菜单里展示的说明文字，渲染时再翻译 */
  detailKey: string;
  kind: 'content' | 'ai';
  type?: string;
  action?: string;
}

export const CONTENT_SLASH_COMMANDS: SlashCommandDef[] = [
  { label: '/heading', detailKey: 'slash.k1', kind: 'content', type: 'heading' },
  { label: '/bold', detailKey: 'slash.k2', kind: 'content', type: 'bold' },
  { label: '/italic', detailKey: 'slash.k3', kind: 'content', type: 'italic' },
  { label: '/link', detailKey: 'slash.k4', kind: 'content', type: 'link' },
  { label: '/doc-link', detailKey: 'slash.k5', kind: 'content', type: 'doc-link' },
  { label: '/image', detailKey: 'slash.k6', kind: 'content', type: 'image' },
  { label: '/inline-code', detailKey: 'slash.k7', kind: 'content', type: 'inline-code' },
  { label: '/code', detailKey: 'slash.k8', kind: 'content', type: 'code' },
  { label: '/code-js', detailKey: 'slash.k9', kind: 'content', type: 'code:javascript' },
  { label: '/code-ts', detailKey: 'slash.k10', kind: 'content', type: 'code:typescript' },
  { label: '/code-py', detailKey: 'slash.k11', kind: 'content', type: 'code:python' },
  { label: '/code-go', detailKey: 'slash.k12', kind: 'content', type: 'code:go' },
  { label: '/code-java', detailKey: 'slash.k13', kind: 'content', type: 'code:java' },
  { label: '/code-bash', detailKey: 'slash.k14', kind: 'content', type: 'code:bash' },
  { label: '/code-sql', detailKey: 'slash.k15', kind: 'content', type: 'code:sql' },
  { label: '/table', detailKey: 'slash.k16', kind: 'content', type: 'table' },
  { label: '/list', detailKey: 'slash.k17', kind: 'content', type: 'list' },
  { label: '/ordered-list', detailKey: 'slash.k18', kind: 'content', type: 'ordered-list' },
  { label: '/task', detailKey: 'slash.k19', kind: 'content', type: 'task' },
  { label: '/quote', detailKey: 'slash.k20', kind: 'content', type: 'quote' },
  { label: '/toggle', detailKey: 'slash.k21', kind: 'content', type: 'toggle' },
  { label: '/callout', detailKey: 'slash.k22', kind: 'content', type: 'callout' },
  { label: '/mermaid-flow', detailKey: 'slash.k23', kind: 'content', type: 'mermaid-flow' },
  { label: '/mermaid-sequence', detailKey: 'slash.k24', kind: 'content', type: 'mermaid-sequence' },
  { label: '/mermaid-gantt', detailKey: 'slash.k25', kind: 'content', type: 'mermaid-gantt' },
  { label: '/math', detailKey: 'slash.k26', kind: 'content', type: 'math' },
  { label: '/hr', detailKey: 'slash.k27', kind: 'content', type: 'hr' },
];

export const AI_SLASH_COMMANDS: SlashCommandDef[] = [
  { label: '/ai', detailKey: 'slash.k28', kind: 'ai', action: 'open' },
  { label: '/summary', detailKey: 'slash.k29', kind: 'ai', action: 'summarize' },
  { label: '/translate', detailKey: 'slash.k30', kind: 'ai', action: 'translate' },
  { label: '/explain', detailKey: 'slash.k31', kind: 'ai', action: 'explain' },
  { label: '/report', detailKey: 'slash.k32', kind: 'ai', action: 'report' },
];

export const ALL_SLASH_COMMANDS: SlashCommandDef[] = [...CONTENT_SLASH_COMMANDS, ...AI_SLASH_COMMANDS];

// `/` 仅在块级起始位置（行首/空白/Tab/引用符 `>` 之后）触发，
// 避免在输入 `https://`、`/api/v1`、`a/b` 等 URL/路径时误弹命令菜单。
export const SLASH_TRIGGER_REGEX = /(^|[ \t>])\/([a-zA-Z]{0,32})$/;
