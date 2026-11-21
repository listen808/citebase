<template>
  <ApPageContainer
    :title="$t('ws.k84')"
    :subtitle="$t('ws.k96')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApButton variant="primary" size="small" :disabled="!selectedDocId" @click="openSubmitModal">
        <ApIcon name="ShieldCheck" :size="16" /> {{ $t('ws.k30') }}
      </ApButton>
    </template>

    <div :style="listStyle()">
      <!-- 协作范围 -->
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="chipRowStyle">
              <ApIcon name="FileText" :size="18" :color="cv('blue')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k31') }}</span>
            </div>
            <ApButton variant="ghost" size="small" :loading="docLoading" @click="loadDocuments">
              <ApIcon name="RefreshCw" :size="16" /> {{ $t('ws.k32') }}
            </ApButton>
          </div>
        </template>

        <div :style="toolbarStyle">
          <!-- 视角切换以前是「协作范围」卡下面另起的一行，一行里只有它自己；
               并进这条工具行的开头（左边是"看哪些"，与空间/文档选择器同一行） -->
          <ApSegmentedControl v-model="activeTab" :options="tabOptions" size="small" :aria-label="$t('ws.k97')" />
          <div :style="filterStyle">
            <span :style="filterLabelStyle">{{ $t('ws.k33') }}</span>
            <ApSelect v-model="selectedSpaceId" :options="spaceOptions" :placeholder="$t('ws.k73')" searchable @update:model-value="onSpaceChange" />
          </div>
          <div :style="filterStyle">
            <span :style="filterLabelStyle">{{ $t('ws.k34') }}</span>
            <ApSelect v-model="selectedDocId" :options="documentOptions" :placeholder="$t('ws.k74')" searchable @update:model-value="onDocChange" />
          </div>
          <div :style="filterStyle">
            <span :style="filterLabelStyle">{{ $t('ws.k35') }}</span>
            <ApStatusBadge
              v-if="docApproval"
              :status="approvalBadge(docApproval.status)"
              :label="approvalLabel(docApproval.status)"
              size="small"
            />
            <span v-else :style="hintStyle">{{ selectedDocId ? $t('vWorkspaceUi.k10') : $t('vWorkspaceUi.k11') }}</span>
          </div>
        </div>
        <p :style="hintStyle">
          {{ $t('ws.k36') }}
        </p>
      </ApCard>

      <!-- 待我审批 -->
      <ApCard v-if="activeTab === 'pending'" size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="chipRowStyle">
              <ApIcon name="Inbox" :size="18" :color="cv('orange')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k37') }}</span>
              <span :style="countBadgeStyle">{{ pendingTotal }}</span>
            </div>
            <ApButton variant="ghost" size="small" :loading="pendingLoading" @click="loadPending">{{ $t('ws.refresh') }}</ApButton>
          </div>
        </template>

        <ApTable :columns="pendingColumns" :data="pendingRows" :loading="pendingLoading" density="compact">
          <template #actions="{ row }">
            <div :style="actionGroupStyle">
              <ApButton variant="primary" size="small" @click="openHandleModal(row as unknown as ApprovalRecord, 'approve')">
                <ApIcon name="Check" :size="16" /> {{ $t('ws.k39') }}
              </ApButton>
              <ApButton variant="danger" size="small" @click="openHandleModal(row as unknown as ApprovalRecord, 'reject')">
                <ApIcon name="X" :size="16" /> {{ $t('ws.k40') }}
              </ApButton>
            </div>
          </template>
          <template #empty>
            <ApEmpty type="inbox" :title="$t('ws.k85')" :description="$t('vWorkspaceUi.k2')" />
          </template>
        </ApTable>
        <ApPagination
          v-if="pendingTotal > pendingPageSize"
          :page="pendingPage"
          :page-size="pendingPageSize"
          :total="pendingTotal"
          @change="onPendingPageChange"
        />
      </ApCard>

      <!-- 我提交的审批 -->
      <ApCard v-if="activeTab === 'submitted'" size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="chipRowStyle">
              <ApIcon name="Send" :size="18" :color="cv('blue')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k41') }}</span>
              <span :style="countBadgeStyle">{{ submittedTotal }}</span>
            </div>
            <ApButton variant="ghost" size="small" :loading="submittedLoading" @click="loadSubmitted">{{ $t('ws.refresh') }}</ApButton>
          </div>
        </template>

        <ApTable :columns="submittedColumns" :data="submittedRows" :loading="submittedLoading" density="compact">
          <template #empty>
            <ApEmpty type="list" :title="$t('ws.k86')" :description="$t('vWorkspaceUi.k3')" />
          </template>
        </ApTable>
        <ApPagination
          v-if="submittedTotal > submittedPageSize"
          :page="submittedPage"
          :page-size="submittedPageSize"
          :total="submittedTotal"
          @change="onSubmittedPageChange"
        />
      </ApCard>

      <!-- 版本里程碑 -->
      <ApCard v-if="activeTab === 'tags'" size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="chipRowStyle">
              <ApIcon name="Milestone" :size="18" :color="cv('purple')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k42') }}</span>
              <span :style="countBadgeStyle">{{ tags.length }}</span>
            </div>
            <ApButton variant="primary" size="small" :disabled="!selectedDocId" @click="openTagModal">
              <ApIcon name="Plus" :size="16" /> {{ $t('ws.k43') }}
            </ApButton>
          </div>
        </template>

        <div :style="toolbarStyle">
          <div :style="filterStyle">
            <span :style="filterLabelStyle">{{ $t('ws.k44') }}</span>
            <ApSelect v-model="tagScope" :options="tagScopeOptions" @update:model-value="loadTags" />
          </div>
          <div v-if="tagScope === 'doc'" :style="filterStyle">
            <span :style="filterLabelStyle">{{ $t('ws.k45') }}</span>
            <ApInput v-model="tagVersionFilter" type="number" :placeholder="$t('ws.k75')" />
          </div>
          <div :style="actionGroupStyle">
            <ApButton variant="secondary" size="small" :loading="tagsLoading" @click="loadTags">{{ $t('ws.k46') }}</ApButton>
            <ApButton variant="ghost" size="small" @click="resetTagFilter">{{ $t('ws.reset') }}</ApButton>
          </div>
        </div>

        <ApTable :columns="tagColumns" :data="tags" :loading="tagsLoading" density="compact" row-key="id">
          <template #actions="{ row }">
            <ApButton variant="danger" size="small" @click="askDeleteTag(row as unknown as VersionTag)">
              <ApIcon name="Trash2" :size="16" /> {{ $t('ws.delete') }}
            </ApButton>
          </template>
          <template #empty>
            <ApEmpty type="list" :title="$t('ws.k87')" :description="$t('vWorkspaceUi.k4')" />
          </template>
        </ApTable>
      </ApCard>

      <!-- 提及 -->
      <template v-if="activeTab === 'mentions'">
        <ApCard size="small">
          <template #header>
            <div :style="headerRowStyle">
              <div :style="chipRowStyle">
                <ApIcon name="AtSign" :size="18" :color="cv('teal')" />
                <span :style="sectionTitleStyle">{{ $t('ws.k49') }}</span>
                <span :style="countBadgeStyle">{{ mentions.length }}</span>
              </div>
              <ApButton variant="ghost" size="small" :loading="mentionsLoading" @click="loadMentions">{{ $t('ws.refresh') }}</ApButton>
            </div>
          </template>

          <ApTable :columns="mentionColumns" :data="mentions" :loading="mentionsLoading" density="compact" row-key="user_id">
            <template #empty>
              <ApEmpty type="list" :title="$t('ws.k88')" description="在正文里写 @用户名，保存后由后端解析入库" />
            </template>
          </ApTable>
        </ApCard>

        <ApCard size="small">
          <template #header>
            <div :style="headerRowStyle">
              <div :style="chipRowStyle">
                <ApIcon name="Users" :size="18" :color="cv('blue')" />
                <span :style="sectionTitleStyle">{{ $t('ws.k50') }}</span>
              </div>
            </div>
          </template>

          <div :style="listStyle()">
            <ApTextarea
              v-model="mentionDraft"
              :label="$t('ws.k62')"
              :placeholder="$t('ws.k76')"
              :rows="5"
            />
            <div :style="actionGroupStyle">
              <ApButton variant="secondary" size="small" :loading="previewLoading" :disabled="!mentionDraft.trim()" @click="runMentionPreview">
                {{ $t('ws.k51') }}
              </ApButton>
            </div>
            <div v-if="previewTokens.length" :style="tagRowStyle">
              <span v-for="token in previewTokens" :key="token" :style="previewChipStyle(token)">
                @{{ token }} · {{ resolveName(token) || $t('fallback.k73') }}
              </span>
            </div>
            <p :style="hintStyle">{{ $t('ws.k52') }}</p>
          </div>
        </ApCard>

        <ApCard size="small">
          <template #header>
            <div :style="headerRowStyle">
              <div :style="chipRowStyle">
                <ApIcon name="Bell" :size="18" :color="cv('orange')" />
                <span :style="sectionTitleStyle">{{ $t('ws.k53') }}</span>
                <span :style="countBadgeStyle">{{ mentionNotices.length }}</span>
              </div>
              <ApButton variant="ghost" size="small" :loading="noticeLoading" @click="loadMentionNotices">{{ $t('ws.refresh') }}</ApButton>
            </div>
          </template>

          <ApTable :columns="noticeColumns" :data="mentionNotices" :loading="noticeLoading" density="compact">
            <template #actions="{ row }">
              <ApButton
                variant="ghost"
                size="small"
                :disabled="(row as unknown as NotificationItem).is_read === 1"
                @click="markRead(row as unknown as NotificationItem)"
              >
                {{ (row as unknown as NotificationItem).is_read === 1 ? '已读' : '标记已读' }}
              </ApButton>
            </template>
            <template #empty>
              <ApEmpty type="inbox" :title="$t('ws.k89')" description="有人在文档中 @ 你时会收到通知" />
            </template>
          </ApTable>
        </ApCard>
      </template>

      <!-- 翻译管理 -->
      <template v-if="activeTab === 'translations'">
        <ApCard size="small">
          <template #header>
            <div :style="headerRowStyle">
              <div :style="chipRowStyle">
                <ApIcon name="Languages" :size="18" :color="cv('indigo')" />
                <span :style="sectionTitleStyle">{{ $t('ws.k54') }}</span>
                <span :style="countBadgeStyle">{{ translations.length }}</span>
              </div>
              <ApButton variant="primary" size="small" :disabled="!selectedDocId" @click="openTranslationModal">
                <ApIcon name="Plus" :size="16" /> {{ $t('ws.k55') }}
              </ApButton>
            </div>
          </template>

          <ApTable :columns="translationColumns" :data="translations" :loading="translationsLoading" density="compact" row-key="id">
            <template #actions="{ row }">
              <ApButton variant="danger" size="small" @click="askDeleteTranslation(row as unknown as DocumentTranslation)">
                <ApIcon name="Trash2" :size="16" /> {{ $t('ws.k56') }}
              </ApButton>
            </template>
            <template #empty>
              <ApEmpty type="list" :title="$t('ws.k90')" :description="$t('vWorkspaceUi.k5')" />
            </template>
          </ApTable>
        </ApCard>

        <ApCard size="small">
          <template #header>
            <div :style="headerRowStyle">
              <div :style="chipRowStyle">
                <ApIcon name="Globe" :size="18" :color="cv('gray')" />
                <span :style="sectionTitleStyle">{{ $t('ws.k57') }}</span>
              </div>
            </div>
          </template>
          <div :style="tagRowStyle">
            <span v-for="lang in languages" :key="lang.code" :style="tagChipStyle">{{ lang.native_name }}（{{ lang.code }}）</span>
            <span v-if="!languages.length" :style="hintStyle">{{ $t('ws.k58') }}</span>
          </div>
        </ApCard>
      </template>
    </div>

    <!-- 提交审批 -->
    <ApModal v-model="showSubmitModal" :title="$t('ws.k91')" size="medium">
      <ApFormItem :label="$t('ws.k34')" required :error="submitDocError">
        <ApSelect v-model="submitForm.document_id" :options="documentOptions" :placeholder="$t('ws.k77')" searchable />
      </ApFormItem>
      <ApFormItem :label="$t('ws.k63')" required :error="submitApproverError" :hint="$t('vWorkspaceUi.k6')">
        <ApSelect v-model="submitForm.approver" :options="approverOptions" :placeholder="$t('ws.k78')" searchable />
      </ApFormItem>
      <ApFormItem :label="$t('ws.k64')">
        <ApTextarea v-model="submitForm.comment" :placeholder="$t('ws.k79')" :rows="4" :maxlength="1000" />
      </ApFormItem>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showSubmitModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="submitting" @click="handleSubmit">{{ $t('ws.k30') }}</ApButton>
      </template>
    </ApModal>

    <!-- 处理审批 -->
    <ApModal v-model="showHandleModal" :title="handleForm.decision === 'approve' ? '通过审批' : '驳回审批'" size="small">
      <p :style="hintStyle">
        文档《{{ docNameOf(handlingApproval?.document_id ?? 0) }}》 · 提交人 {{ userNameOf(handlingApproval?.submitter ?? 0) }}
      </p>
      <ApFormItem :label="$t('ws.k65')" :hint="$t('vWorkspaceUi.k7')">
        <ApTextarea v-model="handleForm.comment" :placeholder="$t('ws.k65')" :rows="4" :maxlength="1000" />
      </ApFormItem>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showHandleModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton
          :variant="handleForm.decision === 'approve' ? 'primary' : 'danger'"
          size="small"
          :loading="handling"
          @click="handleDecision"
        >
          {{ handleForm.decision === 'approve' ? $t('vWorkspaceUi.k12') : $t('vWorkspaceUi.k13') }}
        </ApButton>
      </template>
    </ApModal>

    <!-- 新建里程碑 -->
    <ApModal v-model="showTagModal" :title="$t('ws.k92')" size="medium">
      <ApFormItem :label="$t('ws.k66')" required :error="tagError">
        <ApInput v-model="tagForm.tag_name" :placeholder="$t('ws.k80')" :maxlength="100" />
      </ApFormItem>
      <ApFormItem :label="$t('ws.k67')" required :hint="$t('vWorkspaceUi.k8')">
        <ApInput v-model="tagForm.version" type="number" placeholder="0" />
      </ApFormItem>
      <ApFormItem :label="$t('ws.k68')">
        <ApSelect v-model="tagForm.color" :options="tagColorOptions" />
      </ApFormItem>
      <ApFormItem :label="$t('ws.k69')">
        <ApTextarea v-model="tagForm.note" :placeholder="$t('ws.k81')" :rows="3" :maxlength="500" />
      </ApFormItem>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showTagModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="tagSaving" @click="handleCreateTag">{{ $t('ws.create2') }}</ApButton>
      </template>
    </ApModal>

    <!-- 关联译文 -->
    <ApModal v-model="showTranslationModal" :title="$t('ws.k93')" size="medium">
      <ApFormItem :label="$t('ws.k70')" required>
        <ApSelect v-model="translationForm.lang" :options="languageOptions" :placeholder="$t('ws.k82')" searchable />
      </ApFormItem>
      <ApFormItem :label="$t('ws.k71')" required :error="translationError" :hint="$t('vWorkspaceUi.k9')">
        <ApSelect v-model="translationForm.translated_document_id" :options="documentOptions" :placeholder="$t('ws.k83')" searchable />
      </ApFormItem>
      <ApFormItem :label="$t('ws.k72')">
        <ApSelect v-model="translationForm.translation_status" :options="translationStatusOptions" />
      </ApFormItem>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showTranslationModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="translationSaving" @click="handleCreateTranslation">{{ $t('ws.k61') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showTagDeleteModal"
      :title="$t('ws.k94')"
      :message="`确定删除里程碑「${deletingTag?.tag_name ?? ''}」吗？此操作不可撤销。`"
      type="danger"
      :confirm-text="$t('ws.delete')"
      :loading="tagDeleting"
      @confirm="confirmDeleteTag"
    />

    <ApConfirmModal
      v-model="showTranslationDeleteModal"
      :title="$t('ws.k95')"
      :message="`确定解除「${deletingTranslation ? langLabel(deletingTranslation.lang) : ''}」译文关联吗？译文文档本身不会被删除。`"
      type="danger"
      :confirm-text="$t('ws.k56')"
      :loading="translationDeleting"
      @confirm="confirmDeleteTranslation"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref } from 'vue';
import { logger } from '@/utils/logger';
import { AppleLayout, cv } from '@/utils/core/apple-tokens';
import { getApiErrorStatus } from '@/api/client';
import { formatTime } from '@/utils/format/format-time';
import {
  actionGroupStyle,
  aiSubCountBadgeStyle as countBadgeStyle,
  aiSubFilterLabelStyle as filterLabelStyle,
  aiSubFilterStyle as filterStyle,
  aiSubSectionHeaderStyle as headerRowStyle,
  aiSubSectionMetaStyle as hintStyle,
  aiSubSectionTitleStyle as sectionTitleStyle,
  aiSubTagRowStyle as tagRowStyle,
  aiSubTagStyle as tagChipStyle,
  listStyle,
  statusRowStyle as chipRowStyle,
  tagStyle,
  toolbarStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import {
  getDocumentApproval,
  handleApproval,
  listPendingApprovals,
  listSubmittedApprovals,
  submitApproval,
  type ApprovalRecord,
  type ApprovalStatus,
} from '@/api/modules/document/approval';
import {
  createVersionTag,
  deleteVersionTag,
  getDocumentVersionTags,
  getSpaceVersionTags,
  getVersionTags,
  type VersionTag,
} from '@/api/modules/document/version-tag';
import { getDocumentMentions, previewMentions, type MentionUser } from '@/api/modules/document/mention';
import {
  createTranslation,
  deleteTranslation,
  getDocumentTranslations,
  getSupportedLanguages,
  type DocumentTranslation,
  type LanguageInfo,
  type TranslationStatus,
} from '@/api/modules/document/translation';
import { getDocumentTree } from '@/api/modules/document/document';
import { getUsers } from '@/api/modules/user/user';
import { getNotifications, markNotificationRead, type NotificationItem } from '@/api/modules/social/notification';
import { useSpaceStore } from '@/stores/workspace/space';
import { useAuthStore } from '@/stores/auth/auth';
import { useUIStore } from '@/stores/app/ui';
import type { Document, Space, User } from '@/types/models';

const ui = useUIStore();
const spaceStore = useSpaceStore();
const auth = useAuthStore();

type TabValue = 'pending' | 'submitted' | 'tags' | 'mentions' | 'translations';

const activeTab = ref<TabValue>('pending');
const tabOptions = [
  { label: t('ws.k37'), value: 'pending' },
  { label: t('ws.k41'), value: 'submitted' },
  { label: t('ws.k42'), value: 'tags' },
  { label: t('ws.k98'), value: 'mentions' },
  { label: t('ws.k99'), value: 'translations' },
];

/** 里程碑标识色：落库存语义色名，渲染时映射到已注册设计令牌 */
const TAG_PALETTE: Record<string, string> = {
  blue: cv('blue'),
  green: cv('green'),
  orange: cv('orange'),
  purple: cv('purple'),
  teal: cv('teal'),
  indigo: cv('indigo'),
  red: cv('red'),
};
const tagColorOptions = Object.keys(TAG_PALETTE).map((key) => ({ label: key, value: key }));
const translationStatusOptions = [
  { label: t('ws.k100'), value: 'draft' },
  { label: t('ws.k101'), value: 'published' },
];

// ===== 作用域数据 =====
const spaces = ref<Space[]>([]);
const users = ref<User[]>([]);
const documents = ref<{ id: number; name: string }[]>([]);
const selectedSpaceId = ref<number | null>(null);
const selectedDocId = ref<number | null>(null);
const docLoading = ref(false);

const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));
const documentOptions = computed(() => documents.value.map((d) => ({ label: d.name, value: d.id })));
const userNames = computed(() => {
  const map = new Map<number, string>();
  users.value.forEach((u) => map.set(u.id, u.given_name || u.username));
  return map;
});
const approverOptions = computed(() =>
  users.value
    .filter((u) => u.id !== auth.user?.id)
    .map((u) => ({ label: `${u.given_name || u.username}（${u.username}）`, value: u.id })),
);

function userNameOf(id: number) {
  if (!id) return '—';
  return userNames.value.get(id) ?? `用户 #${id}`;
}

function docNameOf(id: number) {
  if (!id) return '—';
  const hit = documents.value.find((d) => d.id === id);
  return hit ? hit.name : `文档 #${id}`;
}

function resolveName(token: string) {
  const hit = users.value.find((u) => u.username === token || u.given_name === token);
  return hit ? hit.given_name || hit.username : '';
}

function previewChipStyle(token: string) {
  return tagStyle(resolveName(token) ? cv('green') : cv('gray'));
}

// ===== 审批 =====
const pendingRows = ref<ApprovalRecord[]>([]);
const pendingTotal = ref(0);
const pendingPage = ref(1);
const pendingPageSize = 20;
const pendingLoading = ref(false);

const submittedRows = ref<ApprovalRecord[]>([]);
const submittedTotal = ref(0);
const submittedPage = ref(1);
const submittedPageSize = 20;
const submittedLoading = ref(false);

const docApproval = ref<ApprovalRecord | null>(null);

function approvalLabel(status: ApprovalStatus) {
  if (status === 'approved') return '已通过';
  if (status === 'rejected') return '已驳回';
  return '待审批';
}

function approvalBadge(status: ApprovalStatus): 'warning' | 'enabled' | 'error' {
  if (status === 'approved') return 'enabled';
  if (status === 'rejected') return 'error';
  return 'warning';
}

async function loadPending() {
  pendingLoading.value = true;
  try {
    const res = await listPendingApprovals({ page: pendingPage.value, page_size: pendingPageSize });
    pendingRows.value = res?.data ?? [];
    pendingTotal.value = res?.total ?? 0;
  } catch (e) {
    logger.error('加载待我审批失败:', e);
    ui.showToast(t('ws.k119'), 'error');
  } finally {
    pendingLoading.value = false;
  }
}

async function loadSubmitted() {
  submittedLoading.value = true;
  try {
    const res = await listSubmittedApprovals({ page: submittedPage.value, page_size: submittedPageSize });
    submittedRows.value = res?.data ?? [];
    submittedTotal.value = res?.total ?? 0;
  } catch (e) {
    logger.error('加载我提交的审批失败:', e);
    ui.showToast(t('ws.k120'), 'error');
  } finally {
    submittedLoading.value = false;
  }
}

function onPendingPageChange(page: number) {
  pendingPage.value = page;
  loadPending();
}

function onSubmittedPageChange(page: number) {
  submittedPage.value = page;
  loadSubmitted();
}

async function loadDocApproval() {
  docApproval.value = null;
  if (!selectedDocId.value) return;
  try {
    docApproval.value = await getDocumentApproval(selectedDocId.value);
  } catch (e) {
    // 后端无记录时返回 404，属于正常态
    if (getApiErrorStatus(e) !== 404) {
      logger.error('加载文档审批状态失败:', e);
      ui.showToast(t('ws.k121'), 'error');
    }
  }
}

const pendingColumns = computed<TableColumn<ApprovalRecord>[]>(() => [
  { key: 'document_id', title: t('ws.k34'), ellipsis: true, render: (row) => docNameOf(row.document_id) },
  { key: 'submitter', title: t('ws.k102'), width: '16%', render: (row) => userNameOf(row.submitter) },
  { key: 'comment', title: t('ws.k64'), ellipsis: true, render: (row) => row.comment || '—' },
  { key: 'submit_time', title: t('ws.k103'), width: '20%', render: (row) => formatTime(row.submit_time) },
]);

const submittedColumns = computed<TableColumn<ApprovalRecord>[]>(() => [
  { key: 'document_id', title: t('ws.k34'), ellipsis: true, render: (row) => docNameOf(row.document_id) },
  { key: 'approver', title: t('ws.k63'), width: '14%', render: (row) => userNameOf(row.approver) },
  { key: 'status', title: t('ws.status'), width: '10%', render: (row) => approvalLabel(row.status) },
  { key: 'comment', title: t('ws.k65'), ellipsis: true, render: (row) => row.comment || '—' },
  { key: 'submit_time', title: t('ws.k103'), width: '16%', render: (row) => formatTime(row.submit_time) },
  { key: 'handle_time', title: t('ws.k105'), width: '16%', render: (row) => (row.handle_time ? formatTime(row.handle_time) : '未处理') },
]);

const showSubmitModal = ref(false);
const submitting = ref(false);
const submitDocError = ref('');
const submitApproverError = ref('');
const submitForm = ref({
  document_id: null as number | null,
  approver: null as number | null,
  comment: '',
});

function openSubmitModal() {
  submitForm.value = {
    document_id: selectedDocId.value,
    approver: null,
    comment: '',
  };
  submitDocError.value = '';
  submitApproverError.value = '';
  showSubmitModal.value = true;
}

async function handleSubmit() {
  const documentId = submitForm.value.document_id;
  const approver = submitForm.value.approver;
  submitDocError.value = documentId ? '' : '请选择文档';
  submitApproverError.value = approver ? '' : '请选择审批人';
  if (!documentId || !approver) return;

  submitting.value = true;
  try {
    await submitApproval({
      document_id: documentId,
      approver,
      comment: submitForm.value.comment || undefined,
    });
    ui.showToast(t('ws.k122'), 'success');
    showSubmitModal.value = false;
    await Promise.all([loadPending(), loadSubmitted(), loadDocApproval()]);
  } catch (e) {
    logger.error('提交审批失败:', e);
    ui.showToast((e as Error).message || t('fallback.k62'), 'error');
  } finally {
    submitting.value = false;
  }
}

const showHandleModal = ref(false);
const handling = ref(false);
const handlingApproval = ref<ApprovalRecord | null>(null);
const handleForm = ref({ decision: 'approve' as 'approve' | 'reject', comment: '' });

function openHandleModal(row: ApprovalRecord, decision: 'approve' | 'reject') {
  handlingApproval.value = row;
  handleForm.value = { decision, comment: '' };
  showHandleModal.value = true;
}

async function handleDecision() {
  const approval = handlingApproval.value;
  if (!approval) return;
  handling.value = true;
  try {
    await handleApproval(approval.id, {
      decision: handleForm.value.decision,
      comment: handleForm.value.comment || undefined,
    });
    ui.showToast(handleForm.value.decision === 'approve' ? '已通过审批' : '已驳回审批', 'success');
    showHandleModal.value = false;
    await Promise.all([loadPending(), loadSubmitted(), loadDocApproval()]);
  } catch (e) {
    logger.error('处理审批失败:', e);
    ui.showToast((e as Error).message || t('fallback.k46'), 'error');
  } finally {
    handling.value = false;
  }
}

// ===== 版本里程碑 =====
const tags = ref<VersionTag[]>([]);
const tagsLoading = ref(false);
const tagScope = ref<'doc' | 'space'>('doc');
const tagScopeOptions = [
  { label: t('ws.k106'), value: 'doc' },
  { label: t('ws.k107'), value: 'space' },
];
const tagVersionFilter = ref('');

async function loadTags() {
  tags.value = [];
  try {
    if (tagScope.value === 'space') {
      if (!selectedSpaceId.value) return;
      tags.value = (await getSpaceVersionTags(selectedSpaceId.value)) ?? [];
      return;
    }
    if (!selectedDocId.value) return;
    const version = Number(tagVersionFilter.value);
    tags.value =
      tagVersionFilter.value !== '' && Number.isFinite(version)
        ? (await getVersionTags(selectedDocId.value, version)) ?? []
        : (await getDocumentVersionTags(selectedDocId.value)) ?? [];
  } catch (e) {
    logger.error('加载版本里程碑失败:', e);
    ui.showToast(t('ws.k123'), 'error');
  }
}

function resetTagFilter() {
  tagScope.value = 'doc';
  tagVersionFilter.value = '';
  loadTags();
}

const tagColumns = computed<TableColumn<VersionTag>[]>(() => [
  { key: 'tag_name', title: t('ws.k108'), width: '18%', render: (row) => row.tag_name },
  { key: 'version', title: t('ws.k109'), width: '10%', render: (row) => (row.version === 0 ? '当前' : `v${row.version}`) },
  { key: 'document_id', title: t('ws.k34'), ellipsis: true, render: (row) => docNameOf(row.document_id) },
  { key: 'note', title: t('ws.k69'), ellipsis: true, render: (row) => row.note || '—' },
  { key: 'created_by', title: t('ws.k110'), width: '14%', render: (row) => row.creator_name || userNameOf(row.created_by) },
  { key: 'create_time', title: t('ws.k111'), width: '18%', render: (row) => formatTime(row.create_time) },
]);

const showTagModal = ref(false);
const tagSaving = ref(false);
const tagError = ref('');
const tagForm = ref({
  tag_name: '',
  version: '0',
  color: 'blue',
  note: '',
});

function openTagModal() {
  tagForm.value = { tag_name: '', version: '0', color: 'blue', note: '' };
  tagError.value = '';
  showTagModal.value = true;
}

async function handleCreateTag() {
  if (!selectedDocId.value) return;
  if (!tagForm.value.tag_name.trim()) {
    tagError.value = t('sentUi.k59');
    return;
  }
  tagSaving.value = true;
  try {
    await createVersionTag(selectedDocId.value, {
      tag_name: tagForm.value.tag_name.trim(),
      version: Number(tagForm.value.version) || 0,
      color: tagForm.value.color,
      note: tagForm.value.note.trim() || undefined,
    });
    ui.showToast(t('ws.k124'), 'success');
    showTagModal.value = false;
    await loadTags();
  } catch (e) {
    logger.error('创建里程碑失败:', e);
    ui.showToast(getApiErrorStatus(e) === 409 ? '该版本下同名里程碑已存在' : (e as Error).message || t('fallback.k18'), 'error');
  } finally {
    tagSaving.value = false;
  }
}

const showTagDeleteModal = ref(false);
const tagDeleting = ref(false);
const deletingTag = ref<VersionTag | null>(null);

function askDeleteTag(row: VersionTag) {
  deletingTag.value = row;
  showTagDeleteModal.value = true;
}

async function confirmDeleteTag() {
  const tag = deletingTag.value;
  if (!tag) return;
  tagDeleting.value = true;
  try {
    await deleteVersionTag(tag.id);
    ui.showToast(t('ws.k125'), 'success');
    showTagDeleteModal.value = false;
    deletingTag.value = null;
    await loadTags();
  } catch (e) {
    logger.error('删除里程碑失败:', e);
    ui.showToast(t('ws.k126'), 'error');
  } finally {
    tagDeleting.value = false;
  }
}

// ===== @提及 =====
const mentions = ref<MentionUser[]>([]);
const mentionsLoading = ref(false);
const mentionDraft = ref('');
const previewTokens = ref<string[]>([]);
const previewLoading = ref(false);

const mentionColumns = computed<TableColumn<MentionUser>[]>(() => [
  { key: 'user_id', title: t('ws.k112'), width: '14%' },
  { key: 'username', title: t('ws.k113'), width: '30%' },
  { key: 'given_name', title: t('ws.k114'), render: (row) => row.given_name || row.username },
]);

async function loadMentions() {
  mentions.value = [];
  if (!selectedDocId.value) return;
  mentionsLoading.value = true;
  try {
    mentions.value = (await getDocumentMentions(selectedDocId.value)) ?? [];
  } catch (e) {
    logger.error('加载提及名单失败:', e);
    ui.showToast(t('ws.k127'), 'error');
  } finally {
    mentionsLoading.value = false;
  }
}

async function runMentionPreview() {
  previewLoading.value = true;
  try {
    const res = await previewMentions(mentionDraft.value);
    previewTokens.value = res?.mentions ?? [];
    if (!previewTokens.value.length) ui.showToast(t('ws.k128'), 'info');
  } catch (e) {
    logger.error('提及预览失败:', e);
    ui.showToast(t('ws.k129'), 'error');
  } finally {
    previewLoading.value = false;
  }
}

const mentionNotices = ref<NotificationItem[]>([]);
const noticeLoading = ref(false);

const noticeColumns = computed<TableColumn<NotificationItem>[]>(() => [
  { key: 'title', title: t('ws.k115'), ellipsis: true },
  { key: 'related_id', title: t('ws.k34'), width: '20%', render: (row) => docNameOf(row.related_id) },
  { key: 'create_time', title: t('ws.time'), width: '20%', render: (row) => formatTime(row.create_time) },
  { key: 'is_read', title: t('ws.status'), width: '10%', render: (row) => (row.is_read === 1 ? '已读' : '未读') },
]);

async function loadMentionNotices() {
  noticeLoading.value = true;
  try {
    const res = await getNotifications({ type: 'all', page: 1, page_size: 50 });
    // 后端提及通知无独立类型（type=document），按标题识别
    mentionNotices.value = (res?.data ?? []).filter((n) => n.title.includes('提到了你'));
  } catch (e) {
    logger.error('加载提及通知失败:', e);
    ui.showToast(t('ws.k130'), 'error');
  } finally {
    noticeLoading.value = false;
  }
}

async function markRead(row: NotificationItem) {
  try {
    await markNotificationRead(row.id);
    row.is_read = 1;
  } catch (e) {
    logger.error('标记已读失败:', e);
    ui.showToast(t('ws.k131'), 'error');
  }
}

// ===== 翻译 =====
const languages = ref<LanguageInfo[]>([]);
const translations = ref<DocumentTranslation[]>([]);
const translationsLoading = ref(false);

const languageOptions = computed(() =>
  languages.value.map((l) => ({ label: `${l.native_name}（${l.code}）`, value: l.code })),
);

function langLabel(code: string) {
  const hit = languages.value.find((l) => l.code === code);
  return hit ? `${hit.native_name}（${code}）` : code;
}

const translationColumns = computed<TableColumn<DocumentTranslation>[]>(() => [
  { key: 'lang', title: t('ws.k70'), width: '22%', render: (row) => langLabel(row.lang) },
  { key: 'translated_document_id', title: t('ws.k71'), ellipsis: true, render: (row) => docNameOf(row.translated_document_id) },
  { key: 'translation_status', title: t('ws.status'), width: '14%', render: (row) => (row.translation_status === 'published' ? '已发布' : '草稿') },
  { key: 'document_id', title: t('ws.k117'), width: '18%', render: (row) => docNameOf(row.document_id) },
  { key: 'created_at', title: t('ws.k118'), width: '20%', render: (row) => formatTime(row.created_at) },
]);

async function loadLanguages() {
  try {
    languages.value = (await getSupportedLanguages()) ?? [];
  } catch (e) {
    logger.error('加载语言列表失败:', e);
  }
}

async function loadTranslations() {
  translations.value = [];
  if (!selectedDocId.value) return;
  translationsLoading.value = true;
  try {
    translations.value = (await getDocumentTranslations(selectedDocId.value)) ?? [];
  } catch (e) {
    logger.error('加载翻译组失败:', e);
    ui.showToast(t('ws.k132'), 'error');
  } finally {
    translationsLoading.value = false;
  }
}

const showTranslationModal = ref(false);
const translationSaving = ref(false);
const translationError = ref('');
const translationForm = ref({
  lang: null as string | number | null,
  translated_document_id: null as number | null,
  translation_status: 'draft' as TranslationStatus,
});

function openTranslationModal() {
  translationForm.value = {
    lang: languages.value[0]?.code ?? null,
    translated_document_id: null,
    translation_status: 'draft',
  };
  translationError.value = '';
  showTranslationModal.value = true;
}

async function handleCreateTranslation() {
  const documentId = selectedDocId.value;
  const target = translationForm.value.translated_document_id;
  if (!documentId) return;
  if (!target) {
    translationError.value = t('sentUi.k60');
    return;
  }
  translationSaving.value = true;
  try {
    await createTranslation(documentId, {
      lang: String(translationForm.value.lang ?? ''),
      translated_document_id: target,
      translation_status: translationForm.value.translation_status,
    });
    ui.showToast(t('ws.k133'), 'success');
    showTranslationModal.value = false;
    await loadTranslations();
  } catch (e) {
    logger.error('关联译文失败:', e);
    ui.showToast((e as Error).message || t('fallback.k13'), 'error');
  } finally {
    translationSaving.value = false;
  }
}

const showTranslationDeleteModal = ref(false);
const translationDeleting = ref(false);
const deletingTranslation = ref<DocumentTranslation | null>(null);

function askDeleteTranslation(row: DocumentTranslation) {
  deletingTranslation.value = row;
  showTranslationDeleteModal.value = true;
}

async function confirmDeleteTranslation() {
  const item = deletingTranslation.value;
  if (!item) return;
  translationDeleting.value = true;
  try {
    await deleteTranslation(item.id);
    ui.showToast(t('ws.k134'), 'success');
    showTranslationDeleteModal.value = false;
    deletingTranslation.value = null;
    await loadTranslations();
  } catch (e) {
    logger.error('解除译文关联失败:', e);
    ui.showToast(t('ws.k135'), 'error');
  } finally {
    translationDeleting.value = false;
  }
}

// ===== 作用域切换 =====
function flattenDocs(tree: Document[], out: { id: number; name: string }[]) {
  tree.forEach((doc) => {
    if (doc.type !== 2) out.push({ id: doc.id, name: doc.name });
    if (doc.children?.length) flattenDocs(doc.children, out);
  });
}

async function loadDocuments() {
  documents.value = [];
  if (!selectedSpaceId.value) return;
  docLoading.value = true;
  try {
    const tree = await getDocumentTree(selectedSpaceId.value);
    const out: { id: number; name: string }[] = [];
    flattenDocs(Array.isArray(tree) ? tree : [], out);
    documents.value = out;
  } catch (e) {
    logger.error('加载文档树失败:', e);
    ui.showToast(t('ws.k136'), 'error');
  } finally {
    docLoading.value = false;
  }
}

async function onSpaceChange() {
  selectedDocId.value = null;
  await loadDocuments();
  await Promise.all([loadTags(), loadMentionNotices()]);
}

async function onDocChange() {
  await Promise.all([loadDocApproval(), loadTags(), loadMentions(), loadTranslations()]);
}

async function loadUsers() {
  try {
    const res = await getUsers({ page: 1, page_size: 200 });
    users.value = res?.data ?? [];
  } catch (e) {
    logger.error('加载用户列表失败:', e);
  }
}

onMounted(async () => {
  await Promise.all([loadUsers(), loadLanguages(), loadMentionNotices(), loadPending(), loadSubmitted()]);
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const first = spaces.value[0];
    if (first) {
      selectedSpaceId.value = first.id;
      await loadDocuments();
    }
  } catch (e) {
    logger.error('加载空间列表失败:', e);
    ui.showToast(t('ws.k137'), 'error');
  }
});
</script>
