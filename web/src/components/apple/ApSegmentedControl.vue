<template>
  <div :style="containerStyle" role="tablist" :aria-label="ariaLabel || groupLabel">
    <button
      v-for="(option, idx) in options"
      :key="option.value"
      :style="buttonStyle(option, idx)"
      :disabled="option.disabled"
      role="tab"
      :aria-selected="modelValue === option.value"
      @click="select(option.value)"
      @focus="focusedIndex = idx"
      @blur="focusedIndex = -1"
      @keydown="handleKeydown($event, idx)"
    >
      <component :is="option.icon" v-if="option.icon && isComponent(option.icon)" :size="16" />
      <span v-else-if="option.icon" :style="iconStyle">{{ option.icon }}</span>
      {{ option.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue';
import {computed, shallowRef} from 'vue';
import { AppleSegmentSize, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleInteractionState } from '@/utils/core/apple-tokens';

export interface SegmentedOption {
  label: string;
  value: string | number;
  icon?: string | Component;
  disabled?: boolean;
}

const props = withDefaults(defineProps<{
  modelValue: string | number;
  options: SegmentedOption[];
  size?: keyof typeof AppleSegmentSize;
  /** 分组名。缺省时用各选项标题拼，读屏至少能说清「这是切什么的」 */
  ariaLabel?: string;
  /** 窄容器里允许换行成两排（P1-7：390 宽时 4 个意图项横向滚动会切掉最后一项） */
  wrap?: boolean;
}>(), {
  size: 'medium',
  ariaLabel: '',
  wrap: false,
});

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | number): void;
}>();

const groupLabel = computed(() => props.options.map((o) => o.label).join(' / '));

function isComponent(icon: string | Component): icon is Component {
  return typeof icon !== 'string';
}

function select(value: string | number) {
  emit('update:modelValue', value);
}

const focusedIndex = shallowRef(-1);
// 按压反馈不在这里做：注入层的 `[role="tab"]:active { opacity: .62 }` 全站兜底。
// 原先组件自己记 pressedIndex，而 pointerup 会因为重渲染丢掉 —— 那一档就永远停在
// "看着像被按着"，`check:motion` M1 报的"按下与悬停完全相同"就是它的残影。

function handleKeydown(e: KeyboardEvent, idx: number) {
  const currentOption = props.options[idx];
  if (!currentOption) return;
  
  const enabledOptions = props.options.filter(o => !o.disabled);
  const enabledIdx = enabledOptions.findIndex(o => o.value === currentOption.value);
  if (enabledIdx === -1) return;
  
  if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
    e.preventDefault();
    const dir = e.key === 'ArrowRight' ? 1 : -1;
    let next = enabledIdx + dir;
    if (next < 0) next = enabledOptions.length - 1;
    if (next >= enabledOptions.length) next = 0;
    const nextOption = enabledOptions[next];
    if (nextOption) {
      select(nextOption.value);
    }
  }
}

const containerStyle = computed(() => ({
  display: props.wrap ? 'flex' : 'inline-flex',
  ...(props.wrap ? { flexWrap: 'wrap' as const } : {}),
  // 轨道是**控件表面**，不是实心灰块：light-gray 是中性 palette，玻璃档下仍是实心 hex，
  // 于是这条轨道成了页面上少数几块"完全不透"的面 —— 用户报「按钮等元素的玻璃效果没了」。
  // 换到 surface-field（glass 下是极淡的半透明 ink）后，它透出环境色相；
  // 实心/纸感外观下 surface-field 自动退回该外观自己的实心值，语义不丢。
  background: cv('surface-field'),
  // 轨道坐在哪块板上是未知的（文档树里坐 chrome、工具条里坐 canvas），
  // 只靠底色差表达凹陷在暗 canvas 上量出来只有 Δ3 —— 等于没有。
  // 补一条发丝边：凹陷的第二种画法，与所在板明度无关。
  border: `1px solid ${cv('border-light')}`,
  // 轨道同样是流水面里的控件（一页可能有好几条），不付模糊 —— 红线 ≤3，透出环境色相由 surface-field 给。
  borderRadius: AppleRadius['22px'],
  // 纵向内距从 sm(8) 收到 xs(4)：外盒 = 分段项高 + 2×内距，原先 small 档量出来是 48px，
  // 而同一条工具条里的搜索框 / ApSelect / 主按钮都是 40px —— 药丸比它包着的控件还高一档。
  // 收 4px 之后 small 外盒正好 40（32 + 4×2），与同行控件同高；横向仍留 8px 给标签呼吸。
  // 内距只取 AppleSpace 的档，E6 才不会被"凑出来的数"抓到。
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  // 分段项是 flexShrink: 0 + nowrap（中文被挤窄会逐字断行，媒体页实测过），
  // 所以父容器一窄下来项就会爬出这颗灰色药丸。长内容抗压巡检量到越界 212px。
  // 药丸自己横向滚动是唯一既不断行、又不让标签画到邻居身上的解。
  maxWidth: '100%',
  overflowX: props.wrap ? 'visible' as const : 'auto' as const,
  gap: AppleSpace.xs }));

function buttonStyle(option: SegmentedOption, idx: number) {
  const isActive = props.modelValue === option.value;
  const isSmall = props.size === 'small';
  const isFocused = focusedIndex.value === idx;
  return {
    fontFamily: AppleType.bodyEmphasis.font,
    fontSize: isSmall ? AppleType.microUIBold.size : AppleType.bodyEmphasis.size,
    fontWeight: String(AppleType.bodyEmphasis.weight),
    lineHeight: '1',
    padding: isSmall ? AppleSegmentSize.small.padding : AppleSegmentSize.medium.padding,
    height: isSmall ? AppleSegmentSize.small.height : AppleSegmentSize.medium.height,
    // 分段项被挤窄时中文会逐字断行（媒体页实测「全部/图片/附件」全断成两行）
    whiteSpace: 'nowrap' as const,
    flexShrink: '0',
    // 选中项与轨道同一个圆角体系：轨道是 22px 药丸，选中块就得是胶囊。
    // 原先它用 control(10px) + surface-panel —— 一个方角块贴在药丸里，读成"换了一块板"，
    // 而不是"这一项抬起来了"（/space/1 文档树那排「全部/收藏/最近」实测）。
    borderRadius: isActive ? AppleRadius.pill : AppleRadius.control,
    border: 'none',
    cursor: option.disabled ? AppleInteractionState.disabledCursor : 'pointer',
    // 底色走 bg-hover（比所在板更亮的一档）而不是 surface-panel：panel 是内容板档，
    // 拿它当"选中"就是在一块板上又贴了一块同材质的板。
    background: isActive ? cv('bg-hover') : 'transparent',
    color: option.disabled
      ? cv('text-disabled')
      : isActive
        ? cv('text-primary')
        : cv('text-secondary'),
    // 选中块上缘补一道玻璃发丝（抬起的边），与控件档同源
    boxShadow: isFocused ? (isActive ? `${cv('shadow-level2')}, inset 0 0.5px 0 ${cv('glass-rim')}, ${cv('blue-focus-ring')}` : cv('blue-focus-ring')) : isActive ? `${cv('shadow-level2')}, inset 0 0.5px 0 ${cv('glass-rim')}` : 'none',
    transition: AppleTransition.smooth,
    // 只在禁用时写 opacity：内联声明的优先级高于注入层的 `[role="tab"]:active`，
    // 这里若恒写 '1'，全站那条按压兜底就被这一处静默废掉
    ...(option.disabled ? { opacity: AppleInteractionState.disabledOpacity } : {}),
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: AppleSpace.xs,
    minWidth: isSmall ? AppleSegmentSize.small.minWidth : AppleSegmentSize.medium.minWidth };
}

const iconStyle = computed(() => ({
  fontSize: AppleType.controlLabel.size,
  lineHeight: '1' }));
</script>
