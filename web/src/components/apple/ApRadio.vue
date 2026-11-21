<template>
  <label :for="inputId" :style="wrapperStyle" @mouseenter="hover = true" @mouseleave="hover = false">
    <span :style="circleStyle" @click.prevent="select">
      <span v-if="isSelected" :style="dotStyle" />
    </span>
    <input
      :id="inputId"
      type="radio"
      :checked="isSelected"
      :disabled="disabled"
      :value="value"
      :name="name"
      :aria-checked="isSelected"
      :style="{ position: 'absolute', opacity: 0, width: 0, height: 0 }"
      @change="select"
      @focus="focused = true"
      @blur="focused = false"
    />
    <span v-if="label" :style="labelStyle">{{ label }}</span>
  </label>
</template>

<script setup lang="ts">
import { computed, shallowRef, useId } from 'vue';
import { AppleInteractionState, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleLineHeight } from '@/utils/core/apple-tokens';

const props = withDefaults(
  defineProps<{
    modelValue?: string | number | null;
    value: string | number;
    label?: string;
    disabled?: boolean;
    name?: string;
  }>(),
  {
    modelValue: null,
  },
);

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | number): void;
}>();

const hover = shallowRef(false);
const focused = shallowRef(false);
const inputId = useId();

const isSelected = computed(() => props.modelValue === props.value);

function select() {
  if (props.disabled) return;
  emit('update:modelValue', props.value);
}

const wrapperStyle = computed(() => ({
  display: 'inline-flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  cursor: props.disabled ? AppleInteractionState.disabledCursor : 'pointer',
  opacity: props.disabled ? AppleInteractionState.disabledOpacity : '1',
  userSelect: 'none' as const,
  position: 'relative' as const,
}));

const circleStyle = computed(() => {
  const base: Record<string, string> = {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '20px',
    height: '20px',
    borderRadius: AppleRadius.circle,
    border: isSelected.value
      ? `2px solid ${cv('blue')}`
      : `2px solid ${cv('soft-border-gray')}`,
    // 与 ApCheckbox 同一条：未选中的控件面取 surface-field，不取卡片面 surface-panel
    background: cv('surface-field'),
    transition: AppleTransition.smooth,
    flexShrink: '0',
  };

  if (hover.value && !props.disabled && !isSelected.value) {
    base.borderColor = cv('blue');
  }

  if (focused.value) {
    base.boxShadow = cv('blue-focus-ring');
  }

  return base;
});

const dotStyle = computed(() => ({
  width: '10px',
  height: '10px',
  borderRadius: AppleRadius.circle,
  background: cv('blue'),
}));

const labelStyle = computed(() => ({
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: props.disabled ? cv('text-disabled') : cv('text-primary'),
  lineHeight: AppleLineHeight.loose,
}));
</script>
