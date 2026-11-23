<template>
  <div :style="layoutStyle">
    <a href="#main-content" class="apple-skip-link">{{ $t('layoutUi.k1') }}</a>
    <div :style="bodyStyle">
      <!-- 移动端侧边栏抽屉 -->
      <ApDrawer
        v-if="isMobile"
        v-model="mobileMenuOpen"
        placement="left"
        width="280px"
      >
        <template #title>
          {{ $t('layoutUi.k2') }}
        </template>
        <ApSidebar />
      </ApDrawer>
      
      <aside v-if="!hideSidebar && !isMobile" :style="sidebarShellStyle">
        <ApSidebar :collapsed="uiStore.sidebarCollapsed" />
      </aside>
      <!-- 侧栏收起/展开把手（2026-10-26 定稿，用户选定悬浮把手方案）：骑缝在侧栏与内容交界线上，
           Figma/多面板应用语言。不占任何行空间、不进品牌区、不进搜索行；平时半透明淡显、
           hover 变清晰。放 MainLayout 而非 ApSidebar：aside 壳 overflow:hidden 会裁掉骑缝的半边。 -->
      <button
        v-if="!hideSidebar && !isMobile"
        class="sidebar-collapse-handle"
        :class="{ collapsed: uiStore.sidebarCollapsed }"
        :style="{ left: uiStore.sidebarCollapsed ? AppleLayout.sidebarCollapsed : AppleLayout.sidebarWidth, zIndex: String(AppleZIndex.sticky) }"
        :title="$t(uiStore.sidebarCollapsed ? 'shell.k88' : 'shell.k87')"
        :aria-label="$t(uiStore.sidebarCollapsed ? 'shell.k88' : 'shell.k87')"
        @click="uiStore.toggleSidebar()"
      >
        <ApIcon :name="uiStore.sidebarCollapsed ? 'PanelLeftOpen' : 'PanelLeftClose'" :size="16" />
      </button>
      <!-- main 与页脚一起放在这一列里：页脚要在 <main> 之外才算得上一级 contentinfo 地标，
           而 bodyStyle 是横排（侧栏 + 正文），直接塞进去会变成第三列 -->
      <div :style="mainColumnStyle">
        <main id="main-content" tabindex="-1" :style="mainWrapperStyle">
          <div data-ap-scroll :style="contentStyle">
            <slot />
            <!-- P2-5：滚动容器的 padding-bottom 不计入可滚动高度（Chrome 实测），
                 末行卡片仍会被浮层页脚盖住；用真实占位元素才把这段距离带进 scrollHeight。 -->
            <div aria-hidden="true" :style="scrollEndSpacerStyle" />
          </div>
        </main>
        <ApFooter overlay />
      </div>
    </div>
    <!-- 顶栏是**浮层**，不是壳里独占的一行：它钉在视口顶，内容从它底下滚过去。
         玻璃要压在真内容上才读得出材质（压在环境渐变上时它只是一条略深的色带），
         而"内容滚进来时壳加厚"那条滚动边缘效应也只有这个几何下才成立。
         代价只有一处：内容滚动区自己留 padding-top，把正文起始位置让到顶栏下沿。
         **不要**用继承的变量去抬内容区里的 sticky 元素 —— 变量会一路传到编辑器、预览、
         表格这些自带滚动容器的地方，把它们钉在容器内下方一个壳高处，
         中间那条缝就是内容透出来的位置（实测遮挡过，已撤）。那些 sticky 本来就该贴自己容器的顶边。 -->
    <ApNavBar />
    <ApSpotlightSearch ref="spotlightRef" />
    <ApKeyboardShortcuts ref="shortcutsRef" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, shallowRef } from 'vue';
import { useRoute } from 'vue-router';
import ApNavBar from '@/components/apple/ApNavBar.vue';
import ApSidebar from '@/components/apple/ApSidebar.vue';
import ApDrawer from '@/components/apple/ApDrawer.vue';
import ApFooter from '@/components/apple/ApFooter.vue';
import ApSpotlightSearch from '@/components/apple/ApSpotlightSearch.vue';
import ApKeyboardShortcuts from '@/components/apple/ApKeyboardShortcuts.vue';
import { cv, AppleFont, AppleLayout, AppleZIndex } from '@/utils/core/apple-tokens';
import { useMobile } from '@/composables/ui/useMobile';
import { useUIStore } from '@/stores/app/ui';

const route = useRoute();
const spotlightRef = ref<InstanceType<typeof ApSpotlightSearch> | null>(null);
const shortcutsRef = ref<InstanceType<typeof ApKeyboardShortcuts> | null>(null);
const { isMobile } = useMobile();
const uiStore = useUIStore();
const mobileMenuOpen = shallowRef(false);

const hideSidebar = computed(() => route.path.startsWith('/space/'));

const isFullScreen = computed(() => {
  return route.path.startsWith('/space/');
});

/** 侧栏壳：宽度收放与 ApSidebar 同步（状态与持久化在 useUIStore，设置页「侧边栏展开」开关也写它）。
    刻意不加 width 过渡：G17/G27 只准动合成器属性，低频操作瞬时切换即可 */
const sidebarShellStyle = computed(() => ({
  width: uiStore.sidebarCollapsed ? AppleLayout.sidebarCollapsed : AppleLayout.sidebarWidth,
  minWidth: uiStore.sidebarCollapsed ? AppleLayout.sidebarCollapsed : AppleLayout.sidebarWidth,
  overflow: 'hidden' as const,
  flexShrink: 0,
  // 侧栏从顶栏下沿开始，不钻到浮层顶栏背后（2026-11-01，用户判「顶栏品牌区特别灰白」）。
  // 顶栏是浮层、横跨整行，侧栏若仍从 y=0 起，左上 240×48 就是**两层 chrome 叠在一起**：
  // 顶栏 0.28×134 压侧栏 0.28×134，合成后比右侧（顶栏压画布）亮 30+ 个亮度单位，
  // 还在 x=240 处留一条竖直接缝 —— 一份材质被涂了两遍，读起来像三种灰。
  // 顶栏自己已经是一条完整的壳横边，侧栏这一列应当从它下沿起算：
  // 让位放在壳上（这里），ApSidebar 自己的 padding-top 只留内容间距，不进顶栏背后。
  paddingTop: AppleLayout.shellBarHeight,
  boxSizing: 'border-box' as const,
}));

/** 侧栏占位宽度，给顶栏下沿那条渐隐带让位用（见 themes.ts 里 nav-bar::after 的 left）。
    挂在根上：ApNavBar 是根 div 的子节点，变量能继承进伪元素；反过来写在 nav 自己身上
    拿不到布局态（它不知道侧栏开没开）。0 表示"整条都是内容列"（移动端抽屉/全屏页）。 */
const sidebarInset = computed(() =>
  hideSidebar.value || isMobile.value
    ? '0px'
    : uiStore.sidebarCollapsed
      ? AppleLayout.sidebarCollapsed
      : AppleLayout.sidebarWidth,
);

const layoutStyle = computed(() => ({
  background: cv('surface-canvas'),
  fontFamily: AppleFont.text,
  WebkitFontSmoothing: 'antialiased',
  MozOsxFontSmoothing: 'grayscale',
  display: 'flex',
  flexDirection: 'column' as const,
  height: '100dvh',
  overflow: 'hidden' as const,
  '--ap-shell-sidebar-width': sidebarInset.value,
}));

const bodyStyle = {
  position: 'relative' as const,
  display: 'flex',
  flex: '1',
  overflow: 'hidden' as const,
  minHeight: '0',
};

const mainColumnStyle = {
  flex: '1',
  display: 'flex' as const,
  flexDirection: 'column' as const,
  minWidth: '320px',
  minHeight: '0',
  overflow: 'hidden' as const,
};

const mainWrapperStyle = {
  flex: '1',
  display: 'flex' as const,
  flexDirection: 'column' as const,
  overflow: 'hidden' as const,
  minHeight: '0',
  minWidth: '320px',
};

const contentStyle = computed(() => ({
  display: 'flex' as const,
  flexDirection: 'column' as const,
  flex: '1' as const,
  overflowY: isFullScreen.value ? 'hidden' as const : 'auto' as const,
  // scroll-padding-top 是锚点版（R9）：没有它，hash 跳转会把目标停在浮层顶栏底下。
  scrollPaddingTop: 'var(--apple-shell-bar-height)',
  // 始终预留滚动条空间，避免页面切换时因滚动条出现/消失导致内容向右抖动
  scrollbarGutter: isFullScreen.value ? 'auto' : 'stable',
  minHeight: '0' as const,
  // 顶栏是浮层，所以**任何模式**都要让出一个壳高：/space/* 这类全屏页（编辑器与阅读页）
  // 之前给 0，正文直接压在浮层顶栏底下 —— 用户报的"很多地方都遮挡"就是这里。
  // box-sizing 必须一起给：这个容器是 flex:1，content-box 下 padding 会把盒子撑到可用高度
  // 之外，底部正好被顶掉一个壳高（另一类看不见的遮挡）。
  boxSizing: 'border-box' as const,
  // 左右留白只允许一层负责：统一交给 ApPageContainer（它的 compact / 全宽规则按页型决定），
  // 布局层再加一次会让同类页面的正文左边界相差 24px。
  // 顺序：padding 简写会重置 padding-top，所以顶栏让位那一档必须写在它后面
  // （check-style-tokens 有这条判据 —— 写反过一次，搜索框因此跑到顶栏背后"消失"）。
  padding: '0',
  paddingTop: 'var(--apple-shell-bar-height)',
}));

// 底栏浮层化（ApFooter overlay）的对价：内容滚区末尾留出底栏高度，
// 最后一行正文不会被叠在上面的页脚永久盖住，滚动中内容则真的从页脚底下穿过去。
const scrollEndSpacerStyle = { flexShrink: 0, height: 'var(--apple-shell-bar-height)' };

function handleKeyDown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
    e.preventDefault();
    e.stopPropagation();
    spotlightRef.value?.open();
    return;
  }
  if ((e.ctrlKey || e.metaKey) && e.key === '/') {
    e.preventDefault();
    e.stopPropagation();
    shortcutsRef.value?.open();
    return;
  }
}

function handleToggleMobileMenu() {
  mobileMenuOpen.value = !mobileMenuOpen.value;
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown);
  window.addEventListener('toggle-mobile-menu', handleToggleMobileMenu);
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeyDown);
  window.removeEventListener('toggle-mobile-menu', handleToggleMobileMenu);
});
</script>

<style scoped>
/* 侧栏收起/展开把手：骑缝在侧栏与内容的交界线上（left 由模板按侧栏宽令牌计算）。
   平时半透明淡显，hover/聚焦变清晰；::before 把 20px 的视觉把手扩成 36px 的命中区，
   触屏也能点准。箭头方向随状态（模板切换图标）。 */
.sidebar-collapse-handle {
  position: absolute;
  top: 50%;
  transform: translate(-50%, -50%);
  width: calc(var(--apple-space-16px) + var(--apple-space-2px) * 4);
  height: calc(var(--apple-space-16px) * 2 + var(--apple-space-12px));
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid var(--apple-border);
  border-radius: var(--apple-space-10px);
  background: var(--apple-surface-float);
  color: var(--apple-text-tertiary);
  cursor: pointer;
  opacity: 0.45;
  transition: opacity var(--apple-dur-fast) var(--apple-easing-standard), color var(--apple-dur-fast) var(--apple-easing-standard);
}
.sidebar-collapse-handle::before {
  content: '';
  position: absolute;
  inset: calc(var(--apple-space-8px) * -1);
}
.sidebar-collapse-handle:hover,
.sidebar-collapse-handle:focus-visible {
  opacity: 1;
  color: var(--apple-text-secondary);
}
</style>

