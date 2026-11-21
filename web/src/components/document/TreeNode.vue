<template>
  <div>
    <div
      ref="rowRef"
      role="treeitem"
      :aria-selected="isSelected"
      :aria-expanded="hasChildren ? isExpanded : undefined"
      :tabindex="0"
      :draggable="canDrag"
      :style="rowStyle"
      @click="handleClick"
      @keydown="handleKeydown"
      @focusin="$emit('focus', node.id)"
      @mouseenter="hover = true"
      @mouseleave="hover = false"
      @contextmenu.prevent="emit('contextmenu', $event, { id: node.id, name: node.name, type: node.type, space_id: node.space_id, parent_id: node.parent_id })"
      @dragstart="onDragStart"
      @dragenter="onDragEnter"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
      @dragend="onDragEnd"
    >
      <span :style="indentStyle" />
      <span
        v-if="hasChildren"
        :style="toggleBtnStyle"
        role="button"
        tabindex="-1"
        :aria-label="$t('docUi1.k85')"
        @click.stop="$emit('toggle', node.id)"
      >
        <ApIcon
          name="ChevronRight"
          :size="14"
          :style="{ transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)', transition: AppleTransition.fast }"
        />
      </span>
      <span v-else :style="placeholderStyle" />
      <ApIcon v-if="node.type === 1" name="FileText" :size="14" :style="iconStyle" />
      <ApIcon v-else name="Folder" :size="14" :style="iconStyle" />
      <span :style="nameStyle" :title="node.name">
        <template v-if="searchText">
          <span v-for="(part, i) in highlightParts" :key="i" :style="part.matched ? highlightMatchStyle : {}">{{ part.text }}</span>
        </template>
        <template v-else>{{ node.name }}</template>
      </span>
      <span
        v-if="hover && node.type === 2"
        :style="addActionStyle"
        role="button"
        tabindex="-1"
        :title="$t('docUi1.k90')"
        @click.stop="$emit('create', node.id, node.space_id, 1)"
      >
        <ApIcon name="Plus" :size="14" />
      </span>
      <div v-if="dragOverPosition === 'before'" :style="dropIndicatorStyle('before')" />
      <div v-if="dragOverPosition === 'after'" :style="dropIndicatorStyle('after')" />
    </div>

    <!-- 子节点容器 -->
    <div v-if="showChildrenContainer" v-show="isExpanded" :style="childrenContainerStyle" role="group">
        <!-- 加载中状态 -->
        <div v-if="isLoadingChildren" :style="loadingIndicatorStyle">
          <span>{{ $t('docUi1.k13') }}</span>
        </div>
        <!-- 空目录提示（只在确认无子节点时显示，避免加载闪烁） -->
        <div v-else-if="!node.has_children && (!node.children || node.children.length === 0)" :style="emptyDirStyle">
          <span>{{ $t('docUi1.k89') }}</span>
        </div>
        <!-- 子节点列表 -->
        <template v-else>
          <TreeNode
            v-for="child in node.children"
            :key="child.id"
            :node="child"
            :expanded="expanded"
            :selected="selected"
            :loading-children="loadingChildren"
            :level="level + 1"
            :search-text="searchText"
            :focused-id="focusedId"
            :search-match-id="searchMatchId"
            :cut-doc-id="cutDocId"
            :can-drag="canDrag"
            @toggle="$emit('toggle', $event)"
            @select="$emit('select', $event)"
            @create="(parentId: number, spaceId: number, type: 1 | 2) => $emit('create', parentId, spaceId, type)"
            @rename="$emit('rename', $event)"
            @delete="$emit('delete', $event)"
            @move="$emit('move', $event)"
            @copy="$emit('copy', $event)"
            @contextmenu="(event, doc) => $emit('contextmenu', event, doc)"
            @drag-reorder="(sourceId, targetId, position) => $emit('drag-reorder', sourceId, targetId, position)"
            @focus="$emit('focus', $event)"
            @scroll-to-match="$emit('scroll-to-match', $event)"
          />
        </template>
      </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch, nextTick } from 'vue';
import type { Document } from '@/types/models';
import { cv, AppleFont, AppleFontWeight, AppleSpace, AppleRadius, AppleTransition, AppleType, AppleZIndex, AppleLineHeight } from '@/utils/core/apple-tokens';

const props = defineProps<{
  node: Document;
  expanded: Set<number>;
  selected?: number;
  loadingChildren?: Set<number>;
  level: number;
  searchText?: string;
  focusedId?: number | null;
  searchMatchId?: number | null;
  cutDocId?: number | null;
  canDrag?: boolean;
}>();

const emit = defineEmits<{
  (e: 'toggle', id: number): void;
  (e: 'select', doc: { id: number; space_id: number; type: number }): void;
  (e: 'rename', doc: { id: number; name: string }): void;
  (e: 'delete', doc: { id: number; name: string }): void;
  (e: 'contextmenu', event: MouseEvent, doc: { id: number; name: string; type: number; space_id: number; parent_id?: number }): void;
  (e: 'create', parentId: number, spaceId: number, type: 1 | 2): void;
  (e: 'move', doc: { id: number; name: string; type: number; space_id: number }): void;
  (e: 'copy', doc: { id: number; name: string; type: number; space_id: number }): void;
  (e: 'drag-reorder', sourceId: number, targetId: number, position: 'before' | 'after' | 'inside'): void;
  (e: 'focus', id: number): void;
  (e: 'scroll-to-match', id: number): void;
  (e: 'drag-check', sourceId: number, targetId: number, position: 'before' | 'after' | 'inside'): boolean;
}>();

const hover = shallowRef(false);
const rowRef = ref<HTMLElement | null>(null);

const isExpanded = computed(() => props.expanded.has(props.node.id));
const isSelected = computed(() => props.selected === props.node.id);
const isLoadingChildren = computed(() => {
  return !!props.loadingChildren?.has(props.node.id);
});
const hasChildren = computed(() => {

  return (props.node.children && props.node.children.length > 0) || (props.node.type === 2 && props.node.has_children);
});

const showChildrenContainer = computed(() => {
  if (isLoadingChildren.value) return true;
  if (hasChildren.value) return true;
  return props.node.type === 2 && !props.node.has_children && !!props.node.children && props.node.children.length === 0;
});

const highlightParts = computed(() => {
  const text = props.node.name;
  const keyword = (props.searchText || '').trim();
  if (!keyword) return [{ text, matched: false }];
  const lowerText = text.toLowerCase();
  const lowerKeyword = keyword.toLowerCase();
  const parts: { text: string; matched: boolean }[] = [];
  let lastIndex = 0;
  let idx = lowerText.indexOf(lowerKeyword, lastIndex);
  while (idx !== -1) {
    if (idx > lastIndex) parts.push({ text: text.slice(lastIndex, idx), matched: false });
    parts.push({ text: text.slice(idx, idx + keyword.length), matched: true });
    lastIndex = idx + keyword.length;
    idx = lowerText.indexOf(lowerKeyword, lastIndex);
  }
  if (lastIndex < text.length) parts.push({ text: text.slice(lastIndex), matched: false });
  return parts;
});

const highlightMatchStyle = {
  background: cv('search-highlight'),
  borderRadius: AppleRadius['2px'],
  padding: `0 ${AppleSpace.xxs}`,
};

// 选中时自动滚动到视图中
watch(
  isSelected,
  async (selected) => {
    if (selected && rowRef.value) {
      await nextTick();
      rowRef.value?.scrollIntoView({
        block: 'nearest',
        behavior: 'smooth' });
    }
  }
);

// 搜索命中时自动滚动到视图中
watch(
  () => props.searchMatchId,
  async (matchId) => {
    if (matchId === props.node.id && rowRef.value) {
      await nextTick();
      rowRef.value?.scrollIntoView({
        block: 'nearest',
        behavior: 'smooth' });
      emit('scroll-to-match', props.node.id);
    }
  }
);

const isCut = computed(() => props.cutDocId != null && props.cutDocId === props.node.id);

// 层级几何：缩进步长、箭头/占位符统一宽度，保证同层文件夹与叶子严格对齐
const INDENT_STEP = 12;
const MAX_INDENT = 144; // 12 层封顶，避免深层塌缩后全部同缩进
const TOGGLE_W = 18;

const rowStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xxs,
  // P2-8：22px 行高 + 14px 字号挤在一起，30 行树几乎连成一片；抬到 32px 给点击与扫读留余量
  height: AppleSpace.xl,
  padding: `0 ${AppleSpace.xxs} 0 ${AppleSpace.xxs}`,
  margin: `0 ${AppleSpace.xxs}`,
  borderRadius: AppleRadius.small,
  cursor: 'pointer',
  opacity: isDragging.value ? 0.5 : isCut.value ? 0.5 : 1,
  background: dragOverPosition.value === 'inside'
    ? cv('blue-selection')
    : isSelected.value ? cv('blue-selection-strong') : hover.value ? cv('bg-hover') : 'transparent',
  transition: AppleTransition.fast,
  position: 'relative' as const,
}));

function dropIndicatorStyle(position: 'before' | 'after') {
  return {
    position: 'absolute' as const,
    [position === 'before' ? 'top' : 'bottom']: '0px',
    left: `${(props.level || 0) * INDENT_STEP + 4}px`,
    right: AppleSpace.sm,
    height: AppleSpace.xxs,
    background: cv('blue'),
    borderRadius: AppleRadius['1px'],
    boxShadow: `0 0 4px ${cv('blue')}`,
    pointerEvents: 'none' as const,
    zIndex: AppleZIndex.base,
  };
}

const indentStyle = computed(() => ({
  width: `${Math.min(props.level * INDENT_STEP, MAX_INDENT)}px`,
  flexShrink: 0 }));


const placeholderStyle = {
  width: `${TOGGLE_W}px`,
  flexShrink: 0 };

const toggleBtnStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: `${TOGGLE_W}px`,
  height: `${TOGGLE_W}px`,
  flexShrink: 0,
  cursor: 'pointer',
  borderRadius: AppleRadius.small,
  color: cv('text-tertiary'),
  transition: AppleTransition.fast,
};

const addActionStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: `${TOGGLE_W}px`,
  height: `${TOGGLE_W}px`,
  flexShrink: 0,
  cursor: 'pointer',
  borderRadius: AppleRadius.small,
  color: cv('text-tertiary'),
  transition: AppleTransition.fast,
};

const iconStyle = computed(() => ({
  color: isSelected.value ? cv('blue') : 
    props.node.type === 2 ? cv('folder-icon') : cv('text-quaternary'),
  flexShrink: 0,
  minWidth: '15px'
}));

const nameStyle = computed(() => ({
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: isSelected.value ? String(AppleFontWeight.medium) : String(AppleFontWeight.regular),
  lineHeight: AppleLineHeight.default,
  color: isSelected.value ? cv('blue') : cv('text-primary'),
  letterSpacing: AppleType.controlLabel.letterSpacing,
  whiteSpace: 'nowrap' as const,
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  marginLeft: AppleSpace.xs,
  flexGrow: 1,
  textDecoration: isCut.value ? 'line-through' : 'none',
}));

function handleClick() {
  emit('select', {
    id: props.node.id,
    space_id: props.node.space_id,
    type: props.node.type });

}


const childrenContainerStyle = {
  marginLeft: '0px',
};

const loadingIndicatorStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  color: cv('text-quaternary'),
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
};

const emptyDirStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  color: cv('text-quaternary'),
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
};

function handleKeydown(e: KeyboardEvent) {
  switch (e.key) {
    case 'Enter':
    case ' ':
      e.preventDefault();
      handleClick();
      break;
    case 'ArrowRight':
      if (hasChildren.value && !isExpanded.value) {
        e.preventDefault();
        emit('toggle', props.node.id);
      }
      break;
    case 'ArrowLeft':
      if (hasChildren.value && isExpanded.value) {
        e.preventDefault();
        emit('toggle', props.node.id);
      }
      break;
  }
}

const dragOverPosition = shallowRef<'before' | 'after' | 'inside' | null>(null);
const isDragging = shallowRef(false);

function onDragStart(e: DragEvent) {
  if (!props.canDrag) { e.preventDefault(); return; }
  e.dataTransfer!.setData('text/plain', String(props.node.id));
  e.dataTransfer!.effectAllowed = 'move';
  isDragging.value = true;
}

function onDragEnter(e: DragEvent) {
  if (!props.canDrag) return;
  e.preventDefault();
}

function onDragOver(e: DragEvent) {
  if (!props.canDrag) return;
  e.preventDefault();
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const y = e.clientY - rect.top;
  const h = rect.height;
  if (props.node.type === 2 && y > h * 0.2 && y < h * 0.8) {
    dragOverPosition.value = 'inside';
    e.dataTransfer!.dropEffect = 'move';
  } else if (y <= h * 0.5) {
    dragOverPosition.value = 'before';
    e.dataTransfer!.dropEffect = 'move';
  } else {
    dragOverPosition.value = 'after';
    e.dataTransfer!.dropEffect = 'move';
  }
}

function onDrop(e: DragEvent) {
  if (!props.canDrag) return;
  e.preventDefault();
  const sourceId = Number(e.dataTransfer!.getData('text/plain'));
  if (sourceId === props.node.id || !dragOverPosition.value) return;
  emit('drag-reorder', sourceId, props.node.id, dragOverPosition.value);
  dragOverPosition.value = null;
}

function onDragLeave(e: DragEvent) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const x = e.clientX;
  const y = e.clientY;
  if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
    dragOverPosition.value = null;
  }
}

function onDragEnd() {
  dragOverPosition.value = null;
  isDragging.value = false;
}
</script>
