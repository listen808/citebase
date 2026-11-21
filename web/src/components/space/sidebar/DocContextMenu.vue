<template>
  <!-- 菜单 Teleport 到 body（对齐 ApSelect 的 P1-8）：否则它渲染在侧栏内部 DOM 里，
       侧栏滚动容器 overflow / transform 会把 fixed 菜单裁掉或顶偏。 -->
  <Teleport to="body">
  <div
    v-if="visible && doc"
    ref="menuRef"
    class="context-menu apple-glass-layer"
    :style="[menuStyle, { left: boundedPosition.x + 'px', top: boundedPosition.y + 'px' }]"
    @click.stop
  >
    <template v-for="(group, gi) in menuGroups" :key="gi">
      <template v-for="(item, ii) in group.items" :key="ii">
        <button type="button" class="context-menu-item" :style="[itemStyle, hoveredItem === `${gi}-${ii}` ? itemHoverStyle : {}, item.danger ? dangerStyle : {}]" @click="handleAction(item.action)" @mouseenter="hoveredItem = `${gi}-${ii}`" @mouseleave="hoveredItem = ''" @mousedown="activeItem = `${gi}-${ii}`" @mouseup="activeItem = ''">
          <ApIcon :name="item.icon" :size="16" />
          <span>{{ item.label }}</span>
        </button>
      </template>
      <div v-if="gi < menuGroups.length - 1" :style="dividerStyle" />
    </template>
  </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, nextTick, ref, watch } from 'vue';
import type { IconName } from '@/utils/icons';
import { AppleFont, AppleRadius, AppleSpace, AppleTransition, AppleType, AppleZIndex, cv, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { clampPosition } from '@/composables/useViewportBoundary';
import type { ContextMenuDoc } from '@/composables/ui/useContextMenu';
import { useUIStore } from '@/stores/app/ui';
import { copyText } from '@/utils/clipboard';

const props = defineProps<{
  visible: boolean;
  position: { x: number; y: number };
  doc: ContextMenuDoc | null;
  canEdit?: boolean;
}>();

const emit = defineEmits<{
  (e: 'create', parentId: number, spaceId: number, type: 1 | 2): void;
  (e: 'rename', doc: { id: number; name: string }): void;
  (e: 'delete', doc: { id: number; name: string; type?: number }): void;
  (e: 'move', doc: { id: number; name: string; type: number; space_id: number }): void;
  (e: 'copy', doc: { id: number; name: string; type: number; space_id: number }): void;
  (e: 'close'): void;
}>();
const { t } = useI18n();

const ui = useUIStore();

interface MenuItem {
  label: string;
  icon: IconName;
  action: string;
  danger?: boolean;
}

const menuWidth = 180;
// 实测高度（而非写死 220）：右键菜单按权限/类型最多 8 项，写死值会低估真实高度，
// 短视口下 clampPosition 无法翻到指针上方 → 菜单尾部被裁掉。
// 必须 nextTick 后才量 offsetHeight——同 tick 读会把上一份内容的高度套到本次，或拿到 0。
const menuRef = ref<HTMLElement | null>(null);
const measuredHeight = ref(220);
watch(
  () => [props.visible, props.doc, props.canEdit],
  async () => {
    await nextTick();
    if (menuRef.value) measuredHeight.value = menuRef.value.offsetHeight;
  },
  { immediate: true }
);

const boundedPosition = computed(() => clampPosition(props.position.x, props.position.y, menuWidth, measuredHeight.value));

const menuGroups = computed(() => {
  if (!props.doc) return [];
  const groups: { items: MenuItem[] }[] = [];
  const doc = props.doc;
  const canEdit = props.canEdit !== false;

  if (canEdit && doc.type === 2) {
    groups.push({ items: [
      { label: t('spaceUi.k88'), icon: 'FilePlus', action: 'createDoc' },
      { label: t('spaceUi.k89'), icon: 'FolderInput', action: 'createFolder' },
    ]});
  }

  groups.push({ items: [
    { label: t('spaceUi.k90'), icon: 'ExternalLink', action: 'openInNewTab' },
    { label: t('spaceUi.k91'), icon: 'Link2', action: 'copyLink' },
  ]});

  if (canEdit) {
    groups.push({ items: [
      { label: t('spaceUi.k92'), icon: 'FolderOutput', action: 'moveTo' },
      { label: t('spaceUi.k93'), icon: 'Files', action: 'copyTo' },
    ]});

    groups.push({ items: [
      { label: t('spaceUi.rename'), icon: 'Pencil', action: 'rename' },
    ]});

    groups.push({ items: [
      { label: t('spaceUi.delete'), icon: 'Trash2', action: 'delete', danger: true },
    ]});
  }

  return groups;
});

function handleAction(action: string) {
  if (!props.doc) return;
  const doc = props.doc;

  switch (action) {
    case 'createDoc':
      emit('create', doc.id, doc.space_id, 1);
      emit('close');
      break;
    case 'createFolder':
      emit('create', doc.id, doc.space_id, 2);
      emit('close');
      break;
    case 'openInNewTab': {
      const url = `${window.location.origin}/space/${doc.space_id}/doc/${doc.id}`;
      window.open(url, '_blank', 'noopener,noreferrer');
      emit('close');
      break;
    }
    case 'copyLink': {
      const url = `${window.location.origin}/space/${doc.space_id}/doc/${doc.id}`;
      copyText(url)
        .then((ok) => {
          ui.showToast(ok ? t('spaceUi.k95') : t('spaceUi.k96'), ok ? 'success' : 'error');
        })
        .finally(() => emit('close'));
      break;
    }
    case 'moveTo':
      emit('move', { id: doc.id, name: doc.name, type: doc.type, space_id: doc.space_id });
      emit('close');
      break;
    case 'copyTo':
      emit('copy', { id: doc.id, name: doc.name, type: doc.type, space_id: doc.space_id });
      emit('close');
      break;
    case 'rename':
      emit('rename', { id: doc.id, name: doc.name });
      emit('close');
      break;
    case 'delete':
      emit('delete', { id: doc.id, name: doc.name, type: doc.type });
      emit('close');
      break;
  }
}

// 右键菜单是"用户主动叫出来的交互层"，必须压在被动反馈的 toast 之上，所以显式抬一档。
const menuStyle = menuSurfaceStyle({
  position: 'fixed' as const,
  zIndex: String(AppleZIndex.toast),
  padding: AppleSpace.xs,
  minWidth: '160px',
});

const itemStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm, width: '100%',
  padding: `${AppleSpace.sm} ${AppleSpace.md}`, border: 'none', background: 'transparent',
  borderRadius: AppleRadius.small, fontFamily: AppleFont.text, fontSize: AppleType.microUI.size,
  color: cv('text-primary'), cursor: 'pointer', transition: AppleTransition.fast };

const dangerStyle = { color: cv('red') };

const hoveredItem = ref('');
const activeItem = ref('');
const itemHoverStyle = { background: cv('bg-hover') };

const dividerStyle = {
  height: '1px',
  background: cv('border-light'),
  margin: `${AppleSpace.xs} ${AppleSpace.sm}`,
};
</script>
