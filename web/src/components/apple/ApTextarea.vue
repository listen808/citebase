<template>
  <ApFieldWrapper :label="label" :required="required" :error="error" :field-id="inputId">
    <div :style="inputContainerStyle" @mouseenter="hovered = true" @mouseleave="hovered = false">
      <textarea
        :id="inputId"
        ref="textareaRef"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :style="inputStyle"
        :rows="rows"
        :maxlength="maxlength"
        :aria-label="ariaLabel || undefined"
        :aria-invalid="error ? true : undefined"
        :aria-describedby="error ? `${inputId}-error` : charCount !== undefined ? `${inputId}-charcount` : undefined"
        :aria-required="required"
        @input="$emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
        @focus="focused = true"
        @blur="focused = false"
      />
    </div>
    <span v-if="charCount !== undefined" :id="`${inputId}-charcount`" :style="charCountStyle">
      {{ modelValue.length }}/{{ charCount }}
    </span>
  </ApFieldWrapper>
</template>

<script setup lang="ts">
import { computed, shallowRef, useId, inject } from 'vue';
import { fieldIdKey } from '@/utils/core/form-field-id';
import { AppleFont, AppleFontWeight, AppleInteractionState, AppleRadius, AppleTransition, AppleType, cv, AppleLineHeight, AppleInputSize } from '@/utils/core/apple-tokens';
import type { AppleControlSize } from '@/utils/core/apple-tokens';
import ApFieldWrapper from './ApFieldWrapper.vue';

const props = withDefaults(defineProps<{
  modelValue: string;
  label?: string;
  placeholder?: string;
  disabled?: boolean;
  error?: string;
  rows?: number;
  size?: AppleControlSize;
  required?: boolean;
  id?: string;
  maxlength?: number;
  charCount?: number;
  /** 等宽字体，用于粘贴代码 / Markdown 原文 */
  mono?: boolean;
  /** 覆盖按 size 推导的最小高度，如 '250px' */
  minHeight?: string;
  /** 不显示 label 时的可读名字（聊天输入框这类「视觉上不需要标题」的控件） */
  ariaLabel?: string;
}>(), {
  size: 'medium',
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
const textareaRef = shallowRef<HTMLTextAreaElement | null>(null);

defineExpose({
  focus: () => textareaRef.value?.focus(),
});

const sizeConfig: Record<'small' | 'medium' | 'large', { padding: string; fontSize: string; minHeight: string }> = {
  small: { padding: AppleInputSize.small.padding, fontSize: AppleInputSize.small.fontSize, minHeight: AppleInputSize.small.height },
  medium: { padding: AppleInputSize.medium.padding, fontSize: AppleInputSize.medium.fontSize, minHeight: AppleInputSize.medium.height },
  large: { padding: AppleInputSize.large.padding, fontSize: AppleInputSize.large.fontSize, minHeight: AppleInputSize.large.height },
};

const inputContainerStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  position: 'relative' as const,
}));

const inputStyle = computed(() => ({
  fontFamily: props.mono ? AppleFont.mono : AppleFont.text,
  fontSize: sizeConfig[props.size].fontSize,
  fontWeight: String(AppleFontWeight.regular),
  lineHeight: AppleLineHeight.loose,
  // 与 ApInput 同一条：手抄的 bodyPrimary(17px) 字距压在 13/14px 的文本域上，删掉与 ApSelect 对齐
  color: cv('text-primary'),
  // 同 ApInput：焦点环由容器画，原生 outline 会叠出第二圈
  outline: 'none',
  background: cv('surface-field'),

  border: props.error ? `1px solid ${cv('red')}` : `1px solid ${cv('soft-border-gray')}`,
  borderRadius: AppleRadius.control,
  padding: sizeConfig[props.size].padding,
  minHeight: props.minHeight || sizeConfig[props.size].minHeight,
  transition: AppleTransition.smooth,
  opacity: props.disabled ? AppleInteractionState.disabledOpacity : '1',
  cursor: props.disabled ? AppleInteractionState.disabledCursor : 'text',
  width: '100%',
  resize: 'vertical' as const,
  boxShadow: focused.value && !props.error
    ? cv('blue-focus-ring')
    : 'none',
  borderColor: focused.value && !props.error && !props.disabled
    ? cv('blue')
    : hovered.value && !props.error && !props.disabled
      ? cv('mid-border-gray')
      : props.error ? cv('red') : cv('soft-border-gray'),
}));

const charCountStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  alignSelf: 'flex-end',
};
</script>
