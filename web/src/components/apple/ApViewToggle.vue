<template>
  <ApSegmentedControl
    :model-value="modelValue"
    :options="options"
    :size="size"
    @update:model-value="(v) => emit('update:modelValue', v as 'card' | 'table')"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import ApSegmentedControl from './ApSegmentedControl.vue';
import { AppleSegmentSize } from '@/utils/core/apple-tokens';

withDefaults(defineProps<{
  modelValue: 'card' | 'table';
  /**
   * 必须转发下去。原先这个包装不接 size，于是视图切换恒取 ApSegmentedControl 的默认 medium
   * （内层项 40px / 文字 17px），而同一行 `#leading` 里页面自己写的「全部/文档/空间」是 small
   * （32px / 13px）—— 两个互斥选项组并排差 8px 高、4px 字号。
   * 默认值定在 small 而不是沿用 medium：工具条是"怎么看"的辅助控件，与统计 chip、面包屑同档。
   */
  size?: keyof typeof AppleSegmentSize;
}>(), {
  size: 'small',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: 'card' | 'table'): void
}>()

const { t } = useI18n();
const options = [
  { label: t('apUi1.k48'), value: 'card' as const },
  { label: t('apUi1.k49'), value: 'table' as const },
]
</script>
