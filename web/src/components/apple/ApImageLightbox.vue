<template>
  <Teleport to="body">
    <div
      v-if="modelValue && current"
      ref="panelRef"
      data-ap="image-lightbox"
      :style="overlayStyle"
      role="dialog"
      aria-modal="true"
      :aria-label="label"
      tabindex="-1"
      @click.self="close"
    >
      <ApButton
        variant="overlay"
        size="mini"
        icon-only
        :aria-label="$t('apUi1.k5')"
        :style="closeStyle"
        @click="close"
      >
        <X :size="16" />
      </ApButton>

      <ApButton
        v-if="items.length > 1"
        variant="overlay"
        size="small"
        icon-only
        :aria-label="$t('apUi1.k6')"
        :style="navStyle('left')"
        @click="step(-1)"
      >
        <ChevronLeft :size="16" />
      </ApButton>

      <figure :style="figureStyle">
        <img :src="current.src" :alt="current.alt || current.name || ''" :style="imageStyle">
        <figcaption v-if="caption" :style="captionStyle">{{ caption }}</figcaption>
      </figure>

      <ApButton
        v-if="items.length > 1"
        variant="overlay"
        size="small"
        icon-only
        :aria-label="$t('apUi1.k7')"
        :style="navStyle('right')"
        @click="step(1)"
      >
        <ChevronRight :size="16" />
      </ApButton>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch, nextTick, ref } from 'vue';
import { ChevronLeft, ChevronRight, X } from 'lucide-vue-next';
import { AppleRadius, AppleSpace, AppleType, AppleZIndex, cv } from '@/utils/core/apple-tokens';
import { useFocusTrap, useScrollLock } from '@/composables/ui/useFocusTrap';
import { getOverlayStyle } from '@/styles/component/overlay';
import ApButton from '@/components/apple/ApButton.vue';

/**
 * 图片灯箱全站只此一份。
 * 并档前有四处各写一遍：正文图片预览、附件列表、文档附件面板（多图带翻页）、媒体管理
 * （那份干脆塞在 ApModal 里）。四处的遮罩、图片尺寸、翻页与可访问名字各不相同 ——
 * 附件面板那处既没有 `role=dialog` 也没有给关闭按钮起名，而它只在点缩略图之后才出现，
 * 运行时探针（只看渲染态）永远量不到，属于静态门禁与运行时巡检中间的那道缝。
 */
export interface LightboxItem {
  src: string;
  alt?: string;
  /** 说明行左半：文件名 */
  name?: string;
  /** 说明行中段：大小等附加信息 */
  meta?: string;
}

const props = withDefaults(defineProps<{
  modelValue: boolean;
  items: LightboxItem[];
  index?: number;
  label?: string;
}>(), {
  index: 0,
  label: '图片预览',
});

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
  (e: 'update:index', value: number): void;
}>();

const panelRef = ref<HTMLElement | null>(null);
const { setupFocusTrap, handleTabKey, saveActiveElement, restoreActiveElement } = useFocusTrap(panelRef);
const { lock, unlock } = useScrollLock();

const at = ref(props.index);
const current = computed<LightboxItem | null>(() => props.items[at.value] || props.items[0] || null);
const caption = computed(() => {
  const it = current.value;
  if (!it) return '';
  const parts = [it.name, it.meta].filter(Boolean) as string[];
  if (props.items.length > 1) parts.push(`${at.value + 1} / ${props.items.length}`);
  return parts.join(' · ');
});

function step(delta: number) {
  const n = props.items.length;
  if (n < 2) return;
  at.value = (at.value + delta + n) % n;      // 首尾相连：翻到头再按就回到另一头
  emit('update:index', at.value);
}

function close() {
  emit('update:modelValue', false);
}

watch(() => props.modelValue, (open) => {
  if (open) {
    at.value = props.index;
    lock();
    saveActiveElement();
    nextTick(() => { setupFocusTrap(); panelRef.value?.focus(); });
  } else {
    unlock();
    restoreActiveElement();
  }
}, { immediate: true });

function onKeydown(e: KeyboardEvent) {
  if (!props.modelValue) return;
  if (e.key === 'ArrowLeft') { e.preventDefault(); step(-1); }
  else if (e.key === 'ArrowRight') { e.preventDefault(); step(1); }
  else if (e.key === 'Escape') { e.preventDefault(); close(); }
  else handleTabKey(e);
}
document.addEventListener('keydown', onKeydown);
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown);
  unlock();
  restoreActiveElement();
});

const overlayStyle = computed(() => getOverlayStyle(AppleZIndex.modal, {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: AppleSpace['2xl'],
}));

const figureStyle = {
  margin: '0',
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  gap: AppleSpace.sm,
  minWidth: 0,
};

const imageStyle = {
  maxWidth: '90vw',
  maxHeight: '82vh',
  objectFit: 'contain' as const,
  borderRadius: AppleRadius.small,
  // 图片自身的衬底（透明 PNG 会露出来）属浮层档：lightbox 是一块浮层，
  // 原先写 surface-elevated —— 那一档是"内容滚到顶栏底下"的壳面加厚档，不是浮层底
  background: cv('surface-float'),
};

const captionStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('on-accent'),
  textAlign: 'center' as const,
  maxWidth: '90vw',
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const closeStyle = {
  position: 'absolute' as const,
  top: AppleSpace.lg,
  right: AppleSpace.lg,
};

const navStyle = (side: 'left' | 'right') => ({
  position: 'absolute' as const,
  [side]: AppleSpace.lg,
  top: '50%',
  // 不写这句，按钮的**上沿**在 50% 处，看着就是偏下；居中的是顶边而不是中心
  transform: 'translateY(-50%)',
});
</script>
