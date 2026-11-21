/**
 * echarts 生命周期骨架（此前三个图表组件各写一遍：主题订阅 + resize 监听 + 卸载清理）。
 *
 * 只收口"环境响应"部分；实例管理策略（单例复用 / dispose 重建 / 多实例 + ResizeObserver）
 * 因组件而异，保留在各组件内。useChart() 全量抽象曾评估过——三个组件的实例语义差异
 * （防闪屏 vs 幂等重画 vs 批量实例）大于公共部分，强行统一反而要加一堆开关。
 */
import { onMounted, onBeforeUnmount } from 'vue';
import { subscribeTheme } from '@/utils/core/echarts-theme';

export function useChartLifecycle(opts: { onResize?: () => void; onThemeChange?: () => void }) {
  let unsubscribeTheme: (() => void) | null = null;
  const onResize = opts.onResize;
  const onThemeChange = opts.onThemeChange;

  onMounted(() => {
    if (onResize) window.addEventListener('resize', onResize);
    if (onThemeChange) unsubscribeTheme = subscribeTheme(onThemeChange);
  });

  onBeforeUnmount(() => {
    if (onResize) window.removeEventListener('resize', onResize);
    unsubscribeTheme?.();
    unsubscribeTheme = null;
  });
}
