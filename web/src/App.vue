<template>
  <div id="app-root">
    <RouterProgress />
    <component :is="layoutComponent">
      <template #default>
        <router-view v-slot="{ Component }">
          <ErrorBoundary>
            <transition name="page" mode="out-in" @before-leave="onTransitionStart" @after-enter="onTransitionEnd">
              <component :is="Component" :key="routeKey" />
            </transition>
          </ErrorBoundary>
        </router-view>
      </template>
    </component>
    <!-- Toast 挂在壳层之上、布局之外：它由 ui.showToast 这条唯一数据源驱动，
         而登录 / 安装 / 分享这几页走的是 BlankLayout —— 原先挂载点在 MainLayout 里，
         那些页面调用 showToast() 一个都不显示（静默丢失反馈）。 -->
    <ApToast />
  </div>
</template>

<script setup lang="ts">
import { computed, watch, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { useUIStore } from '@/stores/app/ui';
import { useAuthStore } from '@/stores/auth/auth';
import { useAIStore } from '@/stores/ai/ai';
import { startNotificationSocket, stopNotificationSocket } from '@/composables/websocket/useNotificationSocket';
import RouterProgress from '@/components/layout/RouterProgress.vue';
import ErrorBoundary from '@/components/common/ErrorBoundary.vue';
import MainLayout from '@/components/layout/MainLayout.vue';
import BlankLayout from '@/components/layout/BlankLayout.vue';
import ApToast from '@/components/apple/ApToast.vue';
import { setTransitionActive } from '@/composables/ui/useMobile';
import { installSpecularLight } from '@/composables/ui/useSpecularLight';

const route = useRoute();
const _ui = useUIStore();
const _auth = useAuthStore();
const aiStore = useAIStore();

// 登录后加载 AI 配置 + 建立通知 WebSocket
onMounted(() => {
  installSpecularLight();
  if (_auth.isLoggedIn) {
    startNotificationSocket();
    aiStore.loadConfig();
  }
});
watch(
  () => _auth.isLoggedIn,
  (loggedIn) => {
    if (loggedIn) {
      startNotificationSocket();
      aiStore.loadConfig();
    } else {
      stopNotificationSocket();
      aiStore.$reset();
    }
  },
);

function onTransitionStart() {
  setTransitionActive(true);
}

function onTransitionEnd() {
  setTransitionActive(false);
}

const layoutComponent = computed(() => {
  const layout = route.meta?.layout;
  if (layout === 'blank') return BlankLayout;
  return MainLayout;
});

const routeKey = computed(() => {
  if (route.name === 'space' || route.name === 'document') {
    return `space-${route.params.spaceId}`;
  }
  return route.path;
});
</script>

<style>
html, body, #app-root {
  min-height: 100%;
  margin: 0;
  padding: 0;
}

/* 页过渡的时长全部走时长令牌（原先这里是 0.35s / 0.4s / 0.2s 三处硬写，
   个性化调不动它 —— 见 apple-tokens.ts 的 BASE_DURATION_MS） */
.page-enter-active {
  transition: opacity var(--apple-dur-pageEnterOpacity) var(--apple-easing-emphasized),
              transform var(--apple-dur-pageMove) var(--apple-easing-emphasized);
  will-change: transform, opacity;
}

.page-leave-active {
  transition: opacity var(--apple-dur-pageLeave) var(--apple-easing-standard),
              transform var(--apple-dur-pageLeave) var(--apple-easing-standard);
  will-change: transform, opacity;
}

.page-enter-from {
  opacity: 0;
  transform: translateY(12px) scale(0.97);
}

.page-leave-to {
  opacity: 0;
  transform: translateY(-6px) scale(0.99);
}

:root[data-theme='dark'] {
  color-scheme: dark;
}

/* 全局主题切换过渡：仅对 body 和直接子元素应用，避免全树 transition 影响性能 */
#app-root,
#app-root > * {
  transition: background-color var(--apple-dur-themeCrossfade) var(--apple-easing-standard),
              color var(--apple-dur-themeCrossfade) var(--apple-easing-standard),
              border-color var(--apple-dur-themeCrossfade) var(--apple-easing-standard),
              box-shadow var(--apple-dur-themeCrossfade) var(--apple-easing-standard);
}


/* 全局文本选择 - Apple Blue */

/* Apple Minimalist - 减弱动画偏好：这一档整组搬到了 apple-tokens.ts 的 reduceMotionCss()。
   原先它住在下面这个 @media 里，于是**应用内选了 reduced 而系统开关没开**的那条路径
   什么都不会发生（@media 根本不成立）。时长也已经走 --apple-dur-* 令牌，不再有硬写的 0.35s。 */

@media print {
  body { margin: 0; }
  #app-root > * > aside,
  #app-root > * > nav,
  .sidebar,
  [class*="sidebar"],
  [class*="toolbar"],
  [class*="context-menu"],
  button,
  input,
  select { display: none !important; }
  #app-root { display: block !important; }
}
</style>
