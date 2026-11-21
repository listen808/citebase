import { useMobile } from '@/composables/ui/useMobile';

/**
 * 历史门面：22 个视图 import 的是 `useBreakpoint`。
 * 实现与阈值只有一处（`useMobile` 的模块单例：一个 resize 监听、一份防抖、一个转场门控），
 * 这里不再自己比较边界 —— 上一版并行算着 `isMobile/isTablet/isDesktop`，
 * 改 `AppleBreakpoint` 时两处会分叉，而 `isDesktop` 全站 0 个消费者。
 */
export const useBreakpoint = useMobile;
