<template>
  <div :style="wrapperStyle" role="search" @click="!disabled && handleFocus()">
    <Search :size="16" :style="{ color: cv('blue'), flexShrink: 0, flex: '0 0 auto' }" aria-hidden="true" />
    <input
      :id="inputId"
      ref="inputRef"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :style="inputStyle"
      type="text"
      :aria-label="ariaLabel || placeholder || 'Search'"
      @input="onInput"
      @focus="onFocus"
      @blur="onBlur"
      @keydown="onKeydown"
    />
    <button
      v-if="modelValue"
      :style="clearBtnStyle"
      :title="$t('common.clearSearch')"
      :aria-label="$t('common.clearSearch')"
      type="button"
      @click="clear"
    >
      <X :size="16" aria-hidden="true" />
    </button>

    <div v-if="showDropdown && dropdownItems.length > 0" class="apple-glass-layer" :style="dropdownStyle">
      <div
        v-for="(item, idx) in dropdownItems"
        :key="idx"
        :style="suggestionItemStyle(idx === activeIndex)"
        @mousedown.prevent="selectSuggestion(item)"
        @mouseenter="activeIndex = idx"
      >
        <Search :size="12" :style="{ color: cv('text-quaternary'), flexShrink: 0 }" />
        <span :style="suggestionTextStyle">{{ item }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, onBeforeUnmount } from 'vue';
import { Search, X } from 'lucide-vue-next';
import { AppleDuration, AppleFontWeight, AppleInputSize, AppleInteractionState, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { clearBtnStyle as formClearBtnStyle } from '@/styles/component/form-field-styles';

const props = withDefaults(defineProps<{
  modelValue: string;
  placeholder?: string;
  debounce?: number;
  id?: string;
  ariaLabel?: string;
  suggestions?: string[];
  showSuggestions?: boolean;
  disabled?: boolean;
}>(), {
  disabled: false,
});

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void;
  (e: 'search', value: string): void;
  (e: 'select-suggestion', value: string): void;
}>();

const uniqueId = `ap-search-${Math.random().toString(36).substr(2, 9)}`;
const inputId = computed(() => props.id || uniqueId);
const isFocused = shallowRef(false);
const activeIndex = shallowRef(-1);
const inputRef = ref<HTMLInputElement | null>(null);

let timer: ReturnType<typeof setTimeout> | null = null;

function handleFocus() {
  inputRef.value?.focus();
}

const dropdownItems = computed(() => {
  if (!props.suggestions || !props.showSuggestions) return [];
  const q = props.modelValue.trim().toLowerCase();
  if (!q) return props.suggestions.slice(0, 8);
  return props.suggestions.filter(s => s.toLowerCase().includes(q)).slice(0, 8);
});

const showDropdown = computed(() =>
  isFocused.value && props.showSuggestions && dropdownItems.value.length > 0
);

function onInput(e: Event) {
  const value = (e.target as HTMLInputElement).value;
  emit('update:modelValue', value);
  activeIndex.value = -1;

  if (timer) clearTimeout(timer);
  timer = setTimeout(() => {
    emit('search', value);
  }, props.debounce ?? AppleDuration.debounceSearch);
}

function onFocus() {
  isFocused.value = true;
  activeIndex.value = -1;
}

function onBlur() {
  setTimeout(() => {
    isFocused.value = false;
    activeIndex.value = -1;
  }, 150);
}

function onKeydown(e: KeyboardEvent) {
  if (!showDropdown.value) return;
  if (e.key === 'ArrowDown') {
    e.preventDefault();
    activeIndex.value = Math.min(activeIndex.value + 1, dropdownItems.value.length - 1);
  } else if (e.key === 'ArrowUp') {
    e.preventDefault();
    activeIndex.value = Math.max(activeIndex.value - 1, -1);
  } else if (e.key === 'Enter' && activeIndex.value >= 0) {
    e.preventDefault();
    selectSuggestion(dropdownItems.value[activeIndex.value] ?? '');
  } else if (e.key === 'Escape') {
    isFocused.value = false;
    activeIndex.value = -1;
  }
}

function selectSuggestion(value: string) {
  emit('update:modelValue', value);
  emit('select-suggestion', value);
  emit('search', value);
  isFocused.value = false;
  activeIndex.value = -1;
}

function clear() {
  emit('update:modelValue', '');
  emit('search', '');
}

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer);
});

const wrapperStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  background: cv('surface-field'),
  border: isFocused.value ? `1px solid ${cv('blue')}` : `1px solid ${cv('soft-border-gray')}`,
  minHeight: AppleInputSize.medium.height,
  borderRadius: AppleRadius.control,
  padding: `${AppleSpace.xs} ${AppleSpace.md}`,
  width: '100%',
  boxShadow: isFocused.value ? cv('blue-focus-ring') : cv('shadow-subtle'),
  transition: AppleTransition.default,
  position: 'relative' as const,
  opacity: props.disabled ? AppleInteractionState.disabledOpacity : '1',
  cursor: props.disabled ? AppleInteractionState.disabledCursor : 'text',
}));

const inputStyle = computed(() => ({
  flex: '1',
  // 撑满外壳：原先只写 flex:1（只管横向），input 自己只有 18px 高，而外壳是 40px
  // 的字段 —— 上下各 11px 对指针是死的（实测点字段顶部 4px 处，焦点落到 <main>）。
  // 逐组件几何普查（check:boxes B1）量到 search/input 全站 25 处 18px。
  // 用交叉轴拉伸而不是 height:100%：父盒是 align-items:center 的 flex 行，
  // 百分比高度在那儿解不出 40px。
  alignSelf: 'stretch' as const,
  border: 'none',
  // 焦点环由外壳画（boxShadow: blue-focus-ring）。以前这里没关掉原生 outline，
  // 键盘聚焦时输入框里 2px 蓝框 + 外壳 1px 环 + 6px 光晕 = 三重焦点框。
  outline: 'none',
  background: 'transparent',
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  minWidth: '0',
  fontWeight: String(AppleFontWeight.regular) }));

const clearBtnStyle = computed(() => ({
  ...formClearBtnStyle,
  position: undefined,
  right: undefined,
  top: undefined,
  transform: undefined,
  width: AppleSpace.lg,
  height: AppleSpace.lg,
  borderRadius: AppleRadius.circle,
  background: cv('bg-secondary'),
  color: cv('text-secondary'),
  padding: '0',
  flexShrink: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
}));

const dropdownStyle = menuSurfaceStyle({
  position: 'absolute' as const,
  top: '100%',
  left: '0',
  right: '0',
  marginTop: AppleSpace.sm,
  overflow: 'hidden' as const,
  padding: AppleSpace.xs,
});

const suggestionItemStyle = (active: boolean) => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderRadius: AppleRadius.micro,
  cursor: 'pointer',
  background: active ? cv('blue-selection') : 'transparent',
  color: active ? cv('blue-text') : cv('text-primary'),
  transition: AppleTransition.smooth,
});

const suggestionTextStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis' as const,
  whiteSpace: 'nowrap' as const,
};
</script>