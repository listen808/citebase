<template>
  <div :style="wrapperStyle">
    <input
      :id="inputId"
      class="apple-focus-ring ap-slider"
      :class="{ 'is-disabled': disabled, 'is-hovered': hovered }"
      :value="modelValue"
      type="range"
      :min="min"
      :max="max"
      :step="step"
      :disabled="disabled"
      :style="sliderStyle"
      :aria-label="ariaLabel"
      @input="handleInput"
      @mouseenter="hovered = true"
      @mouseleave="hovered = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, ref, useId } from 'vue';
import { fieldIdKey } from '@/utils/core/form-field-id';
import { AppleInputSize, AppleTiming, AppleInteractionState } from '@/utils/core/apple-tokens';

const props = withDefaults(defineProps<{
  modelValue: number;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  ariaLabel?: string;
}>(), {
  min: 0,
  max: 1,
  step: 0.1,
  disabled: false,
});

const emit = defineEmits<{
  (e: 'update:modelValue', value: number): void;
}>();

// 外层 ApFormItem 的 <label for> 用的是它下发的 id；滑块不接就永远悬空，
// 点标签跳不到控件、读屏也念不出这块 label。
const inputId = inject(fieldIdKey, undefined) ?? useId();

const hovered = ref(false);

function handleInput(e: Event) {
  const target = e.target as HTMLInputElement;
  emit('update:modelValue', Number(target.value));
}

const wrapperStyle = {
  width: '100%',
};

const sliderStyle = computed(() => ({
  width: '100%',
  // 交互条 32px、视觉轨道仍是 4px —— 原先 input 自己就是 4px 高的背景条，
  // 鼠标要命中那 4 个像素才能拖动，也是全站唯一一个不在控件天阶上的表单控件
  // （运行时巡检 E4 实测 input[range] 链条 4px>18px>65px>139px，没有一档落在
  // 24/28/32/40/44 上）。现在背景交给 ::-webkit-slider-runnable-track 画，
  // input 本身只是热区，取 small 档 32px 与同一行的输入框/下拉同高。
  height: AppleInputSize.small.height,
  appearance: 'none' as const,
  WebkitAppearance: 'none' as const,
  background: 'transparent',
  cursor: props.disabled ? AppleInteractionState.disabledCursor : 'pointer',
  opacity: props.disabled ? AppleInteractionState.disabledOpacity : '1',
  transition: `opacity ${AppleTiming.fast}`,
  // P1-8：已填充段。轨道伪元素拿不到 value，由这里把比例写成自定义属性，
  // 轨道背景用它做两段渐变（左蓝右灰）。原先 4px 的 bg-tertiary 压在白卡上几乎看不见。
  '--ap-slider-fill': `${fillPercent.value}%`,
}));

const fillPercent = computed(() => {
  const span = props.max - props.min;
  if (span <= 0) return 0;
  return Math.min(100, Math.max(0, ((props.modelValue - props.min) / span) * 100));
});
</script>

<style scoped>
/* 滑块拇指样式 — 需要 ::-webkit-slider-thumb 伪类，无法用 inline :style 实现 */
input[type="range"].ap-slider::-webkit-slider-thumb {
  appearance: none;
  -webkit-appearance: none;
  width: var(--apple-slider-thumb-size);
  height: var(--apple-slider-thumb-size);
  /* input 由 4px 变 32px 后，WebKit 不再把拇指对齐到细轨道中心，
     要自己把它压回轨道中线上（含 2px 描边的外框高度） */
  margin-top: calc((var(--apple-slider-track-height) - var(--apple-slider-thumb-size)) / 2 - var(--apple-slider-thumb-border));
  border-radius: 50%;
  background: var(--apple-blue);
  cursor: pointer;
  border: var(--apple-slider-thumb-border) solid var(--apple-text-primary);
  box-shadow: var(--apple-shadow-thumb);
  transition: transform var(--apple-dur-fast) var(--apple-easing-standard), box-shadow var(--apple-dur-fast) var(--apple-easing-standard);
}

input[type="range"].ap-slider:hover::-webkit-slider-thumb {
  transform: scale(1.15);
  box-shadow: var(--apple-shadow-thumb-hover);
}

input[type="range"].ap-slider:active::-webkit-slider-thumb {
  transform: scale(0.97);
}

input[type="range"].ap-slider:focus-visible::-webkit-slider-thumb {
  box-shadow: 0 0 0 3px var(--apple-focus-ring-color);
}

input[type="range"].ap-slider.is-disabled::-webkit-slider-thumb {
  cursor: not-allowed;
}

input[type="range"].ap-slider::-moz-range-thumb {
  width: var(--apple-slider-thumb-size);
  height: var(--apple-slider-thumb-size);
  border-radius: 50%;
  background: var(--apple-blue);
  cursor: pointer;
  border: var(--apple-slider-thumb-border) solid var(--apple-text-primary);
  box-shadow: var(--apple-shadow-thumb);
  transition: transform var(--apple-dur-fast) var(--apple-easing-standard), box-shadow var(--apple-dur-fast) var(--apple-easing-standard);
}

input[type="range"].ap-slider:hover::-moz-range-thumb {
  transform: scale(1.15);
  box-shadow: var(--apple-shadow-thumb-hover);
}

input[type="range"].ap-slider:active::-moz-range-thumb {
  transform: scale(0.97);
}

input[type="range"].ap-slider:focus-visible::-moz-range-thumb {
  box-shadow: 0 0 0 3px var(--apple-focus-ring-color);
}

input[type="range"].ap-slider::-webkit-slider-runnable-track {
  height: var(--apple-slider-track-height);
  border-radius: var(--apple-radius-micro);
  /* input 本身改成 32px 的热区后，看得见的细轨道由这里画。
     未填充段用 border 色（原先 bg-tertiary 在白卡上等于没有轨道），已填充段由 --ap-slider-fill 切分。 */
  background: linear-gradient(
    to right,
    var(--apple-blue) 0 var(--ap-slider-fill, 0%),
    var(--apple-border) var(--ap-slider-fill, 0%) 100%
  );
}

input[type="range"].ap-slider::-moz-range-track {
  height: var(--apple-slider-track-height);
  border-radius: var(--apple-radius-micro);
  background: var(--apple-border);
}

input[type="range"].ap-slider::-moz-range-progress {
  height: var(--apple-slider-track-height);
  border-radius: var(--apple-radius-micro);
  background: var(--apple-blue);
}
</style>
