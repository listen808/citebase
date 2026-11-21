import { createApp, type Component } from 'vue';
import { createPinia } from 'pinia';
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate';

import App from './App.vue';
import router from './router';
import { useAuthStore } from './stores/auth/auth';
import { useUIStore } from './stores/app/ui';
import { useHistoryStore } from './stores/workspace/history';
import { injectCSSVariables } from './utils/core/apple-tokens';
import { loadSiteBrand } from './utils/core/site-brand';
import { migrateLegacyRawKeys } from './utils/core/storage';
import { logger } from '@/utils/logger';
import { initHighlightStyles } from './styles/theme/highlight-tokens';
import { installGlassAdaptive } from './composables/ui/useGlassAdaptive';
import i18n from './i18n';

// Apple Design System 组件全局注册（确保生产包 resolveComponent 可用）
import ApAvatar from '@/components/apple/ApAvatar.vue';
import ApBreadcrumb from '@/components/apple/ApBreadcrumb.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApCheckbox from '@/components/apple/ApCheckbox.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApDescriptionList from '@/components/apple/ApDescriptionList.vue';
import ApDrawer from '@/components/apple/ApDrawer.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApError from '@/components/apple/ApError.vue';
import ApErrorPage from '@/components/apple/ApErrorPage.vue';
import ApFieldWrapper from '@/components/apple/ApFieldWrapper.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApNavBar from '@/components/apple/ApNavBar.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApPagination from '@/components/apple/ApPagination.vue';
import ApPopover from '@/components/apple/ApPopover.vue';
import ApProgress from '@/components/apple/ApProgress.vue';
import ApRadio from '@/components/apple/ApRadio.vue';
import ApSearch from '@/components/apple/ApSearch.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSidebar from '@/components/apple/ApSidebar.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApToast from '@/components/apple/ApToast.vue';
import ApTrendIndicator from '@/components/apple/ApTrendIndicator.vue';
import ApViewToggle from '@/components/apple/ApViewToggle.vue';
const { t } = i18n.global;

// ===========================================
// 生产环境日志控制
// 生产环境中仅禁用 console.log，保留 warn/error 用于问题诊断
// ===========================================
if (import.meta.env.PROD) {
  // eslint-disable-next-line no-console
  console.log = () => {};
}


const app = createApp(App);

// Apple Design System 组件全局注册
const appleComponents: [string, Component][] = [
  [ApAvatar.name || 'ApAvatar', ApAvatar],
  [ApBreadcrumb.name || 'ApBreadcrumb', ApBreadcrumb],
  [ApButton.name || 'ApButton', ApButton],
  [ApCard.name || 'ApCard', ApCard],
  [ApCheckbox.name || 'ApCheckbox', ApCheckbox],
  [ApConfirmModal.name || 'ApConfirmModal', ApConfirmModal],
  [ApDescriptionList.name || 'ApDescriptionList', ApDescriptionList],
  [ApDrawer.name || 'ApDrawer', ApDrawer],
  [ApEmpty.name || 'ApEmpty', ApEmpty],
  [ApError.name || 'ApError', ApError],
  [ApErrorPage.name || 'ApErrorPage', ApErrorPage],
  [ApFieldWrapper.name || 'ApFieldWrapper', ApFieldWrapper],
  [ApFormItem.name || 'ApFormItem', ApFormItem],
  [ApEntityCard.name || 'ApEntityCard', ApEntityCard],
  [ApIcon.name || 'ApIcon', ApIcon],
  [ApInput.name || 'ApInput', ApInput],
  [ApMarkdownPreview.name || 'ApMarkdownPreview', ApMarkdownPreview],
  [ApModal.name || 'ApModal', ApModal],
  [ApNavBar.name || 'ApNavBar', ApNavBar],
  [ApPageContainer.name || 'ApPageContainer', ApPageContainer],
  [ApPagination.name || 'ApPagination', ApPagination],
  [ApPopover.name || 'ApPopover', ApPopover],
  [ApProgress.name || 'ApProgress', ApProgress],
  [ApRadio.name || 'ApRadio', ApRadio],
  [ApSearch.name || 'ApSearch', ApSearch],
  [ApSegmentedControl.name || 'ApSegmentedControl', ApSegmentedControl],
  [ApSelect.name || 'ApSelect', ApSelect],
  [ApSidebar.name || 'ApSidebar', ApSidebar],
  [ApSkeleton.name || 'ApSkeleton', ApSkeleton],
  [ApStatCard.name || 'ApStatCard', ApStatCard],
  [ApStatusBadge.name || 'ApStatusBadge', ApStatusBadge],
  [ApSwitch.name || 'ApSwitch', ApSwitch],
  [ApTable.name || 'ApTable', ApTable],
  [ApTextarea.name || 'ApTextarea', ApTextarea],
  [ApToast.name || 'ApToast', ApToast],
  [ApTrendIndicator.name || 'ApTrendIndicator', ApTrendIndicator],
  [ApViewToggle.name || 'ApViewToggle', ApViewToggle],
];
for (const [name, component] of appleComponents) {
  app.component(name, component);
}

// 设计系统可寻址性：给每个 Ap* 组件的根元素打 data-ap="<kebab>"。
// 本库全靠 inline :style，组件根节点既没有 class 也没有 data 属性，
// 运行时巡检（web/scripts/glossary-audit.mjs）与 e2e 因此没有稳定锚点 ——
// 上一版探针写 [class*=avatar] 全部假阴性就是这个原因。
// 用全局 mixin 统一打标记：新增组件自动覆盖，不会像手改 41 个文件那样漏。
const AP_COMPONENT_RE = /^Ap[A-Z0-9]/;
const apSlug = (s: string) => s
  .replace(/^Ap/, '')
  .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
  .toLowerCase();
app.mixin({
  mounted() {
    const opts = this.$options as { name?: string; __name?: string };
    const name = opts.__name || opts.name || '';
    if (!AP_COMPONENT_RE.test(name)) return;
    const el = this.$el as unknown;
    // 多根组件的 $el 是注释占位节点；Teleport 根同理。这类交给组件自己标。
    if (el instanceof Element && !el.hasAttribute('data-ap')) {
      el.setAttribute('data-ap', apSlug(name));
    }
  },
});

// 初始化 Pinia 并添加持久化插件
const pinia = createPinia();
pinia.use(piniaPluginPersistedstate);
app.use(pinia);

// 全局错误处理
app.config.errorHandler = (err, instance, info) => {
  logger.error(`[Vue Error] ${info ?? ''}`, err);
  try {
    const ui = useUIStore();
    ui.showToast(t('tsUi3.k1'), 'error');
  } catch {
    // UI store 可能尚未初始化
  }
};

// 全局未捕获的 Promise 错误
window.addEventListener('unhandledrejection', (event) => {
  console.warn('[Unhandled Rejection]', event.reason);
});

// 全局未捕获的同步错误
window.addEventListener('error', (event) => {
  // 忽略 ResizeObserver 循环错误（这是浏览器的已知问题，通常无害）
  if (event.message && event.message.includes('ResizeObserver loop')) {
    return;
  }
  // 已由 errorHandler 处理
});

app.use(i18n);
app.use(router);

// 初始化认证状态并等待路由就绪后再挂载，避免闪烁
async function bootstrap() {
  // 改名（AIWiki → Citebase）留下的旧 localStorage 裸键搬到新名下：草稿、面板宽度。
  // 必须在任何读取之前 —— storage.ts 那层的 `aiwiki:` 回落是按键读时自动采纳的，
  // 但这几个键名是本模块外面手写的字符串，只有这里能一次扫清。
  const migrated = migrateLegacyRawKeys();
  if (migrated > 0) logger.info(`已迁移 ${migrated} 项改名前的本地数据（草稿/面板宽度）`);

  // 注入 CSS 变量到 :root
  injectCSSVariables();

  // 注入全局动画样式

  // 初始化语法高亮样式
  initHighlightStyles();

  // 在挂载前同步应用主题（偏好 + 系统深浅色），避免 FOUC（闪白屏）
  useUIStore().initTheme();
  // 同一时刻应用行为三档（动效强度 / 对比度 / 透明度）：`--apple-motion-scale` 必须
  // 在首帧之前落到 :root 上，否则挂载后第一帧用设计时长、下一帧才缩放 —— 会看见一次跳。
  useUIStore().initPersonalization();

  // 站点展示名与认证初始化并行发起：标签页标题、登录页大标题、移动端头部都取这一个值。
  // 不等它就挂载，首屏会先闪一次代码默认名（管理员改过名字时尤其明显）。
  const brandReady = loadSiteBrand();

  const auth = useAuthStore();
  await auth.init();

  const ui = useUIStore();
  ui.initSidebarState();

  const history = useHistoryStore();
  history.initFromStorage();

  // 站点名先到齐：路由守卫在初始导航里就用它拼标签页标题（siteDocumentTitle），
  // 排在 isReady 之后就等于让第一帧的标题带上代码默认名。
  // 读不到时 loadSiteBrand 自己退回默认名，不会把首屏卡住。
  await brandReady;

  // 等待路由初始导航完成（确保路由守卫中的异步操作结束），
  // 否则 currentRoute 仍指向 '/'，会导致先渲染首页再切换到目标页的闪烁
  await router.isReady();

  app.mount('#app-root');

  // 浮层的自适应加厚必须在挂载之后接线：它靠 MutationObserver 看谁进 DOM，
  // 挂载前挂到 document.body 上还没有那棵树。
  installGlassAdaptive();
}

bootstrap();
