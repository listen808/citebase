import { isTransientApiError, getApiErrorStatus, getApiRetryAfter } from '@/api/client';

/**
 * 崩页与"等一下再试"是两种不同的事实，混成一张红色警告卡会让用户以为系统坏了。
 * 限流（429）与服务端抖动（5xx）都属于后者：等一会儿就好，而且等待时间服务端已经给了。
 *
 * 单独成模块是为了能被直接测到 —— ErrorBoundary 里"子组件抛错 → onErrorCaptured"
 * 这条链在测试环境里走不通（slot 内容的父链归属问题），
 * 于是边界渲染本身没法测；把决策抽出来，测的就是真的那段判断而不是它的替身。
 */
export type ErrorPresentation = {
  headline: string;
  blurb: string;
  /** 是否该显示倒计时并锁住重试按钮；null 表示没有可靠的等待时长 */
  waitSeconds: number | null;
};

/** 服务端没给 Retry-After 时的兜底等待：比"立刻重试"好，比"让用户自己猜"也好。 */
export const DEFAULT_RETRY_WAIT_SECONDS = 5;

/** 默认恒等：单测只验分支判定，不依赖 i18n 实例；界面调用方传 vue-i18n 的 t。 */
export type Translate = (key: string) => string;

export function errorPresentation(err: unknown, t: Translate = (key) => key): ErrorPresentation {
  const status = getApiErrorStatus(err);
  if (status === 429) {
    return {
      headline: t('common.rateLimitTitle'),
      blurb: t('common.rateLimitBlurb'),
      waitSeconds: getApiRetryAfter(err) ?? DEFAULT_RETRY_WAIT_SECONDS,
    };
  }
  if (isTransientApiError(err)) {
    return {
      headline: t('common.serviceUnavailableTitle'),
      blurb: t('common.serviceUnavailableBlurb'),
      waitSeconds: getApiRetryAfter(err) ?? DEFAULT_RETRY_WAIT_SECONDS,
    };
  }
  return {
    headline: t('common.genericErrorTitle'),
    blurb: t('common.genericErrorBlurb'),
    waitSeconds: null,
  };
}
