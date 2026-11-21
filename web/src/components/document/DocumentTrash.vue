<template>
  <ApModal v-model="showModal" :title="$t('docUi1.k18')" size="large">
    <div v-if="loading" :style="emptyStyle">
      <span :style="{ color: cv('text-secondary') }">{{ $t('docUi1.k13') }}</span>
    </div>
    <div v-else-if="items.length === 0" :style="emptyStyle">
      <span :style="{ color: cv('text-secondary') }">{{ $t('docUi1.k14') }}</span>
    </div>
    <div v-else :style="listStyle">
      <div
        v-for="item in items"
        :key="item.document_id"
        :style="rowStyle"
      >
        <div :style="infoStyle">
          <span :style="nameStyle">{{ item.name }}</span>
          <span :style="badgeStyle(item.type)">{{ typeLabel(item.type) }}</span>
          <span :style="timeStyle">{{ formatDeletedAt(item.deleted_at) }}</span>
        </div>
        <div :style="actionsStyle">
          <ApButton variant="ghost" size="small" :loading="restoringId === item.document_id" @click="onRestore(item.document_id)">
            {{ $t('docUi1.k15') }}
          </ApButton>
          <ApButton variant="danger" size="small" :loading="deletingId === item.document_id" @click="onPermanentDelete(item.document_id, item.name)">
            {{ $t('docUi1.k16') }}
          </ApButton>
        </div>
      </div>
    </div>
    <template #footer>
      <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('docUi1.close') }}</ApButton>
    </template>

    <ApConfirmModal
      v-model="showPermanentConfirm"
      :title="$t('docUi1.k16')"
      :message="`确定彻底删除「${pendingDeleteName}」吗？此操作不可恢复。`"
      type="danger"
      :confirm-text="$t('docUi1.k16')"
      :require-input-confirm="true"
      :confirm-match-text="pendingDeleteName"
      :confirm-placeholder="`请输入「${pendingDeleteName}」确认彻底删除`"
      @confirm="confirmPermanentDelete"
    />
  </ApModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, shallowRef, watch } from 'vue';
import { AppleFont, AppleRadius, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import { formatDateTime } from '@/utils/format/format-time';
import { getSpaceTrash, restoreDocument, permanentDeleteDocument, type TrashItem } from '@/api/modules/document/document';
import { useUIStore } from '@/stores/app/ui';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';

const props = defineProps<{
  modelValue: boolean;
  spaceId: number;
}>();
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
  (e: 'restored', documentId: number): void;
  (e: 'permanent-deleted', documentId: number): void;
}>();
const { t } = useI18n();

const ui = useUIStore();
const showModal = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val),
});

const items = shallowRef<TrashItem[]>([]);
const loading = shallowRef(false);
const restoringId = shallowRef<number | null>(null);
const deletingId = shallowRef<number | null>(null);
const showPermanentConfirm = shallowRef(false);
const pendingDeleteId = shallowRef<number | null>(null);
const pendingDeleteName = shallowRef('');

async function loadTrash() {
  if (!props.spaceId) return;
  loading.value = true;
  try {
    items.value = await getSpaceTrash(props.spaceId);
  } catch {
    ui.showToast(t('docUi1.k19'), 'error');
    items.value = [];
  } finally {
    loading.value = false;
  }
}

watch(() => props.modelValue, (val) => {
  if (val) loadTrash();
});

function typeLabel(type: number) {
  return type === 2 ? '目录' : '文档';
}

function badgeStyle(type: number) {
  return {
    display: 'inline-block',
    padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.small,
    background: type === 2 ? cv('pale-apple-gray') : cv('blue-light'),
    color: type === 2 ? cv('text-secondary') : cv('blue'),
    fontSize: AppleType.microUI.size,
    fontFamily: AppleFont.text,
    marginLeft: AppleSpace.sm,
  };
}

function formatDeletedAt(ts: number) {
  if (!ts) return '-';
  return formatDateTime(ts * 1000, 'YYYY-MM-DD HH:mm:ss');
}

async function onRestore(documentId: number) {
  restoringId.value = documentId;
  try {
    await restoreDocument(documentId);
    ui.showToast(t('docUi1.k20'), 'success');
    items.value = items.value.filter(i => i.document_id !== documentId);
    emit('restored', documentId);
  } catch {
    ui.showToast(t('docUi1.k21'), 'error');
  } finally {
    restoringId.value = null;
  }
}

function onPermanentDelete(documentId: number, name: string) {
  pendingDeleteId.value = documentId;
  pendingDeleteName.value = name;
  showPermanentConfirm.value = true;
}

async function confirmPermanentDelete() {
  if (!pendingDeleteId.value) return;
  deletingId.value = pendingDeleteId.value;
  showPermanentConfirm.value = false;
  try {
    await permanentDeleteDocument(pendingDeleteId.value);
    ui.showToast(t('docUi1.k22'), 'success');
    items.value = items.value.filter(i => i.document_id !== pendingDeleteId.value);
    emit('permanent-deleted', pendingDeleteId.value);
  } catch {
    ui.showToast(t('docUi1.k23'), 'error');
  } finally {
    deletingId.value = null;
    pendingDeleteId.value = null;
    pendingDeleteName.value = '';
  }
}

const emptyStyle = {
  padding: `${AppleSpace.xl} 0`,
  textAlign: 'center' as const,
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
};

const listStyle = {
  maxHeight: '60vh',
  overflowY: 'auto' as const,
};

const rowStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.md} ${AppleSpace.md}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  gap: AppleSpace.md,
};

const infoStyle = {
  display: 'flex',
  alignItems: 'center',
  flex: 1,
  minWidth: 0,
};

const nameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
  maxWidth: '280px',
};

const timeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginLeft: AppleSpace.md,
};

const actionsStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexShrink: 0,
};
</script>
