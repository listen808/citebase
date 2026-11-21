// 回归护栏：全站进度条只有 ApProgress 一个实现（2026-10-02 收了 15 处手搓轨道）。
// 这里钉住四件当时各不相同的事：高度档位、scaleX 而非动 width、可访问身份、色档阈值只有一套。
import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import ApProgress from './ApProgress.vue';
import { AppleSpace } from '@/utils/core/apple-tokens';
import { toneForLoad, toneForScore } from '@/utils/core/progress-tone';

const mountBar = (props: Record<string, unknown>) =>
  mount(ApProgress, { props: { value: 50, label: '完成度', ...props } as never });

// happy-dom 的 el.style.transform 恒空（它没实现这个 camelCase getter），
// 但 style 属性是对的 —— 所以这里读属性串，别误判成组件没渲染出变换。
// 填充层是根节点唯一的子 div（querySelectorAll 只往下找，不含根节点本身）。
const fillOf = (w: { element: Element }) => w.element.querySelectorAll('div')[0] as HTMLElement;

const declared = (el: unknown, prop: string) =>
  ((el as HTMLElement).getAttribute('style') || '').split(';')
    .map((s) => s.trim())
    .find((s) => s.startsWith(`${prop}:`))
    ?.slice(prop.length + 1)
    .trim();

describe('ApProgress', () => {
  it('三档高度都取自 AppleSpace 的纵向尺，不是自造像素', () => {
    const expected = { hair: AppleSpace.xxs, thin: AppleSpace.xs, bar: AppleSpace.sm };
    for (const [size, height] of Object.entries(expected)) {
      expect(declared(mountBar({ size }).element, 'height'), `size=${size}`).toBe(height);
    }
  });

  it('填充用 scaleX，不用百分比宽度 —— 进度每帧都在变，动 width 就是每帧一次重排', () => {
    const fill = fillOf(mountBar({ value: 42 }));
    expect(declared(fill, 'transform')).toBe('scaleX(0.42)');
    expect(declared(fill, 'width')).toBe('100%');
  });

  it('值越界只裁不假报：视觉裁到 0/1，可访问值仍是原值取整', () => {
    const over = mountBar({ value: 150 });
    expect(declared(fillOf(over), 'transform')).toBe('scaleX(1)');
    expect(over.attributes('aria-valuenow')).toBe('150');
    const under = mountBar({ value: -8 });
    expect(declared(fillOf(under), 'transform')).toBe('scaleX(0)');
    expect(under.attributes('aria-valuenow')).toBe('-8');
  });

  it('带 role=progressbar 与可访问名字（原先 13 处手搓条一个都没有）', () => {
    const w = mountBar({ label: '向量完成度' });
    expect(w.attributes('role')).toBe('progressbar');
    expect(w.attributes('aria-label')).toBe('向量完成度');
    expect(w.attributes('aria-valuemin')).toBe('0');
    expect(w.attributes('aria-valuemax')).toBe('100');
  });

  it('edge 变体钉在视口顶边且不画轨道底色（路由进度与阅读进度共用同一套）', () => {
    const edge = mountBar({ size: 'hair', edge: true }).element;
    expect(declared(edge, 'position')).toBe('fixed');
    expect(declared(edge, 'background-color')).toBe('transparent');
    const normal = mountBar({}).element;
    expect(declared(normal, 'position')).toBeUndefined();
    expect(declared(normal, 'background-color')).not.toBe('transparent');
  });
});

describe('色档阈值只有一套', () => {
  it('越高越好：0.8 / 0.5 两条线', () => {
    expect(toneForScore(0.92)).toBe('success');
    expect(toneForScore(0.8)).toBe('success');
    expect(toneForScore(0.62)).toBe('warning');
    expect(toneForScore(0.49)).toBe('danger');
  });

  it('越高越糟：默认 70 / 85，资源自己的安全线由调用方给', () => {
    expect(toneForLoad(45)).toBe('accent');
    expect(toneForLoad(75)).toBe('warning');
    expect(toneForLoad(88)).toBe('danger');
    expect(toneForLoad(82, 80, 90)).toBe('warning');
  });
});
