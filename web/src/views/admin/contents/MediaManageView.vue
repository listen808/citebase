<template>
  <ApPageContainer
    :title="$t('admin.k48')"
    :subtitle="$t('admin.k52')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <AdminCrudPage :stats="stats" :loading="loading" density="compact" :error-text="loadError" @retry="retryLoad">
      <template #filter-extra>
        <!-- 空间/文档这两级作用域以前在内容区另起一行（crudFilterBarStyle），
             和这一行的两个切换加起来共两行 chrome；并回来只剩一行 -->
        <div :style="filterSelectGroupStyle()">
          <label :style="labelStyle">{{ $t('admin.k41') }}</label>
          <ApSelect
            v-model="selectedSpaceId"
            :options="spaceOptions"
            size="small"
            :placeholder="$t('admin.k46')"
            @update:model-value="onSpaceChange"
          />
        </div>

        <div :style="filterSelectGroupStyle()">
          <label :style="labelStyle">{{ $t('admin.k42') }}</label>
          <!-- P2-11：未选空间时禁用态几乎不可读，用占位文案把原因说出来 -->
          <ApSelect
            v-model="selectedDocId"
            :options="documentOptions"
            size="small"
            :placeholder="selectedSpaceId ? $t('admin.k47') : $t('admin.k56')"
            :disabled="!selectedSpaceId || documentLoading"
            @update:model-value="onDocChange"
          />
        </div>
        <div :style="toolbarDividerStyle()" />
        <ApSegmentedControl
          :model-value="mediaFilter"
          :options="mediaFilterOptions"
          size="small"
          :aria-label="$t('admin.k53')"
          @update:model-value="(v: string | number) => mediaFilter = String(v) as 'all' | 'image' | 'attachment'"
        />
        <div :style="toolbarDividerStyle()" />
        <ApSegmentedControl v-model="viewMode" :options="viewModeOptions" size="small" :aria-label="$t('admin.k54')" />
        <template v-if="selectedItems.size > 0">
          <div :style="toolbarDividerStyle()" />
          <ApButton variant="danger" size="small" @click="openBatchDelete">
            <ApIcon name="Trash2" :size="16" />
            {{ $t('adminForm.k55', { p1: selectedItems.size }) }}
          </ApButton>
        </template>
      </template>
      <template #actions>
        <ApButton variant="primary" size="small" @click="triggerUpload">
          <ApIcon name="Upload" :size="16" />{{ $t('admin.upload') }}
        </ApButton>
        <input
          ref="fileInputRef"
          type="file"
          :aria-label="$t('admin.k55')"
            :accept="attachAccept"
          multiple
          :style="hiddenInputStyle"
          @change="handleFileSelect"
        />
      </template>
      <template #content>
        <div v-if="loading">
          <div v-if="viewMode === 'grid'" :style="responsiveMediaGridStyle()">
            <ApCard v-for="i in 4" :key="i" padding="0" accent-color="transparent">
              <ApSkeleton :style="{ height: `${120 + (i % 3) * 40}px`, borderRadius: AppleRadius.card }" />
            </ApCard>
          </div>
          <div v-else :style="tableContainerStyle">
            <ApSkeleton variant="list" :lines="4" />
          </div>
        </div>

        <!-- P1-59: 本页 #content 覆盖了 AdminCrudPage 内置 ApError 分支，需求失败时在内容区显式呈现错误+重试，
             否则失败会被误读为“该文档暂无资源” -->
        <ApError v-else-if="loadError" :message="loadError" show-retry @retry="retryLoad" />

        <ApEmpty v-else-if="!selectedDocId" icon="Folder" :title="$t('admin.k49')" />

        <ApEmpty v-else-if="attachments.length === 0" icon="FileText" :title="$t('adminForm.k56', { p1: selectedType === 'image' ? $t('adminForm.k57') : selectedType === 'attachment' ? $t('adminForm.k58') : $t('adminForm.k59') })" />

        <template v-else-if="viewMode === 'grid'">
          <div :style="responsiveMediaGridStyle()">
            <ApCard
              v-for="item in attachments"
              :key="item.id"
              padding="0"
              :style="selectedItems.has(item.id) ? { outline: `2px solid ${themeColor}`, outlineOffset: '-2px' } : {}"
              :accent-color="selectedItems.has(item.id) ? themeColor : 'transparent'"
              hover-lift
              @click="isImage(item) ? openPreview(item) : toggleSelection(item.id)"
            >
              <div :style="gridThumbStyle" @mouseenter="hoveredImg = item.id" @mouseleave="hoveredImg = null" @touchstart="hoveredImg = item.id">
                <img
                  v-if="isImage(item)"
                  :src="resolveFileUrl(item.url)"
                  :alt="item.name"
                  :style="gridImgStyle"
                  loading="lazy"
                  @error="handleImageError"
                />
                <div v-else :style="gridFileIconStyle">
                  <ApIcon name="FileIcon" :size="AppleIconSize.status" :color="cv('on-accent')" />
                </div>
                <div v-if="isImage(item) && hoveredImg === item.id" :style="gridOverlayStyle">
                  <ApIcon name="Eye" :size="20" :color="cv('on-accent')" />
                </div>
                <div v-if="selectedItems.has(item.id)" :style="gridCheckOverlayStyle">
                  <ApIcon name="Check" :size="20" :color="cv('on-accent')" />
                </div>
              </div>
              <div :style="gridMetaStyle">
                <span :style="gridNameStyle" :title="item.name">{{ item.name }}</span>
                <div :style="gridInfoRowStyle">
                  <span :style="gridSizeStyle">{{ formatSize(item.size) }}</span>
                  <span :style="gridSizeStyle">{{ formatTime(item.create_time) }}</span>
                </div>
              </div>
              <div :style="gridActionRowStyle">
                <a :href="getDownloadUrl(item.url)" target="_blank" :style="gridDownloadStyle" download>{{ $t('admin.download') }}</a>
                <ApButton variant="danger" size="mini" @click.stop="openDelete(item)"><ApIcon name="Trash2" :size="16" />{{ $t('admin.delete') }}</ApButton>
              </div>
            </ApCard>
          </div>
        </template>

        <template v-else>
          <div :style="tableContainerStyle">
            <ApTable :columns="tableColumns" :data="attachments" :loading="false" density="compact">
              <template #actions="{ row }">
                <div :style="actionGroupStyle">
                  <a v-if="row.url" :href="getDownloadUrl(row.url)" target="_blank" :style="gridDownloadStyle" download>{{ $t('admin.download') }}</a>
                  <ApButton variant="danger" size="mini" @click.stop="openDelete(row)"><ApIcon name="Trash2" :size="16" />{{ $t('admin.delete') }}</ApButton>
                </div>
              </template>
            </ApTable>
          </div>
        </template>
      </template>
      <template #modals>
        <ApImageLightbox v-model="showPreview" :items="previewItems" :label="$t('admin.k45')" />

        <ApConfirmModal
          v-model="showDeleteModal"
          :title="$t('admin.k50')"
          :message="$t('adminForm.k60', { p1: selectedType === 'image' ? $t('adminForm.k57') : $t('adminForm.k58') })"
          :confirm-text="$t('admin.delete')"
          :cancel-text="$t('admin.cancel')"
          type="danger"
          @confirm="confirmDelete"
        />

        <ApConfirmModal
          v-model="showBatchDeleteModal"
          :title="$t('admin.k51')"
          :message="$t('adminForm.k61', { p1: selectedItems.size })"
          :confirm-text="$t('admin.delete')"
          :cancel-text="$t('admin.cancel')"
          type="danger"
          :require-input-confirm="true"
          :confirm-placeholder="$t('lastAttrsUi.k1')"
          :confirm-match-text="$t('common.delete')"
          @confirm="confirmBatchDelete"
        />
      </template>
    </AdminCrudPage>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ATTACHMENT_ACCEPT, overSizeMessage } from '@/constants/upload-limits';
import { ref, shallowRef, computed, onMounted, watch } from 'vue';
import { icons } from '@/utils/icons';
import {AppleFontWeight, AppleGridFloor, AppleIconSize, AppleLayout, AppleRadius, AppleSpace, AppleType, AppleZIndex, appleFixedColumns, cv, subtleBg} from '@/utils/core/apple-tokens';
import { labelStyle, actionGroupStyle, filterSelectGroupStyle, toolbarDividerStyle } from '@/styles/layout/page-styles';
import { getDocumentImages, getDocumentAttachments, uploadAttachment, deleteAttachment, type Attachment } from '@/api/modules/document/attachment';
import { useSpaceStore } from '@/stores/workspace/space';
import { formatTime } from '@/utils/format/format-time';
import { formatSize, resolveFileUrl, getDownloadUrl } from '@/utils/domain/file';
import { useUIStore } from '@/stores/app/ui';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import { useBreakpoint } from '@/utils/responsive';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApError from '@/components/apple/ApError.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApTable from '@/components/apple/ApTable.vue'
import ApImageLightbox from '@/components/apple/ApImageLightbox.vue'
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue'
import ApIcon from '@/components/apple/ApIcon.vue';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import type { StatConfig } from '@/components/admin/AdminCrudPage.vue';
import type { TableColumn } from '@/components/apple/ApTable.vue';

const ui = useUIStore();
const spaceStore = useSpaceStore();
const themeColor = cv('blue');
const { isMobile } = useBreakpoint();

const viewMode = ref<'grid' | 'list'>('grid');
const viewModeOptions = [
  { value: 'grid' as const, label: t('admin.k56') },
  { value: 'list' as const, label: t('admin.k57') },
];
watch(viewMode, () => { selectedItems.value = new Set(); });

const mediaFilter = ref<'all' | 'image' | 'attachment'>('all');
const mediaFilterOptions = [
  { label: t('admin.all'), value: 'all' as const, icon: icons['Image'] },
  { label: t('admin.k59'), value: 'image' as const, icon: icons['Image'] },
  { label: t('admin.k60'), value: 'attachment' as const, icon: icons['Paperclip'] },
];

const selectedType = ref<'all' | 'image' | 'attachment'>('all');

watch(mediaFilter, (val) => {
  selectedType.value = val;
  if (selectedDocId.value) {
    fetchList();
  }
});
const attachAccept = ATTACHMENT_ACCEPT;
const selectedSpaceId = shallowRef<number | null>(null);
const selectedDocId = shallowRef<number | null>(null);

const {
  loading, loadError, retryLoad,
  items: attachments,
  showDeleteModal,
  openDelete,
  handleDelete: confirmDelete,
  fetchList,
} = useAdminCrud<Attachment>({
  fetchFn: async () => {
    if (!selectedDocId.value) return { data: [], total: 0, page: 1, page_size: 200 };
    if (selectedType.value === 'image') {
      const images = await getDocumentImages(selectedDocId.value);
      return { data: images, total: images.length, page: 1, page_size: 200 };
    }
    const source = selectedType.value === 'all' ? -1 : 0;
    const allAttachments = await getDocumentAttachments(selectedDocId.value, source);
    return { data: allAttachments, total: allAttachments.length, page: 1, page_size: 200 };
  },
  deleteFn: deleteAttachment,
  itemName: t('adminForm.k59'),
});

const spaces = ref<{ id: number; name: string; doc_count?: number }[]>([]);
const documents = ref<{ id: number; name: string }[]>([]);
const documentLoading = shallowRef(false);

interface DocTreeNode {
  id: number;
  name: string;
  children?: DocTreeNode[];
}

function flattenDocumentTree(tree: DocTreeNode[]): { id: number; name: string }[] {
  const result: { id: number; name: string }[] = [];
  for (const doc of tree) {
    result.push({ id: doc.id, name: doc.name });
    if (doc.children && Array.isArray(doc.children) && doc.children.length > 0) {
      result.push(...flattenDocumentTree(doc.children));
    }
  }
  return result;
}

const selectedItems = ref<Set<number>>(new Set());

const showBatchDeleteModal = shallowRef(false);

function openBatchDelete() {
  if (selectedItems.value.size === 0) return;
  showBatchDeleteModal.value = true;
}

async function confirmBatchDelete() {
  // P1-12: 删除前用当前列表的 id 集合过滤，杜绝删除界面已不可见的跨文档资源
  const visibleIds = new Set(attachments.value.map(a => a.id));
  const ids = Array.from(selectedItems.value).filter(id => visibleIds.has(id));
  let successCount = 0;
  const failedIds: number[] = [];
  for (const id of ids) {
    try {
      await deleteAttachment(id);
      successCount++;
    } catch {
      failedIds.push(id);
    }
  }
  if (failedIds.length === 0) {
    ui.showToast(t('sentUi.k4', { p1: successCount }), 'success');
  } else if (successCount > 0) {
    ui.showToast(t('sentUi.k5', { p1: successCount, p2: failedIds.length }), 'info');
  } else {
    ui.showToast(t('sentUi.k6', { p1: failedIds.length }), 'error');
  }
  selectedItems.value = new Set(failedIds);
  showBatchDeleteModal.value = false;
  fetchList();
}

const showPreview = shallowRef(false);
const previewImage = ref<Attachment | null>(null);

const spaceOptions = computed(() =>
  spaces.value.map(space => ({
    label: t('sentUi.k7', { p1: space.name, p2: space.doc_count ?? 0 }),
    value: space.id
  }))
);

const documentOptions = computed(() =>
  documents.value.map(doc => ({
    label: doc.name,
    value: doc.id
  }))
);

const totalFiles = computed(() => attachments.value.length);
const totalImages = computed(() => attachments.value.filter(a => a.source === 1).length);
const totalAttachments = computed(() => attachments.value.filter(a => a.source === 0).length);
const totalSize = computed(() => attachments.value.reduce((sum, a) => sum + (a.size || 0), 0));

const stats = computed<StatConfig[]>(() => [
  { label: t('admin.k61'), value: totalFiles.value, icon: 'Image', color: themeColor },
  { label: t('admin.k62'), value: totalImages.value, icon: 'Image', color: cv('blue') },
  { label: t('admin.k63'), value: totalAttachments.value, icon: 'Paperclip', color: cv('orange') },
  { label: t('admin.k64'), value: formatSize(totalSize.value), icon: 'HardDrive', color: cv('green') },
]);

function isImage(item: Attachment): boolean {
  return item.source === 1;
}

const tableColumns: TableColumn<Attachment>[] = [
  { key: 'name', title: t('admin.k65') },
  { key: 'size', title: t('admin.k66'), render: (row: Attachment) => formatSize(row.size) },
  { key: 'source', title: t('admin.type'), render: (row: Attachment) => (row.source === 1 ? t('adminForm.k57') : t('adminForm.k58')) },
  { key: 'create_time', title: t('admin.k68'), render: (row: Attachment) => row.create_time ? formatTime(row.create_time) : '-' },
];

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces();
    spaces.value = spaceStore.mySpaces;
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message: t('admin.k69');
    ui.showToast(t('admin.k71') + msg, 'error');
  }
}

async function onSpaceChange(value: string | number | null) {
  const spaceId = typeof value === 'number' ? value : null;
  selectedSpaceId.value = spaceId;
  selectedDocId.value = null;
  attachments.value = [];
  // P1-12: 切换空间后旧勾选集合指向另一个文档的资源，必须清空，否则会跨文档误删
  selectedItems.value = new Set();

  if (!spaceId) {
    documents.value = [];
    return;
  }

  documentLoading.value = true;
  try {
    const { getDocumentTree } = await import('@/api/modules/document/document');
    const docs = await getDocumentTree(spaceId);
    const treeData = Array.isArray(docs) ? docs : [];
    documents.value = flattenDocumentTree(treeData);
  } catch {
    ui.showToast(t('admin.k72'), 'error');
    documents.value = [];
  } finally {
    documentLoading.value = false;
  }
}

function onDocChange(value: string | number | null) {
  const docId = typeof value === 'number' ? value : null;
  selectedDocId.value = docId;
  // P1-12: 切换文档时清空勾选，避免对已不可见的旧文档资源执行批量删除
  selectedItems.value = new Set();
  if (!docId) {
    attachments.value = [];
    return;
  }
  fetchList();
}

// 说明行与遮罩都在 ApImageLightbox 里；以前这张图塞在 ApModal 里，
// 和另外三处"点开看大图"是同一件事的第四种写法
const previewItems = computed(() => {
  const img = previewImage.value;
  return img?.url ? [{ src: resolveFileUrl(img.url), name: img.name, meta: formatSize(img.size) }] : [];
});

function openPreview(img: Attachment) {
  previewImage.value = img;
  showPreview.value = true;
}

function handleImageError(e: Event) {
  const target = e.target as HTMLImageElement | null;
  if (target) target.style.opacity = '0.5';
}

function toggleSelection(id: number) {
  const newSet = new Set(selectedItems.value);
  if (newSet.has(id)) {
    newSet.delete(id);
  } else {
    newSet.add(id);
  }
  selectedItems.value = newSet;
}

const fileInputRef = ref<HTMLInputElement | null>(null);

function triggerUpload() {
  fileInputRef.value?.click();
}

async function handleFileSelect(e: Event) {
  const input = e.target as HTMLInputElement;
  const files = input.files;
  if (!files || files.length === 0) {
    input.value = '';
    return;
  }
  if (!selectedDocId.value) {
    ui.showToast(t('admin.k73'), 'info');
    input.value = '';
    return;
  }
  for (const file of Array.from(files)) {
    const oversize = overSizeMessage(file, 'attachment');
    if (oversize) {
      ui.showToast(oversize, 'error');
      continue;
    }
    try {
      await uploadAttachment(selectedDocId.value, file);
      ui.showToast(t('sentUi.k8', { p1: file.name }), 'success');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message: t('admin.k70');
      ui.showToast(t('sentUi.k9', { p1: file.name, p2: msg }), 'error');
    }
  }
  input.value = '';
  fetchList();
}

onMounted(() => {
  loadSpaces();
});

const hoveredImg = ref<number | null>(null);

const hiddenInputStyle = { display: 'none' };

const tableContainerStyle = {
  borderRadius: AppleRadius.card,
  overflow: 'hidden',
};

const gridThumbStyle = {
  position: 'relative' as const,
  height: '120px',
  overflow: 'hidden',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: cv('bg-secondary'),
};

// 缩略图不跟随 hover 缩放：悬停反馈由遮罩上的「查看」图标承担（同一张图既放大又出遮罩是两套反馈）。
const gridImgStyle = {
  width: '100%',
  height: '100%',
  objectFit: 'cover' as const,
};

const gridFileIconStyle = {
  width: AppleSpace['3xl'],
  height: AppleSpace['3xl'],
  borderRadius: AppleRadius.large,
  background: cv('blue'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const gridOverlayStyle = {
  position: 'absolute' as const,
  inset: '0',
  background: cv('overlay'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const gridCheckOverlayStyle = {
  position: 'absolute' as const,
  inset: '0',
  background: subtleBg(cv('red'), 30, 'red-bg-subtle'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  zIndex: String(AppleZIndex.raised),
};

const gridMetaStyle = {
  padding: AppleSpace.xs,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const gridNameStyle = {
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleType.bodyEmphasis.weight),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const gridInfoRowStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
};

const gridSizeStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};

const gridActionRowStyle = {
  display: 'flex',
  borderTop: `1px solid ${cv('border-light')}`,
};

const gridDownloadStyle = {
  flex: '1',
  padding: AppleSpace.xs,
  border: 'none',
  borderRight: `1px solid ${cv('border-light')}`,
  background: 'transparent',
  color: cv('blue'),
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  cursor: 'pointer',
  textDecoration: 'none',
  textAlign: 'center' as const,
  display: 'inline-block',
};

function responsiveMediaGridStyle() {
  return {
    display: 'grid',
    gridTemplateColumns: isMobile.value ? appleFixedColumns(2) : `repeat(auto-fill, ${AppleGridFloor.tile})`,
    gap: AppleSpace.sm,
  };
}





</script>
