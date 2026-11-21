<template>
  <div :style="wrapperStyle">
    <label v-if="label" :style="computedLabelStyle" :for="associate ? fieldId : undefined">
      <span v-if="required" :style="requiredStyle">*</span>
      {{ label }}
    </label>
    <slot />
    <span v-if="error" :id="`${fieldId}-error`" :style="computedErrorStyle" role="alert">{{ error }}</span>
    <span v-else-if="hint" :style="hintStyle">{{ hint }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed, type CSSProperties } from 'vue';
import { labelStyle as formLabelStyle, requiredStyle as formRequiredStyle, errorStyle as formErrorStyle, wrapperStyle as formWrapperStyle } from '@/styles/component/form-field-styles';
import { AppleFont, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';

const props = withDefaults(defineProps<{
  label?: string;
  required?: boolean;
  error?: string;
  hint?: string;
  fieldId: string;
  /** 分组类控件（分段控件/单选组）不是 labelable 元素，for 关联不上；传 false 只出文字，不写悬空的 for。
      必须显式默认 true：Vue 对布尔 prop 做缺失转换，不写默认值时「没传」等于 false，会把所有 label 的 for 抹掉。 */
  associate?: boolean;
  labelStyle?: CSSProperties;
  errorStyle?: CSSProperties;
}>(), {
  associate: true,
});

const wrapperStyle = formWrapperStyle;

const computedLabelStyle = computed(() =>
  props.labelStyle ?? formLabelStyle(props.error)
);

const requiredStyle = formRequiredStyle;

const computedErrorStyle = computed(() =>
  props.errorStyle ?? formErrorStyle()
);

const hintStyle: CSSProperties = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginTop: AppleSpace.xs,
};
</script>
