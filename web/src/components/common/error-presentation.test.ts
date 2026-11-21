import { describe, it, expect } from 'vitest';
import { errorPresentation, DEFAULT_RETRY_WAIT_SECONDS } from './error-presentation';
import { ApiError } from '@/api/client';

// 测的是"崩页 vs 等一下再试"这个判断本身。
// 试过挂 ErrorBoundary 测渲染：slot 内容的父链归属让 onErrorCaptured 在测试环境里收不到抛错
// （setup 里抛 → 子节点照常渲染；render 里抛 → 整棵子树空白），那两种写法都测不到真东西，
// 于是把决策抽成纯函数，测的就是线上真正跑的那段判断。
describe('errorPresentation', () => {
  it('429 走限流分支，并给出等待秒数', () => {
    const p = errorPresentation(new ApiError('操作过于频繁，请稍后再试', { status: 429, retryAfter: 7 }));
    expect(p.headline).toBe('common.rateLimitTitle');
    expect(p.blurb).toBe('common.rateLimitBlurb');
    expect(p.waitSeconds).toBe(7);
  });

  it('429 没带 Retry-After 时用兜底等待，而不是"立刻重试"', () => {
    expect(errorPresentation(new ApiError('操作过于频繁，请稍后再试', { status: 429 })).waitSeconds)
      .toBe(DEFAULT_RETRY_WAIT_SECONDS);
  });

  it('5xx 走服务抖动分支，文案不冒充限流', () => {
    const p = errorPresentation(new ApiError('bad gateway', { status: 502 }));
    expect(p.headline).toBe('common.serviceUnavailableTitle');
    expect(p.headline).not.toBe('common.rateLimitTitle');
    expect(p.waitSeconds).not.toBeNull();
  });

  it('4xx 真错误（400/403/404）仍算页面出问题，不被误降级成"稍后再试"', () => {
    for (const status of [400, 403, 404]) {
      const p = errorPresentation(new ApiError('nope', { status }));
      expect(p.headline, `status=${status}`).toBe('common.genericErrorTitle');
      expect(p.waitSeconds, `status=${status}`).toBeNull();
    }
  });

  it('非 ApiError 的运行时异常走崩溃那一支', () => {
    const p = errorPresentation(new Error('cannot read property x of undefined'));
    expect(p.headline).toBe('common.genericErrorTitle');
    expect(p.blurb).toBe('common.genericErrorBlurb');
    expect(p.waitSeconds).toBeNull();
  });
});
