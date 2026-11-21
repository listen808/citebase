import type { IconNode } from 'morphicons/vue';

/**
 * morphicons 形变需要的图标几何数据（lucide 的 IconNode=[tag,attrs][] 格式）。
 * 刻意内联、不引 `lucide` 数据包：那个包在 Vite dev 里反复出现 "Failed to resolve import"，
 * 且为 5 个图标多引一整个运行时依赖不划算。几何与 lucide 一致；若日后要加形变对，
 * 从这里补对应 IconNode 即可（保持 viewBox 24、stroke 由 ApMorphIcon 统一给）。
 */
export const ChevronLeft: IconNode = [['path', { d: 'm15 18-6-6 6-6' }]];
export const ChevronRight: IconNode = [['path', { d: 'm9 18 6-6-6-6' }]];
export const ChevronDown: IconNode = [['path', { d: 'm6 9 6 6 6-6' }]];
export const Maximize2: IconNode = [
  ['path', { d: 'M15 3h6v6' }],
  ['path', { d: 'm21 3-7 7' }],
  ['path', { d: 'm3 21 7-7' }],
  ['path', { d: 'M9 21H3v-6' }],
];
export const Minimize2: IconNode = [
  ['path', { d: 'm14 10 7-7' }],
  ['path', { d: 'M20 10h-6V4' }],
  ['path', { d: 'm3 21 7-7' }],
  ['path', { d: 'M4 14h6v6' }],
];
