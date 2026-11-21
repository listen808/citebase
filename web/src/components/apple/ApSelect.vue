<template>
  <ApFieldWrapper :label="label" :required="required" :error="error" :field-id="selectId">
    <div ref="wrapperRef" data-ap-select :style="wrapperStyle">
      <div :id="selectId" :style="selectContainerStyle" role="combobox" aria-haspopup="listbox" :aria-expanded="showDropdown" :aria-controls="showDropdown ? listboxId : undefined" :aria-autocomplete="searchable ? 'list' : 'none'" tabindex="0" @click.stop="toggleDropdown" @keydown="handleKeydown" @mouseenter="containerHovered = true" @mouseleave="containerHovered = false">
        <span v-if="selectedLabel" :style="selectedTextStyle">{{ selectedLabel }}</span>
        <span v-else :style="placeholderStyle">{{ placeholder || $t('common.selectPlaceholder') }}</span>
        <ChevronDown :size="16" :style="chevronStyle" />
      </div>
      <Teleport to="body">
      <div v-if="showDropdown" :id="listboxId" ref="listboxRef" class="apple-glass-layer" :style="dropdownStyle" role="listbox" data-ap-select-dropdown @click.stop>
        <div v-if="searchable" :style="searchBoxStyle">
          <Search :size="14" :style="searchIconStyle" />
          <input
            ref="searchInputRef"
            v-model="searchKeyword"
            type="text"
            :style="searchInputStyle"
            :placeholder="$t('common.search')"
            @click.stop
            @keydown.stop
            @keydown.esc="showDropdown = false"
          />
        </div>
        <div :style="optionsListStyle">
          <div
            v-for="option in filteredOptions"
            :key="option.value"
            :style="optionStyle(option)"
            role="option"
            :aria-selected="modelValue === option.value"
            @click.stop="selectOption(option)"
            @mouseenter="hoveredValue = option.value"
            @mouseleave="hoveredValue = null"
          >
            <Check v-if="modelValue === option.value" :size="14" :style="checkIconStyle" />
            <span v-else :style="checkPlaceholderStyle" />
            <span>{{ option.label }}</span>
          </div>
          <div v-if="!filteredOptions.length" :style="emptyStyle">{{ searchKeyword ? $t('common.noMatch') : $t('common.noOptions') }}</div>
        </div>
      </div>
      </Teleport>
    </div>
  </ApFieldWrapper>
</template>

<script setup lang="ts">
import { computed, inject, ref, shallowRef, onBeforeUnmount, onMounted, nextTick, useId } from 'vue';
import { fieldIdKey } from '@/utils/core/form-field-id';
import { ChevronDown, Check, Search } from 'lucide-vue-next';
import { AppleFont, AppleInputSize, AppleInteractionState, AppleRadius, AppleSpace, AppleTransition, AppleType, AppleZIndex, cv, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import type { AppleControlSize } from '@/utils/core/apple-tokens';
import { emptyStateStyle as emptyStyle, scrollSingleAxis } from '@/styles/layout/page-styles';
import ApFieldWrapper from './ApFieldWrapper.vue';
import { useClickOutsideClose } from '@/composables/ui/useClickOutsideClose';

export interface SelectOption {
  label: string;
  value: string | number;
  disabled?: boolean;
}

const props = withDefaults(
  defineProps<{
    modelValue: string | number | null;
    options: SelectOption[];
    placeholder?: string;
    label?: string;
    disabled?: boolean;
    error?: string;
    size?: AppleControlSize;
    required?: boolean;
    searchable?: boolean;
  }>(),
  {
    placeholder: '',
    size: 'medium',
    searchable: false,
  },
);

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | number | null): void;
}>();

const wrapperRef = ref<HTMLElement | null>(null);
const searchInputRef = ref<HTMLInputElement | null>(null);
const listboxRef = ref<HTMLElement | null>(null);
const showDropdown = shallowRef(false);
const hoveredValue = shallowRef<string | number | null>(null);
const containerHovered = shallowRef(false);
const searchKeyword = ref('');
// useId() 必须在 setup 期取一次。原来写成 computed(() => `ap-select-${useId()}`)，
// 每次求值都发一个新 id：aria-controls 与面板 id 因此会各说各话，
// 重渲染后 label[for] 也指向已经不存在的目标。外层 ApFormItem 有下发 id 时优先用它。
const uid = useId();
const injectedFieldId = inject(fieldIdKey, undefined);
const selectId = computed(() => injectedFieldId || `ap-select-${uid}`);
const listboxId = computed(() => `${selectId.value}-listbox`);
const dropdownTop = ref('');
const dropdownBottom = ref('');
const dropdownLeft = ref('');
const dropdownWidth = ref('');
// P1-8: 下拉面板 z-index（跟随所在弹窗/抽屉层级动态计算）
const dropdownZIndex = ref<number>(AppleZIndex.dropdown);

const selectedLabel = computed(() => {
  const opt = props.options.find((o) => o.value === props.modelValue);
  return opt?.label || '';
});

const filteredOptions = computed(() => {
  if (!props.searchable || !searchKeyword.value.trim()) return props.options;
  const kw = searchKeyword.value.trim().toLowerCase();
  return props.options.filter(o => o.label.toLowerCase().includes(kw));
});

/** 下拉箭头占位宽度统一用 AppleSpace.xl，三档共用 */
const chevronGutter = AppleSpace.xl;
const sizeMap: Record<string, { height: string; fontSize: string; padding: string }> = {
  small: { height: AppleInputSize.small.height, fontSize: AppleInputSize.small.fontSize, padding: `0 ${chevronGutter} 0 ${AppleSpace.sm}` },
  medium: { height: AppleInputSize.medium.height, fontSize: AppleInputSize.medium.fontSize, padding: `0 ${AppleSpace.xl} 0 ${AppleSpace.sm}` },
  large: { height: AppleInputSize.large.height, fontSize: AppleInputSize.large.fontSize, padding: `0 ${chevronGutter} 0 ${AppleSpace.md}` },
};

const wrapperStyle = computed(() => ({
  position: 'relative' as const,
  width: '100%',
}));

const selectContainerStyle = computed(() => {
  const size = sizeMap[props.size || 'medium'];
  return {
    display: 'flex',
    alignItems: 'center',
    height: size?.height || AppleInputSize.medium.height,
    padding: size?.padding || `0 ${chevronGutter} 0 ${AppleSpace.sm}`,
    fontSize: size?.fontSize || AppleType.controlLabel.size,
    fontFamily: AppleFont.text,
    color: props.modelValue != null ? cv('near-black-ink') : cv('secondary-neutral-gray'),
    background: props.disabled ? cv('bg-tertiary') : cv('surface-field'),
    border: props.error
      ? `1px solid ${cv('red')}`
      : showDropdown.value
        ? `1px solid ${cv('blue')}`
        : containerHovered.value && !props.disabled
          ? `1px solid ${cv('mid-border-gray')}`
          : `1px solid ${cv('soft-border-gray')}`,
    borderRadius: AppleRadius.control,
    cursor: props.disabled ? AppleInteractionState.disabledCursor : 'pointer',
    transition: AppleTransition.smooth,
    userSelect: 'none' as const,
    opacity: props.disabled ? AppleInteractionState.disabledOpacity : '1',
    position: 'relative' as const,
    boxShadow: showDropdown.value && !props.error
      ? cv('blue-focus-ring')
      : 'none',
  };
});

const selectedTextStyle = {
  flex: '1',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const placeholderStyle = {
  flex: '1',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const chevronStyle = computed(() => ({
  position: 'absolute' as const,
  right: '10px',
  color: cv('secondary-neutral-gray'),
  transition: AppleTransition.smooth,
  transform: showDropdown.value ? 'rotate(180deg)' : 'rotate(0deg)',
}));

// P1-8: 面板 Teleport 到 body，与 ApModal 遮罩同属根层叠上下文；
// 若触发器位于弹窗/抽屉内，必须抬高到所在浮层的 z-index 之上，否则选项被完全遮挡。
// 这是全站唯一一处需要按宿主浮层动态抬档的菜单，抬档走 extra，不另起一份表面。
const dropdownStyle = computed(() => menuSurfaceStyle({
  position: 'fixed' as const,
  top: dropdownTop.value || undefined,
  bottom: dropdownBottom.value || undefined,
  left: dropdownLeft.value,
  right: 'auto',
  width: dropdownWidth.value,
  zIndex: String(dropdownZIndex.value),
  padding: AppleSpace.xs,
}));

const searchBoxStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  marginBottom: AppleSpace.xs,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const searchIconStyle = {
  color: cv('text-tertiary'),
  flexShrink: 0,
};

const searchInputStyle = {
  flex: '1',
  border: 'none',
  background: 'transparent',
  fontSize: AppleType.controlLabel.size,
  fontFamily: AppleFont.text,
  color: cv('text-primary'),
  padding: '0',
};

const optionsListStyle = {
  maxHeight: '240px',
  // 选项标签已截断，容器仍要显式两轴 + 常驻槽位（同 ApSidebar/ChatSidebar）
  ...scrollSingleAxis,
};

const optionStyle = computed(() => {
  return (option: SelectOption) => ({
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.sm} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.small,
    cursor: option.disabled ? AppleInteractionState.disabledCursor : 'pointer',
    fontSize: AppleType.controlLabel.size,
    fontFamily: AppleFont.text,
    color: option.disabled ? cv('text-disabled') : cv('near-black-ink'),
    background:
      hoveredValue.value === option.value && !option.disabled ? cv('pale-apple-gray') : 'transparent',
    opacity: option.disabled ? AppleInteractionState.disabledOpacity : '1',
    transition: AppleTransition.smooth,
  });
});

const checkIconStyle = {
  flexShrink: 0,
  color: cv('blue'),
};

const checkPlaceholderStyle = {
  width: '14px',
  flexShrink: 0,
};

function updateDropdownPosition() {
  if (!wrapperRef.value) return;
  const rect = wrapperRef.value.getBoundingClientRect();
  const viewportHeight = window.innerHeight;
  const spaceBelow = viewportHeight - rect.bottom - 4;
  if (spaceBelow < 200 && rect.top > spaceBelow) {
    dropdownTop.value = 'auto';
    dropdownBottom.value = `${viewportHeight - rect.top + 4}px`;
  } else {
    dropdownTop.value = `${rect.bottom + 4}px`;
    dropdownBottom.value = 'auto';
  }
  dropdownLeft.value = `${rect.left}px`;
  dropdownWidth.value = `${rect.width}px`;
  dropdownZIndex.value = resolveDropdownZIndex();
}

// P1-8: 若触发器位于 ApModal/ApDrawer（role="dialog"）内部，读取其遮罩层的实际 z-index 并加一，
// 保证 Teleport 到 body 的下拉面板绘制在弹窗之上
function resolveDropdownZIndex(): number {
  const el = wrapperRef.value;
  if (!el) return AppleZIndex.dropdown;
  const dialog = el.closest('[role="dialog"]');
  if (!dialog) return AppleZIndex.dropdown;
  const overlay = dialog.parentElement;
  const z = overlay ? Number.parseInt(getComputedStyle(overlay).zIndex, 10) : Number.NaN;
  return Number.isFinite(z) && z >= AppleZIndex.modal ? z + 1 : AppleZIndex.modal + 1;
}

// P1-34: 统一“打开下拉”逻辑（定位 + 聚焦搜索框），供鼠标与全部键盘分支复用；
// 此前键盘路径只置 showDropdown=true 不算位置，面板渲染在视口角落与触发框脱节
function openDropdown() {
  showDropdown.value = true;
  nextTick(() => {
    updateDropdownPosition();
    if (props.searchable) {
      searchKeyword.value = '';
      searchInputRef.value?.focus();
    }
  });
}

function toggleDropdown(e: MouseEvent) {
  if (props.disabled) return;
  e.stopPropagation();
  if (showDropdown.value) {
    showDropdown.value = false;
  } else {
    openDropdown();
  }
}

function selectOption(option: SelectOption) {
  if (option.disabled) return;
  emit('update:modelValue', option.value);
  showDropdown.value = false;
  searchKeyword.value = '';
}

// 面板 Teleport 到 body，所以"自己人"要把面板本体也算进去。
// 原来这份实现靠 isClosing + 50ms 定时器躲"click 冒泡时点了另一个触发器会先关再开"，
// 换成 pointerdown capture 之后那个竞态不存在了，定时器一并去掉。
useClickOutsideClose(
  () => showDropdown.value,
  () => { showDropdown.value = false; },
  () => [wrapperRef.value, listboxRef.value],
);

function handleKeydown(e: KeyboardEvent) {
  if (props.disabled) return;
  const enabledOptions = filteredOptions.value.filter(o => !o.disabled);
  if (!enabledOptions.length) return;
  const currentIdx = enabledOptions.findIndex(o => o.value === props.modelValue);

  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault();
    if (!showDropdown.value) {
      openDropdown(); // P1-34: 键盘打开同样要计算位置
      return;
    }
    const dir = e.key === 'ArrowDown' ? 1 : -1;
    let next = currentIdx === -1 ? 0 : currentIdx + dir;
    if (next < 0) next = enabledOptions.length - 1;
    if (next >= enabledOptions.length) next = 0;
    const selectedOption = enabledOptions[next];
    if (selectedOption) {
      emit('update:modelValue', selectedOption.value);
    }
  } else if (e.key === 'Home') {
    e.preventDefault();
    if (showDropdown.value && enabledOptions.length > 0) {
      const firstOption = enabledOptions[0];
      if (firstOption) {
        emit('update:modelValue', firstOption.value);
      }
    }
  } else if (e.key === 'End') {
    e.preventDefault();
    if (showDropdown.value && enabledOptions.length > 0) {
      const lastOption = enabledOptions[enabledOptions.length - 1];
      if (lastOption) {
        emit('update:modelValue', lastOption.value);
      }
    }
  } else if (e.key === 'Enter') {
    e.preventDefault();
    if (!showDropdown.value) {
      openDropdown(); // P1-34
    } else {
      showDropdown.value = false;
    }
  } else if (e.key === 'Escape') {
    e.preventDefault();
    showDropdown.value = false;
  } else if (e.key === ' ') {
    e.preventDefault();
    if (!showDropdown.value) {
      openDropdown(); // P1-34
    }
  }
}

function handleScroll() {
  if (showDropdown.value) {
    updateDropdownPosition();
  }
}

onMounted(() => {
  document.addEventListener('scroll', handleScroll, true);
});

onBeforeUnmount(() => {
  document.removeEventListener('scroll', handleScroll, true);
});
</script>
