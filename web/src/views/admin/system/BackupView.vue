<template>
  <ApPageContainer
    :title="$t('adminSys.k38')"
    :subtitle="$t('adminSys.k41')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApButton variant="ghost" size="small" :loading="loading" @click="loadBackups">
        <ApIcon name="RefreshCw" :size="16" />{{ $t('adminSys.refresh') }}
      </ApButton>
      <ApButton
        variant="ghost"
        size="small"
        :loading="importing"
        :disabled="isBusy()"
        @click="fileInput?.click()"
      >
        <ApIcon name="Upload" :size="16" />{{ importing ? $t('adminSys.importing', { n: importPercent }) : $t('adminSys.import') }}
      </ApButton>
      <ApButton variant="primary" size="small" :loading="creating" @click="handleCreate">
        <ApIcon name="Plus" :size="16" />{{ $t('adminSys.k31') }}
      </ApButton>
      <input
        ref="fileInput"
        type="file"
        accept=".zip"
        :aria-label="$t('adminSys.import')"
        :style="hiddenInputStyle"
        @change="handleImportFile"
      >
    </template>

    <ApCard size="small">
      <template #header>
        <div :style="statusRowStyle">
          <ApIcon name="Database" :size="18" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('adminSys.k32') }}</span>
        </div>
      </template>

      <div :style="toolbarStyle">
        <span :style="totalStyle">{{ $t('adminSys.k381', { n: backups.length }) }}</span>
      </div>

      <!-- P1-10：覆盖式恢复是不可逆操作，警示必须与"共 N 个备份"这种普通说明分开成条 -->
      <div v-if="backups.length" :style="restoreWarningStyle">
        <ApIcon name="AlertTriangle" :size="16" :color="cv('orange')" />
        <span>{{ $t('adminSys.k33') }}</span>
      </div>

      <ApTable density="compact" row-key="id" :columns="columns" :data="backups" :loading="loading">
        <template #empty>{{ $t('adminSys.k34') }}</template>
        <template #actions="{ row }">
          <ApButton
            variant="ghost"
            size="small"
            :loading="busyId === (row as unknown as BackupInfo).id"
            @click="handleDownload(row as unknown as BackupInfo)"
          >
            <ApIcon name="Download" :size="16" />{{ $t('adminSys.download') }}
          </ApButton>
          <ApButton variant="outline" size="small" @click="askRestore(row as unknown as BackupInfo)">
            <ApIcon name="RotateCcw" :size="16" />{{ $t('adminSys.k36') }}
          </ApButton>
          <ApButton variant="danger" size="small" @click="askDelete(row as unknown as BackupInfo)">
            <ApIcon name="Trash2" :size="16" />{{ $t('adminSys.delete') }}
          </ApButton>
        </template>
      </ApTable>
    </ApCard>

    <ApConfirmModal
      v-model="showRestoreModal"
      :title="$t('adminSys.k39')"
      :message="restoreMessage"
      type="danger"
      :confirm-text="$t('adminSys.k36')"
      :confirm-placeholder="$t('lastAttrsUi.k2')"
      require-input-confirm
      @confirm="confirmRestore"
    />

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('adminSys.k40')"
      :message="deleteMessage"
      type="danger"
      :confirm-text="$t('adminSys.delete')"
      @confirm="confirmDelete"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref, shallowRef } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';

import { logger } from '@/utils/logger';
import { AppleLayout, AppleSpace, cv } from '@/utils/core/apple-tokens';
import { sectionTitleStyle, statusRowStyle, toolbarStyle, totalStyle, warningBannerStyle } from '@/styles/layout/page-styles';
import {
  backupDownloadUrl,
  createBackup,
  deleteBackup,
  importBackup,
  listBackups,
  restoreBackup,
  type BackupInfo,
} from '@/api/modules/system/backup';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();

const backups = ref<BackupInfo[]>([]);
const loading = shallowRef(false);
const creating = shallowRef(false);
const fileInput = ref<HTMLInputElement | null>(null);
const hiddenInputStyle = { display: 'none' } as const;
const importing = shallowRef(false);
const importPercent = shallowRef(0);
/** 行内正在进行的操作所属备份 ID，用于单行 loading 与并发守卫 */
const busyId = shallowRef<string | null>(null);

const showRestoreModal = shallowRef(false);
const showDeleteModal = shallowRef(false);
const restoring = shallowRef(false);
const deleting = shallowRef(false);
const target = ref<BackupInfo | null>(null);

const columns = [
  { key: 'file_name', title: t('adminSys.k42'), ellipsis: true },
  { key: 'size_mb', title: t('adminSys.k43') },
  { key: 'create_at', title: t('adminSys.k28') },
];

const restoreMessage = computed(() =>
  t('uiText.k51', { p1: target.value?.file_name ?? '', p2: t('adminSys.k36') }),
);

const deleteMessage = computed(() =>
  t('uiText.k52', { p1: target.value?.file_name ?? '' }),
);

// 警示条：与普通说明区分开（琥珀底 + 描边），只补本条与表格之间的间距
const restoreWarningStyle = { ...warningBannerStyle, marginBottom: AppleSpace.md };

/** 下载 / 导入 / 恢复 / 删除互斥：同一时刻只允许一个针对备份的写操作在飞 */
function isBusy() {
  return creating.value || importing.value || busyId.value !== null || restoring.value || deleting.value;
}

async function loadBackups() {
  loading.value = true;
  try {
    const res = await listBackups();
    // 后端无备份时返回 null，必须兜底成空数组
    backups.value = res.backups ?? [];
  } catch (e: unknown) {
    logger.error('Failed to load backups:', e);
    ui.showToast((e as Error).message || t('fallback.k27'), 'error');
  } finally {
    loading.value = false;
  }
}

async function handleCreate() {
  if (isBusy()) return;
  creating.value = true;
  try {
    const info = await createBackup();
    ui.showToast(t('sentUi.k19', { p1: info?.size_mb ?? '' }).trim(), 'success');
    await loadBackups();
  } catch (e: unknown) {
    logger.error('Failed to create backup:', e);
    ui.showToast((e as Error).message || t('fallback.k14'), 'error');
  } finally {
    creating.value = false;
  }
}

async function handleDownload(row: BackupInfo) {
  if (busyId.value !== null) return;
  busyId.value = row.id;
  try {
    // 大包交给浏览器原生下载：流式落盘、不进 JS 内存、不受 axios 30s 超时约束
    const url = await backupDownloadUrl(row.id);
    const a = document.createElement('a');
    a.href = url;
    a.download = row.file_name || `${row.id}.zip`;
    a.rel = 'noopener';
    a.click();
    ui.showToast(t('adminSys.k44'), 'success');
  } catch (e: unknown) {
    logger.error('Failed to download backup:', e);
    ui.showToast((e as Error).message || t('fallback.k6'), 'error');
  } finally {
    busyId.value = null;
  }
}

async function handleImportFile(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  // 允许重选同一个文件：清空 value，否则 change 不再触发
  input.value = '';
  if (!file || importing.value) return;
  importing.value = true;
  importPercent.value = 0;
  try {
    await importBackup(file, (p) => { importPercent.value = p; });
    ui.showToast(t('adminSys.imported'), 'success');
    await loadBackups();
  } catch (err: unknown) {
    logger.error('Failed to import backup:', err);
    ui.showToast((err as Error).message || t('fallback.k14'), 'error');
  } finally {
    importing.value = false;
    importPercent.value = 0;
  }
}

function askRestore(row: BackupInfo) {
  target.value = row;
  showRestoreModal.value = true;
}

function askDelete(row: BackupInfo) {
  target.value = row;
  showDeleteModal.value = true;
}

async function confirmRestore() {
  const row = target.value;
  if (!row || restoring.value) return;
  restoring.value = true;
  showRestoreModal.value = false;
  try {
    // 后端校验 query confirm=1，restoreBackup 内部已带上
    await restoreBackup(row.id);
    ui.showToast(t('adminSys.k45'), 'success');
  } catch (e: unknown) {
    logger.error('Failed to restore backup:', e);
    ui.showToast((e as Error).message || t('fallback.k58'), 'error');
  } finally {
    restoring.value = false;
    target.value = null;
  }
}

async function confirmDelete() {
  const row = target.value;
  if (!row || deleting.value) return;
  deleting.value = true;
  showDeleteModal.value = false;
  try {
    await deleteBackup(row.id);
    ui.showToast(t('adminSys.k46'), 'success');
    await loadBackups();
  } catch (e: unknown) {
    logger.error('Failed to delete backup:', e);
    ui.showToast((e as Error).message || t('fallback.k19'), 'error');
  } finally {
    deleting.value = false;
    target.value = null;
  }
}

onMounted(() => loadBackups());
</script>
