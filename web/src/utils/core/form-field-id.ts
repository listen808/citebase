import type { InjectionKey } from 'vue';

/**
 * ApFormItem 往下发的字段 id。
 * 没有这条注入时，ApFormItem 给 <label for> 生成一个 id、控件内部又生成另一个 id，
 * 两者永不相等 —— 点标签不聚焦、错误文案读屏关联不上。
 */
export const fieldIdKey: InjectionKey<string> = Symbol('ap-field-id');
