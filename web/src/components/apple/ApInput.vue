<template>
  <ApFieldWrapper :label="label" :required="required" :error="error" :field-id="inputId">
    <div :style="inputContainerStyle" @mouseenter="hovered = true" @mouseleave="hovered = false">
      <input
        :id="inputId"
        :type="actualType"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        :style="inputStyle"
        :maxlength="maxlength"
        :min="min"
        :max="max"
        :step="step"
        :autocomplete="autocomplete"
        :aria-label="ariaLabel"
        :aria-invalid="error ? true : undefined"
        :aria-describedby="error ? `${inputId}-error` : charCount !== undefined ? `${inputId}-charcount` : undefined"
        :aria-required="required"
        @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
        @focus="focused = true"
        @blur="focused = false"
      />
      <button
        v-if="type === 'password' && modelValue"
        :style="togglePwdBtnStyle"
        type="button"
        tabindex="0"
        :aria-label="passwordVisible ? '隐藏密码' : '显示密码'"
        @click="passwordVisible = !passwordVisible"
      >
        <EyeOff v-if="passwordVisible" :size="16" />
        <Eye v-else :size="16" />
      </button>
      <button
        v-if="clearable && modelValue && !disabled && !loading"
        :style="clearBtnStyle"
        type="button"
        tabindex="0"
        :aria-label="$t('apUi1.k8')"
        @click="$emit('update:modelValue', '')"
      >
        <X :size="16" />
      </button>
      <span v-if="loading" :style="spinnerStyle" aria-hidden="true"></span>
    </div>
    <span v-if="charCount !== undefined" :id="`${inputId}-charcount`" :style="charCountStyle">
      {{ modelValue.length }}/{{ charCount }}
    </span>
  </ApFieldWrapper>
</template>

<script setup lang="ts">
import { computed, shallowRef, useId, inject } from 'vue';
import { fieldIdKey } from '@/utils/core/form-field-id';
import { Eye, EyeOff, X } from 'lucide-vue-next';
import { AppleAnimation, AppleFont, AppleFontWeight, AppleInputSize, AppleInteractionState, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleLineHeight, AppleLayout } from '@/utils/core/apple-tokens';
import type { AppleControlSize } from '@/utils/core/apple-tokens';
import { clearBtnStyle as formClearBtnStyle } from '@/styles/component/form-field-styles';
import ApFieldWrapper from './ApFieldWrapper.vue';

const props = withDefaults(defineProps<{
  modelValue: string;
  label?: string;
  placeholder?: string;
  type?: string;
  disabled?: boolean;
  error?: string;
  size?: AppleControlSize;
  clearable?: boolean;
  maxlength?: number;
  // 数值/日期档的钳制边界。必须显式声明成 props：组件模板根是 ApFieldWrapper，
  // 未声明的属性会透传到 wrapper 的 <div> 上，浏览器根本看不到 min/max ——
  // DocumentEditor 传了 `:min="1" :max="20"` 却完全不生效，就是这个原因。
  min?: number | string;
  max?: number | string;
  step?: number | string;
  charCount?: number;
  required?: boolean;
  id?: string;
  autocomplete?: string;
  ariaLabel?: string;
  loading?: boolean;
  readonly?: boolean;
}>(), {
  size: 'medium',
  type: 'text',
  disabled: false,
  clearable: false,
  required: false,
  loading: false,
});

defineEmits<{
  (e: 'update:modelValue', value: string): void;
}>();
// inject 只能在 setup 同步期调用，先取出来再进 computed
const injectedFieldId = inject(fieldIdKey, undefined);
const autoId = useId();
// ApFormItem 下发 id 时以它为准：<label for> 与 aria-describedby 才能对上同一个控件
const inputId = computed(() => props.id || injectedFieldId || autoId);

const focused = shallowRef(false);
const hovered = shallowRef(false);
const passwordVisible = shallowRef(false);

const actualType = computed(() => {
  if (props.type === 'password') {
    return passwordVisible.value ? 'text' : 'password';
  }
  return props.type;
});

const sizeConfig: Record<string, { height: string; fontSize: string; padding: string }> = {
  small: { height: AppleInputSize.small.height, fontSize: AppleInputSize.small.fontSize, padding: AppleInputSize.small.padding, },
  medium: { height: AppleInputSize.medium.height, fontSize: AppleInputSize.medium.fontSize, padding: AppleInputSize.medium.padding, },
  large: { height: AppleInputSize.large.height, fontSize: AppleInputSize.large.fontSize, padding: AppleInputSize.large.padding, },
};

const inputContainerStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  position: 'relative' as const,
}));

const inputStyle = computed(() => {
  const size = sizeConfig[props.size];
  const isDisabled = props.disabled || props.loading;
  const isHovered = hovered.value && !isDisabled && !focused.value;
  return {
    fontFamily: AppleFont.text,
    fontSize: size?.fontSize || AppleType.controlLabel.size,
    fontWeight: String(AppleFontWeight.regular),
    lineHeight: AppleLineHeight.title,
    // 这里原先抄的是 bodyPrimary(17px) 那一档的字距，而字号取自控件表（13/14px）——
    // 等于给 14px 的输入框上了 17px 的字距。同一行的 ApSelect 根本没有字距，
    // 于是"同一角色三种答案"。全站事实上的排版是字距 normal（684 处取字号的消费里 656 处如此），
    // 所以这里删掉错值与 ApSelect 对齐，而不是再发明第三种。
    color: cv('text-primary'),
    // 焦点环由外层容器画（见下方 boxShadow: focused ? blue-focus-ring），
    // 原生 outline 再画一圈就是双重焦点框（键盘聚焦时实测能看到内外两个蓝框）
    outline: 'none',
    background: props.loading ? cv('bg-secondary') : cv('surface-field'),

    border: props.error
      ? `1px solid ${cv('red')}`
      : `1px solid ${cv('soft-border-gray')}`,
    borderRadius: AppleRadius.control,
    padding: size?.padding || `0 ${AppleSpace.md}`,
    paddingRight: props.type === 'password' ? `${AppleSpace.xl}` : size?.padding?.split(' ')[1] || `${AppleSpace.md}`,
    minHeight: size?.height || AppleLayout.touchTargetMin,
    transition: AppleTransition.smooth,
    opacity: isDisabled ? AppleInteractionState.disabledOpacity : '1',
    cursor: isDisabled ? AppleInteractionState.disabledCursor : 'text',
    flex: 1,
    boxShadow: focused.value && !props.error
      ? cv('blue-focus-ring')
      : 'none',
    borderColor: focused.value && !props.error && !isDisabled
      ? cv('blue')
      : isHovered && !props.error
        ? cv('mid-border-gray')
        : props.error ? cv('red') : cv('soft-border-gray'),
  };
});

const togglePwdBtnStyle = {
  position: 'absolute' as const,
  right: `${AppleSpace.sm}`,
  background: 'none',
  border: 'none',
  color: cv('text-tertiary'),
  cursor: 'pointer',
  padding: '0',
  width: AppleLayout.touchTargetMin,
  height: AppleLayout.touchTargetMin,
  lineHeight: '1',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const clearBtnStyle = computed(() => ({
  ...formClearBtnStyle,
  right: props.type === 'password' ? `${AppleSpace.xl}` : `${AppleSpace.sm}`,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-tertiary'),
  padding: '0',
  width: AppleLayout.touchTargetMin,
  height: AppleLayout.touchTargetMin,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  top: undefined,
  transform: undefined,
}));

const charCountStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  alignSelf: 'flex-end',
};

const spinnerStyle = {
  position: 'absolute' as const,
  right: AppleSpace.sm,
  display: 'inline-block',
  width: AppleInteractionState.spinnerSizeSmall,
  height: AppleInteractionState.spinnerSizeSmall,
  border: `${AppleInteractionState.spinnerBorderWidthSmall} solid currentColor`,
  borderTopColor: 'transparent',
  borderRadius: AppleRadius.circle,
  animation: `${AppleAnimation.spin} ${AppleInteractionState.spinnerDuration} linear infinite`,
  color: cv('text-tertiary'),
};
</script>
