import { cv, AppleZIndex } from '@/utils/core/apple-tokens';

/**
 * 全屏遮罩 = **只染色，不预磨砂**。
 * 2026-11-01 去掉 `blur` 参数：遮罩自己糊一层（blur(4px)）等于把浮层唯一的证据
 * ——「锐利背景被这块玻璃折射」——提前抹掉，实测弹窗与聚光在糊过的背景上都读成实心盒。
 * 模糊归浮层自己付（.apple-glass-layer 的 ::before）。卡片 loading 遮罩那种
 * "压的是自己内容"的局部遮罩另算，走 AppleGlassBlur.mask，不在这里。
 */
export function getOverlayStyle(
  zIndex: number = AppleZIndex.modal,
  extra: Record<string, string | number> = {},
) {
  return {
    position: 'fixed' as const,
    top: '0',
    left: '0',
    width: '100%',
    height: '100%',
    background: cv('overlay'),
    zIndex: String(zIndex),
    ...extra,
  };
}
