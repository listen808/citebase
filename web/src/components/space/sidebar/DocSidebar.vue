<template>
  <div data-glass-surface="doc-tree" :style="sidebarComputedStyle" role="navigation" :aria-label="spaceName ? `${spaceName} 侧边栏` : '文档侧边栏'">
    <div :style="sidebarCollapsed ? collapsedHeaderBarStyle : headerStyle">
      <template v-if="sidebarCollapsed">
        <div :style="collapsedHeaderStyle">
          <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('spaceUi.k101')" :aria-label="$t('spaceUi.k101')" @click="$emit('toggleSidebar')">
            <ApIcon name="ChevronRight" :size="16" />
          </ApButton>
        </div>
      </template>
      <template v-else>
        <div :style="headerLeftStyle">
          <div :style="spaceIconStyle">{{ spaceNameInitial }}</div>
          <div :style="headerTitleWrapStyle">
            <span :style="titleStyle">{{ spaceName || $t('common.document') }}</span>
            <span v-if="(docCount ?? 0) > 0" :style="docCountStyle">{{ docCount }} {{ $t('cExtraUi.k12') }}</span>
          </div>
        </div>
        <div :style="actionsStyle">
          <ApButton variant="ghost" size="mini" icon-only :tooltip="getSortLabel()" :aria-label="getSortLabel()" @click="cycleSort">
            <ApIcon name="ArrowUpDown" :size="16" />
          </ApButton>
          <ApButton variant="ghost" size="mini" icon-only :tooltip="allExpanded ? '折叠所有' : '展开所有'" :aria-label="allExpanded ? '折叠所有目录' : '展开所有目录'" @click="toggleExpandAll">
            <ApMorphIcon :icon="allExpanded ? ChevronDownData : ChevronRightData" :size="16" />
          </ApButton>
          <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('spaceUi.k102')" :aria-label="$t('spaceUi.k102')" @click="$emit('trash')">
            <ApIcon name="Trash2" :size="16" />
          </ApButton>
          <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('spaceUi.k50')" :aria-label="$t('spaceUi.k50')" @click="$emit('create', 0, spaceId, 1)">
            <ApIcon name="Plus" :size="16" />
          </ApButton>
          <ApButton v-if="isMobile" variant="ghost" size="mini" icon-only :aria-label="$t('spaceUi.k103')" @click="$emit('close')">
            <ApIcon name="X" :size="16" />
          </ApButton>
        </div>
      </template>
    </div>

    <div v-if="!sidebarCollapsed" :style="searchStyle">
      <ApSearch
        :model-value="searchText"
        :placeholder="$t('spaceUi.k97')"
        :debounce="200"
        @update:model-value="$emit('update:searchText', $event)"
        @search="$emit('search')"
      />
    </div>

    <div :style="sidebarCollapsed ? collapsedBodyStyle : bodyStyle">
      <template v-if="sidebarCollapsed">
        <ApButton variant="ghost" size="mini" icon-only :tooltip="$t('spaceUi.k101')" :aria-label="$t('spaceUi.k101')" @click="$emit('toggleSidebar')">
          <ApIcon name="ChevronRight" :size="16" />
        </ApButton>
      </template>
      <template v-else>
      <div :style="segmentedWrapStyle">
        <ApSegmentedControl v-model="activeSection" :options="sectionOptions" size="small" />
      </div>

      <template v-if="activeSection === 'all'">
        <div v-if="loading && !tree.length" :style="treeSkeletonStyle">
          <div v-for="i in 6" :key="i" :style="treeSkeletonItemStyle(i)">
            <div :style="skeletonLineStyle(Math.random() * 40 + 60)" />
          </div>
        </div>
        <ApEmpty v-else-if="!tree.length" icon="Folder" :title="$t('spaceUi.k98')" />
        <DocumentTree
          v-else
          :nodes="sortedTree"
          :expanded="expandedNodes"
          :selected="selectedId || undefined"
          :loading-children="loadingChildren"
          :search-text="searchText"
          :focused-id="localFocusedId"
          :search-match-id="searchMatchId"
          :can-drag="canDragReorder"
          @toggle="$emit('toggleNode', $event)"
          @select="handleDocSelect"
          @create="(parentId, spaceId, type) => $emit('create', parentId, spaceId, type)"
          @rename="$emit('rename', $event)"
          @delete="$emit('delete', $event)"
          @move="$emit('move', $event)"
          @copy="$emit('copy', $event)"
          @contextmenu="contextMenu.show"
          @drag-reorder="(sourceId, targetId, position) => $emit('drag-reorder', sourceId, targetId, position)"
          @focus="handleTreeFocus"
          @scroll-to-match="$emit('scroll-to-match', $event)"
          @keydown="treeKeydown"
        />
      </template>

      <template v-else-if="activeSection === 'collected'">
        <div v-if="collectionStore.loading" :style="flatListStyle">
          <div v-for="i in 4" :key="i" :style="flatItemStyle">
            <div :style="{ width: '13px', height: '13px', borderRadius: AppleRadius['2px'], background: cv('bg-tertiary'), flexShrink: 0 }" />
            <div :style="{ height: '14px', borderRadius: AppleRadius.micro, background: cv('bg-tertiary'), flex: 1, maxWidth: `${60 + Math.random() * 30}%` }" />
          </div>
        </div>
        <ApEmpty v-else-if="!collectionStore.items.length" icon="Star" :title="$t('spaceUi.k99')" />
        <div v-else :style="flatListStyle">
          <div v-for="item in collectionStore.items" :key="item.id" :style="flatItemStyle" @click="handleFlatDocClick(item.resource_id, item.space_id ?? 0, 1)">
            <ApIcon name="FileText" :size="14" :style="{ color: cv('text-quaternary'), flexShrink: 0 }" />
            <span :style="flatNameStyle">{{ item.name || $t('fallback.k74') }}</span>
            <span v-if="getSpaceName(item.space_id ?? 0)" :style="flatSubStyle">{{ getSpaceName(item.space_id ?? 0) }}</span>
          </div>
        </div>
      </template>

      <template v-else-if="activeSection === 'recent'">
        <div v-if="recentDocs.loading.value" :style="flatListStyle">
          <div v-for="i in 4" :key="i" :style="flatItemStyle">
            <div :style="{ width: '13px', height: '13px', borderRadius: AppleRadius['2px'], background: cv('bg-tertiary'), flexShrink: 0 }" />
            <div :style="{ height: '14px', borderRadius: AppleRadius.micro, background: cv('bg-tertiary'), flex: 1, maxWidth: `${50 + Math.random() * 40}%` }" />
            <div :style="{ height: '12px', borderRadius: AppleRadius.micro, background: cv('bg-tertiary'), width: '50px' }" />
          </div>
        </div>
        <ApEmpty v-else-if="recentDocs.error.value" icon="Clock" :title="$t('spaceUi.k106')" />
        <ApEmpty v-else-if="!recentDocs.docs.value.length" icon="Clock" :title="$t('spaceUi.k100')" />
        <div v-else :style="flatListStyle">
          <div v-for="doc in recentDocs.docs.value" :key="doc.id" :style="flatItemStyle" @click="handleFlatDocClick(doc.id, doc.space_id, 1)">
            <ApIcon name="FileText" :size="14" :style="{ color: cv('text-quaternary'), flexShrink: 0 }" />
            <span :style="flatNameStyle">{{ doc.name }}</span>
            <span :style="flatSubStyle">{{ formatRelativeTime(doc.update_time) }}</span>
          </div>
        </div>
      </template>
      </template>
    </div>

    <DocContextMenu
      :visible="contextMenu.visible.value"
      :position="contextMenu.position.value"
      :doc="contextMenu.doc.value"
      :can-edit="canEdit"
      @create="(p, s, t) => $emit('create', p, s, t)"
      @rename="(d) => $emit('rename', d)"
      @delete="(d) => $emit('delete', d)"
      @move="(d) => $emit('move', d)"
      @copy="(d) => $emit('copy', d)"
      @close="contextMenu.hide"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, ref, shallowRef, watch } from 'vue';
import { ChevronDown as ChevronDownData, ChevronRight as ChevronRightData } from '@/components/apple/morph-icon-data';
import ApMorphIcon from '@/components/apple/ApMorphIcon.vue';
import { AppleLayout, AppleRadius, cv } from '@/utils/core/apple-tokens';
import ApSearch from '@/components/apple/ApSearch.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import type { SegmentedOption } from '@/components/apple/ApSegmentedControl.vue';
import DocumentTree from '@/components/document/DocumentTree.vue';
import DocContextMenu from '@/components/space/sidebar/DocContextMenu.vue';
import { useTreeKeyboard } from '@/composables/tree/useTreeKeyboard';
import { useTreeSort } from '@/composables/tree/useTreeSort';
import { useContextMenu } from '@/composables/ui/useContextMenu';
import { useRecentDocs } from '@/composables/ui/useRecentDocs';
import { useCollectionStore } from '@/stores/workspace/collection';
import { useSpaceStore } from '@/stores/workspace/space';
import { formatRelativeTime as formatRelativeTimeShared } from '@/utils/format/format-time';
import type { Document, Space } from '@/types/models';
import { sidebarStyle, getSpaceIconStyle, headerStyle, headerLeftStyle, headerTitleWrapStyle, titleStyle, docCountStyle, actionsStyle, searchStyle, bodyStyle, segmentedWrapStyle, treeSkeletonStyle, treeSkeletonItemStyle, skeletonLineStyle, flatListStyle, flatItemStyle, flatNameStyle, flatSubStyle, collapsedBodyStyle, collapsedHeaderStyle, collapsedHeaderBarStyle } from '@/styles/layout/doc-sidebar-styles';

const props = defineProps<{
  spaceId: number;
  spaceName?: string;
  docCount?: number;
  tree: Document[];
  filteredTree: Document[];
  expandedNodes: Set<number>;
  selectedId: number | null;
  searchText: string;
  loading: boolean;
  loadingChildren?: Set<number>;
  sidebarCollapsed: boolean;
  isMobile: boolean;
  canEdit?: boolean;
  focusedId?: number | null;
  searchMatchId?: number | null;
}>();

const emit = defineEmits<{
  (e: 'toggleSidebar'): void;
  (e: 'create', parentId: number, spaceId: number, type: 1 | 2): void;
  (e: 'search'): void;
  (e: 'toggleNode', id: number): void;
  (e: 'selectDoc', doc: { id: number; space_id: number; type: number }): void;
  (e: 'locateDoc', docId: number): void;
  (e: 'close'): void;
  (e: 'update:searchText', value: string): void;
  (e: 'expandAll'): void;
  (e: 'collapseAll'): void;
  (e: 'trash'): void;
  (e: 'rename', doc: { id: number; name: string }): void;
  (e: 'delete', doc: { id: number; name: string; type?: number }): void;
  (e: 'move', doc: { id: number; name: string; type: number; space_id: number }): void;
  (e: 'copy', doc: { id: number; name: string; type: number; space_id: number }): void;
  (e: 'show-context-menu', event: MouseEvent, doc: { id: number; name: string; type: number; space_id: number; parent_id?: number }): void;
  (e: 'drag-reorder', sourceId: number, targetId: number, position: 'before' | 'after' | 'inside'): void;
  (e: 'update:focusedId', id: number | null): void;
  (e: 'scroll-to-match', id: number): void;
}>();
const { t } = useI18n();

const { sortedTree, sortOrder: _sortOrder, cycleSort, getSortLabel } = useTreeSort(() => props.filteredTree);

const allExpanded = computed(() => {
  let total = 0;
  function checkAllExpanded(nodes: Document[]): boolean {
    for (const node of nodes) {
      const hasLoadedChildren = node.children && node.children.length > 0;
      const hasUnloadableChildren = node.type === 2 && node.has_children;
      if (hasLoadedChildren || hasUnloadableChildren) {
        total++;
        if (!props.expandedNodes.has(node.id)) return false;
        if (node.children && node.children.length > 0) {
          if (!checkAllExpanded(node.children)) return false;
        }
      }
    }
    return true;
  }
  const allExpandedResult = checkAllExpanded(sortedTree.value);
  return total > 0 && allExpandedResult;
});

function toggleExpandAll() {
  if (allExpanded.value) {
    emit('collapseAll');
  } else {
    emit('expandAll');
  }
}
const contextMenu = useContextMenu();
const localFocusedId = ref<number | null>(props.focusedId ?? null);
watch(() => props.focusedId, (val) => { localFocusedId.value = val ?? null; });
const canDragReorder = computed(() => props.canEdit !== false);

function findNode(nodes: Document[], id: number): Document | undefined {
  for (const node of nodes) {
    if (node.id === id) return node;
    if (node.children) {
      const found = findNode(node.children, id);
      if (found) return found;
    }
  }
  return undefined;
}

const { handleKeydown: treeKeydown } = useTreeKeyboard({
  tree: sortedTree,
  expandedNodes: computed(() => props.expandedNodes),
  focusedId: localFocusedId,
  selectedId: computed(() => props.selectedId),
  onToggle: (id: number) => emit('toggleNode', id),
  onSelect: (doc: Document) => emit('selectDoc', { id: doc.id, space_id: doc.space_id, type: doc.type }),
  onRename: (id: number) => {
    const node = findNode(sortedTree.value, id);
    if (node) emit('rename', { id: node.id, name: node.name });
  },
  onDelete: (id: number) => {
    const node = findNode(sortedTree.value, id);
    if (node) emit('delete', { id: node.id, name: node.name, type: node.type });
  },
});

function handleTreeFocus(id: number) {
  localFocusedId.value = id;
  emit('update:focusedId', id);
}

function handleDocSelect(doc: { id: number; space_id: number; type: number }) {
  emit('selectDoc', doc);
}

const activeSection = shallowRef<string>('all');
const sectionOptions: SegmentedOption[] = [
  { label: t('spaceUi.all'), value: 'all' },
  { label: t('spaceUi.favorite'), value: 'collected' },
  { label: t('spaceUi.k105'), value: 'recent' },
];

const collectionStore = useCollectionStore();
const spaceStore = useSpaceStore();
const recentDocs = useRecentDocs();

watch(activeSection, (section) => {
  if (section === 'collected') {
    collectionStore.loadCollections({ type: 1 });
  }
  if (section === 'recent' && !recentDocs.docs.value.length) {
    recentDocs.loadRecentDocs();
  }
});

function getSpaceName(spaceId: number): string {
  if (!spaceId) return '';
  const list = spaceStore.spaces ?? [];
  const space = list.find((s: Space) => s.id === spaceId);
  return space?.name || '';
}

// 一周是「还算近期」的界限，属于本视图的取舍；说法本身归 time.*（口径下沉 format-time.ts）。
function formatRelativeTime(timestamp: number): string {
  return formatRelativeTimeShared(timestamp, { recentDays: 7, older: 'full' });
}

function handleFlatDocClick(id: number, docSpaceId: number, type: number) {
  if (docSpaceId && docSpaceId !== props.spaceId) {
    emit('selectDoc', { id, space_id: docSpaceId, type });
    return;
  }
  activeSection.value = 'all';
  emit('locateDoc', id);
}

// ============ 样式 ============

// 头像占位字母取的是**兜底标签自己的首字母**，不是硬编码一个「文」：
// 英文档下兜底标签是 "Document"，写死 '文' 会让一个纯装饰字符变成中文残留
// （check:i18n-runtime 报的就是它，而它确实该报 —— 这不是数据，是代码写的字）。
const spaceNameInitial = computed(() => (props.spaceName || t('common.document')).slice(0, 1).toUpperCase());

const sidebarWidth = shallowRef(parseInt(AppleLayout.sidebarWidth));
const sidebarComputedStyle = sidebarStyle(props.isMobile, sidebarWidth, props.sidebarCollapsed);
const spaceIconStyle = getSpaceIconStyle();
</script>
