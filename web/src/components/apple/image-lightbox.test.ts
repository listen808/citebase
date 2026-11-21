import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import ApImageLightbox from './ApImageLightbox.vue';

/**
 * 灯箱的四条契约：dialog 语义、每个可聚焦控件都有名字、多图才有翻页、翻页会往外说。
 * 这四条在并档前是"三处有一处没有"的状态，而它只在点开之后才存在，
 * 运行时巡检（只看渲染态）与按文件扫的静态门禁都够不着，所以钉在这里。
 */
const ITEMS = [
  { src: '/a.png', name: '架构图.png', meta: '1.2 MB' },
  { src: '/b.png', name: '流程图.png', meta: '820 kB' },
];

const mk = (props: Record<string, unknown> = {}) => mount(ApImageLightbox, {
  props: { modelValue: true, items: ITEMS, index: 0, ...props },
  global: { stubs: { teleport: true } },
  attachTo: document.body,
});

describe('ApImageLightbox', () => {
  it('关着的时候什么都不挂', () => {
    const w = mount(ApImageLightbox, {
      props: { modelValue: false, items: ITEMS },
      global: { stubs: { teleport: true } },
    });
    expect(w.find('[role="dialog"]').exists()).toBe(false);
  });

  it('打开就是一个模态对话框，带可访问名字', () => {
    const dlg = mk().get('[role="dialog"]');
    expect(dlg.attributes('aria-modal')).toBe('true');
    expect(dlg.attributes('aria-label')).toBe('图片预览');
  });

  it('关闭按钮、翻页按钮都有可访问名字（图标按钮不能靠图标自己说明）', () => {
    const names = mk().findAll('button').map((b) => b.attributes('aria-label'));
    expect(names.every((n) => !!n && n.trim().length > 0)).toBe(true);
    expect(names).toContain('上一张');
    expect(names).toContain('下一张');
  });

  it('单图不给翻页按钮', () => {
    const w = mount(ApImageLightbox, {
      props: { modelValue: true, items: ITEMS.slice(0, 1) },
      global: { stubs: { teleport: true } },
    });
    const names = w.findAll('button').map((b) => b.attributes('aria-label'));
    expect(names).not.toContain('下一张');
    expect(names).not.toContain('上一张');
  });

  it('翻页往外报下标，并从当前下标起显示', async () => {
    const w = mk({ index: 1 });
    expect(w.html()).toContain('流程图.png');
    await w.findAll('button').find((b) => b.attributes('aria-label') === '下一张')?.trigger('click');
    expect(w.emitted('update:index')?.at(-1)).toEqual([0]);   // 末张再翻回到第一张
  });

  it('说明行同时给文件名、大小与计数', () => {
    expect(mk().text()).toContain('架构图.png · 1.2 MB · 1 / 2');
  });

  it('Esc 关掉：键盘用户进得来也要出得去', async () => {
    const w = mk();
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await w.vm.$nextTick();
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([false]);
  });
});
