import { describe, expect, it } from 'vitest';
import { effectScope, nextTick } from 'vue';
import { useSheetGesture } from './useSheetGesture';

/**
 * 钉住"静置在满程时不许留 transform"这一条。
 *
 * 为什么值得单独测：`translate3d(0,0,0)` 看上去是恒等变换，肉眼和所有像素类工装都没有意见，
 * 但它照样把这一层变成内部 `position: fixed` 后代的**包含块**，并多开一个层叠上下文。
 * 全站三个手势浮层（ApDrawer / MobileTocFab / SpaceView 移动端侧栏）共用这一份样式，
 * 而 SpaceView 抽屉里的右键菜单正是被这件事困住的：它的坐标全靠"抽屉恰好贴在原点"这个巧合。
 * 看不见、但确实改了层叠语义的属性，只能靠断言它**不存在**来守。
 */
const TRAVEL = 300;
// 包一层 effectScope：composable 里有 onScopeDispose（停 RAF / 摘监听），
// 直接在它外面调用会打 Vue warn —— 那不是被测行为，是测试自己的接线噪音。
const mk = () => {
  const scope = effectScope();
  return scope.run(() =>
    useSheetGesture({ axis: 'x', travel: () => TRAVEL, onDismiss: () => {} }),
  )!;
};

const styleOf = (g: ReturnType<typeof mk>) => g.transformStyle.value as Record<string, string>;

describe('useSheetGesture.transformStyle', () => {
  it('静置在满程（offset 0）时一个变换属性都不留', async () => {
    const g = mk();
    await nextTick();
    expect(g.offset.value).toBe(0);
    const s = styleOf(g);
    expect(s.transform).toBeUndefined();
    // willChange 也不能留：静置时白占一层合成层
    expect(s.willChange).toBeUndefined();
  });

  it('停在"关"那一头（offset = 满程）必须写 transform —— 不写它就贴在原点了', async () => {
    const g = mk();
    g.park(TRAVEL);
    await nextTick();
    const s = styleOf(g);
    expect(s.transform).toBe('translate3d(300px, 0, 0)');
    expect(s.willChange).toBe('transform');
  });

  it('从满程走回 0 之后要把 transform 撤掉，而不是留一个 0', async () => {
    const g = mk();
    g.park(TRAVEL);
    await nextTick();
    expect(styleOf(g).transform).toContain('translate3d');
    g.park(0);
    await nextTick();
    const s = styleOf(g);
    // 这一条就是原先失效的地方：位移归零了，包含块却永久留着
    expect(s.transform).toBeUndefined();
    expect(s.willChange).toBeUndefined();
  });

  it('半途停住（既不是 0 也不是满程）仍要写 transform', async () => {
    const g = mk();
    g.park(120);
    await nextTick();
    expect(styleOf(g).transform).toBe('translate3d(120px, 0, 0)');
  });

  it('direction=-1（左侧抽屉）时正位移朝屏幕左走', async () => {
    const scope = effectScope();
    const g = scope.run(() =>
      useSheetGesture({ axis: 'x', travel: () => TRAVEL, onDismiss: () => {}, direction: -1 }),
    )!;
    g.park(TRAVEL);
    await nextTick();
    expect(styleOf(g).transform).toBe('translate3d(-300px, 0, 0)');
  });
});
