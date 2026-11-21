<template>
  <Teleport to="body">
    <div data-ap="toast" :style="containerStyle" role="alert" aria-live="polite" aria-atomic="true">
      <TransitionGroup name="ap-toast">
        <div
          v-for="toast in allToasts"
          :key="toast.id"
          :style="getToastStyle(toast.type)"
          aria-label="toast"
        >
          <span :style="iconStyle(toast.type)"><component :is="typeIcon(toast.type)" :size="14" /></span>
          <span :style="messageStyle">{{ toast.message }}</span>
          <ApButton
            variant="ghost"
            size="mini"
            icon-only
            :aria-label="$t('apUi1.k47')"
            @click="dismissToast(toast)"
          ><X :size="16" /></ApButton>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { Check, X, AlertTriangle, Info } from 'lucide-vue-next';
import { useUIStore } from '@/stores/app/ui';
import { useToast } from '@/utils/toast';
import { AppleFont, AppleRadius, AppleSpace, AppleType, cv, AppleZIndex, AppleFontWeight } from '@/utils/core/apple-tokens';
import ApButton from '@/components/apple/ApButton.vue';

const ui = useUIStore();
const { removeToast: removeGlobalToast } = useToast();

interface ToastItem { id: number; message: string; type: string }

const allToasts = computed<ToastItem[]>(() => ui.toasts);

function dismissToast(toast: ToastItem) {
  ui.removeToast(toast.id);
  removeGlobalToast(toast.id);
}

function typeIcon(type: string) {
  const icons: Record<string, typeof Check> = {
    success: Check,
    error: X,
    warning: AlertTriangle,
    info: Info,
  };
  return icons[type] || Info;
}

const containerStyle = {
  position: 'fixed' as const,
  bottom: '40px',
  left: '50%',
  transform: 'translateX(-50%)',
  display: 'flex',
  flexDirection: 'column-reverse' as const,
  gap: AppleSpace.sm,
  zIndex: String(AppleZIndex.toast),
  pointerEvents: 'none' as const,
  maxHeight: '50vh',
  overflowY: 'visible' as const,
};

function getToastStyle(type: string) {
  const typeColors: Record<string, string> = {
    success: cv('green'),
    error: cv('red'),
    warning: cv('orange'),
    info: cv('blue'),
  };
  const bgColor = typeColors[type] || cv('text-primary');

  return {
    background: bgColor,
    color: cv('on-accent'),
    fontFamily: AppleFont.text,
    fontSize: AppleType.controlLabel.size,
    fontWeight: String(AppleFontWeight.medium),
    lineHeight: String(AppleType.bodyPrimary.lineHeight),
    letterSpacing: AppleType.bodyPrimary.letterSpacing,
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    borderRadius: AppleRadius.card,
    boxShadow: cv('shadow-modal'),
    // 这里原先挂着 backdropFilter: cv('glass-blur') —— 而上面 background 是不透明的类型色，
    // 模糊一个从不透光的东西没有任何视觉效果，只付一次 GPU，还白占一处玻璃层预算（红线 ≤3）。
    // 普查门禁 check:glass 把这两行扫出来的：壳面必须交代自己压在谁上面，不透光的就别铺玻璃。
    minWidth: '200px',
    maxWidth: 'min(560px, 90vw)',
    textAlign: 'left' as const,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-start',
    gap: AppleSpace.sm,
    pointerEvents: 'auto' as const,
  };
}

function iconStyle(type: string) {
  const typeBg: Record<string, string> = {
    success: cv('green-bg-subtle'),
    error: cv('red-bg-subtle'),
    warning: cv('orange-bg-subtle'),
    info: cv('blue-bg-subtle'),
  };
  const typeFg: Record<string, string> = {
    success: cv('green-text'),
    error: cv('red-text'),
    warning: cv('orange-text'),
    info: cv('blue-text'),
  };
  return {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: '22px',
    height: '22px',
    borderRadius: AppleRadius.circle,
    background: typeBg[type] || cv('bg-tertiary'),
    color: typeFg[type] || cv('text-secondary'),
    fontSize: AppleType.microUI.size,
    fontWeight: String(AppleFontWeight.semibold),
    flexShrink: '0' as const,
  };
}

const messageStyle = {
  flex: '1',
  textAlign: 'left' as const,
  wordBreak: 'break-word' as const,
};
</script>

<style scoped>
.ap-toast-enter-active,
.ap-toast-leave-active {
  transition: opacity var(--apple-dur-default) var(--apple-easing-standard), transform var(--apple-dur-default) var(--apple-easing-emphasized);
}
.ap-toast-enter-from {
  opacity: 0;
  transform: translateY(20px) scale(0.96);
}
.ap-toast-leave-to {
  opacity: 0;
  transform: translateY(10px) scale(0.96);
}
</style>
