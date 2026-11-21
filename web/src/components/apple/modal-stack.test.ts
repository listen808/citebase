import { describe, expect, it, afterEach } from 'vitest';
import { defineComponent, h, nextTick, reactive } from 'vue';
import { flushPromises, mount } from '@vue/test-utils';
import ApModal from './ApModal.vue';
import { pushModal, popModal, isTopModal, openModalCount } from '@/composables/ui/useModalStack';

/**
 * 钉住"叠层弹窗的键盘归属只有一份栈"这件事。
 *
 * 为什么值得单独成文：这条栈原先声明在 ApModal 的 `<script setup>` 里，于是每个实例各持
 * 一份只含自己的数组，Esc 的"我在栈顶吗"恒真 —— 两层弹窗同开时一次 Esc 全关。
 * 而它在**单层**场景下表现得完全正确，运行时巡检只看得到渲染态，静态门禁读不出作用域问题，
 * 所以只有"两层同开按一次 Esc"这条用例能抓住它。
 */

const mounted: ReturnType<typeof mount>[] = [];

/** 卸载并从登记表里摘掉，避免 afterEach 再 unmount 一次（happy-dom 下二次卸载会抛） */
function dispose(wrapper: ReturnType<typeof mount>) {
  const i = mounted.indexOf(wrapper);
  if (i > -1) mounted.splice(i, 1);
  wrapper.unmount();
}

afterEach(() => {
  // 栈是模块级的，用例之间必须排干净，否则上一例残留的那层会让下一例的栈顶判断错位
  while (mounted.length) mounted.pop()!.unmount();
  expect(openModalCount()).toBe(0);
});

function mkStacked() {
  const state = reactive({ outer: true, inner: true });
  const wrapper = mount(
    defineComponent({
      setup: () => () =>
        h('div', [
          h(
            ApModal,
            {
              modelValue: state.outer,
              title: '外层',
              'onUpdate:modelValue': (v: boolean) => {
                state.outer = v;
              },
            },
            { default: () => h('span', 'OUTER') },
          ),
          h(
            ApModal,
            {
              modelValue: state.inner,
              title: '内层',
              'onUpdate:modelValue': (v: boolean) => {
                state.inner = v;
              },
            },
            { default: () => h('span', 'INNER') },
          ),
        ]),
    }),
    {
      global: { stubs: { teleport: true }, mocks: { $t: (k: string) => k } },
      attachTo: document.body,
    },
  );
  mounted.push(wrapper);
  return { wrapper, state };
}

async function pressEscape() {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await flushPromises();
  await nextTick();
}

describe('useModalStack', () => {
  it('栈空时谁都不是栈顶（不能让"没入栈"误判成"我最大"）', () => {
    expect(openModalCount()).toBe(0);
    expect(isTopModal('a')).toBe(false);
  });

  it('后入栈的才是栈顶，pop 掉它就把栈顶交还给前一层', () => {
    pushModal('a');
    pushModal('b');
    expect(isTopModal('b')).toBe(true);
    expect(isTopModal('a')).toBe(false);
    popModal('b');
    expect(isTopModal('a')).toBe(true);
    popModal('a');
  });

  it('重复入栈只算一层，pop 不存在的 id 是空操作', () => {
    pushModal('a');
    pushModal('a');
    expect(openModalCount()).toBe(1);
    popModal('zzz');
    expect(openModalCount()).toBe(1);
    popModal('a');
    expect(openModalCount()).toBe(0);
  });
});

describe('ApModal 叠层', () => {
  it('两层同开，按一次 Esc 只关最上面那层', async () => {
    const { state } = mkStacked();
    expect(openModalCount()).toBe(2);

    await pressEscape();

    expect(state.inner).toBe(false);
    // 这条就是原来失效的那半边：每实例一份栈时，两个 keydown 监听都认为自己最大
    expect(state.outer).toBe(true);
  });

  it('上层关掉后，下一次 Esc 交给下层', async () => {
    const { state } = mkStacked();
    await pressEscape();
    await pressEscape();
    expect(state.inner).toBe(false);
    expect(state.outer).toBe(false);
    expect(openModalCount()).toBe(0);
  });

  it('开着的时候被卸载，必须自己出栈（否则栈只进不出，后面的弹窗再也收不到 Esc）', async () => {
    const { wrapper } = mkStacked();
    expect(openModalCount()).toBe(2);
    dispose(wrapper);
    expect(openModalCount()).toBe(0);
  });
});
