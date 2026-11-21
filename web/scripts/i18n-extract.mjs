// i18n 抽取工装：把"会被渲染的中文"换成 $t()/t() 调用，并产出一份待译清单。
//
// 用法（在 web/ 下）：
//   node scripts/i18n-extract.mjs --ns=aiChat src/views/ai/AIChatView.vue src/components/ai/chat/*.vue
//   → 就地改文件；打印 JSON 到 stdout（key → 中文原文），另存成 pending 文件后再合并进语言包。
//
// 为什么是脚本而不是一只只手改：剩余面是 207 个文件 / 去重 3,465 条，手改会在第 30 个文件
// 开始漏，而漏掉的中文没有任何东西会发现（这正是这批工作的起因）。
//
// 但脚本**只碰确定安全的四类位置**：模板文本节点、模板静态属性、对象字面量里的展示字段、
// 以及 toast / *error*/*msg* 变量的赋值。其余一律不动并打印出来给人看 ——
// 因为脚本里还有一大类中文是**数据值**（与后端返回值比对的枚举、分组名、匹配用的字符串），
// 把它们翻译成英文不是改文案，是改行为。宁可漏，不可错。
//
// 跑完必须做的三件事：npm run type-check、npm run test（含 i18n-sync 与 closure 两台账）、
// 以及把 pending 里的键补进 locales/{zh-CN,en-US}.ts —— 没补键之前 closure 门禁会红，这是故意的。

import fs from 'node:fs';
import path from 'node:path';

const argv = process.argv.slice(2);
const nsArg = argv.find((a) => a.startsWith('--ns='));
const NS = nsArg ? nsArg.split('=')[1] : 'ui';
const files = argv.filter((a) => !a.startsWith('--'));
if (!NS || files.length === 0) {
  console.error('用法：node scripts/i18n-extract.mjs --ns=<命名空间> <文件...>');
  process.exit(2);
}

const CJK = /[\u3000-\u303f\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\uff00-\uffef]/;
// 这条曾经写成字形范围 [㐀-U+4DBF一-鿿…]，落盘时掉了两个边界字符，剩下裸连字符，
// 于是 JS 把它解析成 `U+0020–U+303F`：**纯 ASCII 行返回 true，纯中文行返回 false**。
// 抽取路径没被污染（75–228 行一直用显式 㐀-鿿），坏的是这台预筛 ——
// 它看着在筛，实际一条也没筛掉，全靠 321 行那条正确的正则兜底才没报错。
// 教训：字形范围抄一次就可能掉一个字符，码位写法不会被抄错。
const DISPLAY_ATTRS = ['label', 'placeholder', 'title', 'subtitle', 'ok-text', 'cancel-text', 'confirm-text', 'aria-label', 'alt', 'tooltip', 'empty-text', 'search-placeholder', 'description', 'hint', 'message', 'heading', 'text', 'content', 'empty', 'ok-title', 'cancel-title', 'action-text', 'primary-label', 'secondary-label', 'confirm-placeholder', 'noun', 'status-code'];
// 故意**不进名单**的：confirm-match-text —— 危险操作的"输入这个词才能删"令牌，
// 它既显示又参与比对，翻成英文等于改了校验行为（MediaManageView 那处要连后端约定一起定，交给人）。
const DISPLAY_FIELDS = ['label', 'title', 'text', 'desc', 'description', 'placeholder', 'message', 'tooltip', 'emptyText', 'okText', 'cancelText', 'hint', 'subtitle', 'heading'];
const MSG_VAR = /(err|error|msg|message|tip|hint|toast|warning|confirm)/i;

// 常见 UI 词 → 英文标识。命中的用语义键，命不中用 k<序号>：
// 键名可读性只影响人改起来顺不顺手，不值得为它牺牲一次批量的一致性。
const DICT = {
  保存: 'save', 取消: 'cancel', 确定: 'confirm', 确认: 'confirm2', 删除: 'delete', 编辑: 'edit',
  新建: 'create', 创建: 'create2', 搜索: 'search', 刷新: 'refresh', 关闭: 'close', 返回: 'back',
  提交: 'submit', 重置: 'reset', 导出: 'export', 导入: 'import', 复制: 'copy', 加载更多: 'loadMore',
  加载中: 'loading', 暂无数据: 'noData', 操作: 'actions', 名称: 'name', 描述: 'desc', 状态: 'status',
  类型: 'type', 时间: 'time', 详情: 'detail', 更多: 'more', 全部: 'all', 启用: 'enable', 禁用: 'disable',
  是: 'yes', 否: 'no', 提示: 'tip', 警告: 'warning', 成功: 'success', 失败: 'failed', 重试: 'retry',
  上一页: 'prev', 下一页: 'next', 排序: 'sort', 筛选: 'filter', 展开: 'expand', 收起: 'collapse',
  全选: 'selectAll', 反选: 'invert', 重命名: 'rename', 移动: 'move', 收藏: 'favorite', 分享: 'share',
  上传: 'upload', 下载: 'download', 预览: 'preview', 测试: 'test', 应用: 'apply', 添加: 'add',
};

const newKeys = new Map(); // key → zh 原文
const pendingParams = new Map(); // key → 该消息里 {pN} 对应的原表达式（写英文时要靠它判断语序）
const skipped = [];
let replaced = 0;
// vue-i18n 的消息格式里 @ 与 { } 是语法（@:链接、{name} 插值）。
// 这类串一律不自动搬 —— 搬过去就是"渲染那一刻编译失败、靠回退返回原文"的静默坏，
// 必须人工写成 {'@'} / {'{'} 或改成带参数的插值。i18n-batch.mjs 也把这作为整批回退的条件。
const MSG_UNSAFE = /[@{}|]/;
// 但"整句带插值"是**允许**的：`${N} 张图片` 抽成 `{p1} 张图片` 是正确的消息形状。
// 所以判"违例"时要先把自家生成的 {pN} 占位符摘掉，再看剩下还有没有裸的 { } @ |。
const unsafeApartFromParams = (s) => MSG_UNSAFE.test(s.replace(/\{p\d+\}/g, ''));

/**
 * 把模板字面量转成带参消息。返回 null 表示"这条别动"：
 * 嵌套花括号的表达式（`${a ? 'x' : 'y'}`、`${obj[k]}`）不是简单插值，
 * 机械替换会改语义，交给人。
 */
function templateToMessage(lit) {
  const exprs = [];
  const msg = lit.replace(/\$\{([^{}]+)\}/g, (_m, e) => {
    exprs.push(e.trim());
    return `{p${exprs.length}}`;
  });
  if (/\$\{/.test(msg)) return null; // 还有没吃掉的嵌套插值
  if (!/[\u3400-\u9fff]/.test(msg)) return null;
  if (unsafeApartFromParams(msg)) return null;
  const argObj = exprs.length ? `, { ${exprs.map((e, i) => `p${i + 1}: ${e}`).join(', ')} }` : '';
  return { msg, key: keyFor(msg), argObj, exprs };
}

function slug(zh) {
  const hit = DICT[zh.trim()];
  if (hit) return hit;
  const k = `k${newKeys.size + 1}`;
  return k;
}

function keyFor(zh) {
  for (const [existing, val] of newKeys) if (val === zh) return existing;
  const key = `${NS}.${slug(zh)}`;
  if (newKeys.has(key)) {
    // 同键不同文（比如两个都叫 k12 的）→ 加后缀，绝不覆盖前者
    let i = 2;
    while (newKeys.has(`${key}_${i}`)) i++;
    const alt = `${key}_${i}`;
    newKeys.set(alt, zh);
    return alt;
  }
  newKeys.set(key, zh);
  return key;
}

for (const file of files) {
  let src = fs.readFileSync(file, 'utf8');
  const before = src;
  const tplStart = src.indexOf('<template>');
  const tplEnd = src.lastIndexOf('</template>');
  const hasTpl = tplStart >= 0 && tplEnd > tplStart;

  if (hasTpl) {
    let tpl = src.slice(tplStart, tplEnd);
    // 1) 模板文本节点。允许两侧换行与缩进（本仓的模板几乎都是
    //    `<Tag /> 文案\n  <`，第一版把 \n 排除掉，结果一条都没抽到）。
    tpl = tpl.replace(/>(\s*)([^<>{}]*?)(\s*)</g, (m, pre, text, post) => {
      const t = text.trim();
      if (!t || !/[\u3400-\u9fff]/.test(t)) return m; // 只处理含中文的文本节点
      if (/[<>]/.test(t) || MSG_UNSAFE.test(t)) {
        if (MSG_UNSAFE.test(t)) skipped.push(`${file}: 文本节点 '${t.slice(0, 24)}' 含 @/{}/| 消息语法 → 跳过，需人工转义`);
        return m; // 含插值、标签或消息语法的整段跳过，交给人
      }
      replaced++;
      return `>${pre}{{ $t('${keyFor(t)}') }}${post}<`;
    });
    // 2) 静态属性
    for (const attr of DISPLAY_ATTRS) {
      const re = new RegExp(`(?<![:@\\w-])${attr}="([^"]*[\\u3400-\\u9fff][^"]*)"`, 'g');
      tpl = tpl.replace(re, (m, v) => {
        if (MSG_UNSAFE.test(v)) { skipped.push(`${file}: ${attr}="${v.slice(0, 24)}" 含 @ 或 {} → 跳过，需人工转义`); return m; }
        replaced++;
        return `:${attr}="$t('${keyFor(v)}')"`;
      });
    }
    // 2b) 插值里的**分支位置**字面量：`{{ active ? '已收藏' : '收藏' }}`。
    // 只认紧跟在 ? 或 : 后面的串，且前面 12 个字符里出现 == / != / includes( 就跳过 ——
    // `{{ s === '已完成' ? 'Done' : '' }}` 里 '已完成' 是**比较值**，翻了就改判定，不是改文案。
    tpl = tpl.replace(/\{\{[^{}]*\}\}/g, (block) => block.replace(/([?:]\s*)'([^']*[\u3400-\u9fff][^']*)'/g, (m, pre, v, offset) => {
      const before = block.slice(Math.max(0, offset - 12), offset);
      if (/(==|!=|includes\(|indexOf\(|startsWith\(|endsWith\()/.test(before)) return m;
      if (MSG_UNSAFE.test(v)) return m;
      replaced++;
      // 模板里必须用 $t()：setup 作用域的 t 不在渲染上下文里，写 t('…') 会报
      // "Property 't' does not exist"（第一版就是这样，type-check 直接红）。
      return `${pre}$t('${keyFor(v)}')`;
    }));
    // 2c) 模板里的**文本节点紧跟插值**：`{{ n }} 成员`、`共 {{ total }} 个空间` 的前半段。
    // 文本节点判据以 `>` 起头，被前面的 `}}` 挡死，整类漏抽（英文界面上就剩 "4 hour ago 成员" 这种混排）。
    // 这里只吃"**文本节点以插值开头**"的那一类（`<span>{{ n }} 成员</span>`）；
    // 插值前面还有中文的（`共 {{ total }} 个空间`）一律不碰 —— 只翻后半截会渲染出
    // "共 4 Spaces" 这种半中半英的句子，比整条留着更糟。那类要连句子重写成带参数的消息。
    tpl = tpl.replace(/>(\s*)(\{\{[^{}]*\}\})(\s+)([^<>{}\s][^<>{}]*?)(\s*)</g, (m, pre, interp, gap, text, post) => {
      const t = text.trim();
      if (!t || !/[\u3400-\u9fff]/.test(t)) return m;
      if (/[{}<>@|]/.test(t)) return m;
      replaced++;
      return `>${pre}${interp}${gap}{{ $t('${keyFor(t)}') }}${post}<`;
    });
    // 2d) 绑进属性的对象字面量里的展示字段：`:actions="[{ label: '新建空间' }]"`。
    // 原先展示字段只扫 <script>，页面头部按钮那一类整个漏掉。模板里同样必须用 $t()。
    tpl = tpl.replace(new RegExp(`\\b(${DISPLAY_FIELDS.join('|')})\\s*:\\s*'([^']*[\\u3400-\\u9fff][^']*)'`, 'g'), (m, field, v) => {
      if (/[{}<>@|]/.test(v)) return m;
      replaced++;
      return `${field}: $t('${keyFor(v)}')`;
    });
    src = src.slice(0, tplStart) + tpl + src.slice(tplEnd);
  }

  // 3) 脚本里的展示字段与消息赋值
  const scriptFrom = hasTpl ? src.lastIndexOf('</template>') : 0;
  let script = src.slice(scriptFrom);
  // 展示字段。**withDefaults(...) 的默认值块一律不碰**：那里的字面量在组件定义期只求值一次，
  // 换成 t() 就不会随语言切换重算 —— 那是语义错误，不是文案替换。
  // 要翻得改成"prop 可选 + 渲染处 props.x ?? $t(...)"，那是逐处的人工改法（ApTable 的 emptyText 就是这例）。
  const fieldRe = new RegExp(`\\b(${DISPLAY_FIELDS.join('|')})\\s*:\\s*'([^']*[\\u3400-\\u9fff][^']*)'`, 'g');
  const wdStart = script.search(/withDefaults\s*\(/);
  let wdEnd = -1;
  if (wdStart >= 0) {
    let depth = 0;
    for (let i = script.indexOf('(', wdStart); i < script.length; i++) {
      if (script[i] === '(') depth++;
      else if (script[i] === ')') { depth--; if (depth === 0) { wdEnd = i; break; } }
    }
  }
  script = script.replace(fieldRe, (m, field, v, offset) => {
    if (wdEnd >= 0 && offset >= wdStart && offset <= wdEnd) {
      skipped.push(`${file}: ${field}: '${v.slice(0, 20)}' 落在 withDefaults 默认值里，定义期只求值一次 → 不自动替换，需改成"可选 prop + 渲染处 ?? $t()"`);
      return m;
    }
    if (MSG_UNSAFE.test(v)) { skipped.push(`${file}: ${field}: '${v.slice(0, 24)}' 含 @ 或 {} → 跳过，需人工转义`); return m; }
    replaced++;
    return `${field}: t('${keyFor(v)}')`;
  });
  // 3b) 直接写进 DOM 的文案（表格筛选框、排序 title 这类）。
  // 这些不在任何"展示字段"名单里，但确实是用户看得见的字面量。
  const domRe = /\.(placeholder|textContent|title|ariaLabel|innerText)\s*=\s*'([^']*[\u3400-\u9fff][^']*)'/g;
  script = script.replace(domRe, (m, prop, v) => {
    if (MSG_UNSAFE.test(v)) return m;
    replaced++;
    return `.${prop} = t('${keyFor(v)}')`;
  });
  const toastRe = /\bshowToast\(\s*'([^']*[\u3400-\u9fff][^']*)'/g;
  script = script.replace(toastRe, (m, v) => {
    if (MSG_UNSAFE.test(v)) return m;
    replaced++;
    return `showToast(t('${keyFor(v)}')`;
  });
  // 消息赋值。原先写成 `[A-Za-z_$][\w$]*(?:err|error|msg|…)[\w$]*` 并给整条加 'i'，
  // 再在替换时 `new RegExp(msgRe.source, 'g')` 把 'i' 丢了 —— 于是 errorMsg（大写 M）整类漏掉；
  // 补上 'gi' 后那个"名字里含关键词"的正则又会从名字中间开始匹配，把 `.value` 吃掉、
  // 还把捕获组错位成名字的一部分（探针实测：errorMsg.value = '加载失败' 被换成
  // `errorMsg = t('…')`，pendingKeys 里存的值是 "Msg"）。
  // 改成：先完整捕获标识符，再单独用带 i 的 MSG_VAR 判名字，`.value` 原样带回。
  script = script.replace(/\b([A-Za-z_$][\w$]*)((?:\s*\.value)?)\s*=\s*'([^']*[\u3400-\u9fff][^']*)'/g, (m, varName, accessor, v) => {
    if (!MSG_VAR.test(varName)) return m;
    if (MSG_UNSAFE.test(v)) return m;
    replaced++;
    return `${varName}${accessor} = t('${keyFor(v)}')`;
  });
  // 5) **整句带插值**的展示位置：`最多只能上传 ${N} 张图片` → t('ns.k', { p1: N })。
  // 这是剩余欠账里最大的一块（错误提示、限额说明几乎都是这个形状），
  // 但只认三类"确定是给人看"的前文，其余模板字面量（SQL、prompt、文件名）一律不碰。
  const SAFE_BEFORE = [
    /(?:\bui\.)?showToast\($/,
    new RegExp(`\\b(?:${DISPLAY_FIELDS.join('|')})\\s*:\\s*$`),
    /\b[A-Za-z_$][\w$]*(?:err|error|msg|message|tip|hint|toast|warning|confirm)[\w$]*(?:\.value)?\s*=\s*$/i,
    /\.(placeholder|textContent|title|ariaLabel|innerText)\s*=\s*$/,
  ];
  script = script.replace(/`([^`]+)`/g, (m, body, offset) => {
    if (!body.includes('${') || !/[\u3400-\u9fff]/.test(body)) return m;
    const before = script.slice(Math.max(0, offset - 64), offset);
    if (!SAFE_BEFORE.some((re) => re.test(before))) return m;
    const made = templateToMessage(body);
    if (!made) {
      skipped.push(`${file}: 模板整句跳过（嵌套插值或消息语法）：\`${body.slice(0, 44)}\``);
      return m;
    }
    pendingParams.set(made.key, made.exprs);
    replaced++;
    return `t('${made.key}'${made.argObj})`;
  });
  src = src.slice(0, scriptFrom) + script;

  // 需要 t() 的话补 import 与取值方式
  if (/\bt\('/.test(script)) {
    if (!/<script setup/.test(src)) {
      // **纯 .ts 模块**（composable / store / config / guard）里不能调 useI18n()：
      // 它只能在组件 setup 的同步期调用，模块顶层调它 = 导入即抛。
      // 第一版没分这一支，`const { t } = useI18n();` 被插到 guards.ts 第 1 行、
      // main.ts、docTemplates.ts —— type-check 与单测全绿，应用起不来。
      if (!/const \{ t \} = i18n\.global/.test(src)) {
        const lines = src.split('\n');
        let lastImport = -1;
        lines.forEach((l, i) => {
          if (/^import /.test(l)) lastImport = i;
        });
        // 已经 import 过 i18n 的文件不能再来一遍，否则 TS2300 Duplicate identifier。
        // 两种写法都要认：main.ts 用的是相对路径 './i18n'，只认 '@/i18n' 会漏（实测重复了一次）。
        const hasI18nImport = /^import i18n from ['"][^'"]*i18n['"];$/m.test(src);
        lines.splice(lastImport + 1, 0, ...(hasI18nImport ? [] : ["import i18n from '@/i18n';", '']), 'const { t } = i18n.global;');
        src = lines.join('\n');
      }
    } else {
    if (!/import \{ useI18n \} from 'vue-i18n';/.test(src)) {
      const anchor = /(<script setup[^>]*>)/;
      if (anchor.test(src)) src = src.replace(anchor, `$1\nimport { useI18n } from 'vue-i18n';`);
      else skipped.push(`${file}: 找不到 <script setup>，请手工接 useI18n`);
    }
    if (!/const \{ t \} = useI18n\(\)/.test(src)) {
      // 位置要同时满足两件事：① 在最后一个顶层宏之后（否则撞 vue/define-macros-order），
      // ② 在**第一次用到 t 的那行之前**（否则 const 的 TDZ 会让 type-check 报
      // "Block-scoped variable 't' used before its declaration" —— 第一版就栽在这）。
      const lines = src.split('\n');
      let afterMacro = -1;
      for (let i = 0; i < lines.length; i++) {
        if (/^(const \w+ = )?(defineProps|defineEmits|withDefaults\()/.test(lines[i])) {
          let j = i;
          while (j < lines.length && !['}>();', '});'].includes(lines[j].trim())) j++;
          afterMacro = j;
        }
      }
      // 判断"脚本里用到了 t("必须排掉模板里的 $t( —— $ 不是单词字符，\bt\( 会连 $t( 一起命中，
      // 于是纯模板改动的文件也被塞进一行没人用的 const { t }（eslint no-useless-assignment 就是这么来的）。
      const usesT = (line) => /(^|[^$\w.])t\('/.test(line) && !/useI18n/.test(line);
      const tplEndLine = lines.findIndex((l) => l.includes('</template>'));
      const firstUse = lines.findIndex((l, i) => i > tplEndLine && usesT(l));
      // 往上退到"这条语句的起始行"再插。第一次用到 t 的地方经常在一个跨行字面量内部
      // （`const options = [` 换行后才出现 label: t(...)），直接在首次使用那行前面插，
      // 就会把 const 塞进数组中间 —— vue-tsc 报 TS1005/TS1128，整批文件语法作废（批次九实测踩到）。
      const scriptStart = lines.findIndex((l) => l.startsWith('<script setup'));
      const statementStart = (from) => {
        let idx = from;
        while (
          idx > scriptStart + 1 &&
          (/^[ \t]/.test(lines[idx]) || /^[)\]}]/.test(lines[idx].trim()))
        ) idx--;
        let lastImport = -1;
        for (let i = scriptStart + 1; i <= idx; i++) if (/^import /.test(lines[i])) lastImport = i;
        return Math.max(idx, lastImport + 1);
      };
      if (afterMacro >= 0 && (firstUse < 0 || firstUse > afterMacro)) {
        lines.splice(afterMacro + 1, 0, 'const { t } = useI18n();');
      } else if (firstUse >= 0) {
        const idx = statementStart(firstUse);
        lines.splice(idx, 0, 'const { t } = useI18n();');
        if (afterMacro >= idx) skipped.push(`${file}: t() 的首次使用在 defineProps 之前，位置需人工确认`);
      } else {
        lines.splice(lines.findIndex((l) => l.startsWith('<script setup')) + 1, 0, 'const { t } = useI18n();');
      }
      src = lines.join('\n');
    }
    }
  }

  if (src !== before) fs.writeFileSync(file, src, 'utf8');

  // 4) 剩下的中文位置：只报不改
  const rest = src.split('\n');
  rest.forEach((line, i) => {
    if (!CJK.test(line)) return;
    const t = line.trim();
    if (/^(\/\/|\/\*|\*|<!--)/.test(t)) return;
    if (/[\u3400-\u9fff]/.test(t) && !/t\('|t\("|\$t\(/.test(t)) skipped.push(`${file}:${i + 1}: ${t.slice(0, 90)}`);
  });
}

console.log(JSON.stringify({
  namespace: NS,
  replaced,
  pendingKeys: Object.fromEntries(newKeys),
  // 带参消息的 {pN} 各对应哪个原表达式 —— 写英文时要靠它判断语序（中文"最多 N 张"
  // 与英文"at most N images" 参数位置不同，但具名插值让两边能各自排）。
  pendingParams: Object.fromEntries(pendingParams),
  needsHumanEyes: skipped,
}, null, 2));
