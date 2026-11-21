<template>
  <label :for="inputId" :style="wrapperStyle" @mouseenter="hover = true" @mouseleave="hover = false">
    <span :style="boxStyle" @click.prevent="toggle">
      <span v-if="isChecked" :style="checkStyle">
        <Check :size="12" />
      </span>
    </span>
    <input
      :id="inputId"
      type="checkbox"
      :checked="isChecked"
      :disabled="disabled"
      :aria-checked="indeterminate ? 'mixed' : isChecked"
      :style="{ position: 'absolute', opacity: 0, width: 0, height: 0 }"
      @change="toggle"
      @focus="focused = true"
      @blur="focused = false"
    />
    <span v-if="label || $slots.default" :style="labelStyle">
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>

<script setup lang="ts">
import { computed, inject, shallowRef, useId } from 'vue';
import { fieldIdKey } from '@/utils/core/form-field-id';
import { Check } from 'lucide-vue-next';
import { AppleInteractionState, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleLineHeight } from '@/utils/core/apple-tokens';

const props = withDefaults(
  defineProps<{
    modelValue?: boolean;
    label?: string;
    disabled?: boolean;
    indeterminate?: boolean;
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

const isChecked = computed(() => props.modelValue);

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
  minHeight: '44px',
  minWidth: '44px',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
}));

const boxStyle = computed(() => {
  const base: Record<string, string> = {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '20px',
    height: '20px',
    borderRadius: AppleRadius.micro,
    border: isChecked.value
      ? `2px solid ${cv('blue')}`
      : `2px solid ${cv('soft-border-gray')}`,
    // 未选中态是**控件面**（surface-field，凹陷那一档），不是卡片面：
    // ApInput/ApTextarea/ApSelect 都取 field，原先这里取 panel，于是勾选框落在卡片上不凹陷。
    background: isChecked.value ? cv('blue') : cv('surface-field'),
    transition: AppleTransition.smooth,
    flexShrink: '0',
  };

  if (hover.value && !props.disabled) {
    if (!isChecked.value) {
      base.borderColor = cv('blue');
    }
  }

  if (props.indeterminate) {
    base.background = cv('blue');
    base.borderColor = cv('blue');
  }

  if (focused.value) {
    base.boxShadow = cv('blue-focus-ring');
  }

  return base;
});

const checkStyle = computed(() => ({
  // 勾落在一块实心 `cv('blue')` 上（选中态与 indeterminate 都是），
  // 所以前景必须由色值决定（on-blue = contrastInkOn(blue)），不是恒白的 on-accent ——
  // 个性化批3 加了强调色档位，深色外观那一列的白字只有 2.3~2.6:1。
  color: cv('on-blue'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  lineHeight: '1',
}));

const labelStyle = computed(() => ({
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: props.disabled ? cv('text-disabled') : cv('text-primary'),
  lineHeight: AppleLineHeight.loose,
}));
</script>
