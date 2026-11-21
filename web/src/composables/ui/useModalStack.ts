import { ref, type Ref } from 'vue';

/**
 * 叠着的弹窗共享一份栈 —— 所以它必须在**模块作用域**。
 *
 * 原先 `const modalStack: ModalStackEntry[] = []` 写在 `ApModal.vue` 的 `<script setup>` 里，
 * 而 `<script setup>` 整块编译进 `setup()`，于是每个弹窗实例各持一份**只含自己**的数组：
 * Esc 的"我在栈顶吗"恒真、Tab 焦点陷阱的同一条守卫也恒真 ——
 * 叠层时一次 Esc 把所有层一起关掉，两个陷阱互抢焦点。
 * 那段机制写得很完整、注释也讲得很清楚，唯一的问题是它测的是自己那份空栈。
 *
 * 栈只管**键盘归属**，不管 z：弹窗全部恒取 `AppleZIndex.modal`，叠放次序由
 * `Teleport to="body"` + `v-if` 的 DOM 追加顺序决定 —— 后开的自然 append 在后面，
 * 与这里的入栈顺序是同一个事实源。想按深度抬 z 会一头撞进 `tooltip`(10000) 与
 * `toast`(10001) 两档，那是为了修一个不存在的问题去制造一个真问题。
 */
const openModalIds: Ref<string[]> = ref([]);

/** 入栈（同一 id 重复入栈忽略） */
export function pushModal(id: string): void {
  if (openModalIds.value.includes(id)) return;
  openModalIds.value.push(id);
}

export function popModal(id: string): void {
  const index = openModalIds.value.indexOf(id);
  if (index > -1) openModalIds.value.splice(index, 1);
}

/**
 * 只有栈顶那一个该接 Esc / Tab。
 * 不在栈里（已关或未开）一律 false —— 不能让"谁都不在栈上"误判成"我最大"。
 */
export function isTopModal(id: string): boolean {
  const ids = openModalIds.value;
  return ids.length > 0 && ids[ids.length - 1] === id;
}

/** 当前叠了几层。供测试钉住"关掉了就必须出栈"，漏 pop 会让栈只进不出 */
export function openModalCount(): number {
  return openModalIds.value.length;
}
