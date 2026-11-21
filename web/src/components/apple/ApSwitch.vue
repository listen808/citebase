<template>
  <label :for="inputId" :style="wrapperStyle" @mouseenter="hover = true" @mouseleave="hover = false">
    <span :style="trackStyle" @click.prevent="toggle">
      <span :style="thumbStyle" />
    </span>
    <input
      :id="inputId"
      role="switch"
      type="checkbox"
      :checked="modelValue"
      :disabled="disabled"
      :style="{ position: 'absolute', opacity: 0, width: 0, height: 0 }"
      :aria-checked="modelValue"
      :aria-label="label || undefined"
      @change="toggle"
      @focus="focused = true"
      @blur="focused = false"
    />
    <span v-if="label" :style="labelStyle">{{ label }}</span>
  </label>
</template>

<script setup lang="ts">
import { computed, inject, shallowRef, useId } from 'vue';
import { fieldIdKey } from '@/utils/core/form-field-id';
import { useMobile } from '@/composables/ui/useMobile';
import { AppleSwitchSize, AppleInteractionState, AppleRadius, AppleSpace, AppleTiming, AppleType, AppleLayout, cv, AppleLineHeight } from '@/utils/core/apple-tokens';

const props = withDefaults(
  defineProps<{
    modelValue?: boolean;
    label?: string;
    disabled?: boolean;
  }>(),
  {
    modelValue: false,
  },
);

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
}>();

const hover = shallowRef(false);
const focused = shallowRef(false);
const inputId = inject(fieldIdKey, undefined) ?? useId();
const { isMobile } = useMobile();

function toggle() {
  if (props.disabled) return;
  emit('update:modelValue', !props.modelValue);
}

const wrapperStyle = computed(() => ({
  display: 'inline-flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  cursor: props.disabled ? AppleInteractionState.disabledCursor : 'pointer',
  opacity: props.disabled ? AppleInteractionState.disabledOpacity : '1',
  userSelect: 'none' as const,
  position: 'relative' as const,
  // 44px 触控底线只对触屏成立。桌面上把开关抬到 44，同一行 32px 的按钮就一高一低——
  // ApButton 已按这条规则修过（见其 minHeight 注释），开关是漏网的那一个。
  minHeight: isMobile.value ? AppleLayout.touchTargetMin : undefined,
}));

const trackStyle = computed(() => {
  const base: Record<string, string | number> = {
    display: 'inline-block',
    width: AppleSwitchSize.trackWidth,
    height: AppleSwitchSize.trackHeight,
    borderRadius: AppleRadius.control,
    background: props.modelValue ? cv('green') : cv('bg-tertiary'),
    transition: `background ${AppleTiming.smooth}`,
    flexShrink: '0',
    position: 'relative' as const,
  };
  if (hover.value && !props.disabled) {
    if (!props.modelValue) {
      base.background = cv('soft-border-gray');
    }
  }
  if (focused.value) {
    base.boxShadow = cv('blue-focus-ring');
  }
  return base;
});

/** 滑块行程 = 到位位置 - 起始间隙 */
const THUMB_SHIFT = parseInt(AppleSwitchSize.thumbTravel, 10) - parseInt(AppleSwitchSize.thumbGap, 10);

const thumbStyle = computed(() => ({
  position: 'absolute' as const,
  top: '2px',
  left: AppleSwitchSize.thumbGap,
  // 位移走 transform：动 `left` 每帧都在重排（全站唯一一个还在动布局属性的控件），
  // 而 bounce 档的过冲会连着把轨道右边缘一起拽抖。位移量仍由两个尺寸令牌算出来，
  // 不另写第四个常量。
  transform: `translateX(${props.modelValue ? THUMB_SHIFT : 0}px)`,
  width: AppleSwitchSize.thumbSize,
  height: AppleSwitchSize.thumbSize,
  borderRadius: AppleRadius.circle,
  background: cv('surface-knob'),
  boxShadow: cv('shadow-level2'),
  transition: `transform ${AppleTiming.bounce}`,
}));

const labelStyle = computed(() => ({
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: props.disabled ? cv('text-quaternary') : cv('text-primary'),
  lineHeight: AppleLineHeight.loose,
}));
</script>
