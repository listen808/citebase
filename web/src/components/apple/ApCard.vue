<template>
  <div
    :style="cardStyle"
    :class="cardClass"
    :role="role || 'region'"
    :aria-label="ariaLabel"
    :aria-disabled="disabled"
    :tabindex="hoverable !== false && !disabled ? 0 : undefined"
    @mouseenter="hover = true"
    @mouseleave="hover = false; active = false"
    @mousedown="active = true"
    @mouseup="active = false"
    @keydown.enter="handleKeydown"
    @keydown.space.prevent="handleKeydown"
    @click="handleClick"
  >
    <div v-if="$slots.header" :style="headerStyle">
      <slot name="header" />
    </div>
    <div :style="bodyStyle">
      <div v-if="loading" :style="loadingOverlayStyle">
        <span :style="spinnerStyle" aria-hidden="true"></span>
      </div>
      <slot />
    </div>
    <div v-if="$slots.footer" :style="footerStyle">
      <slot name="footer" />
    </div>
  </div>
</template>

<script setup lang="ts">
import {computed, shallowRef} from 'vue';
import { AppleAnimation, AppleZIndex, AppleGlassBlur, AppleInteractionState, AppleRadius, AppleTransition, AppleSpace, cv } from '@/utils/core/apple-tokens';
import type { AppleCardSizeName } from '@/utils/core/apple-tokens';

/**
 * 全站卡片唯一形态：静置不投影（阴影值由主题决定，克制档下都是 none），
 * 层级靠 1px 发丝边框；只有浮层与 hover 才有阴影。
 * 曾有 6 个 variant（elevated/gradient/glass/borderless/outlined）但 0 消费者，
 * 其中 elevated 的静置 shadow-level3 直接违反"卡片静置不投影"——一并删掉，
 * 别留"第二种卡片"的入口（ApGradientCard / ApTooltip 删除同例）。
 */
const props = withDefaults(defineProps<{
  size?: AppleCardSizeName;
  padding?: string;
  hoverable?: boolean;
  role?: string;
  ariaLabel?: string;
  disabled?: boolean;
  loading?: boolean;
}>(), {
  size: 'medium',
  hoverable: true,
  disabled: false,
  loading: false,
});

const emit = defineEmits<{
  (e: 'click'): void;
}>();

const hover = shallowRef(false);
const active = shallowRef(false);

function handleKeydown() {
  if (props.disabled) return;
  if (props.hoverable !== false) {
    emit('click');
  }
}

function handleClick() {
  if (props.disabled) return;
  emit('click');
}

const sizePadding: Record<string, string> = {
  small: AppleSpace.sm,
  medium: cv('space-12px'),
  large: AppleSpace.md,
  xlarge: AppleSpace.lg,
};

const cardClass = ['apple-focus-ring'];

const cardStyle = computed(() => {
  const isDisabled = props.disabled;
  const isHoverable = props.hoverable && !isDisabled;
  const isHovered = hover.value && isHoverable;
  const isActive = active.value && isHoverable;

  const style: Record<string, string> = {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'stretch',
    background: cv('surface-panel'),
    borderRadius: AppleRadius.card,
    border: `1px solid ${cv('border-light')}`,
    boxShadow: cv('shadow-card'),
    transition: AppleTransition.cardLift,
    padding: props.padding || sizePadding[props.size] || AppleSpace.md,
    overflow: 'hidden',
    opacity: isDisabled ? AppleInteractionState.disabledOpacity : '1',
    // 只有真的可点/可悬停的卡片才给 pointer，静态面板不再"看起来能点"
    cursor: isDisabled ? AppleInteractionState.disabledCursor : isHoverable ? 'pointer' : 'default',
    position: 'relative' as const,
    // 卡片是流水面（一屏十几张，声明被 v-for 放大），不付模糊 —— 见 ApButton 那条注释：
    // 同屏实时模糊红线 ≤3，模糊只留给壳层与真浮层。材质层级由 surface-panel 的半透明值表达。
  };

  if (isDisabled) {
    style.cursor = AppleInteractionState.disabledCursor;
    style.pointerEvents = 'auto';
  } else if (isActive) {
    style.transform = `scale(${AppleInteractionState.activeScale})`;
  } else if (isHovered) {
    // 悬停只加深描边，不位移、不放大（整卡 scale 抖动是教条禁止的）
    style.border = `1px solid ${cv('border')}`;
  }

  return style;
});

const headerStyle = {
  borderBottom: `1px solid ${cv('border-light')}`,
  paddingBottom: AppleSpace.sm,
  marginBottom: AppleSpace.sm,
};

const bodyStyle = {
  position: 'relative' as const,
  // 卡片自己是 flex 列（见 cardStyle），正文吃掉剩余高度，
  // 于是同一网格里被拉到等高的卡片，页脚/元信息自然对齐在同一基线。
  flex: '1 1 auto',
  display: 'flex' as const,
  flexDirection: 'column' as const,
  minWidth: 0,
};

const loadingOverlayStyle = {
  position: 'absolute' as const,
  inset: '0',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: cv('surface-panel'),
  backdropFilter: AppleGlassBlur.mask,
  WebkitBackdropFilter: AppleGlassBlur.mask,
  borderRadius: AppleRadius.card,
  zIndex: String(AppleZIndex.base),
};

const spinnerStyle = {
  display: 'inline-block',
  width: AppleInteractionState.spinnerSizeMedium,
  height: AppleInteractionState.spinnerSizeMedium,
  border: `${AppleInteractionState.spinnerBorderWidthMedium} solid ${cv('border-light')}`,
  borderTopColor: cv('blue'),
  borderRadius: AppleRadius.circle,
  animation: `${AppleAnimation.spin} ${AppleInteractionState.spinnerDuration} linear infinite`,
};

const footerStyle = {
  borderTop: `1px solid ${cv('border-light')}`,
  paddingTop: AppleSpace.sm,
  marginTop: AppleSpace.sm,
};
</script>
