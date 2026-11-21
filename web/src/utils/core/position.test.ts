import { describe, it, expect } from 'vitest';
import { calcPosition } from '@/utils/core/position';

/** 只填 calcPosition 实际读取的字段，避免构造完整 DOMRect */
const rect = (r: Partial<DOMRect>) => r as DOMRect;

const trigger = rect({ top: 100, bottom: 120, left: 50, right: 130, width: 80, height: 20 });
const content = rect({ width: 150, height: 200 });

describe('calcPosition 方位计算', () => {
  it('bottom：落在触发器下方并水平居中', () => {
    expect(calcPosition(trigger, content, 'bottom')).toEqual({ top: '128px', left: '15px' });
  });

  it('bottom：gap 参与纵向偏移', () => {
    expect(calcPosition(trigger, content, 'bottom', 20).top).toBe('140px');
  });

  it('right：贴在触发器右侧', () => {
    expect(calcPosition(trigger, content, 'right').left).toBe('138px');
  });

  it('left：内容比触发器宽时水平方向不落到负值', () => {
    const left = Number(calcPosition(trigger, content, 'left').left.replace('px', ''));
    expect(left).toBeGreaterThanOrEqual(8);
  });

  it('未越界时不做夹取，保持居中结果', () => {
    const narrow = rect({ width: 40, height: 20 });
    expect(calcPosition(trigger, narrow, 'bottom')).toEqual({ top: '128px', left: '70px' });
  });
});

describe('calcPosition 视口边界', () => {
  const wide = rect({ width: 400, height: 100 });

  it('默认 minBound 为 8，越界值被夹到下限', () => {
    const { left } = calcPosition(trigger, wide, 'bottom');
    expect(left).toBe('8px');
  });

  it('自定义 minBound 生效', () => {
    expect(calcPosition(trigger, wide, 'bottom', 8, 24).left).toBe('24px');
  });

  it('top 方位在触发器贴近视口顶部时不产生负值', () => {
    const nearTop = rect({ top: 10, bottom: 30, left: 50, right: 130, width: 80, height: 20 });
    const top = Number(calcPosition(nearTop, content, 'top').top.replace('px', ''));
    expect(top).toBeGreaterThanOrEqual(8);
  });
});

describe('calcPosition 视口上界夹取', () => {
  const viewport = { width: 400, height: 300 };

  it('触发器贴近右边缘时，right 方位被拉回视口内', () => {
    const nearRight = rect({ top: 40, bottom: 60, left: 330, right: 390, width: 60, height: 20 });
    const panel = rect({ width: 180, height: 120 });
    expect(calcPosition(nearRight, panel, 'right', 8, 8, viewport).left).toBe('212px');
  });

  it('触发器贴近下边缘时，bottom 方位被拉回视口内', () => {
    const nearBottom = rect({ top: 250, bottom: 270, left: 40, right: 120, width: 80, height: 20 });
    const panel = rect({ width: 150, height: 200 });
    expect(calcPosition(nearBottom, panel, 'bottom', 8, 8, viewport).top).toBe('92px');
  });

  it('内容比视口还大时退化为保住下限而不产生负值', () => {
    const huge = rect({ width: 900, height: 800 });
    expect(calcPosition(trigger, huge, 'bottom', 8, 8, viewport)).toEqual({ top: '8px', left: '8px' });
  });

  it('max-content 面板内容变宽后重算：left + 宽度不得超出视口右缘（2026-10-26 走查实测超 4px 的场景）', () => {
    const nearRight = rect({ top: 100, bottom: 130, left: 1312, right: 1393, width: 81, height: 30 });
    const vw = { width: 1440, height: 741 };
    // 首帧量到的宽（图标/字体未就绪）与稳定后的宽（max-content 撑开）
    const first = calcPosition(nearRight, rect({ width: 224, height: 300 }), 'bottom', 8, 8, vw);
    const stable = calcPosition(nearRight, rect({ width: 236, height: 300 }), 'bottom', 8, 8, vw);
    expect(parseFloat(first.left) + 224).toBeLessThanOrEqual(1440);
    expect(parseFloat(stable.left) + 236).toBeLessThanOrEqual(1440);
  });
});
