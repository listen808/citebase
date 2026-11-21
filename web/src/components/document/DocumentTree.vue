<template>
  <div role="tree" :aria-label="'文档树'" :style="treeStyle" @keydown="$emit('keydown', $event)">
    <TreeNode
      v-for="node in nodes"
      :key="node.id"
      :node="node"
      :expanded="expanded"
      :selected="selected"
      :loading-children="loadingChildren"
      :level="0"
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
  </div>
</template>

<script setup lang="ts">
import type { Document } from '@/types/models';
import TreeNode from './TreeNode.vue';

defineProps<{
  nodes: Document[];
  expanded: Set<number>;
  selected?: number;
  loadingChildren?: Set<number>;
  searchText?: string;
  focusedId?: number | null;
  searchMatchId?: number | null;
  cutDocId?: number | null;
  canDrag?: boolean;
}>();

defineEmits<{
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
  (e: 'keydown', event: KeyboardEvent): void;
}>();

const treeStyle = {
  userSelect: 'none' as const,
};
</script>
