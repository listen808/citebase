<template>
  <!-- 顶部条同时是 banner 与 navigation 两个地标：外层 header 承接样式与玻璃背景，
       内层 nav 用 display:contents 参与同一套 flex 布局，只为给出 aria-label。 -->
  <header ref="navRef" data-ap="nav-bar" data-ap-shell="nav" :style="navStyle" :class="'apple-glass-nav'">
    <nav :aria-label="$t('shell.k6')" :style="{ display: 'contents' }">
    <div :style="leftStyle">
      <!-- 品牌恒为最左锚点（点它回首页）；汉堡 / 返回排在它右侧，不再把品牌推走。
           此前返回键在品牌左边，一旦有历史品牌就右移一个按钮位，看着像"品牌到处跑"。 -->
      <!-- 2026-10-26 终稿：侧栏开关已改为骑缝悬浮把手（见 MainLayout），顶栏不放任何
           与品牌无关的按钮 —— 品牌区只允许品牌（用户四轮反馈的最终结论）。 -->
      <router-link to="/" :style="brandLinkStyle" :aria-label="$t('shell.k14')">
        <ApBrand :size="24" :show-wordmark="!isMobile" />
      </router-link>
      <!-- 2026-10-26 定稿：侧栏开关不在顶栏 —— 品牌区（两侧）不放任何附加按钮（返回键放品牌左
           曾被否过，同理），且与返回键并排语义重复（用户报"展开收起和返回又重复"）。
           开关两态都固定在侧栏搜索行右端（见 ApSidebar），相对位置一致，顶栏不再参与。 -->
      <ApButton
        v-if="isMobile && auth.isLoggedIn"
        variant="ghost"
        size="mini"
        icon-only
        :aria-label="$t('shell.k15')"
        @click="toggleMobileMenu"
      >
        <Menu :size="16" />
      </ApButton>
      <template v-if="auth.isLoggedIn && !isMobile">
        <!-- 窄视口下这三个入口交给汉堡抽屉，导航条只留「返回 / 搜索 / 通知」：
             实测 430px 时右侧按钮簇起于 left 349、宽 284，整簇被切到视口外够不着 -->
        <span :style="navSeparatorStyle">|</span>
        <!-- 返回键（2026-10-26 从品牌右侧挪入导航区）：返回属导航动作，与首页/AI/用户同区；
             品牌区只留品牌（用户报"返回也不能放在品牌位置"）。 -->
        <ApButton
          v-if="canGoBack"
          variant="ghost"
          size="mini"
          icon-only
          :aria-label="$t('shell.k16')"
          @click="handleBack"
        >
          <ChevronLeft :size="16" />
        </ApButton>
        <router-link
          to="/"
          :style="[{ textDecoration: 'none' }, route.path === '/' ? navItemActiveStyle : navItemStyle]"
          :aria-current="route.path === '/' ? 'page' : undefined"
        >{{ $t('shell.k11') }}</router-link>
        <router-link
          v-if="aiStore.enabled"
          to="/ai/workspace"
          :style="[{ textDecoration: 'none' }, route.path.startsWith('/ai') ? navItemActiveStyle : navItemStyle]"
          :aria-current="route.path.startsWith('/ai') ? 'page' : undefined"
        >AI</router-link>
        <router-link
          to="/follow"
          :style="[{ textDecoration: 'none' }, route.path === '/follow' ? navItemActiveStyle : navItemStyle]"
          :aria-current="route.path === '/follow' ? 'page' : undefined"
        >{{ $t('shell.k12') }}</router-link>
      </template>
    </div>

    <div :style="centerStyle"></div>

    <div :style="rightStyle">
      <template v-if="auth.isLoggedIn">
        <ApButton v-if="!isMobile" variant="ghost" size="small" :aria-label="$t('shell.search')" @click="openSpotlight">
          <Search :size="16" :style="{ color: cv('text-quaternary') }" />
          <span :style="searchBtnTextStyle">{{ $t('shell.search') }}</span>
          <span :style="searchBtnShortcutStyle">{{ SEARCH_SHORTCUT_HINT }}</span>
        </ApButton>
        <ApButton v-if="isMobile" variant="ghost" size="mini" icon-only :aria-label="$t('shell.search')" @click="openSpotlight">
          <Search :size="16" />
        </ApButton>
        <!-- 主题菜单抽成组件：登录页原先是 light↔dark 二值按钮，另外三套主题在登录页点不到 -->
        <ApThemeMenu />

        <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('shell.k17')" @click="router.push('/profile')">
          <Settings :size="16" />
        </ApButton>
        <div :style="badgeWrapStyle">
          <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('shell.k18')" @click="router.push('/notifications')">
            <Bell :size="16" />
          </ApButton>
          <span v-if="notificationStore.unreadCount > 0" :style="badgeStyle">{{ notificationStore.unreadCount }}</span>
        </div>
        <!-- 「消息」位原先是个没有任何点击处理的死按钮，「帮助」面板在 /profile
             与移动抽屉里都到得了 —— 两个都从顶栏收掉，右簇只留 搜索/主题/设置/通知。 -->
      </template>
    </div>
    </nav>
  </header>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useScrollEdge, shellSurfaceKey } from '@/composables/ui/useScrollEdge';
import { useShellGlassLift } from '@/composables/ui/useGlassAdaptive';
import { useAuthStore } from '@/stores/auth/auth';
import { useNotificationStore } from '@/stores/social/notification';
import { useAIStore } from '@/stores/ai/ai';
import { SEARCH_SHORTCUT_HINT } from '@/utils/core/constants';
import { AppleFontWeight, AppleLayout, AppleRadius, AppleSpace, AppleType, AppleZIndex, cv } from '@/utils/core/apple-tokens';
import { rowStyle } from '@/styles/layout/page-styles';
import { Settings, Bell, Search, ChevronLeft, Menu } from 'lucide-vue-next';
import { useRouter, useRoute } from 'vue-router';
import { useMobile } from '@/composables/ui/useMobile';
import ApThemeMenu from './ApThemeMenu.vue';
import ApBrand from './ApBrand.vue';
import { onNotificationUpdate } from '@/composables/websocket/useNotificationSocket';
import ApButton from '@/components/apple/ApButton.vue';

const auth = useAuthStore();
const aiStore = useAIStore();
const router = useRouter();
const route = useRoute();
const notificationStore = useNotificationStore();

const { isMobile } = useMobile();

// WebSocket 推送的未读数变更回调（替代 60s 轮询）
let unsubNotification: (() => void) | null = null;

onMounted(() => {
  if (auth.isLoggedIn) {
    // 首次进入立即拉取一次
    notificationStore.loadUnreadCount();
    // 后续由 WebSocket 实时推送驱动，不再轮询
    unsubNotification = onNotificationUpdate(() => notificationStore.loadUnreadCount());
  }
});

onUnmounted(() => {
  if (unsubNotification) {
    unsubNotification();
    unsubNotification = null;
  }
});

const canGoBack = computed(() => {
  if (route.path === '/' || route.path === '/login') return false;
  return window.history.length > 1;
});

function handleBack() {
  router.back();
}

function toggleMobileMenu() {
  window.dispatchEvent(new CustomEvent('toggle-mobile-menu'));
}



function openSpotlight() {
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }));
}

// 导航栏样式 - 壳层表面（与侧栏同一档）
// 底色与模糊**不在这里写**：那两个值已经由全局类 .apple-glass-nav 从表面阶梯取，
// 行内再写一遍等于同一层颜色有两条命名路径（这个仓库为这类别名开过门禁 T12）。
// 顶栏也**不接滚动边缘探针**：它住在 MainLayout 的 flex 列里独占一行，不是那个滚动容器的后代，
// 内容从不从它底下滚过 —— 详见 composables/ui/useScrollEdge.ts 的文件头。
// 顶栏现在是浮层壳：内容会从它底下滚过去，所以它接滚动边缘探针。
// 滚动区不是它的祖先（它是视口钉死的浮层），必须用 [data-ap-scroll] 锚点指名观察对象。
const navRef = ref<HTMLElement | null>(null);
const navScroller = () => document.querySelector('[data-ap-scroll]');
const navScrollEdge = useScrollEdge(navRef, { scroller: navScroller });
// 升到 elevated 那一档之后，厚度再交给自适应：滚过去的是页面自己的底色时它就停在
// 静止档（看着还是那层玻璃），只有暗图/亮图真的滑到身下时才加厚。
useShellGlassLift(navRef, { scroller: navScroller, edge: navScrollEdge });

const navStyle = computed(() => ({
  // 浮层壳：钉在视口顶，内容从它底下滚过去（MainLayout 里它在 body 之后渲染，
  // 定位完全由这里决定）。sticky 在这里不成立 —— 它不在任何滚动容器里。
  position: 'fixed' as const,
  top: '0',
  left: '0',
  right: '0',
  zIndex: String(AppleZIndex.overlay),
  height: AppleLayout.shellBarHeight,
  // 静止态的通透由 .apple-glass-nav 那层给（surface-chrome）；这里只在"有内容压上来"时
  // 升到浮层档 —— 半透明壳压着滚动的内容时，静止档的次级文字会掉到 AA 线以下。
  // 升档之后那枚 elevated 的 alpha 由 useShellGlassLift 按它身后的实测底色改写：
  // 令牌里那枚 0.88/0.96 只是"量不出来"时的兜底，滚过去的是页面自己的底色时它停在静止档。
  backgroundColor: cv(shellSurfaceKey(navScrollEdge.value, 'surface-chrome')),
  // 镜面高光铺在壳面上（非玻璃材质下令牌是 none）：深色下 blur 在近均匀暗底上几乎不可见，
  // 真正说明"这是玻璃"的是这圈随光移动的高光。颜色走材质令牌，纸感/杂志不会长出来。
  backgroundImage: cv('glass-specular'),
  // 模糊两档都付：静止态一档轻的（10px / clear 8px），内容压上来才升到全档（22px / 16px）。
  // 只付 22px 时，静止的顶栏在暗色下读成一块黑板 —— 而玻璃的"材料感"恰好需要一层模糊垫底。
  // 两档都走令牌，solid / 纸感下是 none，不白付 GPU。
  backdropFilter: cv(navScrollEdge.value === 'under' ? 'glass-blur' : 'blur-panel'),
  WebkitBackdropFilter: cv(navScrollEdge.value === 'under' ? 'glass-blur' : 'blur-panel'),
  // 底边只留一条极淡的基线上（rim 令牌），真正的边界信号是 ::before 里那条随光源滑动的亮段
  // （见 apple-tokens.ts 的停靠壳层规则）。border-nav 是浮层的发丝边，不在这里复用。
  borderBottom: `0.5px solid ${cv('glass-rim')}`,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `0 clamp(16px, 3vw, 24px)`,
}));

const leftStyle = rowStyle(AppleSpace.sm);

const brandLinkStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  textDecoration: 'none',
};

const navSeparatorStyle = {
  color: cv('border'),
  fontSize: AppleType.bodyPrimary.size,
  letterSpacing: AppleType.bodyPrimary.letterSpacing,
  margin: `0 ${AppleSpace.xs}`,
};

const navItemStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  cursor: 'pointer',
  transition: 'color var(--apple-dur-interactive) var(--apple-easing-standard)',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.control,
};

const navItemActiveStyle = {
  ...navItemStyle,
  // 与侧栏选中项同一套语言（ApButton variant="tint"）：同色浅染 + 压暗后的同色文字
  color: cv('blue-text'),
  fontWeight: String(AppleFontWeight.medium),
  background: cv('blue-selection'),
};

const centerStyle = {
  flex: '1',
};

const rightStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const searchBtnTextStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-tertiary'),
};

const searchBtnShortcutStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-quaternary'),
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  background: cv('bg-secondary'),
  border: `1px solid ${cv('border-light')}`,
};

const badgeWrapStyle = {
  position: 'relative' as const,
  display: 'inline-flex',
};

const badgeStyle = {
  position: 'absolute' as const,
  top: '-2px',
  right: '-2px',
  minWidth: '16px',
  height: '16px',
  padding: `0 ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
  background: cv('red'),
  color: cv('on-accent'),
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.medium),
  lineHeight: '16px',
  textAlign: 'center' as const,
};
</script>


