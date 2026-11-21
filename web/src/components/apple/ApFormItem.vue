<template>
  <ApFieldWrapper
    :label="label"
    :required="required"
    :error="error"
    :hint="hint"
    :field-id="inputId"
    :associate="associate"
  >
    <slot />
  </ApFieldWrapper>
</template>

<script setup lang="ts">
import { provide, useId } from 'vue';
import { fieldIdKey } from '@/utils/core/form-field-id';
import ApFieldWrapper from './ApFieldWrapper.vue';

const props = withDefaults(defineProps<{
  label?: string;
  error?: string;
  hint?: string;
  required?: boolean;
  /** 显式指定字段 id；不传则自动生成并下发给插槽里的控件 */
  id?: string;
  /** 插槽里放的是分组类控件时置 false：只留文字标题，不发 label[for]。
      同样必须显式默认 true，否则布尔转换会把 for 全抹掉。 */
  associate?: boolean;
}>(), {
  associate: true,
});

const inputId = props.id || useId();
provide(fieldIdKey, inputId);
</script>
