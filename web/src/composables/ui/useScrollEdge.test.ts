import { describe, it, expect } from 'vitest';
import { defineComponent, h, nextTick, ref, watch } from 'vue';
import { mount } from '@vue/test-utils';
import { scrollEdgeState, nearestScroller, shellSurfaceKey, useScrollEdge } from './useScrollEdge';

/**
 * 这条探针唯一的失败方式是"装了但从不触发"，而它一旦不触发，界面看上去完全正常 ——
 * 只是内容滚到顶栏底下时，顶栏还是那层通透的 0.58，次级文字掉到 2.x:1。
 * 所以判据要能在 happy-dom 里真实跑一遍，而不是只测那个纯函数。
 */

function makeScroller(opts: { scrollable: boolean; overflow?: string }) {
  const el = document.createElement('div');
  el.style.overflowY = opts.overflow ?? 'auto';
  // happy-dom 不做布局，scrollHeight/clientHeight 恒为 0；
  // 而"这个 overflow:auto 的壳当前真的能滚吗"恰恰是判据的一部分，所以按场景钉死这两个数。
  Object.defineProperty(el, 'clientHeight', { value: 400, configurable: true });
  Object.defineProperty(el, 'scrollHeight', { value: opts.scrollable ? 2000 : 400, configurable: true });
  return el;
}

describe('scroll edge 探针', () => {
  it('阈值就是 0：内容刚滚进来就必须换档', () => {
    expect(scrollEdgeState(0)).toBe('free');
    // 写成 >8 之类的"防抖余量"会红在这里 —— 而那 8px 正是不可读的那一瞬间
    expect(scrollEdgeState(1)).toBe('under');
    expect(scrollEdgeState(900)).toBe('under');
  });

  it('往上找到真正在滚的容器，跳过硬Overflow但不溢出的壳', () => {
    const outer = makeScroller({ scrollable: false }); // overflow:auto 但内容不溢出 → 永远不会发 scroll
    const inner = makeScroller({ scrollable: true });
    const header = document.createElement('header');
    inner.append(header);
    outer.append(inner);
    document.body.append(outer);
    try {
      expect(nearestScroller(header)).toBe(inner);
    } finally {
      outer.remove();
    }
  });

  it('没有可滚祖先也没有可滚文档时返回 null（效应正确地不装）', () => {
    const plain = document.createElement('div');
    plain.style.overflowY = 'visible';
    const header = document.createElement('header');
    plain.append(header);
    document.body.append(plain);
    try {
      expect(nearestScroller(header)).toBeNull();
    } finally {
      plain.remove();
    }
  });

  it('压在内容上时一律换浮层档，静止时保持原档', () => {
    // 这条规则只允许有一个落点：组件里不许再出现 surface-elevated 的三元判断
    expect(shellSurfaceKey('under', 'bg-secondary')).toBe('surface-elevated');
    expect(shellSurfaceKey('free', 'bg-secondary')).toBe('bg-secondary');
    expect(shellSurfaceKey('free', 'surface-chrome')).toBe('surface-chrome');
    // solid 材质下这两档同色，所以同一行代码在不透明主题里天然惰性 —— 不需要材质判断
    expect(shellSurfaceKey('under', 'surface-chrome')).toBe('surface-elevated');
  });

  it('状态值随滚动一起变化，并且被写进元素属性（工装可核对）', async () => {
    const scroller = makeScroller({ scrollable: true });
    const header = document.createElement('header');
    scroller.append(header);
    document.body.append(scroller);

    const seen: string[] = [];
    const Comp = defineComponent({
      setup() {
        const navRef = ref<HTMLElement | null>(header);
        const edge = useScrollEdge(navRef);
        watch(edge, (v) => seen.push(shellSurfaceKey(v, 'bg-secondary')));
        return () => h('div');
      },
    });
    const wrapper = mount(Comp);
    try {
      scroller.scrollTop = 120;
      scroller.dispatchEvent(new Event('scroll'));
      await nextTick();
      scroller.scrollTop = 0;
      scroller.dispatchEvent(new Event('scroll'));
      await nextTick();
      expect(seen).toEqual(['surface-elevated', 'bg-secondary']);
    } finally {
      wrapper.unmount();
      scroller.remove();
    }
  });

  it('滚动真的把状态写到了元素属性上', async () => {
    const scroller = makeScroller({ scrollable: true });
    const header = document.createElement('header');
    scroller.append(header);
    document.body.append(scroller);

    const Comp = defineComponent({
      setup() {
        const navRef = ref<HTMLElement | null>(header);
        useScrollEdge(navRef);
        return () => h('div');
      },
    });
    const wrapper = mount(Comp);
    try {
      expect(header.getAttribute('data-scroll-edge')).toBe('free');

      scroller.scrollTop = 240;
      scroller.dispatchEvent(new Event('scroll'));
      await nextTick();
      expect(header.getAttribute('data-scroll-edge')).toBe('under');

      scroller.scrollTop = 0;
      scroller.dispatchEvent(new Event('scroll'));
      await nextTick();
      expect(header.getAttribute('data-scroll-edge')).toBe('free');
    } finally {
      wrapper.unmount();
      scroller.remove();
    }
  });
});
