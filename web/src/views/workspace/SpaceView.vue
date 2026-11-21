<template>
  <ApPageContainer no-scroll max-width="none">
    <!-- 移动端顶部栏 -->
    <SpaceMobileHeader
      v-if="isMobile"
      :title="docStore.currentDocument?.name"
      @toggle-menu="showMobileMenu = !showMobileMenu"
    />

    <!-- 移动端侧边栏遮罩：透明度跟着抽屉走，不另铺一条 CSS 过渡 -->
    <div v-if="isMobile && showMobileMenu" :style="mobileOverlayStyle" aria-hidden="true" @click="showMobileMenu = false" />

    <!-- 移动端侧边栏（抽屉） -->
    <DocSidebar
      v-if="isMobile"
      ref="mobileSidebarRef"
      v-model:search-text="treeSearchText"
      :space-id="spaceId"
      :space-name="spaceStore.currentSpace?.name"
      :doc-count="spaceStore.currentSpace?.doc_count"
      :tree="docStore.tree"
      :filtered-tree="filteredTree"
      :expanded-nodes="docStore.expandedNodes"
      :selected-id="selectedDocumentId"
      :loading="docStore.treeLoading"
      :loading-children="docStore.loadingChildren"
      :sidebar-collapsed="false"
      :is-mobile="true"
      :can-edit="canEditInSpace"
      :focused-id="treeFocusedId"
      :search-match-id="searchMatchId"
      v-bind="drawerSpringAttrs"
      class="apple-glass-layer"
      :style="[mobileDrawerStyle, drawerTransformStyle]"
      @pointerdown="onDrawerPointerdown"
      @toggle-sidebar="showMobileMenu = !showMobileMenu"
      @create="openCreateModalWrapper"
      @toggle-node="handleToggleNode"
      @select-doc="handleSelectDoc"
      @locate-doc="handleLocateDoc"
      @close="showMobileMenu = false"
      @expand-all="expandAllNodes"
      @collapse-all="collapseAllNodes"
      @rename="handleRename"
      @delete="handleDelete"
      @move="handleMove"
      @copy="handleCopy"
      @trash="showTrashModal = true"
      @drag-reorder="handleDragReorder"
      @scroll-to-match="searchMatchId = null"
    />

    <!-- 桌面端侧边栏 + 内容区 -->
    <div :style="mainRowStyle">
      <!-- 桌面端侧边栏 -->
      <DocSidebar
        v-if="!isMobile"
        v-model:search-text="treeSearchText"
        :space-id="spaceId"
        :space-name="spaceStore.currentSpace?.name"
        :doc-count="spaceStore.currentSpace?.doc_count"
        :tree="docStore.tree"
        :filtered-tree="filteredTree"
        :expanded-nodes="docStore.expandedNodes"
        :selected-id="selectedDocumentId"
        :loading="docStore.treeLoading"
        :loading-children="docStore.loadingChildren"
        :sidebar-collapsed="false"
        :is-mobile="false"
        :can-edit="canEditInSpace"
        :focused-id="treeFocusedId"
        :search-match-id="searchMatchId"
        :style="{ width: sidebarWidth + 'px', flexShrink: 0 }"
        @create="openCreateModalWrapper"
        @toggle-node="handleToggleNode"
        @select-doc="handleSelectDoc"
        @locate-doc="handleLocateDoc"
        @expand-all="expandAllNodes"
        @collapse-all="collapseAllNodes"
        @rename="handleRename"
        @delete="handleDelete"
        @move="handleMove"
        @copy="handleCopy"
        @trash="showTrashModal = true"
        @drag-reorder="handleDragReorder"
        @scroll-to-match="searchMatchId = null"
      />

      <!-- 侧边栏拖拽手柄 -->
      <div
        v-if="!isMobile"
        :style="dragHandleStyle"
        @pointerdown="onSidebarDrag"
        @mouseenter="dragHandleHover = true"
        @mouseleave="dragHandleHover = false"
      />

      <!-- 内容区 -->
      <Teleport to="body" :disabled="!isFullscreen">
      <div ref="contentRef" :style="contentStyle">
        <!-- P2-7：空间/文档树加载失败后给可重试入口，不能只依赖会消失的 toast -->
        <div v-if="initError" :style="[warningBannerStyle, initErrorBannerStyle]" role="alert">
          <ApIcon name="AlertTriangle" :size="16" :color="cv('orange')" />
          <span>{{ initError === 'space' ? $t('tsUi.k33') : $t('tsUi.k32') }}</span>
          <button type="button" :style="retryLinkStyle" @click="retryInit">{{ $t('lastAttrsUi.retry') }}</button>
        </div>
        <DocContentPanel
          ref="contentPanelRef"
          :current-document="docStore.currentDocument"
          :current-content="docStore.currentContent || ''"
          :space-name="spaceStore.currentSpace?.name || $t('fallback.k94')"
          :crumbs="breadcrumbs"
          :has-back-button="false"
          :has-doc-selected="!!docStore.currentDocument"
          :is-mobile="isMobile"
          :loading="docStore.contentLoading || docStore.spaceInitializing"
          :editing="isEditing"
          :saving="isSaving"
          :highlight-keyword="highlightKeyword"
          :can-edit="canEditInSpace"
          @back="handleBack"
          @navigate-to-space-root="navigateToSpaceRoot"
          @navigate-to-doc="navigateToDoc"
          @save="handleInlineSave"
          @cancel="handleInlineCancel"
        >
          <template #toolbar>
            <DocToolbar
              :is-editor="canEditInSpace"
              :collected="isCollected"
              :editing="isEditing"
              :saving="isSaving"
              :fullscreen="isFullscreen"
              @edit="router.push(`/space/${spaceId}/doc/${docStore.currentDocument?.id}/edit`)"
              @images="handleOpenImages"
              @attachments="handleOpenAttachments"
              @collect="handleCollect"
              @share="handleShare"
              @export="handleExport"
              @export-html="handleExportHTML"
              @export-word="handleExportWord"
              @export-pdf="handleExportPDF"
              @export-md="handleExportMarkdown"
              @import="showImportModal = true"
              @history="handleHistory"
              @ai-summary="handleAISummary"
              @fullscreen="toggleFullscreen"
              @print="handlePrint"
              @delete="handleDeleteDoc"
              @save="contentPanelRef?.handleSave()"
              @cancel="handleInlineCancel"
            />
          </template>

          <template #spaceRoot>
            <SpaceRootContent
              :space-id="spaceId"
              :space-name="spaceStore.currentSpace?.name"
              :creator-name="spaceStore.currentSpace?.creator_name"
              :description="spaceStore.currentSpace?.description"
              :visit-level="spaceStore.currentSpace?.visit_level"
              :doc-count="spaceStore.currentSpace?.doc_count ?? 0"
              :member-count="spaceStore.currentSpace?.member_count ?? 0"
              :tags="spaceStore.currentSpace?.tags ?? ''"
              :space-collected="spaceCollected"
              :can-edit-in-space="canEditInSpace"
              @space-collect="handleSpaceCollect"
              @space-export="onSpaceExport"
              @open-members="openMembersModal"
              @create-doc="openCreateModalWrapper(0, spaceId, 1)"
              @import-docs="showImportModal = true"
              @navigate-to-doc="navigateToDoc"
              @view-graph="router.push('/ai/graph')"
            />
          </template>
        </DocContentPanel>
      </div>
      </Teleport>
    </div>

    <!-- 所有弹窗 -->
    <SpaceModals
      v-model:show-create-modal="showCreateModal"
      v-model:show-template-preview="showTemplatePreview"
      v-model:show-delete-modal="showDeleteModal"
      v-model:show-rename-modal="showRenameModal"
      v-model:show-share-modal="showShareModal"
      v-model:show-history-modal="showHistoryModal"
      v-model:show-images-modal="showImagesModal"
      v-model:show-attachments-modal="showAttachmentsModal"
      v-model:show-move-modal="showMoveModal"
      v-model:show-copy-modal="showCopyModal"
      v-model:show-shortcut-help="showShortcutHelp"
      v-model:create-doc-name="createDocName"
      v-model:create-doc-type="createDocType"
      v-model:selected-template-key="selectedTemplateKey"
      v-model:rename-doc-name="renameDocName"
      :creating-doc="creatingDoc"
      :renaming-doc="renamingDoc"
      :delete-message="deleteMessage"
      :deleting-doc-name="deletingDocName"
      :share-url="shareUrl"
      :shortcut-list="shortcutList"
      :current-doc-id="docStore.currentDocument?.id ?? null"
      :space-id="spaceId"
      :can-edit-in-space="canEditInSpace"
      :moving-doc-id="movingDocId"
      :copying-doc-id="copyingDocId"
      @create-doc="handleCreateDocWrapper"
      @confirm-delete="confirmDeleteDoc"
      @rename-doc="handleRenameDocWrapper"
      @copy-share-url="copyShareUrl"
      @version-rollback="handleVersionRollback"
      @confirm-move="confirmMoveWrapper"
      @confirm-copy="confirmCopyWrapper"
    />
    <SpaceAISummaryModal
      v-model="showAiSummaryModal"
      :loading="aiSummaryLoading"
      :summary-text="aiSummaryText"
    />

    <SpaceMembersModal
      v-model="showMembersModal"
      :space-id="spaceId"
      @show-toast="handleToast"
    />

    <ApConfirmModal
      v-model="showSpaceExportConfirm"
      :title="$t('ws.k20')"
      :message="`确定导出整个空间「${pendingExportSpaceName}」的内容？`"
      :confirm-text="$t('ws.export')"
      @confirm="confirmSpaceExport"
    />

    <DocumentTrash
      v-model="showTrashModal"
      :space-id="spaceId"
      @restored="handleTrashRestored"
      @permanent-deleted="handleTrashPermanentDeleted"
    />

    <BatchImportModal
      v-model="showImportModal"
      :space-id="spaceId"
      @done="docStore.loadTree(spaceId)"
    />

    <EditorModals
      v-model:show-recover-modal="unusedRecoverModal"
      v-model:show-unsaved-modal="unusedUnsavedModal"
      v-model:show-change-log-modal="showChangeLogModal"
      v-model:change-log-input="inlineChangeLog"
      v-model:skip-change-log="unusedSkipChangeLog"
      @confirm-save="confirmInlineSave"
      @cancel-save="showChangeLogModal = false"
    />
  </ApPageContainer>

</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, ref, shallowRef, watch, onMounted, onUnmounted, nextTick } from 'vue';
import { logger } from '@/utils/logger';
import { sanitizeMarkdownHtml } from '@/utils/core/markdown-sanitize';
import { AppleFont, AppleGlass, AppleLayout, AppleRadius, AppleSpace, AppleSpring, AppleTransition, AppleType, AppleZIndex, cv } from '@/utils/core/apple-tokens';
import { THEME_REGISTRY, supportsGlass } from '@/styles/theme/themes';
import { downloadSpace } from '@/api/modules/space/space';
import { getApiErrorStatus } from '@/api/client';
import { useDraft } from '@/composables/space/useDraft';
import { checkCollected } from '@/api/modules/space/collection';
import { useCollectionStore } from '@/stores/workspace/collection';
import { generateSummary } from '@/api/modules/ai/ai-dashboard';

import { useAuthStore } from '@/stores/auth/auth';
import { useSpaceInit } from '@/composables/space/useSpaceInit';
import { useSpaceTree } from '@/composables/space/useSpaceTree';
import { useSpaceShortcuts } from '@/composables/space/useSpaceShortcuts';
import { useDocActions } from '@/composables/space/useDocActions';
import { useDocCRUD } from '@/composables/space/useDocCRUD';
import { useTreeDragReorder } from '@/composables/tree/useTreeDragReorder';
import { useBreakpoint } from '@/utils/responsive';
import { useRecentDocs } from '@/composables/ui/useRecentDocs';
import { useResizable } from '@/composables/ui/useResizable';
import { useSheetGesture } from '@/composables/ui/useSheetGesture';
import { getOverlayStyle } from '@/styles/component/overlay';
import { warningBannerStyle } from '@/styles/layout/page-styles';
import { docTemplates } from '@/config/docTemplates';

import DocSidebar from '@/components/space/sidebar/DocSidebar.vue';
import DocContentPanel from '@/components/space/reader/DocContentPanel.vue';
import DocToolbar from '@/components/space/action/DocToolbar.vue';
import SpaceModals from '@/components/space/action/SpaceModals.vue';
import SpaceMobileHeader from './components/SpaceMobileHeader.vue';
import SpaceAISummaryModal from './components/SpaceAISummaryModal.vue';
import SpaceMembersModal from './components/SpaceMembersModal.vue';
import SpaceRootContent from './components/SpaceRootContent.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import DocumentTrash from '@/components/document/DocumentTrash.vue';
import BatchImportModal from '@/components/space/action/BatchImportModal.vue';
import EditorModals from './editor/EditorModals.vue';

const {
    route, router, docStore, spaceStore, ui, canEditInSpace,
    spaceId, docId, selectedDocumentId, breadcrumbs, contentRef, initError,
    retryInit, mountInit, loadDocument, navigateToSpaceRoot, navigateToDoc, handleSelectDoc, handleLocateDoc, handleBack,
    findSiblingOrParent, cleanup: initCleanup,
  } = useSpaceInit();
  const auth = useAuthStore();
  const collection = useCollectionStore();
  const contentPanelRef = ref<InstanceType<typeof DocContentPanel>>();

const isFromHome = computed(() => route.query.from === 'home');

// 就地编辑状态时记录原始内容快照
const isEditing = shallowRef(false);
const isSaving = shallowRef(false);
const originalContentSnapshot = shallowRef<string>('');
const inlineChangeLog = ref<string>('');
const showChangeLogModal = ref(false);
const pendingInlineContent = ref<string>('');
// EditorModals 占位：快捷编辑仅使用 ChangeLog Modal，其余 modal 始终关闭
const unusedRecoverModal = shallowRef(false);
const unusedUnsavedModal = shallowRef(false);
const unusedSkipChangeLog = shallowRef(false);

// 进入编辑态时记录原始内容快照
watch(isEditing, (editing) => {
  if (editing && docStore.currentContent !== undefined) {
    originalContentSnapshot.value = docStore.currentContent;
  }
});

async function handleInlineSave(content: string, skipChangeLogCheck = false) {
  if (!docStore.currentDocument || isSaving.value) return;
  const docId = docStore.currentDocument.id;

  // 与独立编辑器保持一致：要求填写变更说明，避免版本历史被无意义记录污染
  if (!skipChangeLogCheck && content !== originalContentSnapshot.value && !inlineChangeLog.value.trim()) {
    pendingInlineContent.value = content;
    showChangeLogModal.value = true;
    return;
  }

  isSaving.value = true;
  try {
    // 走 store.update：保存成功后回写 currentDocument 元数据（修改者/更新时间），
    // 确保文档查看页顶部“操作记录”立即刷新（直接调 API 不会更新 currentDocument）
    await docStore.update(docId, {
      content,
      change_log: inlineChangeLog.value.trim() || t('fallback.k50'),
      last_update_time: docStore.currentDocument.update_time || 0,
    });
    docStore.currentContent = content;
    docStore.invalidateDocCache(docId);
    // 保存响应不含 editor_name（修改者姓名由后端按 edit_user_id 反查得到），
    // 必须重新拉取才能让顶部“修改者/更新于”刷新，而非停留在旧值
    await docStore.loadDocument(docId);
    isEditing.value = false;
    originalContentSnapshot.value = '';
    inlineChangeLog.value = '';
    pendingInlineContent.value = '';
    ui.showToast(t('ws.k177'), 'success');
  } catch (e: unknown) {
    // P1-2: 状态码经 ApiError.status 判断；P1-18: 草稿改用 useDraft 的统一键写入，
    // 否则编辑页（useDraft.loadDraft）读不到此处保存的冲突草稿，编辑内容静默丢失
    if (getApiErrorStatus(e) === 409) {
      try { useDraft().saveDraft(docId, content); } catch { /* ignore */ }
      ui.showToast(t('ws.k250'), 'error');
      await docStore.loadDocument(docId);
      isEditing.value = false;
      originalContentSnapshot.value = '';
      inlineChangeLog.value = '';
    } else {
      ui.showToast(t('ws.k251'), 'error');
    }
  } finally {
    isSaving.value = false;
  }
}

function confirmInlineSave() {
  if (!pendingInlineContent.value) return;
  const content = pendingInlineContent.value;
  showChangeLogModal.value = false;
  handleInlineSave(content, true);
}

function handleInlineCancel() {
  isEditing.value = false;
  originalContentSnapshot.value = '';
  if (docStore.currentDocument?.id) {
    docStore.loadDocument(docStore.currentDocument.id);
  }
}

// E 键切换编辑模式
function handleKeydown(e: KeyboardEvent) {
  if ((e.key === 'e' || e.key === 'E') && !e.ctrlKey && !e.metaKey && !e.altKey) {
    const target = e.target as HTMLElement;
    const tag = target?.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || target?.isContentEditable) return;
    if (document.activeElement?.closest('.monaco-editor') || document.activeElement?.closest('[role="textbox"]')) return;
    if (docStore.currentDocument?.type === 1) {
      e.preventDefault();
      isEditing.value = !isEditing.value;
    }
  }
  if (e.key === 'Escape' && isEditing.value) {
    e.preventDefault();
    handleInlineCancel();
  }
}

// 当检测到来自首页时，强制重新加载目录树并只展开第一层
watch([spaceId, isFromHome], async ([newSpaceId, fromHome]) => {
  if (newSpaceId && fromHome) {
    await docStore.loadRootDocuments(newSpaceId, false, true);
  }
}, { immediate: false });

const {
  treeSearchText, treeFocusedId, filteredTree, searchMatchId,
  expandAllNodes, collapseAllNodes, handleToggleNode,
} = useSpaceTree();

const {
  isCollected, shareUrl, checkCollectStatus,
  handleCollect: doCollect, handleExport: doExport, handleExportHTML: doExportHTML,
  handleExportWord: doExportWord, handleExportPDF: doExportPDF, handleExportMarkdown: doExportMarkdown,
  buildShareUrl, copyShareUrl,
} = useDocActions();

const {
  showCreateModal, createDocName, createDocSpaceId, createDocType, creatingDoc,
  showRenameModal, renameDocName, renamingDoc,
  showDeleteModal, deletingDocId, deleteMessage, deletingDocName,
  showMoveModal, movingDocId,
  showCopyModal, copyingDocId,
  openCreateModal, handleCreateDoc, handleRename, handleRenameDoc, handleDelete, confirmDelete,
  handleMove, handleCopy, confirmMove, confirmCopy,
} = useDocCRUD();

const showShareModal = shallowRef(false);
const showHistoryModal = shallowRef(false);
const showImagesModal = shallowRef(false);
const showAttachmentsModal = shallowRef(false);
const showTrashModal = shallowRef(false);
const showImportModal = shallowRef(false);

function openShareModal() { showShareModal.value = true; }
function openHistoryModal() { showHistoryModal.value = true; }
function openImagesModal() { showImagesModal.value = true; }
function openAttachmentsModal() { showAttachmentsModal.value = true; }

function handleToast(message: string, type?: 'success' | 'error' | 'info') {
  ui.showToast(message, type);
}

const { showShortcutHelp, shortcutList, onCollect, onExport } = useSpaceShortcuts(spaceId, openShareModal, openHistoryModal);

const { isMobile } = useBreakpoint();
const showMobileMenu = shallowRef(false);
const mobileSidebarRef = ref<{ $el?: HTMLElement } | null>(null);

/**
 * 移动端侧栏抽屉：位移、越界橡皮筋、松手按速度投影，全部走全站那一份弹簧
 * （见 composables/ui/useSheetGesture.ts）。以前它是 `translateX(-100%) + transition`
 * 的 keyframe 重放——拖到一半松手会等动画跑完才响应，也没有"甩一下就关"的手感。
 */
const drawerTravel = () => (mobileSidebarRef.value?.$el?.offsetWidth
  || Number.parseInt(AppleLayout.spaceSidebarWidth, 10));
const drawerGesture = useSheetGesture({
  axis: 'x',
  direction: -1,          // 左抽屉：朝屏幕左划才是划走
  travel: drawerTravel,
  onDismiss: () => { showMobileMenu.value = false; },
  spec: AppleSpring.sheet,
});
const { offset: drawerOffset, transformStyle: drawerTransformStyle, springAttrs: drawerSpringAttrs, onPointerdown: onDrawerPointerdown } = drawerGesture;

watch(showMobileMenu, (open) => {
  if (open) nextTick(() => { drawerGesture.enter(); });
  else drawerGesture.leave();
});

// 抽屉常驻 DOM（不像 ApDrawer 那样 v-if），所以初始必须停在"关"那一头，
// 否则弹簧从 0 起步 = 一进空间就压着一层侧栏
onMounted(() => {
  if (!showMobileMenu.value) drawerGesture.park(drawerTravel());
});

const { width: sidebarWidth, onDragStart: onSidebarDrag } = useResizable({
  initialWidth: Number.parseInt(AppleLayout.spaceSidebarWidth), minWidth: 280, maxWidth: 400, storageKey: 'citebase-sidebar-width',
});

const selectedTemplateKey = shallowRef('');
const showTemplatePreview = shallowRef(false);

onCollect.value = () => handleCollect();
onExport.value = () => handleExport();

const isFullscreen = shallowRef(false);

function toggleFullscreen() {
  isFullscreen.value = !isFullscreen.value;
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isFullscreen.value) {
    isFullscreen.value = false;
  }
}

onMounted(() => document.addEventListener('keydown', onKeydown));
onUnmounted(() => document.removeEventListener('keydown', onKeydown));

async function handlePrint() {
  const content = docStore.currentContent || '';
  const title = docStore.currentDocument?.name || t('common.document');
  const printWindow = window.open('', '_blank', 'noopener,noreferrer');
  if (!printWindow) return;
  const safeTitle = title.replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const theme = document.documentElement.getAttribute('data-theme') || '';
  const material = document.documentElement.getAttribute('data-material') || 'solid';
  // 全部主题块都要带上：只嵌 light/dark 会让扩展主题导出成浅色。
  // 材质轴（glass）单独成块，同样按 supportsGlass 收集，导出才能保住玻璃观感。
  const themeStyles = THEME_REGISTRY
    .map((meta) => document.getElementById(`apple-${meta.id}-theme`)?.textContent ?? '')
    .concat(
      THEME_REGISTRY
        .filter((meta) => supportsGlass(meta.id))
        .map((meta) => document.getElementById(`apple-${meta.id}-glass-theme`)?.textContent ?? ''),
    )
    .join('\n');

  let html = '';
  try {
    const mod = await import('marked');
    const marked = (mod as { marked?: (s: string) => string }).marked || (mod as { default?: (s: string) => string }).default;
    if (typeof marked === 'function') html = marked(content);
  } catch {
    html = content.replace(/[<>&]/g, (c) => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[c] || c)).replace(/\n/g, '<br>');
  }
  // 白名单统一走 markdown-sanitize.ts（此前打印导出自持一份更窄的白名单）
  const safeHtml = sanitizeMarkdownHtml(html);

  printWindow.document.write(`
    <!DOCTYPE html>
    <html data-theme="${theme}" data-material="${material}">
    <head>
      <meta charset="utf-8">
      <title>${safeTitle}</title>
      <style>
        ${themeStyles}
        ${printStyle}
      </style>
    </head>
    <body>
      <h1>${safeTitle}</h1>
      <div id="content">${safeHtml}</div>
      <script>
        window.onload = function() { window.print(); }
      ${'</scr' + 'ipt>'}
    </body>
    </html>
  `);
  printWindow.document.close();
}

mountInit((parentId, sId, type) => openCreateModalWrapper(parentId, sId, type as 1 | 2));

onMounted(() => {
  window.addEventListener('keydown', handleKeydown);
});

watch(docId, (newDocId, oldDocId) => {
  if (newDocId && newDocId !== oldDocId) {
    isEditing.value = false;
    contentPanelRef.value?.resetScroll();
    loadDocument(Number(newDocId));
    // 消费 source 参数：从全局搜索/命令面板跳转时设置 searchMatchId 触发 TreeNode 滚动定位
    const source = route.query.source as string | undefined;
    if (source === 'spotlight' || source === 'search') {
      nextTick(() => {
        setTimeout(() => docStore.setSearchMatchId(Number(newDocId)), 300);
      });
    }
  } else if (!newDocId && oldDocId) {
    selectedDocumentId.value = null;
    docStore.currentDocument = null;
    docStore.currentContent = '';
    breadcrumbs.value = [];
    docStore.setSearchMatchId(null);
  }
});

// 消费 highlight 参数：从搜索页跳转时高亮文档内容中的搜索关键词
const highlightKeyword = computed(() => (route.query.highlight as string) || '');

const spaceCollected = shallowRef(false);

watch(spaceId, async (newSpaceId) => {
  if (!newSpaceId || !auth.isLoggedIn) {
    spaceCollected.value = false;
    return;
  }
  try {
    const res = await checkCollected(2, newSpaceId);
    spaceCollected.value = res.collected;
  } catch (e) {
    logger.error('Failed to check space collect status:', e);
    spaceCollected.value = false;
  }
}, { immediate: true });

watch(() => auth.isLoggedIn, (loggedIn) => {
  if (loggedIn && spaceId.value) checkSpaceCollectStatus();
});

let monacoPreloaded = false;

watch(() => docStore.currentDocument, (doc) => {
  if (doc && doc.type === 1 && auth.isLoggedIn) {
    checkCollectStatus(1, doc.id);
    const { recordVisit } = useRecentDocs();
    recordVisit({
      id: doc.id, name: doc.name, space_id: doc.space_id,
      space_name: spaceStore.currentSpace?.name || '', update_time: doc.update_time,
    });
    if (!monacoPreloaded && canEditInSpace.value) {
      monacoPreloaded = true;
      const preload = () => { import('monaco-editor'); };
      if (typeof requestIdleCallback !== 'undefined') {
        requestIdleCallback(preload, { timeout: 3000 });
      } else {
        setTimeout(preload, 2000);
      }
    }
  }
});

// 清理 URL 中的 from 参数
watch([spaceId, () => route.query.from], ([newSpaceId, fromParam]) => {
  if (fromParam === 'home' && newSpaceId) {
    router.replace({ path: route.path, query: {} });
  }
}, { immediate: true });

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown);
  initCleanup();
});

const { handleDragReorder: dragReorder } = useTreeDragReorder();

async function handleDragReorder(sourceId: number, targetId: number, position: 'before' | 'after' | 'inside') {
  if (!canEditInSpace.value) return;
  const ok = await dragReorder(docStore.tree, sourceId, targetId, position, (items) => docStore.sort(items), (id, data) => docStore.move(id, data));
  if (!ok && spaceId.value) {
    ui.showToast(t('ws.k252'), 'error');
    await docStore.loadRootDocuments(spaceId.value, true).catch(() => {});
  }
}

function handleRenameDocWrapper() {
  handleRenameDoc((id, newName) => {
    const crumb = breadcrumbs.value.find(c => c.id === id);
    if (crumb) crumb.name = newName;
    if (docStore.currentDocument?.id === id) {
      docStore.currentDocument = { ...docStore.currentDocument, name: newName };
    }
  });
}

function handleCollect() {
  if (!docStore.currentDocument) return;
  doCollect(1, docStore.currentDocument.id);
}

function handleExport() {
  if (!docStore.currentDocument) return;
  doExport(docStore.currentDocument.id, docStore.currentDocument.name);
}

function handleExportHTML() {
  if (!docStore.currentDocument) return;
  doExportHTML(docStore.currentDocument.id, docStore.currentDocument.name);
}

function handleExportWord() {
  if (!docStore.currentDocument) return;
  doExportWord(docStore.currentDocument.id, docStore.currentDocument.name);
}

function handleExportPDF() {
  if (!docStore.currentDocument) return;
  doExportPDF(docStore.currentDocument.id, docStore.currentDocument.name);
}

function handleExportMarkdown() {
  if (!docStore.currentDocument) return;
  doExportMarkdown(docStore.currentDocument.id, docStore.currentDocument.name);
}

const showSpaceExportConfirm = shallowRef(false);
const pendingExportSpaceName = shallowRef('');

function onSpaceExport() {
  if (!spaceId.value) return;
  pendingExportSpaceName.value = spaceStore.currentSpace?.name || '';
  showSpaceExportConfirm.value = true;
}

async function confirmSpaceExport() {
  if (!spaceId.value) return;
  try {
    const blob = await downloadSpace(spaceId.value);
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${pendingExportSpaceName.value || 'space'}.zip`;
    a.click();
    URL.revokeObjectURL(url);
    ui.showToast(t('ws.k253'), 'success');
  } catch (e) {
    logger.error('Space export failed:', e);
    ui.showToast(t('ws.k254'), 'error');
  }
}

function handleShare() {
  if (!docStore.currentDocument) return;
  buildShareUrl(spaceId.value, docStore.currentDocument.id);
  openShareModal();
}

async function handleVersionRollback(version: number) {
  if (!docStore.currentDocument) return;
  try {
    await docStore.rollback(docStore.currentDocument.id, version);
    ui.showToast(t('ws.k180'), 'success');
    showHistoryModal.value = false;
    await docStore.loadDocument(docStore.currentDocument.id);
  } catch {
    ui.showToast(t('ws.k181'), 'error');
  }
}

function handleHistory() {
  if (!docStore.currentDocument) return;
  openHistoryModal();
}

const aiSummaryLoading = shallowRef(false);
const aiSummaryText = shallowRef('');
const showAiSummaryModal = shallowRef(false);

async function handleAISummary() {
  if (!docStore.currentDocument) return;
  if (aiSummaryLoading.value) return;
  aiSummaryLoading.value = true;
  aiSummaryText.value = '';
  showAiSummaryModal.value = true;
  try {
    const res = await generateSummary(docStore.currentDocument.id);
    aiSummaryText.value = res.summary;
  } catch (e) {
    logger.error('生成摘要失败', e);
    aiSummaryText.value = '';
    ui.showToast(e instanceof Error ? e.message: t('ws.k249'), 'error');
    showAiSummaryModal.value = false;
  } finally {
    aiSummaryLoading.value = false;
  }
}

const showMembersModal = shallowRef(false);

async function checkSpaceCollectStatus() {
  if (!spaceId.value || !auth.isLoggedIn) return;
  try {
    const res = await checkCollected(2, spaceId.value);
    spaceCollected.value = res.collected;
  } catch (e) {
    logger.error('Failed to check space collect status:', e);
    spaceCollected.value = false;
  }
}

async function handleSpaceCollect() {
  if (!spaceId.value) return;
  if (!auth.isLoggedIn) {
    // 未登录：引导而不是让请求打出去弹 401
    ui.showToast(t('ws.k255'), 'info');
    return;
  }
  try {
    // 统一走 collection store：乐观更新 + 失败自动回滚，收藏列表同步移除
    const collected = await collection.toggle(2, spaceId.value);
    if (collected === null) return;
    spaceCollected.value = collected;
    ui.showToast(collected ? '已加入收藏' : '已取消收藏', 'success');
  } catch (e) {
    logger.error('Failed to toggle collection:', e);
    ui.showToast(t('ws.k244'), 'error');
  }
}

function openMembersModal() {
  showMembersModal.value = true;
}

function handleOpenImages() {
  if (!docStore.currentDocument) return;
  openImagesModal();
}

function handleOpenAttachments() {
  if (!docStore.currentDocument) return;
  openAttachmentsModal();
}

function handleDeleteDoc() {
  if (!docStore.currentDocument) return;
  handleDelete({ id: docStore.currentDocument.id, name: docStore.currentDocument.name, type: docStore.currentDocument.type });
}

async function confirmDeleteDoc() {
  const deletedId = deletingDocId.value;
  isEditing.value = false;
  originalContentSnapshot.value = '';
  await confirmDelete(() => {
    if (docStore.currentDocument?.id === deletedId) {
      docStore.currentContent = '';
      const next = findSiblingOrParent(deletedId);
      if (next) {
        navigateToDoc(next.id);
      } else {
        navigateToSpaceRoot();
      }
    }
  });
}

function openCreateModalWrapper(parentId: number, sId: number, type: 1 | 2 = 1) {
  selectedTemplateKey.value = '';
  openCreateModal(parentId, sId, type, spaceId.value);
}

async function handleCreateDocWrapper() {
  const template = docTemplates.find(t => t.key === selectedTemplateKey.value);
  await handleCreateDoc((doc) => {
    router.push(`/space/${createDocSpaceId.value}/doc/${doc.id}/edit`);
  }, template?.content);
}

async function confirmMoveWrapper(targetId: number) {
  await confirmMove(targetId, spaceId.value);
}

async function confirmCopyWrapper(targetId: number) {
  await confirmCopy(targetId, spaceId.value);
}

async function handleTrashRestored(documentId: number) {
  await docStore.loadTree(spaceId.value);
  if (documentId) {
    await docStore.expandToDocument(documentId).catch((e) => {
      logger.error('Failed to expand to document:', e);
      ui.showToast(t('ws.k256'), 'error');
    });
    navigateToDoc(documentId);
  }
}

function handleTrashPermanentDeleted(documentId: number) {
  if (docStore.currentDocument?.id === documentId) {
    const next = findSiblingOrParent(documentId);
    if (next) {
      navigateToDoc(next.id);
    } else {
      navigateToSpaceRoot();
    }
  }
}

// ============ 样式 ============

const dragHandleHover = ref(false);

const dragHandleStyle = computed(() => ({
  width: '6px',
  cursor: 'col-resize',
  // 触屏上不加这条，浏览器会把横向拖动判成页面滚动，pointerdown 收到也拖不动
  touchAction: 'none' as const,
  background: dragHandleHover.value ? cv('blue') : 'transparent',
  borderLeft: `1px solid ${dragHandleHover.value ? cv('border') : cv('border-light')}`,
  transition: AppleTransition.default,
  flexShrink: 0,
  alignSelf: 'stretch' as const,
}));

// 移动端抽屉必须高于全局导航栏（ApNavBar 用 overlay 档），否则遮罩盖不住导航、抽屉被压在下面
const mobileOverlayStyle = computed(() => getOverlayStyle(AppleZIndex.drawer, {
  // 遮罩与抽屉共用同一个呈现值：拖到一半就是半遮罩，不存在"面板跟手、背景等动画"
  opacity: String(Math.max(0, 1 - drawerOffset.value / (drawerTravel() || 1))),
}));

const mobileDrawerStyle = {
  position: 'fixed' as const, top: '0', left: '0', bottom: '0', width: AppleLayout.spaceSidebarWidth,
  // 遮罩在模板第 11 行、面板在第 33 行，面板本来就排在后面 —— 同档即可靠 DOM 序压过自己的遮罩，
  // 与 ApDrawer 一致。原先写 `drawer + 1` 是在已注册的档外面又造了一档（而且刚好撞上 modal）。
  background: cv('surface-float'), zIndex: String(AppleZIndex.drawer), display: 'flex',
  flexDirection: 'column' as const, boxShadow: cv('shadow-modal') };

// 失败横幅：贴在内容区顶部，横跨阅读区；重试是一条文字按钮，避免与工具栏按钮抢位置
const initErrorBannerStyle = { margin: AppleSpace.sm, flexShrink: '0' as const };
const retryLinkStyle = {
  marginLeft: 'auto',
  padding: 0,
  border: 'none',
  background: 'transparent',
  color: cv('blue'),
  cursor: 'pointer',
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: 600,
};

const contentStyle = computed(() => ({
  flex: '1 1 0%', overflowY: 'auto' as const, overflowX: 'hidden' as const, minHeight: '0',
  display: 'flex', flexDirection: 'column' as const,
  ...(isFullscreen.value ? {
  position: 'fixed' as const,
  top: '0', left: '0', right: '0', bottom: '0',
  zIndex: String(AppleZIndex.sticky),
  // 全屏时脱离 ApPageContainer 的绘制框，父级 canvas 盖不到，才自己补一层玻璃；
  // 走 AppleGlass.standard 拿亮边 + 上缘高光斜面 + 浮起阴影，只把底色换回更实的
  // surface-float 挡住身后内容（standard 默认 surface-panel 偏透，全屏要更实一档）
  ...AppleGlass.standard,
  background: cv('surface-float'),
  } : {}),
}));

const mainRowStyle = computed(() => ({
  display: 'flex', flexDirection: 'row' as const, flex: '1',
  overflow: 'hidden' as const, minHeight: '0' as const }));

const printStyle = `
  body { font-family: ${AppleFont.text}; max-width: 800px; margin: 0 auto; padding: ${AppleSpace.xl} ${AppleSpace.md}; color: ${cv('text-primary')}; line-height: 1.6; }
  h1 { font-size: ${AppleType.utilityHeading.size}; margin-bottom: 16px; }
  h2 { font-size: ${AppleType.linkActionHeading.size}; margin-top: 24px; }
  h3 { font-size: ${AppleType.bodyPrimary.size}; margin-top: 20px; }
  p { margin: ${AppleSpace.sm} 0; }
  code { background: ${cv('bg-secondary')}; padding: 2px 6px; border-radius: ${AppleRadius.small}; font-size: ${AppleType.controlLabel.size}; }
  pre { background: ${cv('bg-secondary')}; padding: 16px; border-radius: ${AppleRadius.medium}; overflow-x: auto; }
  pre code { background: none; padding: 0; }
  blockquote { border-left: ${AppleSpace.xs} solid ${cv('border-light')}; margin: 16px 0; padding: 8px 16px; color: ${cv('text-secondary')}; }
  table { border-collapse: collapse; width: 100%; margin: 16px 0; }
  th, td { border: 1px solid ${cv('border-light')}; padding: 8px 12px; text-align: left; }
  th { background: ${cv('bg-secondary')}; }
  img { max-width: 100%; }
  a { color: ${cv('blue')}; }
  @media print { body { padding: 0; } }
`;
</script>
