// 回归护栏：<label for> 必须指向真实存在的控件 id。
// 起因：ApFieldWrapper 加了 associate?: boolean 而没写默认值，Vue 对布尔 prop 做缺失转换，
// 「没传」等于 false，于是全站 label 的 for 一次性消失——静态门禁全绿，只有浏览器能发现。
import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import ApInput from './ApInput.vue';
import ApTextarea from './ApTextarea.vue';
import ApCheckbox from './ApCheckbox.vue';
import ApSwitch from './ApSwitch.vue';

const pairs = [
  ['ApInput', ApInput, { modelValue: '', label: '用户名' }],
  ['ApTextarea', ApTextarea, { modelValue: '', label: '正文' }],
  ['ApCheckbox', ApCheckbox, { modelValue: false, label: '记住我' }],
  ['ApSwitch', ApSwitch, { modelValue: false, label: '自动刷新' }],
] as const;

describe('表单控件的 label 关联', () => {
  for (const [name, component, props] of pairs) {
    it(`${name} 带 label 时 for 指向控件自身 id`, () => {
      const w = mount(component as never, { props: props as never });
      const label = w.find('label[for]');
      const control = w.find('input, textarea');
      expect(label.exists(), `${name} 没渲染出 label[for]`).toBe(true);
      expect(control.attributes('id'), `${name} 的控件没有 id`).toBeTruthy();
      expect(label.attributes('for')).toBe(control.attributes('id'));
    });
  }

  it('ApInput 只有 placeholder 时不假装有关联', () => {
    const w = mount(ApInput, { props: { modelValue: '', placeholder: '搜索' } });
    expect(w.find('label[for]').exists()).toBe(false);
  });
});
