<template>
  <div :style="containerStyle">
    <div v-if="loading" :style="loadingStyle">{{ $t('docUi1.k13') }}</div>
    <ApEmpty v-else-if="versions.length === 0" icon="FileText" :title="$t('docUi1.k104')" />
    <template v-else>
      <div v-if="compareSelectMode" :style="compareSelectBarStyle">
        <span :style="{ fontSize: AppleType.microUI.size, color: cv('text-secondary') }">
          已选 {{ compareSelectedIds.length }}/2 个版本进行对比
        </span>
        <ApButton
          variant="primary"
          size="small"
          :disabled="compareSelectedIds.length < 2"
          :tooltip="compareSelectedIds.length < 2 ? '需勾选 2 个版本才能对比' : ''"
          @click="handleCompareTwoVersions"
        >
          {{ $t('docUi1.k91') }}
        </ApButton>
        <ApButton variant="ghost" size="small" @click="exitCompareSelectMode">
          {{ $t('docUi1.cancel') }}
        </ApButton>
      </div>
      <div :style="listStyle">
        <div
          v-for="v in paginatedVersions"
          :key="v.id"
          :style="versionItemStyle(v.id === selectedVersionId)"
          @click="handleVersionClick(v)"
          @mouseenter="hoverId = v.id"
          @mouseleave="hoverId = null"
        >
          <div :style="versionHeaderStyle">
            <label v-if="compareSelectMode" :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm, cursor: 'pointer' }" @click.stop>
              <ApCheckbox :model-value="compareSelectedIds.includes(v.id)" @update:model-value="toggleCompareSelect(v.id)" />
              <span :style="versionNumberStyle">版本 {{ v.version }}</span>
            </label>
            <span v-else :style="versionNumberStyle">版本 {{ v.version }}</span>
            <span :style="versionTimeStyle">{{ formatTime(v.created_at) }}</span>
          </div>
          <div :style="versionMetaStyle">
            <span>{{ v.creator_name || `用户#${v.created_by}` }}</span>
            <span v-if="v.change_log"> · {{ v.change_log }}</span>
          </div>
          <div v-if="v.id === selectedVersionId && !compareSelectMode" :style="actionsStyle">
            <ApButton variant="ghost" size="small" @click="handlePreviewVersion(v)">
              <ApIcon name="Eye" :size="16" />
              {{ $t('docUi1.k92') }}
            </ApButton>
            <ApButton variant="ghost" size="small" @click="handleCompareVersion(v)">
              <ApIcon name="Diff" :size="16" />
              {{ $t('docUi1.k93') }}
            </ApButton>
            <ApButton variant="ghost" size="small" @click="enterCompareSelectMode">
              <ApIcon name="GitCompare" :size="16" />
              {{ $t('docUi1.k94') }}
            </ApButton>
            <ApButton variant="primary" size="small" @click="handleRollback(v.version)">
              {{ $t('docUi1.k95') }}
            </ApButton>
          </div>
        </div>
      </div>

      <div :style="paginationControlStyle">
        <span :style="pageInfoStyle">
          共 {{ versions.length }} 条，第 {{ currentPage }}/{{ totalPages }} 页
        </span>
        <div :style="pageButtonsStyle">
          <ApButton variant="ghost" size="small" :disabled="currentPage <= 1" @click="currentPage--">
            {{ $t('docUi1.prev') }}
          </ApButton>
          <ApButton variant="ghost" size="small" :disabled="currentPage >= totalPages" @click="currentPage++">
            {{ $t('docUi1.next') }}
          </ApButton>
        </div>
      </div>
    </template>

    <ApModal v-model="showPreview" :title="$t('docUi1.k105')" size="medium">
      <div v-if="previewLoading" :style="loadingStyle">{{ $t('docUi1.k13') }}</div>
      <div v-else :style="previewContentStyle">
        <ApMarkdownPreview :content="previewContent" />
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showPreview = false">{{ $t('docUi1.close') }}</ApButton>
      </template>
    </ApModal>

    <ApModal v-model="showCompare" :title="$t('docUi1.k106')" size="xlarge">
      <div v-if="compareLoading" :style="loadingStyle">{{ $t('docUi1.k13') }}</div>
      <div v-else-if="!compareHasChanges" :style="noDiffStyle">
        <span :style="noDiffTextStyle">{{ $t('docUi1.k98') }}</span>
      </div>
      <div v-else :style="monitorInfoGridStyle">
        <div :style="diffToolbarStyle">
          <ApButton
            variant="ghost"
            size="small"
            :style="diffTabStyle(diffMode === 'editor')"
            @click="diffMode = 'editor'"
          >{{ $t('docUi1.k99') }}</ApButton>
          <ApButton
            variant="ghost"
            size="small"
            :style="diffTabStyle(diffMode === 'inline')"
            @click="diffMode = 'inline'"
          >{{ $t('docUi1.k100') }}</ApButton>
          <span :style="diffStatsStyle">
            <span :style="{ color: cv('green') }">+{{ inlineDiff?.added_count ?? 0 }}</span>
            <span :style="{ color: cv('red') }">−{{ inlineDiff?.removed_count ?? 0 }}</span>
          </span>
        </div>
        <div v-if="diffMode === 'editor'" :style="diffEditorContainerStyle">
          <MonacoDiffEditor
            :original="compareOriginal"
            :modified="compareModified"
            language="markdown"
          />
        </div>
        <div v-else :style="inlineDiffContainerStyle">
          <template v-if="inlineDiffLoading">
            <div :style="loadingStyle">{{ $t('docUi1.k101') }}</div>
          </template>
          <template v-else-if="inlineDiff">
            <div
              v-for="(block, idx) in inlineDiff.diff_blocks"
              :key="idx"
              :style="inlineBlockStyle(block.type)"
            >
              <span
                v-for="(line, li) in block.lines"
                :key="li"
                :style="inlineLineStyle(block.type)"
              >{{ line || ' ' }}</span>
            </div>
          </template>
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showCompare = false">{{ $t('docUi1.close') }}</ApButton>
        <ApButton v-if="rollbackTargetVersion !== null && compareHasChanges" variant="danger" size="small" @click="handleConfirmRollbackFromCompare">
          {{ $t('docUi1.k95') }}
        </ApButton>
      </template>
    </ApModal>

    <ApModal v-model="showRollbackConfirm" :title="$t('docUi1.k107')" size="small">
      <p :style="noDiffTextStyle">{{ $t('docUi1.k102') }}</p>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showRollbackConfirm = false">{{ $t('docUi1.cancel') }}</ApButton>
        <ApButton variant="danger" size="small" @click="confirmRollback">{{ $t('docUi1.k103') }}</ApButton>
      </template>
    </ApModal>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted, watch } from 'vue';
import { AppleFont, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleLineHeight, AppleFontWeight } from '@/utils/core/apple-tokens';
import { loadingStyle, monitorInfoGridStyle } from '@/styles/layout/page-styles';
import { getDocumentVersions, getVersionContent, getDocumentContent, diffDocumentVersions, type DiffResult } from '@/api/modules/document/document';
import type { DocumentVersion } from '@/types/models';
import ApButton from '@/components/apple/ApButton.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApCheckbox from '@/components/apple/ApCheckbox.vue';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import MonacoDiffEditor from './MonacoDiffEditor.vue';
import { formatTime } from '@/utils/format/format-time';
import { useUIStore } from '@/stores/app/ui';
import { useDocumentStore } from '@/stores/workspace/document';

const props = defineProps<{
  documentId: number;
}>();

const emit = defineEmits<{
  (e: 'rollback', version: number): void;
}>();
const { t } = useI18n();

const PAGE_SIZE = 10;

const ui = useUIStore();
const docStore = useDocumentStore();

const versions = ref<DocumentVersion[]>([]);
const loading = shallowRef(false);
const selectedVersionId = shallowRef<number | null>(null);
const hoverId = shallowRef<number | null>(null);
const showPreview = shallowRef(false);
const previewContent = shallowRef('');
const previewLoading = shallowRef(false);
const showCompare = shallowRef(false);
const compareLoading = shallowRef(false);
const compareOriginal = shallowRef('');
const compareModified = shallowRef('');
const compareHasChanges = shallowRef(false);
const diffMode = ref<'editor' | 'inline'>('editor');
const inlineDiff = shallowRef<DiffResult | null>(null);
const inlineDiffLoading = shallowRef(false);
const currentPage = shallowRef(1);
const totalCount = shallowRef(0);
const rollbackTargetVersion = shallowRef<number | null>(null);
const compareSelectMode = shallowRef(false);
const compareSelectedIds = ref<number[]>([]);
const showRollbackConfirm = shallowRef(false);
const pendingRollbackVersion = shallowRef<number | null>(null);

function enterCompareSelectMode() {
  compareSelectMode.value = true;
  compareSelectedIds.value = [];
}

function exitCompareSelectMode() {
  compareSelectMode.value = false;
  compareSelectedIds.value = [];
}

function handleVersionClick(v: DocumentVersion) {
  if (compareSelectMode.value) {
    toggleCompareSelect(v.id);
  } else {
    selectedVersionId.value = v.id;
  }
}

function toggleCompareSelect(id: number) {
  const idx = compareSelectedIds.value.indexOf(id);
  if (idx >= 0) {
    compareSelectedIds.value.splice(idx, 1);
  } else if (compareSelectedIds.value.length < 2) {
    compareSelectedIds.value.push(id);
  }
}

async function handleCompareTwoVersions() {
  if (compareSelectedIds.value.length < 2) return;
  const [id1, id2] = compareSelectedIds.value;
  const v1 = versions.value.find(v => v.id === id1);
  const v2 = versions.value.find(v => v.id === id2);
  if (!v1 || !v2) return;

  showCompare.value = true;
  compareLoading.value = true;
  compareOriginal.value = '';
  compareModified.value = '';
  inlineDiff.value = null;
  try {
    const [res1, res2] = await Promise.all([
      getVersionContent(props.documentId, v1.version),
      getVersionContent(props.documentId, v2.version),
    ]);
    compareOriginal.value = res1.content || '';
    compareModified.value = res2.content || '';
    compareHasChanges.value = compareOriginal.value !== compareModified.value;
  } catch {
    ui.showToast(t('docUi1.k108'), 'error');
  } finally {
    compareLoading.value = false;
  }
  exitCompareSelectMode();
}

const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / PAGE_SIZE)));

// 后端已按 currentPage 分页返回，前端无需再次切片
const paginatedVersions = computed(() => versions.value);

async function loadVersions() {
  loading.value = true;
  try {
    const res = await getDocumentVersions(props.documentId, currentPage.value, PAGE_SIZE);
    versions.value = res.data || [];
    totalCount.value = res.total || 0;
  } catch {
    ui.showToast(t('docUi1.k109'), 'error');
    versions.value = [];
    totalCount.value = 0;
  } finally {
    loading.value = false;
  }
}

onMounted(() => { loadVersions(); });

watch(() => props.documentId, () => { currentPage.value = 1; loadVersions(); });
watch(currentPage, () => { loadVersions(); });

async function handlePreviewVersion(v: DocumentVersion) {
  showPreview.value = true;
  previewLoading.value = true;
  previewContent.value = '';
  try {
    const res = await getVersionContent(props.documentId, v.version);
    previewContent.value = res.content || '';
  } catch {
    ui.showToast(t('docUi1.k110'), 'error');
  } finally {
    previewLoading.value = false;
  }
}

async function handleCompareVersion(v: DocumentVersion) {
  showCompare.value = true;
  compareLoading.value = true;
  compareOriginal.value = '';
  compareModified.value = '';
  inlineDiff.value = null;
  rollbackTargetVersion.value = v.version;
  try {
    // 优先复用 store 缓存的当前文档内容，避免重复请求
    const cachedContent = docStore.getCachedContent(props.documentId);
    const currentPromise = cachedContent
      ? Promise.resolve({ content: cachedContent })
      : getDocumentContent(props.documentId);
    const [currentRes, versionRes] = await Promise.all([
      currentPromise,
      getVersionContent(props.documentId, v.version),
    ]);
    compareOriginal.value = versionRes.content || '';
    compareModified.value = currentRes.content || '';
    compareHasChanges.value = compareOriginal.value !== compareModified.value;
  } catch {
    ui.showToast(t('docUi1.k108'), 'error');
  } finally {
    compareLoading.value = false;
  }
}

// 切换到行级差异视图时，从后端加载结构化 diff
watch(diffMode, async (mode) => {
  if (mode !== 'inline' || inlineDiff.value) return;
  inlineDiffLoading.value = true;
  try {
    const fromVer = rollbackTargetVersion.value ?? 0;
    inlineDiff.value = await diffDocumentVersions(props.documentId, fromVer, 0);
  } catch {
    ui.showToast(t('docUi1.k111'), 'error');
  } finally {
    inlineDiffLoading.value = false;
  }
});

// 从对比对话框中点击回滚按钮
function handleConfirmRollbackFromCompare() {
  if (rollbackTargetVersion.value !== null) {
    emit('rollback', rollbackTargetVersion.value);
    showCompare.value = false;
    rollbackTargetVersion.value = null;
  }
}

// 回滚到此版本：直接确认后回滚，覆盖当前内容
function handleRollback(version: number) {
  pendingRollbackVersion.value = version;
  showRollbackConfirm.value = true;
}

function confirmRollback() {
  if (pendingRollbackVersion.value !== null) {
    emit('rollback', pendingRollbackVersion.value);
    pendingRollbackVersion.value = null;
    showRollbackConfirm.value = false;
  }
}

const containerStyle = {
  minHeight: '200px' };

const listStyle = monitorInfoGridStyle;

function versionItemStyle(selected: boolean) {
  return {
    padding: `${AppleSpace.md} ${AppleSpace.md}`,
    borderRadius: AppleRadius.small,
    background: selected ? cv('blue-selection') : hoverId.value ? cv('bg-hover') : 'transparent',
    border: selected ? `1px solid ${cv('blue-selection-strong')}` : '1px solid transparent',
    cursor: 'pointer',
    transition: AppleTransition.default };
}

const versionHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.xs };

const versionNumberStyle = {
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.sectionTitle.size,
  letterSpacing: AppleType.sectionTitle.letterSpacing,
  fontWeight: String(AppleType.bodyEmphasis.weight),
  color: cv('text-primary') };

const versionTimeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary') };

const versionMetaStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  lineHeight: AppleLineHeight.default };

const actionsStyle = {
  marginTop: AppleSpace.sm,
  display: 'flex',
  gap: AppleSpace.sm };

const paginationControlStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginTop: AppleSpace.md,
  paddingTop: AppleSpace.md,
  borderTop: `1px solid ${cv('border-light')}` };

const pageInfoStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary') };

const pageButtonsStyle = {
  display: 'flex',
  gap: AppleSpace.xs };

const compareSelectBarStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  background: cv('blue-selection'),
  borderRadius: AppleRadius.small,
  marginBottom: AppleSpace.sm };

const diffEditorContainerStyle = {
  height: '500px',
  border: `1px solid ${cv('border')}`,
  borderRadius: AppleRadius.small };

// diffContainerStyle: reuse monitorInfoGridStyle (identical)

const diffToolbarStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  padding: `${AppleSpace.xs} 0` };

function diffTabStyle(active: boolean) {
  return {
    padding: `${AppleSpace.xs} ${AppleSpace.md}`,
    fontSize: AppleType.microUI.size,
    fontFamily: AppleFont.text,
    border: 'none',
    borderBottom: active ? `2px solid ${cv('blue')}` : '2px solid transparent',
    background: 'transparent',
    color: active ? cv('blue') : cv('text-secondary'),
    cursor: 'pointer' };
}

const diffStatsStyle = {
  marginLeft: 'auto',
  display: 'flex',
  gap: AppleSpace.md,
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold) };

const inlineDiffContainerStyle = {
  height: '500px',
  overflowY: 'auto' as const,
  border: `1px solid ${cv('border')}`,
  borderRadius: AppleRadius.small,
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUIBold.size,
  lineHeight: AppleLineHeight.spacious };

function inlineBlockStyle(type: string) {
  const bg = type === 'added' ? cv('green-light') : type === 'removed' ? cv('red-light') : 'transparent';
  return { background: bg };
}

function inlineLineStyle(type: string) {
  const color = type === 'added' ? cv('green') : type === 'removed' ? cv('red') : cv('text-primary');
  return {
    display: 'block',
    whiteSpace: 'pre-wrap' as const,
    wordBreak: 'break-word' as const,
    color,
    padding: '0 8px' };
}



const noDiffStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  height: '200px',
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.small,
  background: cv('bg-secondary') };

const noDiffTextStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary') };

const previewContentStyle = {
  maxHeight: '400px',
  overflowY: 'auto' as const,
  padding: AppleSpace.md };
</script>
