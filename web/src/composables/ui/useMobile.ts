import { shallowRef, onMounted, onBeforeUnmount } from 'vue';
import { AppleBreakpoint, AppleDuration } from '@/utils/core/apple-tokens';

const isMobile = shallowRef(false);
const isTablet = shallowRef(false);
// 视口宽度与两个布尔档同源：原先 utils/responsive.ts 的 useBreakpoint() 每个实例
// 自己 ref(window.innerWidth) + 自己挂一个无防抖的 resize 监听（24 个文件），
// 于是一次拖动窗口要重算几十次。现在两边读同一个单例。
export const viewportWidth = shallowRef(typeof window === 'undefined' ? 1440 : window.innerWidth);
const transitionActive = shallowRef(false);
let initialized = false;
let resizeTimer: ReturnType<typeof setTimeout> | null = null;

function checkBreakpoints() {
  const width = window.innerWidth;
  viewportWidth.value = width;
  isMobile.value = width < AppleBreakpoint.tablet;
  isTablet.value = width >= AppleBreakpoint.tablet && width < AppleBreakpoint.desktop;
}

checkBreakpoints();

function handleResize() {
  if (transitionActive.value) return;
  if (resizeTimer) clearTimeout(resizeTimer);
  resizeTimer = setTimeout(checkBreakpoints, AppleDuration.debounceResize);
}

function initBreakpoints() {
  if (initialized) return;
  initialized = true;
  checkBreakpoints();
  window.addEventListener('resize', handleResize, { passive: true });
}

function destroyBreakpoints() {
  if (!initialized) return;
  initialized = false;
  window.removeEventListener('resize', handleResize);
  if (resizeTimer) { clearTimeout(resizeTimer); resizeTimer = null; }
}

let instanceCount = 0;

export function setTransitionActive(active: boolean) {
  transitionActive.value = active;
}

export function useMobile() {
  onMounted(() => {
    instanceCount++;
    initBreakpoints();
  });

  onBeforeUnmount(() => {
    instanceCount--;
    if (instanceCount <= 0) {
      instanceCount = 0;
      destroyBreakpoints();
    }
  });

  return { isMobile, isTablet, width: viewportWidth };
}
