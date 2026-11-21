<template>
  <div :style="barStyle">
    <!-- 左边是"看哪些"：统计数、类型/分区切换；右边是"怎么看"：视图切换与搜索。
         以前视图切换占在最左，页面想再放筛选就只能另起一行 —— 2366 宽实测三行 chrome 212px，
         每行只用到版心的 10~13%。 -->
    <slot name="leading" />
    <div v-if="customButtons?.length" :style="buttonGroupStyle">
      <ApButton
        v-for="btn in customButtons"
        :key="btn.key"
        variant="ghost"
        size="small"
        :style="getFilterBtnStyle(btn.key)"
        @click="emit('customAction', btn.key)"
      >
        {{ btn.label }}
        <span v-if="btn.count !== undefined" :style="badgeStyle">{{ btn.count }}</span>
      </ApButton>
    </div>
    <div :style="toolbarSpacerStyle()" />
    <ApViewToggle
      v-if="viewMode !== undefined"
      :model-value="viewMode"
      @update:model-value="(v: 'card' | 'table') => emit('update:viewMode', v)"
    />
    <ApSearch
      v-if="showSearch"
      :model-value="searchValue || ''"
      :placeholder="searchPlaceholder"
      :style="searchBoxStyle"
      @update:model-value="(v: string) => emit('update:searchValue', v)"
    />
  </div>
</template>

<script setup lang="ts">
import { AppleFont, AppleFontWeight, AppleSpace, AppleRadius, AppleType, cv, AppleTransition } from '@/utils/core/apple-tokens';
import ApButton from '@/components/apple/ApButton.vue';
import { toolbarSpacerStyle } from '@/styles/layout/page-styles';

const props = withDefaults(defineProps<{
  viewMode?: 'card' | 'table';
  showSearch?: boolean;
  searchValue?: string;
  searchPlaceholder?: string;
  activeFilter?: string;
  customButtons?: CustomButton[];
}>(), {
  showSearch: false,
  searchPlaceholder: '搜索...',
});

const emit = defineEmits<{
  (e: 'update:viewMode', value: 'card' | 'table'): void;
  (e: 'update:searchValue', value: string): void;
  (e: 'customAction', key: string): void;
}>();

interface CustomButton {
  key: string;
  label: string;
  count?: number;
  active?: boolean;
  color?: string;
}

const barStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.lg,
  marginBottom: AppleSpace.xl,
  flexWrap: 'wrap' as const,
};

const buttonGroupStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
};

const searchBoxStyle = {
  width: '280px',
  flexShrink: '0',
};

const badgeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  letterSpacing: AppleType.microUI.letterSpacing,
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.pill,
  background: cv('bg-secondary'),
  color: cv('text-tertiary'),
};

function getFilterBtnStyle(key: string): Record<string, string> {
  const btn = props.customButtons?.find(b => b.key === key);
  if (!btn) return {};
  const isActive = props.activeFilter === key || btn.active;
  const c = btn.color || cv('blue');
  return {
    display: 'inline-flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.sm} ${AppleSpace.lg}`,
    borderRadius: AppleRadius.control,
    fontFamily: AppleFont.text,
    fontSize: AppleType.controlLabel.size,
    fontWeight: String(isActive ? AppleFontWeight.medium : AppleFontWeight.regular),
    background: isActive ? `color-mix(in srgb, ${c} 12%, transparent)` : 'transparent',
    color: isActive ? c : cv('text-secondary'),
    border: isActive ? `1px solid color-mix(in srgb, ${c} 20%, transparent)` : `1px solid ${cv('border-light')}`,
    cursor: 'pointer',
    transition: AppleTransition.smooth,
  };
}
</script>