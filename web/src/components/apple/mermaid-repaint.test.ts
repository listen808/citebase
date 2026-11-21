import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { createRouter, createMemoryHistory } from 'vue-router';

/**
 * mermaid 图必须跟着主题重画。
 *
 * 这条测试存在的理由是一段真实缺口：ApMarkdownPreview 的主题 observer 原先只做
 * `clearRenderCache() + extractAndApplyThemeColors()` —— 前者清的是**模块里的 HTML 缓存**，
 * 后者重注入的是**代码块配色**，而已经插进 DOM 的那张 SVG 谁也没动。
 * 于是切到深色，流程图还是浅底，直到内容变一次才被动更新。
 *
 * `mermaid.render()` 出来的 svg 里 fill/stroke 是烤死的字符串，所以唯一的修法是按当前
 * 主题重跑一次渲染。这里把 mermaid 换成假实现，数它被调了几次 ——
 * **假实现是故意的**：happy-dom 里跑真 mermaid 会因为量不到布局而超时，
 * 而这条判据要证的从来不是"图好不好看"，是"换主题有没有再要一张"。
 */
const renderCalls: string[] = [];
/**
 * 默认立刻兑现 render 的 promise（其余四条判据要的是"有没有再要一张"）。
 * `defer` 打开时把它挂在空中不落地 —— 这是造出真竞态的唯一办法：
 * 主题变化落在 `mermaid.initialize()` 之后、SVG 插入 DOM 之前那一段窗口里，
 * 重绘那一路找不到节点，而首张图又是按**旧**主题画的。
 * 不在 initialize 之前换（那一张画出来就是新主题，只画一次才是正确行为），
 * 这一点在上一版判错过，于是测试期望了第二次渲染。
 */
const gate = { deferred: false, release: null as null | (() => void) };
vi.mock('mermaid', () => ({
  default: {
    initialize: (cfg: { themeVariables?: Record<string, string> }) => {
      renderCalls.push(`init:${cfg?.themeVariables?.primaryTextColor ?? '?'}`);
    },
    render: (id: string) => {
      renderCalls.push(`render:${id}`);
      if (!gate.deferred) return Promise.resolve({ svg: '<svg data-mermaid="1"><rect fill="#f5f5f7"/></svg>' });
      return new Promise((resolve) => {
        gate.release = () => resolve({ svg: '<svg data-mermaid="1"><rect fill="#f5f5f7"/></svg>' });
      });
    },
  },
}));

import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';

const MERMAID_DOC = '```mermaid\ngraph TD;A-->B;\n```';
const root = () => document.documentElement;
const renders = () => renderCalls.filter((c) => c.startsWith('render:')).length;

/** ApMarkdownPreview 的 setup 里取了 UI/document store 与 router，挂载必须带上这两件插件 */
const mountPreview = (content: string) =>
  mount(ApMarkdownPreview, {
    props: { content },
    global: {
      plugins: [
        createPinia(),
        createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div />' } }] }),
      ],
    },
  });

/**
 * 首帧的 mermaid 渲染是 `import('mermaid')` 的 promise 尾巴，比 mount 晚若干个 tick。
 * 以前的写法是 `await settle()` 固定等 6 轮 —— 拿到几个全看环境快慢，
 * 同一份实现一会儿"重画了"一会儿"没重画"（实测三条断言两条这样红）。
 * 所以一律等**条件**，不等次数。
 */
async function waitRenders(atLeast: number) {
  await vi.waitFor(() => expect(renders()).toBeGreaterThanOrEqual(atLeast), { timeout: 3000, interval: 20 });
}

describe('ApMarkdownPreview 的 mermaid 重绘', () => {
  beforeEach(() => {
    renderCalls.length = 0;
    gate.deferred = false;
    gate.release = null;
    root().setAttribute('data-theme', 'light');
    root().removeAttribute('data-material');
    root().removeAttribute('data-glass');
  });
  afterEach(() => {
    root().removeAttribute('data-theme');
  });

  it('首帧画一次；换外观后重画，且重画前重新 initialize（主题变量是快照）', async () => {
    const wrapper = mountPreview(MERMAID_DOC);
    await waitRenders(1);
    expect(renders()).toBe(1);

    root().setAttribute('data-theme', 'dark');
    await waitRenders(2);
    // mermaid 的 themeVariables 在 initialize 时拍一次快照；不清 mermaidInitialized 的话，
    // 第二次 render 用的还是第一次的 primaryTextColor —— 那就等于没换色
    expect(renderCalls.filter((c) => c.startsWith('init:')).length).toBeGreaterThanOrEqual(2);
    wrapper.unmount();
  });

  it('换材质也要重画（玻璃与实心的 surface-panel 不是一个值）', async () => {
    const wrapper = mountPreview(MERMAID_DOC);
    await waitRenders(1);
    root().setAttribute('data-material', 'glass');
    await waitRenders(2);
    wrapper.unmount();
  }, 15000); // 连挂两次真组件：并发满载时 5s 不够

  it('透明变体也要重画（data-glass 以前根本不在 attributeFilter 里）', async () => {
    const wrapper = mountPreview(MERMAID_DOC);
    root().setAttribute('data-material', 'glass');
    await waitRenders(1);
    root().setAttribute('data-glass', 'clear');
    await waitRenders(2);
    wrapper.unmount();
  });

  it('一次主题操作连改三个属性只重画一次', async () => {
    const wrapper = mountPreview(MERMAID_DOC);
    await waitRenders(1);
    const before = renders();
    // 合并判据取的是"三个属性的当前签名"而不是"回调来了几次"：
    // 规范把同一微任务里的多条记录合成一次回调，happy-dom 一条派一次，
    // 按次数去重等于测环境（这点在这条测试上红过两次，方向还相反）。
    root().setAttribute('data-theme', 'dark');
    root().setAttribute('data-material', 'glass');
    root().setAttribute('data-glass', 'clear');
    await waitRenders(before + 1);
    await new Promise((r) => setTimeout(r, 80));
    expect(renders()).toBe(before + 1);
    wrapper.unmount();
  });

  /**
   * 这条是 pre-commit 跑红之后补的，而且它自己先判错过一次：
   * 一开始写成"mount 完立刻换主题"，结果只画一张 —— 那是**对的**，
   * 因为那次变化发生在 `initialize()` 之前，画出来的第一张本来就该是新主题。
   * 真正会丢的窗口是**变化落在 render 已发出、SVG 还没进 DOM 之间**：
   * 重绘那一路找不到节点（图还没进 DOM），而手上这张是按旧主题拍的快照。
   * 所以这里用 gate 把 promise 挂在空中不落地，把窗口精确造出来。
   */
  it('主题变化落在 render 在飞、图还没进 DOM 之间时，画完要按新主题补一次', async () => {
    gate.deferred = true;
    const wrapper = mountPreview(MERMAID_DOC);
    await waitRenders(1); // 此刻 mermaid.render() 已发出，promise 被我攥着
    expect(wrapper.element.querySelector(".mermaid-diagram")).toBeNull(); // 图确实还没进 DOM

    root().setAttribute('data-theme', 'dark');
    await new Promise((r) => setTimeout(r, 30)); // 让 observer 的微任务跑完：这一趟找不到节点，重绘 0 张

    gate.deferred = false; // 补的那一张要能落地
    gate.release?.();
    await waitRenders(2);
    expect(wrapper.element.querySelector(".mermaid-diagram")).not.toBeNull();
    gate.release = null;
    wrapper.unmount();
  });

  it('没有图的文档不因为换主题而重排', async () => {
    const wrapper = mountPreview('普通正文，没有 mermaid');
    await new Promise((r) => setTimeout(r, 60));
    const before = renderCalls.length;
    root().setAttribute('data-theme', 'editorial');
    await new Promise((r) => setTimeout(r, 120));
    expect(renderCalls.length).toBe(before);
    wrapper.unmount();
  });
});
