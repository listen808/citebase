<template>
  <Teleport to="body">
    <transition name="apple-modal">
      <div v-if="modelValue" :style="overlayStyle" @click.self="onOverlayClick">
        <div ref="modalRef" class="apple-glass-layer" data-ap="modal" :style="modalStyle" role="dialog" aria-modal="true" :aria-labelledby="titleId" tabindex="-1">
          <div :style="headerStyle">
            <h3 :id="titleId" :style="titleStyle">
              <slot name="title">{{ title }}</slot>
            </h3>
            <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('common.close')" @click="closeModal">
              <X :size="16" />
            </ApButton>
          </div>
          <div :style="bodyStyle">
            <slot />
          </div>
          <div v-if="$slots.footer" :style="footerStyle">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, watch, nextTick, ref, useId } from 'vue';
import { X } from 'lucide-vue-next';
import { AppleRadius, AppleSpace, cv, AppleZIndex, AppleType, AppleModalSize } from '@/utils/core/apple-tokens';
import type { AppleDensity } from '@/utils/core/apple-tokens';
import { useFocusTrap, useScrollLock } from '@/composables/ui/useFocusTrap';
import { pushModal, popModal, isTopModal } from '@/composables/ui/useModalStack';
import { getOverlayStyle } from '@/styles/component/overlay';
import ApButton from '@/components/apple/ApButton.vue';

const props = withDefaults(defineProps<{
  modelValue: boolean;
  width?: string;
  size?: keyof typeof AppleModalSize;
  density?: AppleDensity;
  closeOnOverlay?: boolean;
  beforeClose?: () => boolean | Promise<boolean>;
  /** 标题（纯文本，简化 #title slot 用法；存在 slot 内容时优先 slot） */
  title?: string;
  /** 使用毛玻璃背景（默认开启） */
  glass?: boolean;
}>(), {
  size: 'medium',
  density: 'default',
  closeOnOverlay: true,
  glass: true,
});
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
}>();
const instanceId = useId();
const modalRef = ref<HTMLElement | null>(null);
const titleId = 'ap-modal-title-' + instanceId;

const { setupFocusTrap, handleTabKey, saveActiveElement, restoreActiveElement } = useFocusTrap(modalRef);
const { lock: lockScroll, unlock: unlockScroll, forceUnlock: _forceUnlockScroll } = useScrollLock();

watch(() => props.modelValue, (newValue) => {
  if (newValue) {
    saveActiveElement();
    pushModal(instanceId);
    lockScroll();
    nextTick(() => setupFocusTrap());
  } else {
    popModal(instanceId);
    unlockScroll();
    restoreActiveElement();
  }
}, { immediate: true });

const modalWidth = computed((): string => {
  if (props.width) return props.width;
  return AppleModalSize[props.size] ?? AppleModalSize.medium;
});

async function closeModal() {
  if (props.beforeClose) {
    const canClose = await props.beforeClose();
    if (!canClose) return;
  }
  emit('update:modelValue', false);
}

async function onOverlayClick() {
  if (props.closeOnOverlay !== false) {
    await closeModal();
  }
}

const useGlass = computed(() => props.glass !== false);

// 叠放次序交给 Teleport 的 DOM 追加顺序（见 useModalStack），z 恒取 modal 档：
// 往上抬会撞进 tooltip(10000)/toast(10001)，往下降就不再是模态。
const overlayStyle = computed(() => getOverlayStyle(
  AppleZIndex.modal,
  {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: AppleSpace.lg,
  },
));

const modalStyle = computed(() => {
  const style: Record<string, string> = {
    width: modalWidth.value,
    maxWidth: '90vw',
    maxHeight: 'min(80vh, 900px)',
    minHeight: '200px',
    borderRadius: AppleRadius.float,
    boxShadow: cv('shadow-modal'),
    display: 'flex',
    flexDirection: 'column' as const,
    transformOrigin: 'center center',
    overflow: 'hidden',
  };

  // 模态属浮层面板档（surface-float），与 ApSelect / ApSidebar 菜单 / DocContextMenu 同一档。
  // 原先这里和它们共用 surface-elevated，而 elevated 还得扛「内容滚到顶栏底下」那个极值，
  // 于是模态也被钉在 0.88 白上 —— 身后明明是本页内容却一点透不出来。
  // 再早还写的是 cv('glass-bg')，那名字只是 surface-panel 的转发：浮层和卡片同色，台阶在这里断了。
  style.background = cv('surface-float');
  if (useGlass.value) {
    // 模糊**不在这里写**：它由模板上的 .apple-glass-layer 伪元素承载（入场时随动画一起到位，
    // 见 apple-tokens.ts 的 materialize 注释）。行内再写一遍就会和伪元素叠成两层 backdrop-filter，
    // 既白花一次 GPU，也直接吃掉"玻璃层 ≤3"的预算 —— check:glass 数站点会看见。
    // 发丝边与其余浮层同源（glass-rim）：此前这里单独取 glass-border-nav，
    // 同为浮层两家边亮度差一档（深色 0.06 vs 0.05、浅色 0.55 vs 0.35）。
    style.border = `0.5px solid ${cv('glass-rim')}`;
  }

  return style;
});

const headerStyle = computed(() => ({
  padding: props.density === 'compact'
    ? `${AppleSpace.md} ${AppleSpace.lg}`
    : `${AppleSpace.lg} ${AppleSpace.lg}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
}));

const titleStyle = {
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleType.bodyEmphasis.weight),
  color: cv('text-primary'),
  margin: '0',
  letterSpacing: AppleType.subhead.letterSpacing,
};

const bodyStyle = computed(() => ({
  flex: '1' as const,
  overflowY: 'auto' as const,
  padding: props.density === 'compact'
    ? `${AppleSpace.sm} ${AppleSpace.lg}`
    : `${AppleSpace.md} ${AppleSpace.lg}`,
  minHeight: '0',
}));

const footerStyle = computed(() => ({
  padding: props.density === 'compact'
    ? `${AppleSpace.sm} ${AppleSpace.lg}`
    : `${AppleSpace.md} ${AppleSpace.lg}`,
  borderTop: `1px solid ${cv('border-light')}`,
  display: 'flex',
  justifyContent: 'flex-end' as const,
  gap: AppleSpace.sm,
}));

async function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.modelValue) {
    // 灯箱盖在 modal 之上，但它不在这条栈里（它是另一套浮层）：不先看它一眼，
    // 一次 Esc 会把灯箱和它下面的弹窗一起关掉
    if (document.querySelector('[data-ap="image-lightbox"]')) return;
    if (isTopModal(instanceId)) {
      e.preventDefault();
      await closeModal();
    }
  } else if (props.modelValue) {
    // 只有栈顶那个 modal 处理 Tab，否则叠层时两层焦点陷阱互相抢
    if (isTopModal(instanceId)) {
      handleTabKey(e);
    }
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown);
  popModal(instanceId);
  unlockScroll();
});
</script>

<style scoped>
/* Apple HIG 风格 Modal 弹入动画 */
.apple-modal-enter-active {
  transition: opacity var(--apple-dur-default) var(--apple-easing-standard);
}
.apple-modal-leave-active {
  transition: opacity var(--apple-dur-fast) var(--apple-easing-standard);
}
.apple-modal-enter-from,
.apple-modal-leave-to {
  opacity: 0;
}
.apple-modal-enter-active > div {
  transition: transform var(--apple-dur-modalIn) var(--apple-easing-emphasized),
              opacity var(--apple-dur-default) var(--apple-easing-standard);
}
.apple-modal-leave-active > div {
  transition: transform var(--apple-dur-fast) var(--apple-easing-standard),
              opacity var(--apple-dur-fast) var(--apple-easing-standard);
}
.apple-modal-enter-from > div {
  transform: scale(0.92) translateY(10px);
  opacity: 0;
}
.apple-modal-leave-to > div {
  transform: scale(0.96);
  opacity: 0;
}
</style>
