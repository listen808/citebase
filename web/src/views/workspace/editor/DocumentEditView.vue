<template>
  <ApPageContainer no-scroll max-width="none">
    <div :style="topBarStyle">
      <!-- 编辑页的文档名只出现在面包屑里，整页没有一个 h1；这一枚只给辅助技术，
           视觉上仍由面包屑承担标题职责，避免顶栏出现两个同名标题 -->
      <h1 :style="srOnlyTitleStyle">{{ docStore.currentDocument?.name || $t('fallback.k96') }}</h1>
      <DocBreadcrumb
        :space-name="spaceStore.currentSpace?.name || ''"
        :crumbs="breadcrumbs"
        :has-back-button="false"
        :has-doc-selected="!!docStore.currentDocument"
        edit-mode
        @navigate-to-space-root="navigateToSpaceRoot"
        @navigate-to-doc="navigateToDoc"
      />
      <ApStatusBadge
        :status="isPublished ? 'online' : 'default'"
        size="small"
        :label="isPublished ? '已发布' : '草稿'"
      />
      <ApButton
        size="small"
        :variant="isPublished ? 'secondary' : 'primary'"
        :loading="publishing"
        :disabled="!canEditInSpace"
        :tooltip="isPublished ? '下线：转为草稿' : '发布：标记为已发布'"
        :aria-label="isPublished ? '下线：转为草稿' : '发布：标记为已发布'" @click="togglePublish">
        <ApIcon :name="isPublished ? 'Undo2' : 'Send'" :size="16" />
        <span>{{ isPublished ? $t('vWorkspaceUi.k15') : $t('vWorkspaceUi.k16') }}</span>
      </ApButton>
      <ApButton size="small" :variant="editMode.isCollab.value ? 'primary' : 'ghost'" @click="editMode.toggle()">
        <ApIcon name="Users" :size="16" />
        <span>{{ editMode.isCollab.value ? $t('vWorkspaceUi.k17') : $t('vWorkspaceUi.k18') }}</span>
      </ApButton>
      <ApPopover placement="bottom" trigger="click">
        <ApButton size="small" variant="ghost" :tooltip="$t('ws.k168')" :aria-label="$t('ws.k168')" @click="loadCollabSessions">
          <ApIcon name="UsersRound" :size="16" />
          <span>会话 {{ collabSessions.length }}</span>
        </ApButton>
        <template #content>
          <div :style="{ display: 'flex', flexDirection: 'column', gap: AppleSpace.xs }">
            <span :style="{ fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-tertiary') }">
              {{ sessionsLoading ? $t('vWorkspaceUi.k19') : (collabSessions.length ? $t('vWorkspaceUi.k20') : $t('vWorkspaceUi.k21')) }}
            </span>
            <div
              v-for="session in collabSessions"
              :key="session.user_id"
              :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm }"
            >
              <span :style="{ width: AppleSpace.xs, height: AppleSpace.xs, borderRadius: AppleRadius.circle, background: session.color, flexShrink: '0' }" />
              <span :style="{ fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-primary') }">
                {{ session.user_name || `用户 ${session.user_id}` }}
              </span>
            </div>
          </div>
        </template>
      </ApPopover>
      <ApButton size="small" variant="ghost" @click="showCommentsPanel = !showCommentsPanel">
        <ApIcon name="MessageSquare" :size="16" />
        <span>{{ $t('ws.k158') }}</span>
      </ApButton>
    </div>

    <div :style="editorAreaStyle">
      <div :style="editorMainStyle">
        <DocumentEditor
          v-model="content"
          :document-id="docId"
          :space-id="spaceId"
          :save-status="saveStatus"
          :has-unsaved="hasUnsavedChanges"
          @save="handleSave"
          @rollback="handleRollback"
          @cancel="handleCancel"
        />
      </div>
      <div v-if="showCommentsPanel" class="apple-glass-layer" :style="sidePanelStyle">
        <DocProperties
          :document="docStore.currentDocument"
          :content="content"
          :word-count="docWordCount"
          :reading-time="readingTime"
          @update:icon="handleIconUpdate"
          @update:cover-image="handleCoverUpdate"
        />
        <DocCommentPanel
          :document-id="docId"
          :can-edit="canEditInSpace"
          closable
          @close="showCommentsPanel = false"
        />
      </div>
    </div>

    <EditorModals
      v-model:show-recover-modal="showRecoverModal"
      v-model:show-unsaved-modal="showUnsavedModal"
      v-model:show-change-log-modal="showChangeLogModal"
      v-model:change-log-input="changeLogInput"
      v-model:skip-change-log="skipChangeLog"
      @recover="recoverDraft"
      @discard="discardDraft"
      @continue-editing="handleContinueEditing"
      @confirm-cancel="confirmCancel"
      @confirm-save="confirmSaveWithLog"
      @cancel-save="cancelSaveWithLog"
    />

    <ApModal v-model="showConflictModal" :title="$t('ws.k166')" size="medium">
      <div :style="conflictContainerStyle">
        <p :style="conflictDescStyle">
          {{ $t('ws.k159') }}
        </p>
        <div :style="conflictOptionsStyle">
          <div :style="conflictOptionBoxStyle">
            <h4 :style="conflictTitleStyle">{{ $t('ws.k160') }}</h4>
            <p :style="conflictInfoStyle">{{ conflictLocalContent.length }} {{ $t('vWorkspaceUi.k22') }}</p>
          </div>
          <div :style="conflictOptionBoxStyle">
            <h4 :style="conflictTitleStyle">{{ $t('ws.k161') }}</h4>
            <p :style="conflictInfoStyle">{{ conflictServerContent.length }} {{ $t('vWorkspaceUi.k22') }}</p>
          </div>
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showConflictModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton variant="secondary" size="small" @click="handleUseServerVersion">{{ $t('ws.k162') }}</ApButton>
        <ApButton variant="primary" size="small" @click="handleKeepLocalVersion">{{ $t('ws.k163') }}</ApButton>
      </template>
    </ApModal>

    <ApModal
      v-model="showLockFailedModal"
      :title="$t('ws.k167')"
      size="small"
      :close-on-overlay="false"
    >
      <p :style="conflictDescStyle">{{ $t('ws.k164') }}</p>
      <template #footer>
        <ApButton variant="primary" size="small" @click="handleLockFailedConfirm">{{ $t('ws.confirm') }}</ApButton>
      </template>
    </ApModal>

    <SaveStatusBar
      :word-count="wordCount"
      :save-status="saveStatusBarStatus"
      :word-count-status="wordCountStatus"
      :collab-visible="editMode.isCollab.value"
      :collab-connected="collabConnected"
      :collab-synced="collabSynced"
      :collab-users="collabUsers"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted, watch, onBeforeUnmount } from 'vue';
import { useBreakpoint } from '@/utils/responsive';

const { isMobile } = useBreakpoint();
import { logger } from '@/utils/logger';
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router';
import { useDocumentStore } from '@/stores/workspace/document';
import { siteDocumentTitle } from '@/utils/core/site-brand';
import { useSpaceStore } from '@/stores/workspace/space';

import { useUIStore } from '@/stores/app/ui';
import { useAuthStore } from '@/stores/auth/auth';
import {
  updateDocument,
  getDocumentContent,
  getDocumentByID,
  getDocumentPath,
  publishDocument,
  unpublishDocument,
  getCollabSessions,
  type CollabSession,
} from '@/api/modules/document/document';
// P1-2: 409 冲突判断改用 client 暴露的状态码提取（错误已不再是 AxiosError）
import { getApiErrorStatus } from '@/api/client';

import { useDraft } from '@/composables/space/useDraft';

import { cv, AppleFont, AppleSpace, AppleType, AppleRadius, AppleFontWeight, AppleZIndex, appleFixedColumns } from '@/utils/core/apple-tokens';
import { storageGet, storageSet, StorageKeys } from '@/utils/core/storage';
import { DOCUMENT_SIZE_LIMIT } from '@/utils/core/constants';
import { countWords, estimateReadingTime } from '@/utils/format/reading-time';
import DocumentEditor from '@/components/document/DocumentEditor.vue';
import SaveStatusBar from '@/components/document/SaveStatusBar.vue';
import DocProperties from '@/components/space/sidebar/DocProperties.vue';
import DocCommentPanel from '@/components/space/sidebar/DocCommentPanel.vue';
import DocBreadcrumb from '@/components/space/reader/DocBreadcrumb.vue';
import ApModal from '@/components/apple/ApModal.vue'
import ApButton from '@/components/apple/ApButton.vue'
import ApPageContainer from '@/components/apple/ApPageContainer.vue'
import ApPopover from '@/components/apple/ApPopover.vue'
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue'
import EditorModals from './EditorModals.vue';
import { useEditMode } from '@/composables/editor/useEditMode';

const route = useRoute();
const router = useRouter();
const docStore = useDocumentStore();
const spaceStore = useSpaceStore();
const ui = useUIStore();
const auth = useAuthStore();
const canEditInSpace = computed(() => auth.isEditor(spaceStore.currentSpace?.privilege));
const { saveDraft, loadDraft, saveServerTimestamp, clearDraft, hasDraft, cleanExpiredDrafts } = useDraft();

const spaceId = computed(() => Number(route.params.spaceId));
const docId = computed(() => Number(route.params.docId));
const breadcrumbs = ref<{ id: number; name: string }[]>([]);

const content = shallowRef('');
const lastSaveTime = shallowRef<Date | null>(null);
const showRecoverModal = shallowRef(false);
const showUnsavedModal = shallowRef(false);
const showChangeLogModal = shallowRef(false);
const showConflictModal = shallowRef(false);
const showLockFailedModal = shallowRef(false);
const showCommentsPanel = shallowRef(false);
const changeLogInput = shallowRef('');
const skipChangeLog = shallowRef(false);
const serverContent = shallowRef('');
const lastUpdateTime = shallowRef(0);
const conflictLocalContent = shallowRef('');
const conflictServerContent = shallowRef('');
let saveTimer: ReturnType<typeof setTimeout> | null = null;
let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
const isNavigatingAway = shallowRef(false);

const saveStatus = shallowRef('');
const hasUnsavedChanges = computed(() => content.value !== serverContent.value);

// #7 草稿/发布分离 + 服务端协作会话
const docStatus = shallowRef('');
const docPublishedAt = shallowRef(0);
const publishing = shallowRef(false);
const sessionsLoading = shallowRef(false);
const collabSessions = shallowRef<CollabSession[]>([]);
const isPublished = computed(() =>
  docStatus.value ? docStatus.value === 'published' : docPublishedAt.value > 0,
);
const isSaving = shallowRef(false);

// 编辑模式状态机：独占锁 / Yjs 协作互斥切换
const editMode = useEditMode(docId.value);
const {
  isConnected: collabConnected,
  isSynced: collabSynced,
  connectedUsers: collabUsers,
  getYText,
} = editMode.collab;

// Sync Yjs changes into Monaco when collaboration is active
let isApplyingRemote = false;
let lastSyncedContent = '';
watch(editMode.isCollab, (enabled) => {
  if (enabled) {
    // Listen for Yjs text changes
    const ytext = getYText();
    ytext.observe(() => {
      if (isApplyingRemote) return;
      isApplyingRemote = true;
      const remoteContent = ytext.toString();
      if (remoteContent !== content.value) {
        content.value = remoteContent;
        serverContent.value = remoteContent;
      }
      lastSyncedContent = remoteContent;
      isApplyingRemote = false;
    });
  }
});

// Push local Monaco changes to Yjs when collaboration is active
watch(content, (newContent) => {
  if (!editMode.isCollab.value || isApplyingRemote) return;
  if (newContent === lastSyncedContent) return;
  const ytext = getYText();
  ytext.delete(0, ytext.length);
  ytext.insert(0, newContent);
  lastSyncedContent = newContent;
});

// C2/C3: 协作模式下定时把 Yjs 内容写回后端 document.content（默认 30s）
let collabWritebackTimer: ReturnType<typeof setInterval> | null = null;
watch(editMode.isCollab, (enabled) => {
  if (enabled) {
    collabWritebackTimer = setInterval(async () => {
      if (!docId.value || isApplyingRemote) return;
      try {
        const res = await updateDocument(docId.value, { content: content.value, last_update_time: lastUpdateTime.value });
        // 以服务端返回的 update_time 为准，避免客户端时间偏差导致下次保存误判冲突
        lastUpdateTime.value = res.update_time ?? Math.floor(Date.now() / 1000);
      } catch {
        // 写回失败不阻断协作，下次定时重试
      }
    }, 30000);
  } else if (collabWritebackTimer) {
    clearInterval(collabWritebackTimer);
    collabWritebackTimer = null;
  }
});

const wordCount = computed(() => content.value?.length || 0);
const docWordCount = computed(() => countWords(content.value));
const readingTime = computed(() => estimateReadingTime(content.value));
const wordCountStatus = computed<{ level: 'normal' | 'warning' | 'error'; message: string }>(() => {
  const count = wordCount.value;
  const percent = (count / DOCUMENT_SIZE_LIMIT) * 100;
  if (percent >= 100) return { level: 'error', message: t('ws.k169') };
  if (percent >= 90) return { level: 'warning', message: t('ws.k170') };
  return { level: 'normal', message: '' };
});

const saveStatusBarStatus = computed(() => {
  if (isSaving.value) return 'saving';
  if (saveStatus.value === '已保存') return 'saved';
  if (saveStatus.value === '保存失败') return 'error';
  if (hasUnsavedChanges.value) return 'unsaved';
  return '';
});

onMounted(() => {
  cleanExpiredDrafts();
  loadDocument();
  enterEditMode();
  window.addEventListener('beforeunload', handleBeforeUnload);
  window.addEventListener('keydown', handleKeyDown);
});

async function enterEditMode() {
  if (!docId.value) return;
  const ok = await editMode.enter();
  if (!ok) {
    showLockFailedModal.value = true;
    return;
  }
  // Heartbeat every 30s to keep lock alive (only in exclusive mode)
  heartbeatTimer = setInterval(() => {
    if (docId.value && editMode.isExclusive.value) docStore.heartbeat(docId.value);
  }, 30000);
}

function handleLockFailedConfirm() {
  showLockFailedModal.value = false;
}

watch(showLockFailedModal, (shown) => {
  if (!shown) {
    isNavigatingAway.value = true;
    router.push(`/space/${spaceId.value}/doc/${docId.value}`);
  }
});

function releaseEditMode() {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
  if (collabWritebackTimer) {
    clearInterval(collabWritebackTimer);
    collabWritebackTimer = null;
  }
  editMode.dispose();
}

watch(docId, () => {
  collabSessions.value = [];
  loadDocument();
});

watch([hasUnsavedChanges, () => docStore.currentDocument?.name], ([unsaved, docName]) => {
  // 段落名退回路由标题那一键（route.document_edit），站点名与拼法由 site-brand 统一给 ——
  // 这一条以前自己写死 "编辑文档" 与 " - Citebase"，与 router/guards.ts 各抄一份。
  const baseTitle = docName || t('route.document_edit');
  document.title = siteDocumentTitle(unsaved ? `* ${baseTitle}` : baseTitle);
});

let lastDraftSave = 0;
watch(content, (newContent) => {
  if (!docId.value || newContent === serverContent.value) return;
  const now = Date.now();
  if (now - lastDraftSave >= 5000) {
    saveDraft(docId.value, newContent);
    lastDraftSave = now;
  } else {
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      saveDraft(docId.value!, content.value);
      lastDraftSave = Date.now();
    }, 5000 - (now - lastDraftSave));
  }
});

function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (docId.value && content.value !== serverContent.value) {
    saveDraft(docId.value, content.value);
    ui.showToast(t('ws.k171'), 'info');
    event.preventDefault();
    event.returnValue = '您有未保存的更改，内容已自动保存到草稿。确定要离开吗？';
  }
}

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload);
  window.removeEventListener('keydown', handleKeyDown);
  if (saveTimer) clearTimeout(saveTimer);
  releaseEditMode();
});

function handleKeyDown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault();
    e.stopPropagation();
    handleSave();
  }
}

async function loadDocumentData(id: number) {
  const cachedDoc = docStore.getCachedDoc(id);
  const cachedPath = docStore.getCachedPath(id);
  const cachedContent = docStore.getCachedContent(id);

  const results = await Promise.allSettled([
    cachedContent ? Promise.resolve({ content: cachedContent }) : getDocumentContent(id),
    cachedDoc ? Promise.resolve(cachedDoc) : getDocumentByID(id),
    cachedPath ? Promise.resolve(cachedPath) : getDocumentPath(id).catch(() => [] as { id: number; name: string }[]),
  ]);

  let loaded = '';
  let serverUpdateTime = 0;
  let loadedPath: Array<{ id: number; name: string }> = [];

  if (results[0].status === 'fulfilled') {
    loaded = results[0].value.content || '';
    // 回填 store 缓存，便于后续其他视图复用
    if (!cachedContent) docStore.setCachedContent(id, loaded);
  } else {
    logger.error('Failed to load document content:', results[0].reason);
    ui.showToast(t('ws.k172'), 'warning');
  }

  if (results[1].status === 'fulfilled') {
    const meta = results[1].value as unknown as { update_time?: number; status?: string; published_at?: number };
    serverUpdateTime = meta.update_time || 0;
    docStatus.value = meta.status || '';
    docPublishedAt.value = meta.published_at || 0;
  } else {
    logger.error('Failed to load document meta:', results[1].reason);
    ui.showToast(t('ws.k173'), 'warning');
  }

  if (results[2].status === 'fulfilled') {
    loadedPath = results[2].value as Array<{ id: number; name: string }>;
  }

  serverContent.value = loaded;
  lastUpdateTime.value = serverUpdateTime;
  breadcrumbs.value = loadedPath.map((p) => ({ id: p.id, name: p.name }));
  saveServerTimestamp(id, serverUpdateTime);
  return { loaded, serverUpdateTime };
}

function handleDraftRecovery(loaded: string, serverUpdateTime: number) {
  const draft = loadDraft(docId.value);
  const draftStatus = hasDraft(docId.value, serverUpdateTime);

  if (draft && draft !== loaded) {
    if (draftStatus.expired || draftStatus.serverUpdated) {
      clearDraft(docId.value);
      content.value = loaded;
      ui.showToast(t('ws.k174'), 'info');
    } else {
      content.value = loaded;
      showUnsavedModal.value = false;
      showRecoverModal.value = true;
    }
  } else {
    content.value = loaded;
  }
}

async function loadDocument() {
  try {
    const { loaded, serverUpdateTime } = await loadDocumentData(docId.value);
    handleDraftRecovery(loaded, serverUpdateTime);
    saveStatus.value = '';
  } catch {
    ui.showToast(t('ws.k175'), 'error');
  }
}

async function togglePublish() {
  if (publishing.value) return;
  const nextPublished = !isPublished.value;
  publishing.value = true;
  try {
    if (nextPublished) {
      await publishDocument(docId.value);
    } else {
      await unpublishDocument(docId.value);
    }
    docStatus.value = nextPublished ? 'published' : 'draft';
    if (nextPublished) docPublishedAt.value = Math.floor(Date.now() / 1000);
    ui.showToast(nextPublished ? '文档已发布' : '文档已转为草稿', 'success');
  } catch (error: unknown) {
    const err = error as { message?: string };
    ui.showToast(err?.message || t('fallback.k39'), 'error');
  } finally {
    publishing.value = false;
  }
}

async function loadCollabSessions() {
  sessionsLoading.value = true;
  try {
    const res = await getCollabSessions(docId.value);
    collabSessions.value = res.sessions || [];
  } catch (error) {
    logger.error('Failed to load collab sessions:', error);
    collabSessions.value = [];
  } finally {
    sessionsLoading.value = false;
  }
}

function recoverDraft() {
  const draft = loadDraft(docId.value);
  if (draft) {
    content.value = draft;
    ui.showToast(t('ws.k176'), 'success');
  }
  showRecoverModal.value = false;
}

function discardDraft() {
  clearDraft(docId.value);
  showRecoverModal.value = false;
}

function navigateToSpaceRoot() {
  isNavigatingAway.value = true;
  router.push(`/space/${spaceId.value}`);
}

function navigateToDoc(id: number) {
  isNavigatingAway.value = true;
  router.push(`/space/${spaceId.value}/doc/${id}`);
}

async function handleSave() {
  if (storageGet(StorageKeys.EDITOR_SKIP_CHANGELOG) === '1') {
    changeLogInput.value = '';
    await confirmSaveWithLog();
    return;
  }
  changeLogInput.value = '';
  skipChangeLog.value = false;
  showChangeLogModal.value = true;
}

async function confirmSaveWithLog() {
  showChangeLogModal.value = false;
  if (isSaving.value) return;
  if (skipChangeLog.value) {
    storageSet(StorageKeys.EDITOR_SKIP_CHANGELOG, '1');
  }
  const changeLog = changeLogInput.value || t('fallback.k95');
  const originalSaveFn = async () => {
    const updated = await updateDocument(docId.value, {
      content: content.value,
      change_log: changeLog,
      last_update_time: lastUpdateTime.value,
    });
    serverContent.value = content.value;
    lastSaveTime.value = new Date();
    // 以服务端返回的 update_time 为准，避免客户端时间偏差导致下次保存误判 409 冲突
    lastUpdateTime.value = updated.update_time ?? Math.floor(Date.now() / 1000);
    clearDraft(docId.value);
    docStore.invalidateDocCache(docId.value);
    // 重新拉取元数据（含 editor_name/update_time）
    await docStore.loadDocument(docId.value);
    // 兜底：loadDocument 缓存失效时会先写入树节点占位数据（无 update_time/editor_name），
    // 若该请求被随后的路由跳转取消，占位数据会残留导致顶部元数据不刷新。
    // 这里用保存响应 + 当前登录用户补全，确保跳转后“修改者/更新于”是最新的
    if (docStore.currentDocument?.id === docId.value) {
      const current = docStore.currentDocument;
      docStore.currentDocument = {
        ...current,
        update_time: updated.update_time ?? current.update_time,
        edit_user_id: updated.edit_user_id ?? current.edit_user_id,
        editor_id: updated.edit_user_id ?? current.editor_id,
        editor_name: auth.user?.given_name || auth.user?.username || current.editor_name,
      };
    }
    ui.showToast(t('ws.k177'), 'success');
    if (saveTimer) clearTimeout(saveTimer);

  };

  saveStatus.value = '保存中...';
  isSaving.value = true;
  try {
    await originalSaveFn();
    saveStatus.value = '已保存';
    // 保存成功后跳转到文档查看页
    isNavigatingAway.value = true;
    router.push(`/space/${spaceId.value}/doc/${docId.value}`);
  } catch (error: unknown) {
    const err = error as { message?: string };
    if (getApiErrorStatus(error) === 409) {
      saveStatus.value = '冲突';
      saveDraft(docId.value, content.value);
      conflictLocalContent.value = content.value;
      conflictServerContent.value = '';
      showConflictModal.value = true;
      try {
        const res = await getDocumentContent(docId.value);
        conflictServerContent.value = res.content || '';
      } catch {
        ui.showToast(t('ws.k178'), 'error');
      }
      ui.showToast(t('ws.k179'), 'warning');
    } else {
      saveStatus.value = '保存失败';
      ui.showToast(err?.message || t('fallback.k8'), 'error');
    }
  } finally {
    isSaving.value = false;
  }
}

function cancelSaveWithLog() {
  showChangeLogModal.value = false;
}

async function handleRollback(version: number) {
  try {
    await docStore.rollback(docId.value, version);
    ui.showToast(t('ws.k180'), 'success');
    loadDocument();
  } catch {
    ui.showToast(t('ws.k181'), 'error');
  }
}

async function handleCancel() {
  if (hasUnsavedChanges.value) {
    showUnsavedModal.value = true;
  } else {
    clearDraft(docId.value);
    navigateToDoc(docId.value);
  }
}

function handleContinueEditing() {
  showUnsavedModal.value = false;
  resolvePendingNavigation(false);
}

function confirmCancel() {
  clearDraft(docId.value);
  showUnsavedModal.value = false;
  isNavigatingAway.value = true;
  if (pendingNavigationResolve) {
    resolvePendingNavigation(true);
  } else {
    navigateToDoc(docId.value);
  }
}

function handleKeepLocalVersion() {
  content.value = conflictLocalContent.value;
  showConflictModal.value = false;
  ui.showToast(t('ws.k182'), 'info');
}

async function handleUseServerVersion() {
  content.value = conflictServerContent.value;
  serverContent.value = conflictServerContent.value;
  showConflictModal.value = false;
  clearDraft(docId.value);
  ui.showToast(t('ws.k183'), 'info');
}

let pendingNavigationResolve: ((allow: boolean) => void) | null = null;

function resolvePendingNavigation(allow: boolean) {
  if (pendingNavigationResolve) {
    const resolve = pendingNavigationResolve;
    pendingNavigationResolve = null;
    resolve(allow);
  }
}

onBeforeRouteLeave((to, from, next) => {
  if (isNavigatingAway.value) {
    next();
    return;
  }
  if (hasUnsavedChanges.value) {
    showRecoverModal.value = false;
    showUnsavedModal.value = true;
    pendingNavigationResolve = (allow) => {
      if (allow) {
        clearDraft(docId.value);
        isNavigatingAway.value = true;
        next();
      } else {
        next(false);
      }
      pendingNavigationResolve = null;
    };
  } else {
    clearDraft(docId.value);
    next();
  }
});

const conflictContainerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
};

const conflictDescStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'),
  margin: 0,
};

const conflictOptionsStyle = computed(() => ({
  display: 'grid',
  gridTemplateColumns: isMobile.value ? '1fr' : appleFixedColumns(2),
  gap: AppleSpace.md,
}));

const conflictOptionBoxStyle = {
  flex: 1,
  padding: AppleSpace.md,
  borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`,
  background: cv('surface-panel'),
};

const conflictTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: `0 0 ${AppleSpace.xs} 0`,
};

const conflictInfoStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  margin: 0,
};

const srOnlyTitleStyle = {
  position: 'absolute' as const,
  width: '1px',
  height: '1px',
  margin: '-1px',
  padding: '0',
  overflow: 'hidden',
  clip: 'rect(0 0 0 0)',
  clipPath: 'inset(50%)',
  whiteSpace: 'nowrap' as const,
  border: '0',
};

const topBarStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  flexWrap: 'wrap' as const,
  rowGap: AppleSpace.xs,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  flexShrink: 0,
  minHeight: '36px',
};

const editorAreaStyle = {
  display: 'flex',
  flex: 1,
  minHeight: 0,
  overflow: 'hidden' as const,
};

const editorMainStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  flex: 1,
  minHeight: 0,
  overflow: 'hidden',
};

// 移动端侧栏浮层化，避免把编辑器挤到不足一栏
const sidePanelStyle = computed(() => isMobile.value
  ? {
      position: 'fixed' as const,
      top: 0, right: 0, bottom: 0,
      width: '84%',
      maxWidth: '340px',
      zIndex: AppleZIndex.drawer,
      borderLeft: `1px solid ${cv('border-light')}`,
      background: cv('surface-float'),
      // 抽屉族的阴影取 modal 档，与 ApDrawer / MobileTocFab / SpaceView 的移动端抽屉一致；
      // 原先这里写 shadow-card，于是同一族四个抽屉里只有它薄一档，玻璃边缘厚度也少一圈。
      boxShadow: cv('shadow-modal'),
      display: 'flex',
      flexDirection: 'column' as const,
      overflowY: 'auto' as const,
    }
  : {
      width: '340px',
      flexShrink: 0,
      borderLeft: `1px solid ${cv('border-light')}`,
      // 宽视口下这块侧栏是**版面的一部分**（与编辑器并排、不压在任何东西上），不是浮层，
      // 所以走卡片档；上面窄视口那一支才是真浮在编辑器上的抽屉，走 float 档。
      background: cv('surface-panel'),
      display: 'flex',
      flexDirection: 'column' as const,
      overflowY: 'auto' as const,
    });

async function handleIconUpdate(icon: string) {
  if (!docStore.currentDocument) return;
  try {
    await updateDocument(docStore.currentDocument.id, { icon });
    if (docStore.currentDocument) docStore.currentDocument.icon = icon;
    ui.showToast(t('ws.k184'), 'success');
  } catch {
    ui.showToast(t('ws.k185'), 'error');
  }
}

async function handleCoverUpdate(coverImage: string) {
  if (!docStore.currentDocument) return;
  try {
    await updateDocument(docStore.currentDocument.id, { cover_image: coverImage });
    if (docStore.currentDocument) docStore.currentDocument.cover_image = coverImage;
    ui.showToast(t('ws.k186'), 'success');
  } catch {
    ui.showToast(t('ws.k187'), 'error');
  }
}</script>
