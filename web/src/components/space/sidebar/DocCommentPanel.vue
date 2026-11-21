<template>
  <div :style="panelStyle">
    <div :style="headerStyle" @click="collapsed = !collapsed">
      <div :style="headerLeftStyle">
        <ApIcon name="MessageSquare" :size="14" :style="{ color: cv('text-tertiary') }" />
        <span :style="monitorHealthMetricValueStyle">{{ $t('spaceUi.k69') }}</span>
        <span v-if="totalCount > 0" :style="countBadgeStyle">{{ totalCount }}</span>
      </div>
      <div :style="headerRightStyle">
        <ApButton v-if="closable" variant="ghost" size="mini" icon-only :tooltip="$t('spaceUi.close')" :aria-label="$t('spaceUi.close')" @click.stop="$emit('close')">
          <ApIcon name="X" :size="16" />
        </ApButton>
        <ApButton variant="ghost" size="mini" icon-only tabindex="-1" :aria-label="$t('spaceUi.k76')">
          <ApIcon name="ChevronRight" :size="16" :style="chevronStyle" />
        </ApButton>
      </div>
    </div>

    <div v-show="!collapsed" :style="bodyStyle">
      <!-- Filter bar：三颗以前全是 variant="pill"，也就是**三颗都实心蓝**，
           根本看不出当前筛的是哪一档；并回 ApSegmentedControl 后选中态与 aria-selected 一起回来 -->
      <div :style="filterBarStyle">
        <ApSegmentedControl
          :model-value="activeFilter"
          :options="filterOptions"
          size="small"
          :aria-label="$t('spaceUi.k77')"
          @update:model-value="setFilter"
        />
      </div>

      <div :style="scrollableBodyStyle">
        <div v-if="loading" :style="loadingStyle">
          <ApSkeleton variant="text" :lines="2" />
        </div>

        <template v-else-if="filteredComments.length > 0">
          <div
            v-for="comment in filteredComments"
            :key="comment.id"
            :style="commentThreadStyle(comment.resolved)"
          >
            <!-- Root comment -->
            <div :style="commentItemStyle">
              <div :style="commentHeaderStyle">
                <div :style="commentAuthorStyle">
                  <div :style="avatarStyle">{{ (comment.creator_name || '?').charAt(0).toUpperCase() }}</div>
                  <span :style="authorNameStyle">{{ comment.creator_name || `用户#${comment.creator_id}` }}</span>
                </div>
                <div :style="commentActionsStyle">
                  <span :style="timeStyle">{{ formatTime(comment.created_at) }}</span>
                  <ApButton
                    v-if="comment.creator_id === currentUserId || canEdit"
                    variant="ghost"
                    size="mini"
                    icon-only
                    :style="resolveBtnStyle(comment.resolved)"
                    :tooltip="comment.resolved ? '重新打开' : '标记为已解决'"
                    :aria-label="comment.resolved ? '重新打开' : '标记为已解决'" @click="toggleResolve(comment)">
                    <ApIcon v-if="comment.resolved" name="CheckCircle" :size="16" />
                    <ApIcon v-else name="Circle" :size="16" />
                  </ApButton>
                  <ApButton
                    v-if="comment.creator_id === currentUserId"
                    variant="ghost"
                    size="mini"
                    icon-only
                    :style="deleteBtnStyle"
                    :tooltip="$t('spaceUi.k75')"
                    :aria-label="$t('spaceUi.k75')" @click="onDeleteClick(comment.id)">
                    <ApIcon name="Trash2" :size="16" />
                  </ApButton>
                </div>
              </div>
              <p v-if="comment.selection_text" :style="quoteStyle">
                "{{ comment.selection_text }}"
              </p>
              <p :style="contentStyle">{{ comment.content }}</p>
              <div :style="replyBarStyle">
                <ApButton variant="ghost" size="small" @click="toggleReplyInput(comment.id)">
                  <ApIcon name="MessageSquare" :size="16" />
                  回复 {{ comment.replies?.length || '' }}
                </ApButton>
              </div>
            </div>

            <!-- Replies -->
            <div v-if="comment.replies && comment.replies.length > 0" :style="repliesContainerStyle">
              <div
                v-for="reply in comment.replies"
                :key="reply.id"
                :style="replyItemStyle"
              >
                <div :style="commentHeaderStyle">
                  <div :style="commentAuthorStyle">
                    <div :style="replyAvatarStyle">{{ (reply.creator_name || '?').charAt(0).toUpperCase() }}</div>
                    <span :style="authorNameStyle">{{ reply.creator_name || `用户#${reply.creator_id}` }}</span>
                  </div>
                  <div :style="commentActionsStyle">
                    <span :style="timeStyle">{{ formatTime(reply.created_at) }}</span>
                    <ApButton
                      v-if="reply.creator_id === currentUserId"
                      variant="ghost"
                      size="mini"
                      icon-only
                      :aria-label="$t('spaceUi.k78')"
                      :style="deleteBtnStyle"
                      @click="onDeleteClick(reply.id)"
                    >
                      <ApIcon name="Trash2" :size="16" />
                    </ApButton>
                  </div>
                </div>
                <p :style="contentStyle">{{ reply.content }}</p>
              </div>
            </div>

            <!-- Reply input -->
            <div v-if="replyingTo === comment.id" :style="replyInputContainerStyle">
              <ApTextarea
                v-model="replyContent"
                :rows="2"
                :placeholder="$t('spaceUi.k73')"
                @keydown.enter.meta="submitReply(comment.id)"
                @keydown.enter.ctrl="submitReply(comment.id)"
              />
              <div :style="replyActionsStyle">
                <ApButton variant="ghost" size="mini" @click="cancelReply">{{ $t('spaceUi.cancel') }}</ApButton>
                <ApButton variant="primary" size="mini" :disabled="!replyContent.trim() || submitting" :loading="submitting" @click="submitReply(comment.id)">{{ $t('spaceUi.k70') }}</ApButton>
              </div>
            </div>

            <div v-if="comment !== filteredComments[filteredComments.length - 1]" :style="dividerStyle" />
          </div>
        </template>

        <ApEmpty
          v-else
          icon="MessageSquare"
          :title="activeFilter === 'all' ? '暂无评论' : '无匹配评论'"
          :style="emptyStyle"
        />
      </div>

      <!-- New comment input -->
      <div :style="inputAreaStyle">
        <ApTextarea
          v-model="inputContent"
          :rows="2"
          :placeholder="$t('spaceUi.k74')"
          @keydown.enter.meta="handleSubmit"
          @keydown.enter.ctrl="handleSubmit"
        />
        <div :style="inputBarStyle">
          <span :style="hintStyle">{{ $t('spaceUi.k71') }}</span>
          <ApButton
            variant="primary"
            size="mini"
            :disabled="!inputContent.trim() || submitting"
            :loading="submitting"
            @click="handleSubmit"
          >
            {{ $t('spaceUi.k72') }}
          </ApButton>
        </div>
      </div>
    </div>

    <ApConfirmModal
      v-model="showDeleteConfirm"
      :title="$t('spaceUi.k75')"
      :message="$t('cExtraUi.k11')"
      :confirm-text="$t('spaceUi.delete')"
      type="danger"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, computed, watch } from 'vue';
import { AppleFont, AppleSpace, AppleType, AppleRadius, AppleFontWeight, AppleTransition, cv, AppleLineHeight } from '@/utils/core/apple-tokens';
import { loadingStyle, emptyStateStyle as emptyStyle, monitorHealthMetricValueStyle } from '@/styles/layout/page-styles';
import { formatTime } from '@/utils/format/format-time';
import { createComment, getComments, deleteComment, resolveComment } from '@/api/modules/social/comment';
import { useAuthStore } from '@/stores/auth/auth';
import { useUIStore } from '@/stores/app/ui';
import ApButton from '@/components/apple/ApButton.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import type { CommentItem } from '@/types/models';

const props = withDefaults(defineProps<{
  documentId: number | null;
  canEdit?: boolean;
  closable?: boolean;
}>(), {
  canEdit: false,
  closable: false,
});

const emit = defineEmits<{
  (e: 'commentAdded'): void;
  (e: 'close'): void;
}>();
const { t } = useI18n();

const authStore = useAuthStore();
const ui = useUIStore();

const collapsed = ref(!props.closable);
const comments = ref<CommentItem[]>([]);
const loading = ref(false);
const submitting = ref(false);
const inputContent = ref('');
const showDeleteConfirm = ref(false);
const commentToDelete = ref<number | null>(null);
const activeFilter = ref<'all' | 'open' | 'resolved'>('all');
const filterOptions = [
  { label: t('spaceUi.all'), value: 'all' },
  { label: t('spaceUi.k80'), value: 'open' },
  { label: t('spaceUi.k81'), value: 'resolved' },
];
/** 分段控件往外报的是 string|number，这里收窄回那三档 */
function setFilter(v: string | number) {
  activeFilter.value = v === 'open' || v === 'resolved' ? v : 'all';
}
const replyingTo = ref<number | null>(null);
const replyContent = ref('');

const currentUserId = computed(() => authStore.user?.id ?? 0);

const totalCount = computed(() => {
  let count = 0;
  for (const c of comments.value) {
    count += 1;
    if (c.replies) count += c.replies.length;
  }
  return count;
});

const filteredComments = computed(() => {
  if (activeFilter.value === 'all') return comments.value;
  return comments.value.filter(c => {
    if (activeFilter.value === 'resolved') return c.resolved === 1;
    return c.resolved === 0;
  });
});

async function loadComments() {
  if (!props.documentId) return;
  loading.value = true;
  try {
    comments.value = await getComments(props.documentId);
  } catch {
    ui.showToast(t('spaceUi.k82'), 'error');
  } finally {
    loading.value = false;
  }
}

async function handleSubmit() {
  const content = inputContent.value.trim();
  if (!content || !props.documentId || submitting.value) return;
  submitting.value = true;
  try {
    await createComment(props.documentId, content);
    inputContent.value = '';
    await loadComments();
    emit('commentAdded');
  } catch {
    ui.showToast(t('spaceUi.k83'), 'error');
  } finally {
    submitting.value = false;
  }
}

function toggleReplyInput(commentId: number) {
  if (replyingTo.value === commentId) {
    replyingTo.value = null;
    replyContent.value = '';
  } else {
    replyingTo.value = commentId;
    replyContent.value = '';
  }
}

function cancelReply() {
  replyingTo.value = null;
  replyContent.value = '';
}

async function submitReply(parentId: number) {
  const content = replyContent.value.trim();
  if (!content || !props.documentId || submitting.value) return;
  submitting.value = true;
  try {
    await createComment(props.documentId, content, undefined, undefined, parentId);
    replyContent.value = '';
    replyingTo.value = null;
    await loadComments();
    emit('commentAdded');
  } catch {
    ui.showToast(t('spaceUi.k84'), 'error');
  } finally {
    submitting.value = false;
  }
}

async function toggleResolve(comment: CommentItem) {
  try {
    await resolveComment(comment.id, comment.resolved !== 1);
    await loadComments();
  } catch {
    ui.showToast(t('spaceUi.k85'), 'error');
  }
}

function onDeleteClick(id: number) {
  commentToDelete.value = id;
  showDeleteConfirm.value = true;
}

async function confirmDelete() {
  if (commentToDelete.value === null) return;
  try {
    await deleteComment(commentToDelete.value);
    await loadComments();
    ui.showToast(t('spaceUi.k86'), 'success');
  } catch {
    ui.showToast(t('spaceUi.k87'), 'error');
  } finally {
    commentToDelete.value = null;
  }
}

defineExpose({ loadComments });

watch(() => props.documentId, (newId) => {
  if (newId) {
    comments.value = [];
    inputContent.value = '';
    loadComments();
  } else {
    comments.value = [];
    if (!props.closable) collapsed.value = true;
  }
}, { immediate: true });

const panelStyle = {
  borderTop: `1px solid ${cv('border-light')}`,
  padding: `0 ${AppleSpace.sm}`,
};

const headerStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.xs} 0`,
  cursor: 'pointer',
  userSelect: 'none' as const,
};

const headerLeftStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const headerRightStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xxs,
};

// titleStyle: reuse monitorHealthMetricValueStyle (identical semantics, different domain naming)

const countBadgeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  background: cv('bg-tertiary'),
  borderRadius: AppleRadius.pill,
  padding: '0 5px',
  lineHeight: '16px',
  minWidth: '16px',
  textAlign: 'center' as const,
};

const chevronStyle = computed(() => ({
  color: cv('text-tertiary'),
  transition: AppleTransition.fast,
  transform: collapsed.value ? 'rotate(0deg)' : 'rotate(90deg)',
}));

const bodyStyle = computed(() => ({
  display: 'flex',
  flexDirection: 'column' as const,
  maxHeight: props.closable ? 'none' : '400px',
  overflow: 'hidden' as const,
  flex: props.closable ? '1' : undefined,
  minHeight: props.closable ? '0' : undefined,
}));

const filterBarStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.xs,
};

const scrollableBodyStyle = {
  flex: '1',
  minHeight: '0',
  overflowY: 'auto' as const,
};

function commentThreadStyle(resolved: number) {
  return {
    opacity: resolved ? '0.7' : '1',
    transition: AppleTransition.fast,
  };
}

const commentItemStyle = {
  padding: `${AppleSpace.xxs} 0`,
};

const commentHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.xxs,
};

const commentAuthorStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace['6px'],
};

const avatarStyle = {
  width: '20px',
  height: '20px',
  borderRadius: AppleRadius.circle,
  background: cv('blue'),
  color: cv('on-blue'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  flexShrink: 0,
};

const replyAvatarStyle = {
  width: '16px',
  height: '16px',
  borderRadius: AppleRadius.circle,
  background: cv('purple'),
  color: cv('on-accent'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  flexShrink: 0,
};

const authorNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const commentActionsStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const timeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};

const deleteBtnStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: AppleSpace.xxs,
  borderRadius: AppleRadius.micro,
  cursor: 'pointer',
  transition: AppleTransition.fast,
};

function resolveBtnStyle(resolved: number) {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: AppleSpace.xxs,
    borderRadius: AppleRadius.micro,
    cursor: 'pointer',
    color: resolved ? cv('green') : cv('text-tertiary'),
    transition: AppleTransition.fast,
  };
}

const quoteStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  background: cv('bg-tertiary'),
  borderRadius: AppleRadius.small,
  padding: `${AppleSpace.xs} ${AppleSpace.xs}`,
  margin: `0 0 ${AppleSpace.xxs} 0`,
  borderLeft: `2px solid ${cv('border')}`,
  lineHeight: AppleLineHeight.default,
  whiteSpace: 'pre-wrap' as const,
  wordBreak: 'break-word' as const,
};

const contentStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-primary'),
  lineHeight: AppleLineHeight.loose,
  margin: '0',
  whiteSpace: 'pre-wrap' as const,
  wordBreak: 'break-word' as const,
};

const replyBarStyle = {
  marginTop: AppleSpace.xxs,
};

const repliesContainerStyle = {
  paddingLeft: `${AppleSpace.md}`,
  borderLeft: `2px solid ${cv('border-light')}`,
  marginLeft: AppleSpace.sm,
};

const replyItemStyle = {
  padding: `${AppleSpace.xxs} 0`,
};

const replyInputContainerStyle = {
  marginLeft: AppleSpace.sm,
  marginTop: AppleSpace.xxs,
  padding: AppleSpace.xs,
  background: cv('bg-secondary'),
  borderRadius: AppleRadius.small,
};

const replyActionsStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'flex-end',
  gap: AppleSpace.xs,
  marginTop: AppleSpace.xs,
};

const dividerStyle = {
  height: '1px',
  background: cv('border-light'),
  marginTop: AppleSpace.xs,
};

const inputAreaStyle = {
  marginTop: AppleSpace.xs,
  borderTop: `1px solid ${cv('border-light')}`,
  padding: `${AppleSpace.xs} 0`,
  flexShrink: 0,
};

const inputBarStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginTop: AppleSpace.xs,
};

const hintStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};
</script>
