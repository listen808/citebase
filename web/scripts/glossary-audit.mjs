// 术语表 → 可执行探针：读 docs/glossary/frontend-glossary.md 的术语清单，
// 逐条在运行中的前端上量，输出「命中实例 / 命中页 / 差异样本」三列。
// 用法（需 :3000 dev server 与 :3001 后端在跑）：
//   SNAP_USER=xxx SNAP_PASS=yyy node web/scripts/glossary-audit.mjs [--routes=N]
//
// 设计约束：
//   1. 术语清单的唯一事实源是那份 md，不在本文件里重复维护一份名单；
//   2. 每条术语要么有探针，要么显式标 NO_PROBE（不可自动化判定），绝不用猜测填数；
//   3. 探针只报「与仓库自身契约的差异」，契约来自 docs/engineering/ 与 apple-tokens 令牌。
import { readFileSync, writeFileSync, existsSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';
import { loginForm } from './lib/page-state.mjs';

const ROOT = path.resolve(process.cwd());
const GLOSSARY = path.join(ROOT, 'docs/glossary/frontend-glossary.md');
const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
if (!USER || !PASS) {
  console.error('需要 SNAP_USER / SNAP_PASS 环境变量');
  process.exit(2);
}
const argv = Object.fromEntries(process.argv.slice(2).map((a) => {
  const [k, ...rest] = a.replace(/^--/, '').split('=');
  return [k, rest.join('=')];
}));

const AI_SECTIONS = ['overview', 'analysis', 'search-overview', 'model', 'rag', 'prompts', 'schema', 'knowledge-base', 'import', 'auto-tag', 'sql-history', 'sql-audit-trace', 'sql-example', 'advanced-ask', 'mcp', 'workflows', 'widget', 'write', 'wiki-compile', 'wiki-write', 'observability', 'token', 'operations', 'sql-security', 'ragas-eval', 'data-sensitivity', 'eval-console', 'capability-health', 'ab-experiment'];
const ROUTES = [
  '/login', '/author-login', '/install', '/error', '/share/1',
  '/', '/search', '/space/1', '/space/1/doc/1', '/space/1/doc/1/edit', '/space/1/views',
  '/collections', '/follow', '/activities', '/notifications', '/saved-searches', '/webhooks',
  '/tags', '/templates', '/doc-collab', '/git-sync', '/profile', '/user/9', '/about',
  '/ai/workspace', '/ai/graph',
  ...AI_SECTIONS.map((s) => `/ai/settings?section=${s}`),
  '/dashboard', '/users', '/roles', '/roles/1/users', '/privileges', '/spaces', '/logs',
  '/links', '/contacts', '/media', '/settings', '/settings/email', '/settings/sso',
  '/settings/sso/doc', '/settings/api-keys', '/plugins', '/search-monitor', '/backups',
  '/data-export', '/search-ops',
];

// ── 令牌白名单：从令牌文件本身推导，不在巡检里另抄一份「合法值」清单 ──────
// 教训：Border Radius 上一版把 980px/16px/20px 报成「非语义圆角漂移」，
// 而它们正是 AppleRadius.pill / large / xl；Easing 把 AppleTransition.standard
// 报成非令牌，则是正则写错（见下方 cubic-bezier 的括号转义）。硬抄清单只会
// 把「合规的实现」列成待修，反过来漏掉真漂移。清单唯一可信来源是源码。
const readSrc = (rel) => {
  for (const base of [path.join(ROOT, 'web'), ROOT]) {
    const p = path.join(base, rel);
    if (existsSync(p)) return readFileSync(p, 'utf8');
  }
  return '';
};
const blockOf = (src, name) => {
  const m = new RegExp('export const ' + name + ' = \\{([\\s\\S]*?)\\n\\} as const').exec(src);
  return m ? m[1] : '';
};
const TOKEN_SRC = readSrc('src/utils/core/apple-tokens.ts');
const THEME_SRC = readSrc('src/styles/theme/themes.ts');
const RADIUS_TOKENS = (() => {
  const s = new Set(['0px', '50%', '100%']);
  for (const m of blockOf(TOKEN_SRC, 'AppleRadius').matchAll(/:\s*'([^']+)'/g)) {
    if (m[1].endsWith('px') || m[1].endsWith('%')) s.add(m[1]);
  }
  // var(--apple-radius-*) 的真实落地值在主题表里，逐主题取默认值
  for (const m of THEME_SRC.matchAll(/'--apple-radius-[a-z]+':\s*'([^']+)'/g)) s.add(m[1]);
  return [...s];
})();
const ZINDEX_TOKENS = (() => {
  const s = new Set(['auto', '0']);
  for (const m of blockOf(TOKEN_SRC, 'AppleZIndex').matchAll(/:\s*(\d+)/g)) s.add(m[1]);
  return [...s];
})();
const EASING_TOKENS = (() => {
  const s = new Set(['ease', 'ease-in', 'ease-out', 'ease-in-out', 'linear', 'steps(1, start)']);
  const src = TOKEN_SRC + THEME_SRC;
  for (const m of src.matchAll(/cubic-bezier\([^)]*\)/g)) s.add(m[0]);
  for (const m of src.matchAll(/steps\([^)]*\)/g)) s.add(m[0]);
  return [...s];
})();
if (!RADIUS_TOKENS.length || !ZINDEX_TOKENS.length || !EASING_TOKENS.length) {
  process.stdout.write('警告：未从令牌文件解析出完整白名单（路径或写法变了），此刻圆角/缓动/z-index 判定不可信\n');
}

// ── 页内探针 ────────────────────────────────────────────────────────────
// 约定：每个探针返回 { n, bad:[样本…] }；n=命中实例数，bad=违例样本（最多 6 条）。
const P = `(() => {
  const ALLOWED_H=[24,28,32,40,44];
  const near=(h)=>ALLOWED_H.some(a=>Math.abs(h-a)<=1);
  const txt=(e)=>((e.textContent||'').trim()||e.getAttribute('aria-label')||e.getAttribute('title')||e.getAttribute('placeholder')||'').trim().slice(0,14);
  const vis=(e)=>{const r=e.getBoundingClientRect();return r.width>4&&r.height>4;};
  const box=(e)=>{const r=e.getBoundingClientRect();return {h:Math.round(r.height),w:Math.round(r.width)};};
  const nameOf=(e)=>{
    if((e.textContent||'').trim()) return true;
    if(e.getAttribute('aria-label')||e.getAttribute('title')) return true;
    const id=e.id; if(id){const l=document.querySelector('label[for="'+CSS.escape(id)+'"]'); if(l&&l.textContent.trim()) return true;}
    const wrap=e.closest('label'); return !!(wrap&&wrap.textContent.trim());
  };
  const $ap=(n)=>[].slice.call(document.querySelectorAll('[data-ap="'+n+'"]')).filter(vis);
  // 违例样本必须带出处。「无名按钮@532,119」这种坐标样本我看了没法改，
  // 也分不清是全站一处还是每页一处；带上路由与标签链才谈得上照着修。
  // AI 设置 29 个分区共用同一个 pathname，只报路径会把它们全并成一条；带上 query 才定位得到。
  const where=()=>(location.pathname+location.search).slice(0,34);
  const chain=(e,depth)=>{const p=[];let x=e;for(let i=0;i<(depth||3)&&x;i++){p.unshift(x.tagName.toLowerCase()+(x.getAttribute&&x.getAttribute('data-ap')?'['+x.getAttribute('data-ap')+']':'')+(x.getAttribute&&x.getAttribute('class')?'.'+String(x.getAttribute('class')).split(' ')[0].slice(0,14):''));x=x.parentElement;}return p.join('>');};
  // 第三方编辑器（Monaco/CodeMirror）自己带 z-index:-1 与 #000 文本，
  // 那是它的 DOM，不该记到本项目的样式账上。
  const thirdParty=(e)=>!!(e.closest && e.closest('.monaco-editor,[data-mode-id],.cm-editor,[class*=devtools]'));
  const R={};
  const reg=(k,f)=>{try{R[k]=f();}catch(e){R[k]={n:-1,bad:['PROBE_ERR '+String(e).slice(0,40)]};}};

  // 2. Buttons & Links
  reg('Button',()=>{const es=[...document.querySelectorAll('button')].filter(e=>vis(e)&&!thirdParty(e));
    const noName=es.filter(b=>!nameOf(b)).slice(0,6).map(b=>'无名按钮@'+where()+':'+chain(b));
    const offH=es.filter(b=>!near(box(b).h)).slice(0,6).map(b=>'高度离档 h'+box(b).h+' @'+where()+' text='+txt(b));
    return {n:es.length,bad:noName.concat(offH)};});
  reg('Link',()=>{const as=[...document.querySelectorAll('a')].filter(vis);
    const fake=[...document.querySelectorAll('[role=link]')].filter(e=>e.tagName!=='A'&&vis(e));
    return {n:as.length,bad:[...as.filter(a=>!a.getAttribute('href')).slice(0,4).map(a=>'无href:'+txt(a)),...fake.slice(0,4).map(e=>'非a元素role=link:'+e.tagName+':'+txt(e))]};});

  // 3. Forms
  reg('Input',()=>{const es=[...document.querySelectorAll('input')].filter(e=>!thirdParty(e)&&(vis(e)||e.closest('label')));
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,6).map(e=>'无标签输入 type='+(e.type||'text')+' @'+where()+':'+chain(e))};});
  reg('Label',()=>{const ls=[...document.querySelectorAll('label[for]')];
    const dangling=ls.filter(l=>!document.getElementById(l.getAttribute('for')));
    return {n:ls.length,bad:dangling.slice(0,6).map(l=>'for 指向不存在的 id: '+l.textContent.trim().slice(0,10)+' @'+where())};});
  reg('Placeholder',()=>{const es=[...document.querySelectorAll('input[placeholder],textarea[placeholder]')].filter(e=>vis(e)&&!thirdParty(e));
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,6).map(e=>'只有占位符没有标签 @'+where()+':'+chain(e))};});
  reg('Textarea',()=>{const es=[...document.querySelectorAll('textarea')].filter(e=>!thirdParty(e));
    return {n:es.length,bad:es.filter((t)=>vis(t)&&!nameOf(t)).slice(0,4).map(t=>'多行输入无可读名字 @'+where())};});
  reg('Select',()=>{const raw=document.querySelectorAll('select').length;
    const es=$ap('select').length?$ap('select'):[...document.querySelectorAll('[role=combobox]')].filter(vis);
    return {n:es.length,bad:(raw ? ['原生 select 泄漏 x'+raw] : []).concat(es.filter(e=>!near(box(e).h)).slice(0,6).map(e=>'高度离档 h'+box(e).h+' @'+where()))};});
  reg('Checkbox',()=>{const es=[...document.querySelectorAll('input[type=checkbox],[role=checkbox]')].filter(e=>vis(e)||true);
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,6).map(e=> '复选框无可读名字 @'+where()+':'+chain(e))};});
  reg('Switch',()=>{const es=$ap('switch').length?$ap('switch'):[...document.querySelectorAll('[role=switch]')].filter(vis);
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,6).map(e=> '开关无可读名字 @'+where()+':'+chain(e))};});
  reg('Radio',()=>{const es=[...document.querySelectorAll('input[type=radio],[role=radio]')].filter(vis);
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,4).map(()=> '单选无可读名字')};});
  reg('Form',()=>{const fs=[...document.querySelectorAll('form')];
    return {n:fs.length,bad:fs.map(f=>{const i=f.querySelectorAll('input,textarea,select,[role=combobox]');return i.length&&!f.querySelector('label')?'表单 '+i.length+' 个控件零标签':''}).filter(Boolean).slice(0,4)};});
  reg('Upload',()=>({n:document.querySelectorAll('input[type=file]').length,bad:[]}));
  ['Number Input','Slider','Rate','Date Picker','Time Picker','Color Picker','Cascader','Tree Select','Autocomplete'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));
  reg('Number Input',()=>{const es=[...document.querySelectorAll('input[type=number]')].filter(e=>vis(e)&&!thirdParty(e));
    // 「裸 number」不是缺陷判据：原生 number 自带步进。真正的缺陷是拿不到可读名字。
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,4).map(e=>'数字输入无可读名字 @'+where()+':'+chain(e))};});
  reg('Slider',()=>({n:document.querySelectorAll('input[type=range],[role=slider]').length,bad:[]}));

  // 4. Content
  reg('Table',()=>{const ts=[...document.querySelectorAll('table')].filter(vis);
    const bad=[];for(const t of ts){let a=t.parentElement,sc=null;while(a){const c=getComputedStyle(a);if(c.overflowX==='auto'||c.overflowX==='scroll'){sc=a;break;}a=a.parentElement;}
      // 有横向可滚祖先时，表格比容器宽是正常状态（ApTable 就是这么设计的），
      // 只有「没得滚还被裁掉」才是缺陷。上一版把两种情况混成一条，报了个假阳性。
      if(!sc&&t.scrollWidth>t.clientWidth+4) bad.push('表格内容 '+t.scrollWidth+'px 宽、无横向可滚祖先，列被裁 @'+where()+':'+chain(t,4));
      if(!t.querySelector('caption')&&!t.getAttribute('aria-label')) bad.push('表格无 caption/aria-label');}
    return {n:ts.length,bad:bad.slice(0,6)};});
  reg('List',()=>({n:document.querySelectorAll('ul,ol,[role=list]').length,bad:[]}));
  reg('Card',()=>{const es=$ap('card').concat($ap('entity-card'));
    return {n:es.length,bad:[]};});
  reg('Tag',()=>{const pills=[...document.querySelectorAll('span,div')].filter(e=>{const c=getComputedStyle(e);return vis(e)&&parseFloat(c.borderRadius)>=999&&e.children.length<=2&&(e.textContent||'').trim().length&&e.textContent.trim().length<10;});
    return {n:pills.length,bad:pills.slice(0,6).map(e=>'手写胶囊:'+e.textContent.trim().slice(0,6))};});
  reg('Badge',()=>({n:$ap('status-badge').length,bad:[]}));
  reg('Avatar',()=>{const es=$ap('avatar');
    return {n:es.length,bad:es.filter(e=>{const r=e.getBoundingClientRect();const t=(e.textContent||'').trim();return t.length>2&&r.width<40;}).slice(0,4).map(e=>'整名塞进小方块:'+e.textContent.trim().slice(0,6))};});
  reg('Statistic',()=>({n:$ap('stat-card').length,bad:[]}));
  reg('Tabs',()=>{const tl=[...document.querySelectorAll('[role=tablist]')];
    const tabs=[...document.querySelectorAll('[role=tab]')];
    const orphan=tabs.filter(t=>!t.closest('[role=tablist]'));
    return {n:tabs.length,bad:orphan.slice(0,4).map(t=>'tab 不在 tablist 内:'+t.textContent.trim().slice(0,6)).concat(tl.filter(x=>!x.getAttribute('aria-label')).slice(0,3).map(()=>'tablist 无 aria-label'))};});
  reg('Segmented',()=>{const es=[].slice.call(document.querySelectorAll('[data-ap=segmented-control] [role=tab],[data-ap=segmented-control] button'));
    return {n:es.length,bad:es.filter(e=>!near(box(e).h)).slice(0,6).map(e=>'高度离档 h'+box(e).h)};});
  reg('Empty',()=>{const es=$ap('empty');
    return {n:es.length,bad:es.filter(e=>!e.textContent.trim()).slice(0,4).map(()=>'空态只有图没有文字')};});
  reg('Icon',()=>({n:document.querySelectorAll('svg').length,bad:[...document.querySelectorAll('button,span,div')].filter(e=>e.children.length===0&&/\\p{Extended_Pictographic}/u.test(e.textContent||'')).slice(0,4).map(e=>'emoji 当图标:'+e.textContent.trim().slice(0,2))}));
  reg('Chart',()=>{const cs=[...document.querySelectorAll('canvas')].filter(c=>vis(c)&&!thirdParty(c));
    return {n:cs.length,bad:cs.filter(c=>!c.getAttribute('aria-label')&&!c.closest('[aria-label]')).slice(0,4).map(c=>'canvas 无文字替代/aria-label @'+where()+':'+chain(c,6))};});
  reg('Image',()=>{const is=[...document.querySelectorAll('img')].filter(vis);
    return {n:is.length,bad:is.filter(i=>!i.getAttribute('alt')).slice(0,6).map(i=>'img 无 alt:'+i.src.slice(-18))};});
  reg('Progress',()=>{const es=[...document.querySelectorAll('[role=progressbar]')];
    // 顶部路由进度条平时是 v-show（高度 3px 且 display:none），按可见性过滤会把它当成不存在
    const fake=[...document.querySelectorAll('div')].filter(e=>{const c=getComputedStyle(e);return vis(e)&&parseFloat(c.width)<e.parentElement.clientWidth-2&&/blue|--apple-/.test(c.backgroundColor)&&parseFloat(c.borderRadius)>=99;});
    return {n:es.length,bad:es.filter(e=>e.getAttribute('aria-valuenow')==null).slice(0,3).map(()=>'progressbar 缺 aria-valuenow').concat(fake.length?[('另有 '+fake.length+' 个手绘进度条无 role=progressbar')]:[])};});
  reg('Skeleton',()=>({n:$ap('skeleton').length,bad:[]}));
  ['Descriptions','Timeline','Tree','Carousel','File','Quote','Video','Chat UI','Filter','Sort','App Icon','Collapse'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));
  reg('Tree',()=>{const es=[...document.querySelectorAll('[role=tree],[role=treeitem]')];
    return {n:es.length,bad:es.filter(e=>e.getAttribute('role')==='treeitem'&&!e.closest('[role=tree]')).slice(0,3).map(()=>'treeitem 不在 tree 内')};});
  reg('Collapse',()=>{const es=[...document.querySelectorAll('[aria-expanded]')].filter(e=>vis(e)&&!thirdParty(e));
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,4).map(e=>'可展开控件无可读名字 @'+where()+':'+chain(e))};});
  reg('Filter',()=>({n:[...document.querySelectorAll('[aria-pressed],[role=combobox]')].length,bad:[]}));
  reg('Sort',()=>({n:document.querySelectorAll('[aria-sort]').length,bad:[...document.querySelectorAll('th')].filter(t=>/排序|sort/i.test(t.getAttribute('title')||'')||t.querySelector('button')).slice(0,3).map(()=>'可排序列缺 aria-sort')}));
  reg('Chat UI',()=>({n:document.querySelectorAll('[class*=chat],[class*=Chat]').length>0?1:0,bad:[]}));

  // 5. Dialogs & Feedback
  reg('Modal',()=>{const es=$ap('modal').length?$ap('modal'):[...document.querySelectorAll('[role=dialog],[aria-modal=true]')].filter(vis);
    // 浮层判定要排掉两类正当的 fixed 层：提示条/进度条/搜索面板自己就不是对话框，
    // 而且必须真的装了一块表单或按钮区才算「本该是 dialog 却没标」。
    const fake=[...document.querySelectorAll('div')].filter(e=>{const c=getComputedStyle(e);
      if(!vis(e)||c.position!=='fixed'||parseFloat(c.zIndex)<1000) return false;
      if(e.closest('[role=dialog],[role=alertdialog],[data-ap=toast],[data-ap=spotlight-search],[role=progressbar],.apple-skip-link')) return false;
      // 真对话框自己的遮罩层也是 fixed 大屏幕：它包住 [role=dialog]，不是「该标没标」的浮层
      if(e.querySelector('[role=dialog],[role=alertdialog],[aria-modal=true],[role=search],[data-ap=spotlight-search]')) return false;
      const q=e.getBoundingClientRect();
      return q.width>240&&q.height>120&&e.querySelector('button,input,textarea,[role=option],[role=menuitem]');});
    return {n:es.length,bad:(es.filter(e=>!e.getAttribute('aria-label')&&!e.getAttribute('aria-labelledby')).slice(0,3).map(e=>'dialog 缺可访问名字 @'+where())).concat(fake.length?[('疑似自绘浮层 '+fake.length+' 个未标 role=dialog @'+where()+':'+chain(fake[0]))]:[])};});
  reg('Drawer',()=>({n:$ap('drawer').length,bad:[]}));
  reg('Toast',()=>{const es=$ap('toast');
    return {n:es.length,bad:es.filter(e=>!e.textContent.trim()).slice(0,3).map(()=>'toast 容器无文字')};});
  reg('Tooltip',()=>{const roles=$ap('tooltip').length||document.querySelectorAll('[role=tooltip]').length;
    // 账要算准：只有「交互元素挂着原生 title、又没有 aria-label」才是真会丢信息的那种；
    // 表格单元格为了截断兜底挂 title 是正当用法。也别说「全站无 role=tooltip 实现」——
    // ApTooltip 与 ApPopover 的面板都是 role=tooltip。
    const bare=[...document.querySelectorAll('button[title],a[title],[role=button][title]')].filter((e)=>!e.getAttribute('aria-label'));
    return {n:roles,bad:bare.length?[bare.length+' 个图标按钮只有原生 title、没有 aria-label：title 不吃键盘/触屏 @'+where()+':'+chain(bare[0],4)]:[]};});
  reg('Alert',()=>({n:[...document.querySelectorAll('[role=alert]')].filter(vis).length,bad:[]}));
  ['Notification','Popconfirm','Popover','Result','Spinner'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));
  reg('Popover',()=>({n:document.querySelectorAll('[role=dialog][aria-modal], [role=menu], [role=listbox]').length,bad:[]}));
  reg('Spinner',()=>({n:[...document.querySelectorAll('*')].filter(e=>{const c=getComputedStyle(e);return /spin|loader|loading/i.test(c.animationName||'')||/loader|spinner/i.test(String(e.className));}).length,bad:[]}));

  // 6. Navigation
  reg('Breadcrumb',()=>{const es=[...document.querySelectorAll('nav[aria-label]')].filter(n=>/面包屑|breadcrumb/i.test(n.getAttribute('aria-label')));
    return {n:es.length,bad:[...document.querySelectorAll('span,div')].filter(e=>e.children.length===0&&e.textContent.trim()==='›'&&vis(e)&&!e.closest('nav')).slice(0,3).map(()=>'手搓 › 分隔符未包在 nav 内')};});
  reg('Pagination',()=>{const es=$ap('pagination');
    return {n:es.length,bad:es.filter(e=>!e.getAttribute('aria-label')).slice(0,3).map(()=>'分页容器缺 aria-label')};});
  reg('Search',()=>({n:$ap('search').length,bad:[]}));
  reg('Menu',()=>({n:document.querySelectorAll('[role=menu],[role=menubar]').length,bad:[...document.querySelectorAll('[role=menuitem]')].filter(e=>!e.closest('[role=menu]')).slice(0,3).map(()=>'menuitem 不在 menu 内')}));
  reg('Dropdown',()=>({n:document.querySelectorAll('[aria-haspopup]').length,bad:[...document.querySelectorAll('[aria-haspopup]')].filter(e=>!e.getAttribute('aria-expanded')).slice(0,4).map(()=>'haspopup 控件缺 aria-expanded')}));
  reg('Skip Link',()=>({n:[...document.querySelectorAll('a[href^="#"]')].filter(a=>/跳到|skip/i.test(a.textContent)).length,bad:[]}));
  ['Anchor','Steps','Back to Top Button'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));
  reg('Back to Top Button',()=>({n:[...document.querySelectorAll('button')].filter(b=>{const c=getComputedStyle(b);return vis(b)&&c.position==='fixed'&&parseFloat(c.bottom)<80&&/回到顶部|返回顶部|置顶/i.test((b.getAttribute('aria-label')||'')+(b.textContent||''));}).length,bad:[]}));

  // 7/8. Sections & Layouts（地标元素）
  reg('Header',()=>({n:document.querySelectorAll('header').length,bad:[]}));
  reg('Navbar',()=>({n:document.querySelectorAll('header nav,nav[aria-label]').length,bad:[]}));
  reg('Footer',()=>({n:document.querySelectorAll('footer').length,bad:[]}));
  reg('Sidebar',()=>({n:$ap('sidebar').length,bad:[]}));
  reg('Main',()=>({n:document.querySelectorAll('main').length,bad:[]}));
  ['Hero','CTA','Customer Testimonial','FAQ','Pricing','Social Proof','Logo','Top Nav','Single Page','Doc Layout','Card Grid','Centered Column','Split-screen','Masonry','Responsive Design'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));

  // 9. CSS Layout
  reg('Z-Index',()=>{const known=new Set(${JSON.stringify(ZINDEX_TOKENS)});
    const es=[...document.querySelectorAll('*')].filter(e=>{const z=getComputedStyle(e).zIndex;return z!=='auto'&&!known.has(z)&&vis(e)&&!thirdParty(e);});
    return {n:es.length,bad:es.slice(0,6).map(e=>'裸 z-index '+getComputedStyle(e).zIndex+' @'+where()+':'+chain(e))};});
  reg('Sticky',()=>({n:[...document.querySelectorAll('*')].filter(e=>getComputedStyle(e).position==='sticky'&&vis(e)).length,bad:[]}));
  reg('Overflow',()=>{const all=[...document.querySelectorAll('button,a,[role=button]')].filter(vis);
    const clipped=all.filter(e=>{const c=getComputedStyle(e);return c.overflow==='hidden'&&e.scrollWidth>Math.ceil(e.getBoundingClientRect().width)+2;});
    // n 必须是「查了多少个」而不是「违例几个」，否则零违例会被分类成「未出现」
    return {n:all.length,bad:clipped.slice(0,6).map(e=>'操作被裁:'+txt(e))};});
  reg('Box Model',()=>{const es=[...document.querySelectorAll('button,input,[role=combobox]')].filter(vis);
    const bad=es.filter(e=>{const b=getComputedStyle(e).borderTopWidth;const h=e.getBoundingClientRect().height;const cb=parseFloat(b)||0;return cb>0&&Math.abs(h-parseFloat(e.style.height||0))>1.5&&parseFloat(e.style.height)>0;});
    return {n:es.length,bad:bad.slice(0,4).map(e=>'border 加在 height 之外(缺 border-box) h'+Math.round(e.getBoundingClientRect().height))};});
  ['Space','Margin','Padding','Flex','Grid','Position','Centering'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));
  reg('Grid',()=>({n:[...document.querySelectorAll('*')].filter(e=>getComputedStyle(e).display==='grid'&&vis(e)).length,bad:[]}));
  reg('Flex',()=>({n:[...document.querySelectorAll('*')].filter(e=>getComputedStyle(e).display==='flex'&&vis(e)).length,bad:[]}));

  // 10. Typography
  reg('Text Truncate',()=>{const es=[...document.querySelectorAll('*')].filter(e=>{const c=getComputedStyle(e);return vis(e)&&c.textOverflow==='ellipsis'&&e.scrollWidth>Math.ceil(e.getBoundingClientRect().width)+2;});
    return {n:es.length,bad:es.filter(e=>!e.getAttribute('title')&&!e.getAttribute('aria-label')).slice(0,6).map(e=>'截断但无 title 兜底:'+txt(e)+' @'+where()+':'+chain(e,6))};});
  reg('Typography',()=>{const hs=[...document.querySelectorAll('h1')];
    return {n:hs.length,bad:hs.length!==1?[('H1 数量='+hs.length+'（应恰好 1 个）@'+where()+' 末级标题='+document.querySelectorAll('h2,h3').length)]:[]};});
  reg('Serif & Sans',()=>({n:-2,bad:[]}));

  // 11. Visual Styling
  reg('Divider',()=>({n:document.querySelectorAll('hr').length+[...document.querySelectorAll('div,section')].filter(e=>{const c=getComputedStyle(e);return vis(e)&&parseFloat(c.height)<=2&&parseFloat(c.width)>40&&c.borderTopWidth!=='0px';}).length,bad:[]}));
  // 圆角是否落在令牌档位上。白名单来自 AppleRadius + 各主题 --apple-radius-* 实测值，
  // 所以 pill(980px)/large(16px)/xl(20px) 不再被误报成漂移。
  reg('Border Radius',()=>{const OK=new Set(${JSON.stringify(RADIUS_TOKENS)});
    const vals=new Map();
    [...document.querySelectorAll('*')].filter(e=>vis(e)&&!thirdParty(e)).forEach(e=>{const r=getComputedStyle(e).borderTopLeftRadius;if(!r||OK.has(r))return;if(r.endsWith('%'))return;vals.set(r,(vals.get(r)||0)+1);});
    return {n:vals.size,bad:[...vals.entries()].sort((a,b)=>b[1]-a[1]).slice(0,6).map(([v,c])=>'非语义圆角 '+v+' x'+c)};});
  reg('Design Token',()=>{const hex=[...document.querySelectorAll('*')].filter(e=>{if(!vis(e))return false;const s=e.getAttribute('style')||'';return /#[0-9a-f]{3,8}\\b/i.test(s);});
    return {n:hex.length,bad:hex.slice(0,4).map(e=>'内联样式写死 hex')};});
  reg('Dark Mode',()=>{const bodyInk=getComputedStyle(document.body).color;
    // 两条收紧：① 空文本的装饰 span（复选框方块、开关把手）颜色无从谈起；
    // ② 只在「与当前主题正文色不同」时才算写死 —— 亮色主题下正文本来就是近黑。
    const ink=[...document.querySelectorAll('h1,h2,h3,p,span,button')].filter(e=>vis(e)&&!thirdParty(e)
      &&(e.textContent||'').trim()&&/rgb\\(0, 0, 0\\)/.test(getComputedStyle(e).color)&&getComputedStyle(e).color!==bodyInk);
    return {n:0,bad:ink.length?[('纯黑文字 '+ink.length+' 处 @'+where()+' 例:'+chain(ink[0])+' 「'+(ink[0].textContent||'').trim().slice(0,8)+'」')]:[]};});
  ['Shadow','Opacity','Gradient','Corner Feel','Backdrop Blur','Contrast','Visual Hierarchy'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));

  // 12. Motion
  reg('Transition',()=>({n:[...document.querySelectorAll('*')].filter(e=>{const c=getComputedStyle(e);return vis(e)&&c.transitionProperty!=='all'&&c.transitionProperty!=='none'&&parseFloat(c.transitionDuration)>0;}).length,bad:[]}));
  reg('Animation',()=>({n:[...document.querySelectorAll('*')].filter(e=>{const c=getComputedStyle(e);return vis(e)&&c.animationName!=='none'&&parseFloat(c.animationDuration)>0;}).length,bad:[]}));
  ['Easing','Spring','Fade In / Out'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));

  // 13. Pointer
  reg('Cursor',()=>{const click=[...document.querySelectorAll('button,a[role],input,select,[role=button],[role=tab],[role=combobox],[role=checkbox],[role=menuitem],[role=treeitem]')].filter(e=>vis(e)&&!thirdParty(e));
    return {n:click.length,bad:click.filter(e=>getComputedStyle(e).cursor==='default'||getComputedStyle(e).cursor==='auto').slice(0,6).map(e=>'可点击但光标不是手型:'+e.tagName)};});
  reg('Disabled',()=>{const es=[...document.querySelectorAll('[disabled],[aria-disabled=true]')].filter(e=>vis(e)&&!thirdParty(e));
    return {n:es.length,bad:es.filter(e=>getComputedStyle(e).cursor!=='not-allowed'&&parseFloat(getComputedStyle(e).opacity)>0.7).slice(0,4).map(e=>'禁用态无视觉区分:'+e.tagName+' @'+where()+':'+chain(e)+' 文字='+txt(e))};});
  reg('Focus',()=>({n:-2,bad:[]}));
  reg('Hover',()=>({n:-2,bad:[]}));
  reg('Active',()=>({n:-2,bad:[]}));
  reg('Selection',()=>({n:-2,bad:[]}));
  reg('Drag',()=>({n:document.querySelectorAll('[draggable=true]').length,bad:[]}));

  // 1. Web Basics
  reg('Title Tag',()=>({n:1,bad:document.title&&document.title!=='undefined'?[]:['document.title 为空或 undefined']}));
  reg('Accessibility',()=>{const imgNoAlt=[...document.querySelectorAll('img')].filter(i=>!i.getAttribute('alt')).length;
    const btnNoName=[...document.querySelectorAll('button')].filter(b=>vis(b)&&!nameOf(b)).length;
    const lang=document.documentElement.getAttribute('lang');
    const noMain=!document.querySelector('main');
    return {n:0,bad:[imgNoAlt?('img 缺 alt x'+imgNoAlt+' @'+where()):'',btnNoName?('按钮无名 x'+btnNoName+' @'+where()):'',lang?'':'html 缺 lang @'+where(),noMain?('缺 main 地标 @'+where()+' 字数='+((document.body.innerText||'').replace(/\\s+/g,'').length)+' html长='+document.documentElement.outerHTML.length+' 正文节点='+document.body.querySelectorAll('*').length):''].filter(Boolean)};});
  reg('Favicon',()=>({n:document.querySelectorAll('link[rel*=icon]').length,bad:[]}));
  reg('Page Metadata',()=>({n:document.querySelectorAll('meta[name=description],meta[property="og:description"]').length,bad:[]}));
  reg('Open Graph',()=>({n:document.querySelectorAll('meta[property^="og:"]').length,bad:[]}));
  reg('Web App Manifest',()=>({n:document.querySelectorAll('link[rel=manifest]').length,bad:[]}));
  ['Frontend','Component','State','Markdown','HTML','CSS','DOM','Undo'].forEach(k=>reg(k,()=>({n:-2,bad:[]})));
  reg('Markdown',()=>({n:document.querySelectorAll('[data-md-preview],.md-preview').length,bad:[]}));

  // ── B 类补测：覆盖上面的占位探针。约定：这些术语的组件是真实存在的，
  // 只是要么藏在交互后面（由 SCENARIOS 点开），要么藏在加载窗口里（由 throttle 逼出）。
  // 组件确实不存在的（Progress/Result/Steps/Back to Top）不给探针，改由
  // docs/glossary/applicability.json 声明「产品未实现」，绝不用 0 冒充「一致」。
  reg('Skeleton',()=>({n:[...document.querySelectorAll('[data-ap=skeleton],[role=status][aria-busy=true]')].filter(vis).length,bad:[]}));
  reg('Descriptions',()=>{const ls=$ap('description-list');
    const items=ls.flatMap(l=>[...l.children]).filter(vis);
    return {n:items.length,bad:items.length?['描述列表整棵是 div，未用 dl/dt/dd 语义']:[]};});
  reg('Timeline',()=>{const es=$ap('timeline');
    // 只认组件本体。原先按「有左边框的行」猜时间线，统计卡与列表行的描边全被数进来
    // （报过 234 条），而真正的缺陷（各页手绘）早就收口进 ApTimeline 了。
    const rows=es.reduce((s,t)=>s+[...t.children].filter(vis).length,0);
    const unlabeled=es.filter(t=>!t.getAttribute('aria-label')).length;
    return {n:rows,bad:unlabeled?[('时间线列表缺 aria-label x'+unlabeled+' @'+where())]:[]};});
  reg('Popconfirm',()=>{const ds=[...document.querySelectorAll('[data-ap=modal]')].filter(d=>/取消/.test(d.textContent||'')&&/(确认|删除|确定)/.test(d.textContent||''));
    return {n:ds.length,bad:ds.filter(d=>d.getAttribute('role')!=='alertdialog').slice(0,2).map(()=>'确认框未标 role=alertdialog')};});
  reg('Autocomplete',()=>{const combo=[...document.querySelectorAll('[role=combobox],input[aria-autocomplete]')].filter(vis);
    const lb=[...document.querySelectorAll('[role=listbox]')].filter(vis);
    const opts=[...document.querySelectorAll('[role=option]')].filter(vis);
    // 合上的下拉本来就没有 listbox 在 DOM 里，那是正常状态；真正的缺陷是
    // aria-controls 指向一个不存在的 id（下拉组件的 id 用 useId() 塞进 computed，
    // 每次求值都会变，正是会踩这一条的写法）。
    const broken=combo.filter((c)=>{const id=c.getAttribute('aria-controls');return id&&!document.getElementById(id);});
    return {n:lb.length+opts.length,bad:broken.slice(0,3).map(c=>'aria-controls 指向不存在的 listbox @'+where()+':'+chain(c))};});
  reg('Date Picker',()=>{const es=[...document.querySelectorAll('input[type=date],input[type=month],input[type=datetime-local]')].filter(vis);
    return {n:es.length,bad:es.filter(e=>!nameOf(e)).slice(0,3).map(e=>'日期输入无可读名字 @'+where()+':'+chain(e))};});
  reg('File',()=>{const all=[...document.querySelectorAll('input[type=file]')];
    // display:none 的隐藏 input + 带名字的按钮才是这个库的上传写法，那种不算缺陷；
    // 真的缺陷是「看得见、又没有名字」的文件输入框。
    const shown=all.filter(e=>{const r=e.getBoundingClientRect();return r.width>0&&r.height>0;});
    return {n:all.length,bad:shown.filter(e=>!e.getAttribute('aria-label')&&!nameOf(e)).slice(0,3).map(e=>'可见文件输入无可读名字 @'+where()+':'+chain(e))};});
  reg('Chat UI',()=>{if(!/\\/ai\\/workspace/.test(location.pathname)){return {n:0,bad:[]};}
    const log=[...document.querySelectorAll('[role=log]')].filter(vis);
    const stream=[...document.querySelectorAll('textarea,[contenteditable=true]')].filter(vis);
    return {n:log.length+stream.length,bad:log.length?[]:['对话流未标 role=log，流式增量不会被朗读']};});
  reg('Anchor',()=>{const links=[...document.querySelectorAll('a[href^="#"]')].filter(vis);
    const panel=[...document.querySelectorAll('div')].filter(d=>{const c=getComputedStyle(d);return vis(d)&&c.position==='fixed'&&/^目录/.test((d.textContent||'').trim());}).sort((a,b)=>a.textContent.length-b.textContent.length)[0];
    const items=panel?[...panel.querySelectorAll('div')].filter(d=>vis(d)&&!d.children.length&&(d.textContent||'').trim()):[];
    return {n:links.length+items.length,bad:items.length?['目录条目是 div 不是 a[href^="#"]：不能中键新开、不能复制链接、不进 Tab 序']:[]};});
  reg('Notification',()=>{const es=[...document.querySelectorAll('[data-ap=toast] [role=alert],[data-ap=toast] > div')].filter(vis);
    return {n:es.length,bad:es.filter(e=>!e.textContent.trim()).slice(0,2).map(()=>'通知浮层无文字')};});
  reg('Toast',()=>{const box=$ap('toast')[0];
    const items=box?[...box.children].filter(vis):[];
    return {n:items.length,bad:items.filter(e=>!e.getAttribute('role')).slice(0,2).map(()=>'toast 条目未带 role，靠 aria-live 容器兜底')};});
  reg('Result',()=>({n:[...document.querySelectorAll('[data-ap=error],[data-ap=error-page]')].filter(vis).length,bad:[]}));
  return R;
})()`;

// 适用性声明：项目侧判断，不写进通用术语表
const APPLICABILITY = (() => {
  const p = path.join(ROOT, 'docs', 'glossary', 'applicability.json');
  if (!existsSync(p)) return {};
  try {
    const j = JSON.parse(readFileSync(p, 'utf8'));
    delete j._comment; delete j._verdicts;
    return j;
  } catch { return {}; }
})();
const VERDICT_LABEL = {
  'na-product-form': '不适用·产品形态',
  'pending-interaction': '待交互·B类工装',
  'not-implemented': '产品未实现·无对应组件',
  'manual-only': '需人工验收·避免破坏性点击',
  'data-dependent': '依赖数据·当前无该类内容',
  'covered-by-gate': '已由静态门禁覆盖',
  'not-dom-observable': '非 DOM 可观测',
  'proxy-only': '仅代理指标',
};

// 不可自动化判定的术语（占位探针；有适用性声明的优先按声明走）。
// 曾经在这里的 Descriptions / Timeline / File / Notification / Result / Anchor / Autocomplete
// 已经换成真探针，别再留在这里把实测结果压成「不可判定」。
const NO_PROBE = new Set([
  'Frontend', 'Component', 'State', 'HTML', 'CSS', 'DOM', 'Undo',
  'Masonry', 'Visual Hierarchy', 'Corner Feel',
  'Hero', 'CTA', 'Customer Testimonial', 'FAQ', 'Pricing', 'Social Proof',
  'Rate', 'Color Picker', 'Cascader', 'Tree Select', 'Time Picker',
  'Carousel', 'Quote', 'Video',
  'Popconfirm', 'Steps',
  'Top Nav', 'Single Page', 'Doc Layout', 'Split-screen',
  'Space', 'Margin', 'Padding', 'Position', 'Centering',
  'Serif & Sans', 'Shadow', 'Opacity', 'Gradient', 'Backdrop Blur',
  'Easing', 'Spring', 'Fade In / Out', 'Hover', 'Active', 'Selection',
  'Logo', 'App Icon', 'Number Input',
]);

const parseTerms = () => {
  const md = readFileSync(GLOSSARY, 'utf8');
  const out = [];
  let section = '';
  for (const line of md.split('\n')) {
    const s = /^## \d+\.\s*(.+)$/.exec(line);
    if (s) { section = s[1].trim(); continue; }
    const m = /^\|\s*\[([^\]]+)\]\([^)]*\)\s*\|/.exec(line);
    if (m) out.push({ term: m[1].trim(), section });
  }
  return out;
};

function findChromium() {
  const root = path.join(process.env.USERPROFILE || process.env.HOME || '', 'AppData/Local/ms-playwright');
  if (!existsSync(root)) return undefined;
  const cands = [];
  for (const dir of readdirSync(root)) {
    const m = /^chromium(_headless_shell)?-(\d+)$/.exec(dir);
    if (!m) continue;
    const exe = m[1]
      ? path.join(root, dir, 'chrome-headless-shell-win64', 'chrome-headless-shell.exe')
      : path.join(root, dir, 'chrome-win64', 'chrome.exe');
    if (existsSync(exe)) cands.push([Number(m[2]), exe]);
  }
  cands.sort((a, b) => b[0] - a[0]);
  return cands[0] ? cands[0][1] : undefined;
}


// 场景步骤：这些术语的组件是存在的，但要先交互才渲染。静态巡检把它们误判成「未出现」。
// throttle 在 goto 之前挂路由拦截，用慢网逼出骨架屏 / spinner / 顶部进度条。
// 选择器全部按源码核对过：/notifications 的按钮文案是「全部标记已读」不是「全部已读」，
// /links 的主按钮原先是「添加链接」（2026-10-01 随全站「新建 X」口径改成「新建链接」，
// 探针跟着改），/settings 是「保存此项」，侧栏菜单触发器
// 是 [aria-haspopup=menu]（「更多操作」只是里面那个图标的 aria-label）。
// 刻意不点删除/确认类按钮：ApConfirmModal 只出现在删除文档与回收站彻底删除两处，
// 让巡检机器人去点等于拿用户数据冒险，这两条改走人工验收。
// 同理去掉三条曾经每次都 SKIP 的步骤：/settings 的「保存此项」要先改脏某项配置才出现
// （等于让机器人写系统配置），/notifications 的「全部标记已读」是单向操作（跑一次就再
// 也没有未读），/tags 的「详情」依赖标签统计数据。它们的共同点是「点了就改数据」，
// 该走的不是巡检，是人工验收。
const SCENARIOS = {
  '/': [{ throttle: 1400 }, { hotkey: 'Control+k' }, { type: '文档' }, { wait: 700 }],
  '/users': [{ click: 'button:has-text("新建用户")' }, { wait: 500 }],
  '/collections': [{ throttle: 1400 }, { click: '[data-ap=button]' }, { wait: 500 }],
  '/profile': [{ click: '[data-ap=segmented-control] button >> nth=1' }],
  '/logs': [{ click: 'input[type=date]' }],
  '/dashboard': [{ scroll: 'bottom' }],
  '/space/1/doc/1': [{ scroll: 'bottom' }, { wait: 400 }, { click: 'button[aria-label="目录"]' }, { wait: 600 }],
  // 编辑器工具栏的下拉：ApPopover 的面板就是 role=tooltip，点开才在 DOM 里
  '/space/1/doc/1/edit': [{ click: '[data-ap=popover] [aria-haspopup]' }, { wait: 500 }, { hover: 'button[aria-label^="撤销"]', settle: 500 }],
  '/doc-collab': [{ click: '[data-ap=segmented-control] button >> nth=1' }],
  '/tags': [{ click: 'button:has-text("详情")' }, { wait: 500 }],
  // 侧栏「更多」菜单：role=menu 只有点开才存在；点完不再点第二下，避免误触退出登录
  '/spaces': [{ click: '[aria-haspopup="menu"]' }, { wait: 500 }],
  '/search': [{ click: '[data-ap=search] input' }, { type: '文档' }, { wait: 600 }],
  '/links': [{ click: 'button:has-text("新建链接")' }, { wait: 500 }],
};

// 移动端补测：抽屉式侧栏是 v-if="isMobile"，桌面视口下永远不存在，
// 所以 Drawer / 触屏可达性 / Responsive 只能在窄视口里量。
const MOBILE_ROUTES = ['/', '/space/1/doc/1', '/collections', '/dashboard'];

async function runSteps(page, steps) {
  const done = [];
  let transitions = 0;
  for (const s of steps || []) {
    try {
      if (s.hotkey) { await page.keyboard.press(s.hotkey); done.push('key ' + s.hotkey); }
      else if (s.type) { await page.keyboard.type(s.type, { delay: 40 }); done.push('type ' + s.type); }
      else if (s.hover) {
        // 气泡只在悬停/聚焦时存在；不先悬停，Tooltip 术语永远量不到 role=tooltip。
        // 悬停后鼠标停在该元素上，气泡会一直留到探针读取为止（离开才收）。
        await page.locator(s.hover).first().hover({ timeout: 2500 });
        await page.waitForTimeout(s.settle ?? 450);
        const tips = await page.evaluate(() => document.querySelectorAll('[role=tooltip]').length);
        done.push(tips ? `hover ${s.hover.slice(0, 22)} → ${tips} 个 role=tooltip` : `hover ${s.hover.slice(0, 22)} 未出气泡`);
      }
      else if (s.click) { await page.locator(s.click).first().click({ timeout: 2500 }); done.push('click ' + s.click.slice(0, 28)); }
      else if (s.scroll === 'bottom') { await page.evaluate(() => { const el = document.querySelector('[data-ap-scroll]') || document.scrollingElement; el.scrollTop = el.scrollHeight; }); done.push('scroll bottom'); }
      // Vue <Transition> 的类名只在动画那几百毫秒里挂在节点上，错过就再也量不到，
      // 所以每步之后立刻抽样一次，把命中的过渡计数累加给 Fade In / Out。
      if (s.click || s.hotkey) {
        const hit = await page.evaluate(() => document.querySelectorAll(
          '[class*="-enter-active"],[class*="-leave-active"],[class*="-enter-to"],[class*="-leave-from"]',
        ).length).catch(() => 0);
        transitions += hit;
      }
      if (s.wait) await page.waitForTimeout(s.wait);
    } catch { done.push('SKIP ' + (s.click || s.hotkey || s.type || 'scroll')); }
  }
  return { done, transitions };
}

// ── 2) 伪类与媒体状态（C 类）：真键盘 Tab + hover 隐藏检测 + 减弱动效 ─────
async function auditPseudo(page) {
  const out = {};

  // Focus：只有真实键盘 Tab 才会触发 :focus-visible，programmatic focus() 不算
  const focusBad = [];
  let focusN = 0;
  for (let i = 0; i < 8; i++) {
    await page.keyboard.press('Tab');
    // 焦点环很多是 transition 出来的（ApCard 的 box-shadow 走 0.2s），
    // 只等 60ms 会把「正在淡入的环」读成 none，于是报出假缺陷。
    await page.waitForTimeout(180);
    const r = await page.evaluate(() => {
      const e = document.activeElement;
      if (!e || e === document.body) return null;
      // 焦点环常常画在包装层上（输入框外壳、分段控件容器），只看被聚焦元素本身
      // 会把「外壳亮环」的正确实现误判成没有指示。往上找三层。
      let ring = false; let host = e;
      for (let i = 0; i < 4 && host; i++) {
        const c = getComputedStyle(host);
        if ((c.boxShadow && c.boxShadow !== 'none') || (c.outlineStyle !== 'none' && parseFloat(c.outlineWidth) > 0)) { ring = true; break; }
        host = host.parentElement;
      }
      return { tag: e.tagName, name: (e.textContent || e.getAttribute('aria-label') || e.getAttribute('placeholder') || '').trim().slice(0, 8), ring, chain: (() => { const c = []; let x = e; for (let d = 0; d < 3 && x; d++) { c.push(x.tagName.toLowerCase() + (x.getAttribute('data-ap') ? '[' + x.getAttribute('data-ap') + ']' : '')); x = x.parentElement; } return c.join('>'); })() };
    });
    if (!r) continue;
    focusN++;
    if (!r.ring) focusBad.push(`聚焦无可见指示 ${r.tag}:${r.name} @${new URL(page.url()).pathname} 链=${r.chain}`);
  }
  out.Focus = { n: focusN, bad: focusBad.slice(0, 5) };

  // Hover：opacity 归零的操作，键盘聚焦后仍不可见 = 触屏与键盘都够不着
  out.Hover = await page.evaluate(() => {
    const allBtns = [...document.querySelectorAll('button,[role=button]')].filter((e) => {
      const r = e.getBoundingClientRect(); return r.width > 0 && r.height > 0;
    });
    const es = allBtns.filter((e) => parseFloat(getComputedStyle(e).opacity) < 0.05);
    const bad = [];
    for (const e of es) {
      e.focus();
      if (parseFloat(getComputedStyle(e).opacity) < 0.05) bad.push('hover-only 操作键盘不可达');
      e.blur();
    }
    // n = 查了多少个可见按钮；bad = 其中「靠 hover 才出现、聚焦后仍不可见」的
    return { n: allBtns.length, bad: [...new Set(bad)] };
  });

  // Active：按下态是否与静置态可区分
  out.Active = await page.evaluate(() => {
    const es = [...document.querySelectorAll('button')].filter((e) => { const r = e.getBoundingClientRect(); return r.width > 20 && r.height > 12; }).slice(0, 6);
    const bad = [];
    for (const e of es) {
      const c = getComputedStyle(e);
      if (c.transitionProperty === 'none' && c.transitionDuration === '0s') bad.push('按钮无按下/悬停过渡:' + (e.textContent || '').trim().slice(0, 6));
    }
    return { n: es.length, bad: [...new Set(bad)].slice(0, 4) };
  });

  // Easing：取值是否落在令牌集内。白名单从 apple-tokens.ts 现取现用。
  // 括号必须转义：/cubic-bezier([^)]*)/ 里那对圆括号是「捕获组」而不是字面量，
  // 匹配结果会少掉结尾的 )，于是 AppleTransition.standard 自己被当成「非令牌 easing」
  // 报了 55 类——整条 Easing 差异是工装造出来的，不是页面的。
  out.Easing = await page.evaluate((tokens) => {
    const TOKENS = new Set(tokens);
    const seen = new Map();
    for (const e of document.querySelectorAll('*')) {
      const c = getComputedStyle(e);
      if (!c.transitionDuration || c.transitionDuration === '0s') continue;
      for (const f of (c.transitionTimingFunction || '').match(/cubic-bezier\([^)]*\)|steps\([^)]*\)|[a-z-]+/g) || []) {
        if (TOKENS.has(f)) continue;
        seen.set(f, (seen.get(f) || 0) + 1);
      }
    }
    return { n: seen.size, bad: [...seen.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5).map(([v, c]) => '非令牌 easing ' + v + ' x' + c) };
  }, EASING_TOKENS);

  // Reduced motion：模拟 prefers-reduced-motion: reduce 后动画时长应归零
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.waitForTimeout(200);
  out.Animation = await page.evaluate(() => {
    const live = [...document.querySelectorAll('*')].filter((e) => {
      const c = getComputedStyle(e);
      return c.animationName !== 'none' && parseFloat(c.animationDuration) > 0.05 && e.getBoundingClientRect().width > 0;
    });
    // n = 页面上正在跑动画的元素数；bad = 模拟 reduced-motion 后仍不归零的
    return { n: live.length, bad: live.length ? [live.length + ' 个动画在 reduced-motion 下仍在跑'] : [] };
  });
  await page.emulateMedia({ reducedMotion: null });

  return out;
}


// 页面等到「真的挂载了 + 文字不再增长」为止，而不是固定 sleep。
// 期望路径必须由调用方给出：拿 page.url() 当期望，遇上上一页 beforeunload 把 goto 吞掉时，
// 旧地址会自我确认成功，量完的其实是上一个页面。
async function settlePage(page, wantPath, cap = 6000) {
  const deadline = Date.now() + cap;
  let prev = -1;
  let stable = 0;
  let last = { path: '', len: 0 };
  while (Date.now() < deadline) {
    const st = await page.evaluate(() => ({
      len: (document.body.innerText || '').replace(/\s+/g, '').length,
      path: location.pathname,
      ready: document.readyState,
      mounted: !!document.querySelector('#app-root > *'),
      shell: !!document.querySelector('main,header,form,[data-ap]'),
    })).catch(() => ({ len: 0, path: '', ready: 'loading', mounted: false, shell: false }));
    last = { path: st.path, len: st.len };
    if (wantPath && st.path !== wantPath) { prev = -1; stable = 0; await page.waitForTimeout(200); continue; }
    if (st.ready !== 'complete' || !st.mounted || !st.shell) { prev = -1; stable = 0; await page.waitForTimeout(200); continue; }
    if (st.len === prev && st.len > 0) stable += 1; else stable = 0;
    prev = st.len;
    if (stable >= 2) return { ok: true, path: st.path, len: st.len };
    await page.waitForTimeout(250);
  }
  return { ok: false, path: last.path, len: last.len };
}

const run = async () => {
  // 探针是一整段字符串源码，浏览器一次性编译：一个括号不配对，75 页全部
  // 报「导航失败」，看起来像整站崩了，实际是工装自己写错。先本地编译一遍。
  try {
    new Function(P);
  } catch (e) {
    console.error('探针源码语法错误，整轮巡检会全 0，先修这里：', String(e).slice(0, 200));
    process.exit(2);
  }
  const terms = parseTerms();

  // kinds = 去掉 @路径 之后的差异种类；bad = 带出处的样本。
  // 少了这一层，「样本带路径」会让同一个缺陷按页数炸成几十「类」，头版数字反而更不可读。
  const kindOf = (m) => m.split(' @')[0];
  const agg = new Map(terms.map((t) => [t.term, { inst: 0, pages: 0, bad: new Set(), kinds: new Set(), err: false, noprobe: false }]));
  const browser = await chromium.launch({ executablePath: findChromium() });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  const page = await ctx.newPage();
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));

  await loginForm(page, BASE, USER, PASS);
  console.log(`login: ${page.url().includes('/login') ? 'FAILED' : 'ok'}`);

  const routes = argv.routes ? ROUTES.slice(0, Number(argv.routes)) : ROUTES;
  // 场景步骤的执行结果必须落到报告里：上一版 B 类「全 0」我分不清是选择器没点到
  // 还是判定被声明盖住，只能瞎猜。记下来才能定位。
  const stepResults = {};
  // 逐页无间隔会把后端打到限流（429），守卫随即把人踢回 /login。
  // 与 visual-snapshot 同一个教训：限速不是提速，是让每张快照拍到的都是它自己那一页。
  const GAP_MS = Number(argv.gap ?? 700);
  const degraded = [];
  for (const [i, r] of routes.entries()) {
    try {
      const steps = SCENARIOS[r] || [];
      const th = steps.find((s) => s.throttle);
      if (th) {
        await page.route('**/api/v1/**', async (route) => {
          // 导航会把在途请求取消掉，这时再 continue/fallback 就抛
          // "Route is already handled" —— 必须整段包住，否则一次慢网模拟就崩掉整个巡检
          try {
            const rt = route.request().resourceType();
            if (rt === 'xhr' || rt === 'fetch') await new Promise((r2) => setTimeout(r2, th.throttle));
            await route.continue();
          } catch { /* 请求已被取消，忽略 */ }
        });
      }
      await page.goto(BASE + r, { waitUntil: 'domcontentloaded', timeout: 25000 });
      const wantPath = new URL(BASE + r, BASE).pathname;
      if (th) await page.waitForTimeout(300);
      else {
        // goto 可能被上一页的 beforeunload 吞掉（编辑页注册了它）：那时 page.url() 还是旧地址，
        // 量到的就是上一个页面。所以按目标路径等，等不到就重一次 goto，再不行才算降级。
        let st = await settlePage(page, wantPath);
        if (!st.ok) {
          await page.goto(BASE + r, { waitUntil: 'domcontentloaded', timeout: 25000 }).catch(() => {});
          st = await settlePage(page, wantPath);
        }
        if (!st.ok) {
          degraded.push(`${r} → 停在 ${st.path || '未知页'}（重试后仍未落地）`);
          process.stdout.write(`\n  ! ${r} 导航未落地，跳过计票\n`);
          await page.waitForTimeout(GAP_MS);
          continue;
        }
      }
      // 慢网窗口内先抓一次：骨架屏 / spinner 只在这几十毫秒里存在，
      // 等页面 settle 完再探针就永远测不到，只能靠 throttle 把它们逼出来。
      // 动画名按源码核对：ApSkeleton 用 apple-shimmer、ApTable 用 apple-pulse-soft；
      // 老注释里的 skeleton-loading 早已 0 命中（pulse 关键帧 2026-10-03 作为死码删除）。
      let loadState = null;
      if (th) {
        loadState = await page.evaluate(() => {
          const seen = (e) => e.getBoundingClientRect().width > 0;
          return {
            sk: [...document.querySelectorAll('[data-ap=skeleton],[role=status][aria-busy=true]')].filter(seen).length,
            anim: [...document.querySelectorAll('*')].filter((e) => {
              const c = getComputedStyle(e);
              return /skeleton|pulse|shimmer|spin|loader|loading/i.test(c.animationName || '') && seen(e);
            }).length,
            bar: [...document.querySelectorAll('*')].filter((e) => {
              const c = getComputedStyle(e); const r = e.getBoundingClientRect();
              return c.position === 'fixed' && parseFloat(c.top) === 0 && r.height > 0 && r.height <= 4 && r.width > 0;
            }).length,
          };
        });
        await page.waitForTimeout(th.throttle + 900);
        // 慢网那几条也要等真挂载：固定 +900ms 在启动慢的页面上量到的还是空壳
        await settlePage(page, wantPath);
      }
      // 场景步骤：不先交互进入状态，Modal / Drawer / Menu / Spotlight 就永远不在 DOM 里
      const { done: stepLog, transitions } = await runSteps(page, steps.filter((s) => !s.throttle));
      await page.waitForTimeout(700);
      // 闸门必须贴在测量前面：settle 之后再空掉（路由组件在数据回来的那一刻被卸载）
      // 也会照样量成「没有 h1 / 没有 main」，那种样本一个都不该进统计。
      const pre = await page.evaluate(() => ({
        len: (document.body.innerText || '').replace(/\s+/g, '').length,
        path: location.pathname,
        nodes: document.getElementById('app-root') ? document.getElementById('app-root').querySelectorAll('*').length : 0,
      })).catch(() => ({ len: 0, path: '', nodes: 0 }));
      // 空化多半是「应用还没挂载」——上一页的 beforeunload 会把第一次 goto 拖住，
      // 慢网那几条更是把启动推迟到测量之后。所以先救一次，救不回来才算降级。
      const badPre = () => pre.path !== wantPath || pre.len < 8 || pre.nodes < 20;
      let pre2 = pre;
      if (badPre()) {
        await page.goto(BASE + r, { waitUntil: 'domcontentloaded', timeout: 25000 }).catch(() => {});
        await settlePage(page, wantPath);
        await page.waitForTimeout(500);
        pre2 = await page.evaluate(() => ({
          len: (document.body.innerText || '').replace(/\s+/g, '').length,
          path: location.pathname,
          nodes: document.getElementById('app-root') ? document.getElementById('app-root').querySelectorAll('*').length : 0,
        })).catch(() => ({ len: 0, path: '', nodes: 0 }));
      }
      if (pre2.path !== wantPath || pre2.len < 8 || pre2.nodes < 20) {
        degraded.push(`${r} → 测量前正文 ${pre2.len} 字 / 应用节点 ${pre2.nodes}（重试后仍未挂载）`);
        process.stdout.write(`\n  ! ${r} 测量前页面空化，跳过计票\n`);
        await page.waitForTimeout(GAP_MS);
        continue;
      }
      const res = await page.evaluate(P);
      if (transitions > 0) res['Fade In / Out'] = { n: transitions, bad: [] };
      if (loadState) {
        if (loadState.sk > 0) { res.Skeleton = { n: loadState.sk, bad: [] }; }
        if (loadState.anim > 0) { res.Spinner = { n: loadState.anim, bad: [] }; }
        res.Progress = { n: loadState.bar, bad: [] };
      }
      const sc = await auditPseudo(page);
      for (const [k, v] of Object.entries(sc)) res[k] = v;
      await page.unroute('**/api/v1/**').catch(() => {});
      if (stepLog.length) stepResults[r] = stepLog;
      // 被 429 打回 /login 的页要整条剔除：否则把登录页的 DOM 数成了目标页，
      // 所有计数（尤其「缺 h1 / 缺 main」）都会变成限流的影子。
      const landed = new URL(page.url()).pathname;
      if (landed === '/login' && !r.startsWith('/login')) {
        degraded.push(`${r} → 被 429 弹回登录`);
        process.stdout.write(`\r${String(i + 1).padEnd(3)} ${r.padEnd(42)} → 被弹回登录，跳过计票\n`);
        await page.waitForTimeout(GAP_MS);
        continue;
      }

      for (const [term, v] of Object.entries(res)) {
        const a = agg.get(term);
        if (!a) continue;
        if (v.n === -1) { a.err = true; continue; } if (v.n === -2) { a.noprobe = true; continue; }
        a.inst += v.n;
        if (v.n > 0) a.pages += 1;
        for (const b of (v.bad || [])) { a.bad.add(b); a.kinds.add(kindOf(b)); }
      }
      process.stdout.write(`\r${i + 1}/${routes.length} ${r.padEnd(42)}`);
    } catch (e) {
      process.stdout.write(`\n  ! ${r} 导航失败: ${String(e).slice(0, 70)}\n`);
    }
    await page.waitForTimeout(GAP_MS);
  }
  // ── 移动端补测 ─────────────────────────────────────────────────────
  // 抽屉侧栏是 v-if="isMobile"，1440px 视口下它根本不在 DOM 里，桌面那一圈永远
  // 报「未出现」；横向溢出同理只能在窄视口验。所以只在这几个术语上采信移动端结果，
  // 其余术语不合并，避免同一个指标被两条视口各数一遍。
  const MOBILE_ONLY = new Set(['Drawer', 'Menu', 'Modal', 'Skip Link', 'Back to Top Button', 'Fade In / Out']);
  const mobile = { overflow: [], drawerPages: 0 };
  await page.setViewportSize({ width: 430, height: 932 });
  await page.waitForTimeout(700);
  for (const r of MOBILE_ROUTES) {
    try {
      await page.goto(BASE + r, { waitUntil: 'domcontentloaded', timeout: 25000 });
      await settlePage(page, new URL(BASE + r, BASE).pathname);
      await page.waitForTimeout(300);
      let opened = false;
      try { await page.locator('[aria-label="打开菜单"]').first().click({ timeout: 2500 }); opened = true; } catch { /* 该页无汉堡触发器 */ }
      await page.waitForTimeout(opened ? 700 : 200);
      const res = await page.evaluate(P);
      for (const [term, v] of Object.entries(res)) {
        if (!MOBILE_ONLY.has(term) || !v || v.n <= 0) continue;
        const a = agg.get(term);
        if (!a) continue;
        a.noprobe = false; a.err = false;
        a.inst += v.n; a.pages += 1;
        for (const b of (v.bad || [])) a.bad.add(b);
      }
      if (res.Drawer && res.Drawer.n > 0) mobile.drawerPages += 1;
      const geo = await page.evaluate(() => {
        const cw = document.documentElement.clientWidth;
        const chain = (e) => { const p = []; let x = e; for (let i = 0; i < 3 && x; i++) { p.unshift(x.tagName.toLowerCase() + (x.getAttribute && x.getAttribute('data-ap') ? `[${x.getAttribute('data-ap')}]` : '')); x = x.parentElement; } return p.join('>'); };
        const over = [...document.querySelectorAll('body *')].filter((e) => {
          const c = getComputedStyle(e); const q = e.getBoundingClientRect();
          if (c.position === 'fixed' || parseFloat(c.opacity) < 0.05) return false;
          // 处在可横滚容器里的内容不算「页面溢出」：代码块内部长行本来就该在 <pre> 里滚
          for (let x = e.parentElement; x && x !== document.body; x = x.parentElement) {
            const ox = getComputedStyle(x).overflowX;
            if (ox === 'auto' || ox === 'scroll') return false;
          }
          return q.right > cw + 2 && q.left < cw - 8;
        });
        return { cw, n: over.length, sample: over.slice(0, 3).map((e) => `${chain(e)} w${Math.round(e.getBoundingClientRect().width)} left${Math.round(e.getBoundingClientRect().left)}`) };
      });
      if (geo.n > 0) mobile.overflow.push(`${r} 横向溢出 ${geo.n} 个元素(${geo.sample.join('/')})`);
    } catch (e) {
      process.stdout.write(`\n  ! 移动端 ${r} 失败: ${String(e).slice(0, 60)}\n`);
    }
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  if (mobile.overflow.length) process.stdout.write(`\n移动端：${mobile.overflow.length} 个页面横向溢出\n`);

  await browser.close();

  const a0 = agg.get('Responsive Design');
  if (a0) {
    a0.noprobe = false; a0.err = false;
    a0.inst = MOBILE_ROUTES.length; a0.pages = MOBILE_ROUTES.length;
    for (const m of mobile.overflow) a0.bad.add(m);
  }

  const rows = [];
  for (const t of terms) {
    const a = agg.get(t.term);
    const decl = APPLICABILITY[t.term];
    // 实测优先：只要有任何一页真量到东西（或有违例），声明与占位都不能盖住它。
    // 上一版把 noprobe 也当成「未量到」的条件，于是 Fade In / Out 明明在 5 页量到
    // 过渡类名，却仍显示「不可自动化判定」——占位探针每页都返回 -2，一旦置位就永远翻不回来。
    const measured = !!a && !a.err && (a.pages > 0 || a.bad.size > 0);
    const verdict = measured
      ? (a.kinds.size ? `差异 ${a.kinds.size} 类` : '一致')
      : decl ? (VERDICT_LABEL[decl.verdict] || decl.verdict)
        : NO_PROBE.has(t.term) ? '不可自动化判定'
          : !a ? '无探针'
            : a.err ? '探针异常' : a.noprobe ? '不可自动化判定' : '未出现';
    rows.push({ section: t.section, term: t.term, inst: a ? a.inst : 0, pages: a ? a.pages : 0, verdict, samples: a ? [...a.bad].slice(0, 3) : [] });
  }
  writeFileSync(path.join(ROOT, 'docs', 'audits', 'glossary-audit.json'), JSON.stringify(rows, null, 2), 'utf8');

  let cur = '';
  console.log('\n\n术语 | 命中实例 | 命中页 | 判定 | 差异样本');
  console.log('---|---:|---:|---|---');
  for (const r of rows) {
    if (r.section !== cur) { cur = r.section; console.log(`\n## ${cur}`); }
    console.log(`${r.term} | ${r.inst} | ${r.pages} | ${r.verdict} | ${r.samples.join(' ; ').slice(0, 110)}`);
  }
  const skipped = Object.entries(stepResults).flatMap(([r, ls]) => ls.filter((l) => /^SKIP/.test(l)).map((l) => `${r}  ${l}`));
  const ran = Object.entries(stepResults).flatMap(([r, ls]) => ls.filter((l) => !/^SKIP/.test(l)).map((l) => `${r}  ${l}`));
  console.log(`\n## 场景步骤执行结果（B 类）\n成功 ${ran.length} 步 / 失败 ${skipped.length} 步`);
  for (const l of ran.slice(0, 20)) console.log(`  ok   ${l}`);
  for (const l of skipped.slice(0, 20)) console.log(`  SKIP ${l}`);
  console.log(`\n## 移动端补测（430×932，${MOBILE_ROUTES.length} 页）\n抽屉侧栏命中 ${mobile.drawerPages} 页；横向溢出 ${mobile.overflow.length} 页`);
  for (const m of mobile.overflow.slice(0, 8)) console.log(`  ${m}`);

  const byVerdict = {};
  for (const r of rows) { const k = r.verdict.startsWith('差异') ? '有差异' : r.verdict; byVerdict[k] = (byVerdict[k] || 0) + 1; }
  console.log(`\n合计 ${rows.length} 条术语：` + Object.entries(byVerdict).sort((a, b) => b[1] - a[1]).map(([k, v]) => `${k} ${v}`).join('，'));
  if (degraded.length) {
    console.log(`\n## 本轮有 ${degraded.length} 页未计入统计（环境降级，不是应用缺陷判定）`);
    for (const d of degraded) console.log('  ' + d);
    console.log('  → 这轮的「全站」结论不完整，加大 --gap 重跑后再引用。');
  }
};

run().catch((e) => { console.error(e); process.exit(1); });
